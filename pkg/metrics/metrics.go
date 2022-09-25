package metrics

import (
	"strconv"
	"time"

	"github.com/asecurityteam/rolling"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/runner"
	"github.com/flanksource/commons/logger"
	cmap "github.com/orcaman/concurrent-map"
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
		[]string{"type", "endpoint", "canary_name", "canary_namespace", "owner", "severity", "key", "name"},
	)

	CanaryCheckInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "canary_check_info",
			Help: "Information about the canary check",
		},
		[]string{"type", "endpoint", "canary_name", "canary_namespace", "owner", "severity", "key", "name"},
	)

	OpsSuccessCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "canary_check_success_count",
			Help: "The total number of successful checks",
		},
		[]string{"type", "endpoint", "canary_name", "canary_namespace", "owner", "severity", "key", "name"},
	)

	OpsFailedCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "canary_check_failed_count",
			Help: "The total number of failed checks",
		},
		[]string{"type", "endpoint", "canary_name", "canary_namespace", "owner", "severity", "key", "name"},
	)

	RequestLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "canary_check_duration",
			Help:    "A histogram of the response latency in milliseconds.",
			Buckets: []float64{5, 10, 25, 50, 200, 500, 1000, 3000, 10000, 30000},
		},
		[]string{"type", "endpoint", "canary_name", "canary_namespace", "owner", "severity", "key", "name"},
	)

	Gauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "canary_check",
			Help: "A gauge representing the canaries success (0) or failure (1)",
		},
		[]string{"key", "type", "canary_name", "canary_namespace", "name"},
	)

	GenericGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "canary_check_gauge",
			Help: "A gauge representing duration",
		},
		[]string{"type", "canary_name", "metric", "canary_namespace", "owner", "severity", "key", "name"},
	)

	GenericCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "canary_check_counter",
			Help: "A gauge representing counters",
		},
		[]string{"type", "canary_name", "metric", "value", "canary_namespace", "owner", "severity", "key", "name"},
	)

	GenericHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "canary_check_histogram",
			Help:    "A histogram representing durations",
			Buckets: []float64{5, 10, 25, 50, 200, 500, 1000, 2500, 5000, 10000, 20000},
		},
		[]string{"type", "canary_name", "metric", "canary_namespace", "owner", "severity", "key", "name"},
	)
)

var failed = cmap.New()
var passed = cmap.New()
var latencies = cmap.New()

func init() {
	prometheus.MustRegister(Gauge, CanaryCheckInfo, OpsCount, OpsSuccessCount, OpsFailedCount, RequestLatency, GenericGauge, GenericCounter, GenericHistogram)
}

func RemoveCheck(checks v1.Canary) {
	for _, check := range checks.Spec.GetAllChecks() {
		key := checks.GetKey(check)
		RemoveCheckByKey(key)
	}
}

func RemoveCheckByKey(key string) {
	failed.Remove(key)
	passed.Remove(key)
	latencies.Remove(key)
}

func GetMetrics(key string) (uptime pkg.Uptime, latency pkg.Latency) {
	uptime = pkg.Uptime{}

	fail, ok := failed.Get(key)
	if ok {
		uptime.Failed = int(fail.(*rolling.TimePolicy).Reduce(rolling.Sum))
	}

	pass, ok := passed.Get(key)
	if ok {
		uptime.Passed = int(pass.(*rolling.TimePolicy).Reduce(rolling.Sum))
	}

	lat, ok := latencies.Get(key)
	if ok {
		latency = pkg.Latency{Rolling1H: lat.(*rolling.TimePolicy).Reduce(rolling.Percentile(95))}
	}
	return
}

