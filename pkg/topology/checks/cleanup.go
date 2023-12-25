package checks

import (
	"fmt"
	"time"

	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/job"
	"github.com/flanksource/duty/models"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	DefaultCheckRetentionDays  = 7
	DefaultCanaryRetentionDays = 7
)

var (
	CheckRetentionDays  int
	CanaryRetentionDays int
)

var CleanupChecks = &job.Job{
	Name:       "CleanupChecks",
	Schedule:   "@every 12h",
	Singleton:  true,
	JobHistory: true,
	Fn: func(ctx job.JobRuntime) error {
		if CheckRetentionDays <= 0 {
			CheckRetentionDays = DefaultCheckRetentionDays
		}
		tx := ctx.DB().Exec(`
					DELETE FROM checks
					WHERE
							id NOT IN (SELECT check_id FROM evidences WHERE check_id IS NOT NULL) AND
							(NOW() - deleted_at) > INTERVAL '1 day' * ?
					`, CheckRetentionDays)

		ctx.History.SuccessCount = int(tx.RowsAffected)
		return tx.Error
	},
}

var CleanupCanaries = &job.Job{
	Name:       "CleanupCanaries",
	Schedule:   "@every 12h",
	Singleton:  true,
	RunNow:     true,
	JobHistory: true,
	Fn: func(ctx job.JobRuntime) error {
		if CheckRetentionDays <= 0 {
			CheckRetentionDays = DefaultCheckRetentionDays
		}
		tx := ctx.DB().Exec(`
		DELETE FROM canaries
		WHERE
				id NOT IN (SELECT canary_id FROM checks) AND
				(NOW() - deleted_at) > INTERVAL '1 day' * ?
		`, CanaryRetentionDays)

		ctx.History.SuccessCount = int(tx.RowsAffected)
		return tx.Error
	},
}

// CleanupMetricsGauges removes gauges for checks that no longer exist.
var CleanupMetricsGauges = &job.Job{
	Name:       "CleanupMetricsGauges",
	Schedule:   "@every 1h",
	RunNow:     true,
	Singleton:  true,
	JobHistory: true,
	Fn: func(ctx job.JobRuntime) error {

		sevenDaysAgo := time.Now().Add(-time.Hour * 24 * 7)
		var deletedCheckIDs []string
		if err := ctx.DB().Model(&models.Check{}).Where("deleted_at > ?", sevenDaysAgo).Pluck("id", &deletedCheckIDs).Error; err != nil {
			return fmt.Errorf("Error finding deleted checks: %v", err)
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
