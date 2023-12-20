package canary

import (
	gocontext "context"
	"fmt"
	"time"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/job"
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

var ReconcileChecks = job.Job{
	Name:       "PushChecksToUpstream",
	JobHistory: true,
	Singleton:  true,
	Schedule:   "@every 30m",
	Fn: func(ctx job.JobRuntime) error {
		reconciler := upstream.NewUpstreamReconciler(UpstreamConf, 5)
		return reconciler.SyncAfter(ctx.Context, "checks", ReconcileMaxAge)
	},
}

var SyncCheckStatuses = job.Job{
	Name:       "SyncCheckStatusesWithUpstream",
	JobHistory: true,
	Singleton:  true,
	Schedule:   "@every 1m",
	Fn: func(ctx job.JobRuntime) error {
		err, count := upstream.SyncCheckStatuses(ctx.Context, UpstreamConf, ReconcilePageSize)
		ctx.History.SuccessCount = count
		return err
	},
}

var lastRuntime time.Time
var PullUpstreamCanaries = job.Job{
	Name:       "PullUpstreamCanaries",
	JobHistory: true,
	Singleton:  true,
	Schedule:   "@every 10m",
	Fn: func(ctx job.JobRuntime) error {
		count, err := pull(ctx, UpstreamConf)
		ctx.History.SuccessCount = count
		return err
	},
}

type CanaryPullResponse struct {
	Before   time.Time       `json:"before"`
	Canaries []models.Canary `json:"canaries,omitempty"`
}

func pull(ctx gocontext.Context, config upstream.UpstreamConfig) (int, error) {
	logger.Tracef("pulling canaries from upstream since: %v", lastRuntime)

	client := upstream.NewUpstreamClient(config)
	req := client.Client.R(ctx).QueryParam("since", lastRuntime.Format(time.RFC3339))
	resp, err := req.Get(fmt.Sprintf("canary/pull/%s", config.AgentName))
	if err != nil {
		return 0, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if !resp.IsOK() {
		return 0, fmt.Errorf("upstream responded with status: %s", resp.Status)
	}

	var response CanaryPullResponse
	if err := resp.Into(&response); err != nil {
		return 0, fmt.Errorf("error decoding response: %w", err)
	}

	lastRuntime = response.Before

	if len(response.Canaries) == 0 {
		return 0, nil
	}

	return len(response.Canaries), db.Gorm.Omit("agent_id").Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(&response.Canaries).Error
}

var UpstreamJobs = []job.Job{
	SyncCheckStatuses,
	PullUpstreamCanaries,
	ReconcileChecks,
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
