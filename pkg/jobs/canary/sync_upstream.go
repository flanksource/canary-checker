package canary

import (
	goctx "context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/commons/logger"
	dutyContext "github.com/flanksource/duty/context"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/upstream"
	"gorm.io/gorm/clause"
)

var UpstreamConf upstream.UpstreamConfig

var tablesToReconcile = []string{
	"checks",
	"check_statuses",
}

// ReconcileCanaryResults coordinates with upstream and pushes any resource
// that are missing on the upstream.
func ReconcileCanaryResults() {
	ctx := dutyContext.NewContext(goctx.TODO()).WithDB(db.Gorm, db.Pool)

	jobHistory := models.NewJobHistory("PushCanaryResultsToUpstream", "Canary", "")
	_ = db.PersistJobHistory(jobHistory.Start())
	defer func() { _ = db.PersistJobHistory(jobHistory.End()) }()

	reconciler := upstream.NewUpstreamReconciler(UpstreamConf, 5)
	for _, table := range tablesToReconcile {
		if err := reconciler.Sync(ctx, table); err != nil {
			jobHistory.AddError(err.Error())
			logger.Errorf("failed to sync table %s: %v", table, err)
		} else {
			jobHistory.IncrSuccess()
		}
	}
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

type UpstreamPushJob struct {
	lastRuntime time.Time

	// MaxAge defines how far back we look into the past on startup whe
	// lastRuntime is zero.
	MaxAge time.Duration
}

func (t *UpstreamPushJob) Run() {
	jobHistory := models.NewJobHistory("UpstreamPushJob", "Canary", "")
	_ = db.PersistJobHistory(jobHistory.Start())
	defer func() { _ = db.PersistJobHistory(jobHistory.End()) }()

	if err := t.run(); err != nil {
		jobHistory.AddError(err.Error())
		logger.Errorf("error pushing to upstream: %v", err)
	} else {
		jobHistory.IncrSuccess()
	}
}

func (t *UpstreamPushJob) run() error {
	logger.Tracef("running upstream push job")

	var currentTime time.Time
	if err := db.Gorm.Raw("SELECT NOW()").Scan(&currentTime).Error; err != nil {
		return err
	}

	if t.lastRuntime.IsZero() {
		t.lastRuntime = currentTime.Add(-t.MaxAge)
	}

	pushData := &upstream.PushData{AgentName: UpstreamConf.AgentName}
	if err := db.Gorm.Where("created_at > ?", t.lastRuntime).Find(&pushData.CheckStatuses).Error; err != nil {
		return err
	}

	if err := db.Gorm.Where("updated_at > ?", t.lastRuntime).Find(&pushData.Checks).Error; err != nil {
		return err
	}

	t.lastRuntime = currentTime

	if pushData.Count() == 0 {
		return nil
	}
	logger.Tracef("pushing %d canary results to upstream", pushData.Count())

	// TODO: Fix this after https://github.com/flanksource/canary-checker/pull/1351 is merged
	return nil
}
