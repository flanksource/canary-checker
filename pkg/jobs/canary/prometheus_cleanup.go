package canary

import (
	"context"
	"time"

	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/commons/logger"
	"github.com/prometheus/client_golang/prometheus"
)

// CleanupMetricsGauges removes gauges for checks that no longer exist.
func CleanupMetricsGauges() {
	ctx := context.Background()

	sevenDaysAgo := time.Now().Add(-time.Hour * 24 * 7)
	deletedCheckIDs, err := db.FindDeletedChecksSince(ctx, sevenDaysAgo)
	if err != nil {
		logger.Errorf("Error finding deleted checks: %v", err)
		return
	}
	logger.Debugf("Found %d deleted checks since %s", len(deletedCheckIDs), sevenDaysAgo.Format("2006-01-02 15:04:05"))

	for _, id := range deletedCheckIDs {
		if metrics.Gauge.DeletePartialMatch(prometheus.Labels{"key": id}) > 0 {
			logger.Debugf("Deleted gauge for check: %s", id)
		}
	}
}
