package canary

import (
	"context"
	"time"

	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/canary-checker/pkg/runner"
	"github.com/flanksource/commons/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
)

// CleanUpPrometheusGauges removes Prometheus gauges for checks that no longer exist.
func CleanUpPrometheusGauges() {
	if runner.Prometheus == nil {
		logger.Infof("Prometheus is not running")
		return
	}

	ctx := context.Background()

	result, _, err := runner.Prometheus.Query(ctx, metrics.GaugeOpt.Name, time.Now())
	if err != nil {
		logger.Errorf("Error querying prometheus: %v", err)
		return
	}

	resultVector, ok := result.(model.Vector)
	if !ok {
		logger.Errorf("Unexpected result type: %T", result)
		return
	}

	if len(resultVector) == 0 {
		return
	}

	var checkIDs []string
	for _, r := range resultVector {
		if canaryName, ok := r.Metric["key"]; ok {
			checkIDs = append(checkIDs, string(canaryName))
		}
	}

	deletedChecks, err := db.FindDeletedChecksByIDs(ctx, checkIDs)
	if err != nil {
		logger.Errorf("Error finding deleted checks: %v", err)
		return
	}
	logger.Debugf("Found %d/%d deleted checks", len(deletedChecks), len(checkIDs))

	for _, check := range deletedChecks {
		if metrics.Gauge.DeletePartialMatch(prometheus.Labels{"key": check.ID.String()}) > 0 {
			logger.Debugf("Deleted gauge for check: %s", check.Name)
		}
	}
}