func Record(canary v1.Canary, result *pkg.CheckResult) (_uptime pkg.Uptime, _latency pkg.Latency) {
	if result == nil || result.Check == nil {
		logger.Warnf("%s/%s returned a nil result", canary.Namespace, canary.Name)
		return
	}
	if canary.GetCheckID(result.Check.GetName()) == "" {
		logger.Warnf("%s/%s/%s returned a result for a check that does not exist", canary.Namespace, canary.Name, result.Check.GetName())
		return
	}
	canaryNamespace := canary.Namespace
	canaryName := canary.Name
	name := result.Check.GetName()
	checkType := result.Check.GetType()
	endpoint := canary.GetDescription(result.Check)
	owner := canary.Spec.Owner
	severity := canary.Spec.Severity
	// We are recording aggreated metrics at the canary level, not the individual check level
	key := canary.GetCheckID(result.Check.GetName())
	var fail, pass, latency *rolling.TimePolicy

	_fail, ok := failed.Get(key)
	if !ok {
		fail = rolling.NewTimePolicy(rolling.NewWindow(3600), time.Second)
		failed.Set(key, fail)
	} else {
		fail = _fail.(*rolling.TimePolicy)
	}

	_pass, ok := passed.Get(key)
	if !ok {
		pass = rolling.NewTimePolicy(rolling.NewWindow(3600), time.Second)
		passed.Set(key, pass)
	} else {
		pass = _pass.(*rolling.TimePolicy)
	}

	_latencyV, ok := latencies.Get(key)
	if !ok {
		latency = rolling.NewTimePolicy(rolling.NewWindow(3600), time.Second)
		latencies.Set(key, latency)
	} else {
		latency = _latencyV.(*rolling.TimePolicy)
	}

	if logger.IsTraceEnabled() {
		logger.Tracef(result.String())
	}
	OpsCount.WithLabelValues(checkType, endpoint, canaryName, canaryNamespace, owner, severity, key, name).Inc()
	if result.Duration > 0 {
		RequestLatency.WithLabelValues(checkType, endpoint, canaryName, canaryNamespace, owner, severity, key, name).Observe(float64(result.Duration))
		latency.Append(float64(result.Duration))
	}
	if result.Pass {
		pass.Append(1)
		Gauge.WithLabelValues(key, checkType, canaryName, canaryNamespace, name).Set(0)
		CanaryCheckInfo.WithLabelValues(checkType, endpoint, canaryName, canaryNamespace, owner, severity, key, name).Set(0)
		// TODO: remove the metric in case the check is removed
		// use uuid as key
		// store the check ids in a map[checkName]uuid in the v1.Canary struct``

		// prometheus.Unregister(Gauge.WithLabelValues(checkType, endpoint, canaryName, canaryNamespace, owner, severity, key, name))
		OpsSuccessCount.WithLabelValues(checkType, endpoint, canaryName, canaryNamespace, owner, severity, key, name).Inc()
		// always add a failed count to ensure the metric is present in prometheus
		// for an uptime calculation
		OpsFailedCount.WithLabelValues(checkType, endpoint, canaryName, canaryNamespace, owner, severity, key, name).Add(0)
		for _, m := range result.Metrics {
			switch m.Type {
			case CounterType:
				GenericCounter.WithLabelValues(checkType, endpoint, m.Name, strconv.Itoa(int(m.Value)), canaryNamespace, owner, severity, key, name).Inc()
			case GaugeType:
				GenericGauge.WithLabelValues(checkType, endpoint, m.Name, canaryNamespace, owner, severity, key, name).Set(m.Value)
			case HistogramType:
				GenericHistogram.WithLabelValues(checkType, endpoint, m.Name, canaryNamespace, owner, severity, key, name).Observe(m.Value)
			}
		}
	} else {
		fail.Append(1)
		Gauge.WithLabelValues(key, checkType, canaryName, canaryNamespace, name).Set(1)
		CanaryCheckInfo.WithLabelValues(checkType, endpoint, canaryName, canaryNamespace, owner, severity, key, name).Set(1)
		OpsFailedCount.WithLabelValues(checkType, endpoint, canaryName, canaryNamespace, owner, severity, key, name).Inc()
	}

	_uptime = pkg.Uptime{Passed: int(pass.Reduce(rolling.Sum)), Failed: int(fail.Reduce(rolling.Sum))}
	if latency != nil {
		_latency = pkg.Latency{Rolling1H: latency.Reduce(rolling.Percentile(95))}
	} else {
		_latency = pkg.Latency{}
	}
	return _uptime, _latency
}

func FillLatencies(checkKey string, duration string, latency *pkg.Latency) error {
	if runner.Prometheus == nil || duration == "" {
		return nil
	}
	p95, err := runner.Prometheus.GetHistogramQuantileLatency("0.95", checkKey, duration)
	if err != nil {
		return err
	}
	latency.Percentile95 = p95

	p97, err := runner.Prometheus.GetHistogramQuantileLatency("0.97", checkKey, duration)
	if err != nil {
		return err
	}
	latency.Percentile97 = p97
	p99, err := runner.Prometheus.GetHistogramQuantileLatency("0.99", checkKey, duration)
	if err != nil {
		return err
	}
	latency.Percentile99 = p99
	return nil
}

func FillUptime(checkKey, duration string, uptime *pkg.Uptime) error {
	if runner.Prometheus == nil || duration == "" {
		return nil
	}
	value, err := runner.Prometheus.GetUptime(checkKey, duration)
	if err != nil {
		return err
	}
	uptime.P100 = value
	return nil
}

func UnregisterGauge(checkIds []string) {
	for _, checkID := range checkIds {
		logger.Debugf("Unregistering gauge for checkID %s", checkID)
		Gauge.DeletePartialMatch(prometheus.Labels{"key": checkID})
	}
}
