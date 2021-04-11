package metrics

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/asecurityteam/rolling"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	CounterType   pkg.MetricType = "counter"
	GaugeType     pkg.MetricType = "gauge"
	HistogramType pkg.MetricType = "histogram"

	OpsCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "canary_check_count",
			Help: "The total number of checks",
		},
		[]string{"type", "endpoint", "name", "namespace", "owner", "severity"},
	)

	OpsSuccessCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "canary_check_success_count",
			Help: "The total number of successful checks",
		},
		[]string{"type", "endpoint", "name", "namespace", "owner", "severity"},
	)

	OpsFailedCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "canary_check_failed_count",
			Help: "The total number of failed checks",
		},
		[]string{"type", "endpoint", "name", "namespace", "owner", "severity"},
	)

	RequestLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "canary_check_duration",
			Help:    "A histogram of the response latency in milliseconds.",
			Buckets: []float64{5, 10, 25, 50, 200, 500, 1000, 3000, 10000, 30000},
		},
		[]string{"type", "endpoint", "name", "namespace", "owner", "severity"},
	)

	Guage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "canary_check",
			Help: "A gauge representing the canaries success (0) or failure (1)",
		},
		[]string{"type", "endpoint", "name", "namespace", "owner", "severity"},
	)

	GenericGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "canary_check_gauge",
			Help: "A gauge representing duration",
		},
		[]string{"type", "name", "metric", "namespace", "owner", "severity"},
	)

	GenericCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "canary_check_counter",
			Help: "A gauge representing counters",
		},
		[]string{"type", "name", "metric", "value", "namespace", "owner", "severity"},
	)

	GenericHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "canary_check_histogram",
			Help:    "A histogram representing durations",
			Buckets: []float64{5, 10, 25, 50, 200, 500, 1000, 2500, 5000, 10000, 20000},
		},
		[]string{"type", "name", "metric", "namespace", "owner", "severity"},
	)
)

var failed = make(map[string]*rolling.TimePolicy)
var passed = make(map[string]*rolling.TimePolicy)
var latencies = make(map[string]*rolling.TimePolicy)

func init() {
	prometheus.MustRegister(Guage, OpsCount, OpsSuccessCount, OpsFailedCount, RequestLatency, GenericGauge, GenericCounter, GenericHistogram)
}

func RemoveCheck(checks v1.Canary) {
	for _, check := range checks.Spec.GetAllChecks() {
		key := checks.GetKey(check)
		delete(failed, key)
		delete(passed, key)
		delete(latencies, key)
	}
}

func GetMetrics(key string) (rollingUptime string, rollingLatency time.Duration) {
	fail := failed[key]
	pass := passed[key]
	latency := latencies[key]
	if fail == nil || pass == nil || latency == nil {
		return "", time.Millisecond * 0
	}
	failCount := fail.Reduce(rolling.Sum)
	passCount := pass.Reduce(rolling.Sum)
	percentage := 100.0 * (1 - (failCount / (passCount + failCount)))
	var uptime string
	if percentage == (math.Round(percentage)) {
		uptime = fmt.Sprintf("%.0f/%.0f (%0.f%%)", passCount, failCount+passCount, percentage)
	} else {
		uptime = fmt.Sprintf("%.0f/%.0f (%0.1f%%)", passCount, failCount+passCount, percentage)
	}
	return uptime, time.Duration(latency.Reduce(rolling.Percentile(95))) * time.Millisecond
}

func Record(check v1.Canary, result *pkg.CheckResult) (rollingUptime string, rollingLatency time.Duration) {
	if result == nil || result.Check == nil {
		logger.Warnf("%s/%s returned a nil result", check.Namespace, check.Name)
		return
	}
	namespace := check.Namespace
	name := check.Name
	checkType := result.Check.GetType()
	endpoint := check.GetDescription(result.Check)
	owner := check.Spec.Owner
	severity := check.Spec.Severity
	// We are recording aggreated metrics at the canary level, not the individual check level
	key := check.GetKey(result.Check)

	fail, ok := failed[key]
	if !ok {
		fail = rolling.NewTimePolicy(rolling.NewWindow(3600), time.Second)
		failed[key] = fail
	}

	pass, ok := passed[key]
	if !ok {
		pass = rolling.NewTimePolicy(rolling.NewWindow(3600), time.Second)
		passed[key] = pass
	}

	latency, ok := latencies[key]
	if !ok {
		latency = rolling.NewTimePolicy(rolling.NewWindow(3600), time.Second)
		latencies[key] = latency
	}

	if logger.IsTraceEnabled() {
		logger.Tracef(result.String())
	}
	OpsCount.WithLabelValues(checkType, endpoint, name, namespace, owner, severity).Inc()
	if result.Duration > 0 {
		RequestLatency.WithLabelValues(checkType, endpoint, name, namespace, owner, severity).Observe(float64(result.Duration))
		latency.Append(float64(result.Duration))
	}
	if result.Pass {
		pass.Append(1)
		Guage.WithLabelValues(checkType, endpoint, name, namespace, owner, severity).Set(0)
		OpsSuccessCount.WithLabelValues(checkType, endpoint, name, namespace, owner, severity).Inc()
		// always add a failed count to ensure the metric is present in prometheus
		// for an uptime calculation
		OpsFailedCount.WithLabelValues(checkType, endpoint, name, namespace, owner, severity).Add(0)
		for _, m := range result.Metrics {
			switch m.Type {
			case CounterType:
				GenericCounter.WithLabelValues(checkType, endpoint, m.Name, strconv.Itoa(int(m.Value)), namespace, owner, severity).Inc()
			case GaugeType:
				GenericGauge.WithLabelValues(checkType, endpoint, m.Name, namespace, owner, severity).Set(m.Value)
			case HistogramType:
				GenericHistogram.WithLabelValues(checkType, endpoint, m.Name, namespace, owner, severity).Observe(m.Value)
			}
		}
	} else {
		fail.Append(1)
		Guage.WithLabelValues(checkType, endpoint, name, namespace, owner, severity).Set(1)
		OpsFailedCount.WithLabelValues(checkType, endpoint, name, namespace, owner, severity).Inc()
	}
	failCount := fail.Reduce(rolling.Sum)
	passCount := pass.Reduce(rolling.Sum)
	percentage := 100.0 * (1 - (failCount / (passCount + failCount)))
	var uptime string
	if percentage == (math.Round(percentage)) {
		uptime = fmt.Sprintf("%.0f/%.0f (%0.f%%)", passCount, failCount+passCount, percentage)
	} else {
		uptime = fmt.Sprintf("%.0f/%.0f (%0.1f%%)", passCount, failCount+passCount, percentage)
	}
	return uptime, time.Duration(latency.Reduce(rolling.Percentile(95))) * time.Millisecond
}
