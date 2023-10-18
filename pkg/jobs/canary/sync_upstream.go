package canary

import (
	gocontext "context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/commons/logger"
	dutyContext "github.com/flanksource/duty/context"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/upstream"
	"github.com/flanksource/postq/pg"
	"gorm.io/gorm/clause"
)

var (
	ReconcilePageSize int

	// Only sync data created/updated in the last ReconcileMaxAge duration
	ReconcileMaxAge time.Duration

	// UpstreamConf is the global configuration for upstream
	UpstreamConf upstream.UpstreamConfig
)

const (
	EventPushQueueCreate    = "push_queue.create"
	eventQueueUpdateChannel = "event_queue_updates"
)

// ReconcileChecks coordinates with upstream and pushes any resource
// that are missing on the upstream.
func ReconcileChecks() {
	jobHistory := models.NewJobHistory("PushChecksToUpstream", "Canary", "")
	_ = db.PersistJobHistory(jobHistory.Start())
	defer func() { _ = db.PersistJobHistory(jobHistory.End()) }()

	ctx := dutyContext.NewContext(gocontext.TODO()).WithDB(db.Gorm, db.Pool)
	reconciler := upstream.NewUpstreamReconciler(UpstreamConf, 5)
	if err := reconciler.SyncAfter(ctx, "checks", ReconcileMaxAge); err != nil {
		jobHistory.AddError(err.Error())
		logger.Errorf("failed to sync table 'checks': %v", err)
	} else {
		jobHistory.IncrSuccess()
	}
}

func SyncCheckStatuses() {
	logger.Debugf("running check statuses sync job")

	jobHistory := models.NewJobHistory("SyncCheckStatusesWithUpstream", UpstreamConf.Host, "")
	_ = db.PersistJobHistory(jobHistory.Start())
	defer func() { _ = db.PersistJobHistory(jobHistory.End()) }()

	ctx := dutyContext.NewContext(gocontext.TODO()).WithDB(db.Gorm, db.Pool)
	if err := upstream.SyncCheckStatuses(ctx, UpstreamConf, ReconcilePageSize); err != nil {
		logger.Errorf("failed to run checkstatus sync job: %v", err)
		jobHistory.AddError(err.Error())
		return
	}

	jobHistory.IncrSuccess()
}

type CanaryPullResponse struct {
	Before   time.Time       `json:"before"`
	Canaries []models.Canary `json:"canaries,omitempty"`
}

// UpstreamPullJob pulls canaries from the upstream
type UpstreamPullJob struct {
	lastRuntime time.Time
}

func (t *UpstreamPullJob) Run() {
	jobHistory := models.NewJobHistory("PullUpstreamCanaries", "Canary", "")
	_ = db.PersistJobHistory(jobHistory.Start())
	defer func() { _ = db.PersistJobHistory(jobHistory.End()) }()

	if err := t.pull(UpstreamConf); err != nil {
		jobHistory.AddError(err.Error())
		logger.Errorf("error pulling from upstream: %v", err)
	} else {
		jobHistory.IncrSuccess()
	}
}

func (t *UpstreamPullJob) pull(config upstream.UpstreamConfig) error {
	logger.Tracef("pulling canaries from upstream since: %v", t.lastRuntime)

	endpoint, err := url.JoinPath(config.Host, "upstream", "canary", "pull", config.AgentName)
	if err != nil {
		return fmt.Errorf("error creating url endpoint for host %s: %w", config.Host, err)
	}

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("error creating new http request: %w", err)
	}

	req.SetBasicAuth(config.Username, config.Password)

	params := url.Values{}
	params.Add("since", t.lastRuntime.Format(time.RFC3339))
	req.URL.RawQuery = params.Encode()

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	var response CanaryPullResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("error decoding response: %w", err)
	}

	t.lastRuntime = response.Before

	if len(response.Canaries) == 0 {
		return nil
	}

	logger.Tracef("fetched %d canaries from upstream", len(response.Canaries))

	return db.Gorm.Omit("agent_id").Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(&response.Canaries).Error
}

func StartUpstreamEventQueueConsumer(ctx *context.Context) error {
	consumer, err := upstream.NewPushQueueConsumer(UpstreamConf).EventConsumer()
	if err != nil {
		return err
	}

	pgNotifyChannel := make(chan string)
	go pg.Listen(ctx, eventQueueUpdateChannel, pgNotifyChannel)

	go consumer.Listen(ctx, pgNotifyChannel)
	return nil
}
