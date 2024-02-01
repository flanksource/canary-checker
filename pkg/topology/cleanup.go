package topology

import (
	"fmt"
	"time"

	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/job"
	"github.com/flanksource/duty/models"
	"github.com/prometheus/client_golang/prometheus"
)

var DefaultRetention = time.Hour * 24 * 7

var CleanupSoftDeletedComponents = &job.Job{
	Name:       "CleanupSoftDeletedComponents",
	Schedule:   "@every 24h",
	Singleton:  true,
	JobHistory: true,
	Retention:  job.Retention3Day,
	Fn: func(ctx job.JobRuntime) error {
		ctx.History.ResourceType = job.ResourceTypeComponent
		retention := ctx.Properties().Duration("component.retention.period", DefaultRetention)

		tx := ctx.Context.DB().Exec("DELETE FROM component_relationships WHERE deleted_at < NOW() - interval '1 SECONDS' * ?", int64(retention.Seconds()))
		if tx.Error != nil {
			return tx.Error
		}

		tx = ctx.Context.DB().Exec("DELETE FROM components WHERE deleted_at < NOW() - interval '1 SECONDS' * ?", int64(retention.Seconds()))
		if tx.Error != nil {
			return tx.Error
		}
		ctx.History.SuccessCount = int(tx.RowsAffected)
		return nil
	},
}

var CleanupChecks = &job.Job{
	Name:       "CleanupChecks",
	Schedule:   "@every 12h",
	Singleton:  true,
	JobHistory: true,
	Retention:  job.Retention3Day,
	Fn: func(ctx job.JobRuntime) error {
		retention := ctx.Properties().Duration("check.retention.age", DefaultRetention)
		tx := ctx.DB().Exec(`
					DELETE FROM checks
					WHERE
							id NOT IN (SELECT check_id FROM evidences WHERE check_id IS NOT NULL) AND
							(NOW() - deleted_at) > INTERVAL '1 second' * ?
					`, int64(retention.Seconds()),
		)

		ctx.History.SuccessCount = int(tx.RowsAffected)
		return tx.Error
	},
}

var CleanupCanaries = &job.Job{
	Name:       "CleanupCanaries",
	Schedule:   "@every 12h",
	Singleton:  true,
	JobHistory: true,
	Retention:  job.Retention3Day,
	RunNow:     true,
	Fn: func(ctx job.JobRuntime) error {
		tx := ctx.DB().Exec(`
		DELETE FROM canaries
		WHERE
				id NOT IN (SELECT canary_id FROM checks) AND
				(NOW() - deleted_at) > INTERVAL '1 day' * ?
		`, ctx.Properties().Duration("canary.retention.age", DefaultRetention))

		ctx.History.SuccessCount = int(tx.RowsAffected)
		return tx.Error
	},
}

// CleanupMetricsGauges removes gauges for checks that no longer exist.
var CleanupMetricsGauges = &job.Job{
	Name:       "CleanupMetricsGauges",
	Schedule:   "@every 1h",
	Singleton:  true,
	JobHistory: true,
	Retention:  job.RetentionDay,
	RunNow:     true,
	Fn: func(ctx job.JobRuntime) error {

		sevenDaysAgo := time.Now().Add(-time.Hour * 24 * 7)
		var deletedCheckIDs []string
		if err := ctx.DB().Model(&models.Check{}).Where("deleted_at > ?", sevenDaysAgo).Pluck("id", &deletedCheckIDs).Error; err != nil {
			return fmt.Errorf("error finding deleted checks: %v", err)
		}

		if ctx.IsDebug() {
			ctx.Debugf("Found %d deleted checks since %s", len(deletedCheckIDs), sevenDaysAgo.Format("2006-01-02 15:04:05"))
		}
		for _, id := range deletedCheckIDs {
			if metrics.Gauge.DeletePartialMatch(prometheus.Labels{"key": id}) > 0 {
				logger.Debugf("Deleted gauge for check: %s", id)
				ctx.History.IncrSuccess()
			}
		}
		return nil
	},
}
