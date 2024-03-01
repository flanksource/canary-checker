package canary

import (
	"fmt"
	"time"

	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/job"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/upstream"
	"github.com/flanksource/postq"
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
	ResourceTypeUpstream    = "upstream"
)

var UpstreamJobs = []*job.Job{
	SyncCheckStatuses,
	PullUpstreamCanaries,
	ReconcileChecks,
}

var ReconcileChecks = &job.Job{
	Name:       "ReconcileCanaries",
	JobHistory: true,
	Singleton:  true,
	Retention:  job.RetentionDay,
	RunNow:     true,
	Schedule:   "@every 30m",
	Fn: func(ctx job.JobRuntime) error {
		ctx.History.ResourceType = ResourceTypeUpstream
		ctx.History.ResourceID = UpstreamConf.Host

		if count, err := upstream.ReconcileTable[models.Canary](ctx.Context, UpstreamConf, ReconcilePageSize); err != nil {
			ctx.History.AddError(err.Error())
		} else {
			ctx.History.SuccessCount += count
		}

		if count, err := upstream.ReconcileTable[models.Check](ctx.Context, UpstreamConf, ReconcilePageSize); err != nil {
			ctx.History.AddError(err.Error())
		} else {
			ctx.History.SuccessCount += count
		}

		return nil
	},
}

var SyncCheckStatuses = &job.Job{
	Name:       "SyncCheckStatusesWithUpstream",
	JobHistory: true,
	Singleton:  true,
	Retention:  job.RetentionHour,
	RunNow:     true,
	Schedule:   "@every 30s",
	Fn: func(ctx job.JobRuntime) error {
		ctx.History.ResourceType = ResourceTypeUpstream
		ctx.History.ResourceID = UpstreamConf.Host
		count, err := upstream.SyncCheckStatuses(ctx.Context, UpstreamConf, ReconcilePageSize)
		ctx.History.SuccessCount = count
		return err
	},
}

var lastRuntime time.Time
var PullUpstreamCanaries = &job.Job{
	Name:       "PullUpstreamCanaries",
	JobHistory: true,
	Singleton:  true,
	RunNow:     true,
	Schedule:   "@every 10m",
	Retention:  job.RetentionHour,
	Fn: func(ctx job.JobRuntime) error {
		ctx.History.ResourceType = ResourceTypeUpstream
		ctx.History.ResourceID = UpstreamConf.Host
		count, err := pull(ctx.Context, UpstreamConf)
		ctx.History.SuccessCount = count
		return err
	},
}

type CanaryPullResponse struct {
	Before   time.Time       `json:"before"`
	Canaries []models.Canary `json:"canaries,omitempty"`
}

func pull(ctx context.Context, config upstream.UpstreamConfig) (int, error) {
	logger.Tracef("pulling canaries from upstream since: %v", lastRuntime)

	client := upstream.NewUpstreamClient(config)
	req := client.Client.R(ctx).QueryParam("since", lastRuntime.Format(time.RFC3339)).QueryParam(upstream.AgentNameQueryParam, config.AgentName)
	resp, err := req.Get("canary/pull")
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

	return len(response.Canaries), ctx.DB().Omit("agent_id").Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(&response.Canaries).Error
}

func StartUpstreamEventQueueConsumer(ctx context.Context) error {
	asyncConsumer := postq.AsyncEventConsumer{
		WatchEvents: []string{EventPushQueueCreate},
		Consumer: func(_ctx postq.Context, e postq.Events) postq.Events {
			return upstream.NewPushUpstreamConsumer(UpstreamConf)(ctx, e)
		},
		BatchSize: ctx.Properties().Int("push_queue.batch.size", 50),
		ConsumerOption: &postq.ConsumerOption{
			NumConsumers: ctx.Properties().Int("push_queue.consumers", 5),
			ErrorHandler: func(err error) bool {
				logger.Errorf("error consuming upstream push_queue.create events: %v", err)
				time.Sleep(time.Second)
				return true
			},
		},
	}

	eventConsumer, err := asyncConsumer.EventConsumer()
	if err != nil {
		return err
	}

	pgNotifyChannel := make(chan string)
	go pg.Listen(ctx, eventQueueUpdateChannel, pgNotifyChannel)

	go eventConsumer.Listen(ctx, pgNotifyChannel)
	return nil
}
