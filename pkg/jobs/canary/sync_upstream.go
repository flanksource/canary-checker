package canary

import (
	"fmt"
	"time"

	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/job"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/upstream"
	"gorm.io/gorm/clause"
)

var (
	ReconcilePageSize int

	// Only sync data created/updated in the last ReconcileMaxAge duration
	ReconcileMaxAge time.Duration

	// UpstreamConf is the global configuration for upstream
	UpstreamConf upstream.UpstreamConfig
)

const ResourceTypeUpstream = "upstream"

var UpstreamJobs = []*job.Job{
	ReconcileCanaries,
	PullUpstreamCanaries,
}

var ReconcileCanaries = &job.Job{
	Name:       "ReconcileCanaries",
	Schedule:   "@every 1m",
	Retention:  job.RetentionBalanced,
	Singleton:  true,
	JobHistory: true,
	RunNow:     true,
	Fn: func(ctx job.JobRuntime) error {
		ctx.History.ResourceType = job.ResourceTypeUpstream
		ctx.History.ResourceID = UpstreamConf.Host
		tablesToReconcile := []string{"canaries", "checks", "check_statuses", "check_config_relationships"}
		if count, err := upstream.ReconcileSome(ctx.Context, UpstreamConf, ReconcilePageSize, tablesToReconcile...); err != nil {
			ctx.History.AddError(err.Error())
		} else {
			ctx.History.SuccessCount += count
		}

		return nil
	},
}

var lastRuntime time.Time
var PullUpstreamCanaries = &job.Job{
	Name:       "PullUpstreamCanaries",
	JobHistory: true,
	Singleton:  true,
	RunNow:     true,
	Schedule:   "@every 10m",
	Retention:  job.RetentionFew,
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
