package metrics

import (
	"slices"
	"time"

	"github.com/asecurityteam/rolling"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/runner"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/types"
	cmap "github.com/orcaman/concurrent-map"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/samber/lo"
)

func init() {
	CustomCounters = make(map[string]*prometheus.CounterVec)
	CustomGauges = make(map[string]*prometheus.GaugeVec)
	CustomHistograms = make(map[string]*prometheus.HistogramVec)

	// Register the metrics with a delay because
	// v1.AdditionalCheckMetricLabels is nil during init.
	go func() {
		time.Sleep(time.Second)

		slices.Sort(v1.AdditionalCheckMetricLabels)
		setupMetrics()
	}()
}

func setupMetrics() {
	RequestLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "canary_check_duration",
			Help:    "A histogram of the response latency in milliseconds.",
			Buckets: []float64{5, 10, 25, 50, 200, 500, 1000, 3000, 10000, 30000},
		},
		append(
			[]string{"type", "endpoint", "canary_name", "canary_namespace", "owner", "severity", "key", "name"},
			v1.AdditionalCheckMetricLabels...,
		),
	)

	Gauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "canary_check",
			Help: "A gauge representing the canaries success (0) or failure (1)",
		},
		append([]string{"key", "type", "canary_name", "canary_namespace", "name"}, v1.AdditionalCheckMetricLabels...),
	)

	checkLabels := []string{"type", "endpoint", "canary_name", "canary_namespace", "owner", "severity", "key", "name"}
	checkLabels = append(checkLabels, v1.AdditionalCheckMetricLabels...)

	OpsCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "canary_check_count",
			Help: "The total number of checks",
		},
		checkLabels,
	)

	CanaryCheckInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "canary_check_info",
			Help: "Information about the canary check",
		},
		checkLabels,
	)

	OpsSuccessCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "canary_check_success_count",
			Help: "The total number of successful checks",
		},
		checkLabels,
	)

	OpsFailedCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "canary_check_failed_count",
			Help: "The total number of failed checks",
		},
		checkLabels,
	)

	prometheus.MustRegister(Gauge, CanaryCheckInfo, OpsCount, OpsSuccessCount, OpsFailedCount, RequestLatency)
}

var (
	CounterType   pkg.MetricType = "counter"
	GaugeType     pkg.MetricType = "gauge"
	HistogramType pkg.MetricType = "histogram"

	CustomGauges     map[string]*prometheus.GaugeVec
	CustomCounters   map[string]*prometheus.CounterVec
	CustomHistograms map[string]*prometheus.HistogramVec

	// Global metrics
	CanaryCheckInfo *prometheus.GaugeVec
	Gauge           *prometheus.GaugeVec

	// Check specific metrics
	OpsCount        *prometheus.CounterVec
	OpsFailedCount  *prometheus.CounterVec
	OpsSuccessCount *prometheus.CounterVec
	RequestLatency  *prometheus.HistogramVec
)

var (
	failed    = cmap.New()
	passed    = cmap.New()
	latencies = cmap.New()
)

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

func GetMetrics(key string) (uptime types.Uptime, latency types.Latency) {
	uptime = types.Uptime{}

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
		latency = types.Latency{Rolling1H: lat.(*rolling.TimePolicy).Reduce(rolling.Percentile(95))}
	}
	return
}

func Record(
	ctx context.Context,
	canary v1.Canary,
	result *pkg.CheckResult,
) (_uptime types.Uptime, _latency types.Latency) {
	defer func() {
		e := recover()
		if e != nil {
			ctx.Errorf("panic recording metrics for %s ==> %s", result, e)
		}
	}()

	if result == nil || result.Check == nil {
		ctx.Warnf("returned a nil result")
		return _uptime, _latency
	}

	if canary.GetCheckID(result.Check.GetName()) == "" {
		if val := result.Canary.Labels["transformed"]; val != "true" {
			ctx.Warnf("%s returned a result for a check that does not exist", result.Check.GetName())
		}
		return _uptime, _latency
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

	var additionalLabels []string
	for _, key := range v1.AdditionalCheckMetricLabels {
		if v, ok := result.Check.GetLabels()[key]; ok {
			additionalLabels = append(additionalLabels, v)
		} else {
			// just insert an empty value
			additionalLabels = append(additionalLabels, "")
		}
	}

	checkMetricLabels := append(
		[]string{checkType, endpoint, canaryName, canaryNamespace, owner, severity, key, name},
		additionalLabels...)

	OpsCount.WithLabelValues(checkMetricLabels...).Inc()

	if result.Duration > 0 {
		RequestLatency.WithLabelValues(checkMetricLabels...).Observe(float64(result.Duration))
		latency.Append(float64(result.Duration))
	}

	gaugeLabels := append([]string{key, checkType, canaryName, canaryNamespace, name}, v1.AdditionalCheckMetricLabels...)

	if result.Pass {
		pass.Append(1)
		Gauge.WithLabelValues(gaugeLabels...).Set(0)

		CanaryCheckInfo.WithLabelValues(checkMetricLabels...).Set(0)
		OpsSuccessCount.WithLabelValues(checkMetricLabels...).Inc()
		// always add a failed count to ensure the metric is present in prometheus
		// for an uptime calculation
		OpsFailedCount.WithLabelValues(checkMetricLabels...).Add(0)

		for _, m := range result.Metrics {
			switch m.Type {
			case CounterType:
				if err := getOrCreateCounter(m); err != nil {
					ctx.Errorf("cannot create counter %s with labels %v: %v", m.Name, m.Labels, err)
				}

			case GaugeType:
				if err := getOrCreateGauge(m); err != nil {
					ctx.Errorf("cannot create gauge %s with labels %v: %v", m.Name, m.Labels, err)
				}

			case HistogramType:
				if err := getOrCreateHistogram(m); err != nil {
					ctx.Errorf("cannot create histogram %s with labels %v: %v", m.Name, m.Labels, err)
				}
			}
		}
	} else {
		fail.Append(1)
		Gauge.WithLabelValues(gaugeLabels...).Set(1)

		CanaryCheckInfo.WithLabelValues(checkMetricLabels...).Set(1)
		OpsFailedCount.WithLabelValues(checkMetricLabels...).Inc()
	}

	_uptime = types.Uptime{Passed: int(pass.Reduce(rolling.Sum)), Failed: int(fail.Reduce(rolling.Sum))}
	if latency != nil {
		_latency = types.Latency{Rolling1H: latency.Reduce(rolling.Percentile(95))}
	} else {
		_latency = types.Latency{}
	}
	return _uptime, _latency
}

func getOrCreateGauge(m pkg.Metric) error {
	var gauge *prometheus.GaugeVec
	var ok bool
	if gauge, ok = CustomGauges[m.ID()]; !ok {
		gauge = prometheus.V2.NewGaugeVec(prometheus.GaugeVecOpts{
			VariableLabels: prometheus.UnconstrainedLabels(m.LabelNames()),
			GaugeOpts: prometheus.GaugeOpts{
				Name: m.Name,
			},
		})
		CustomGauges[m.ID()] = gauge
	}

	if metric, err := gauge.GetMetricWith(m.Labels); err != nil {
		return err
	} else {
		metric.Set(m.Value)
		return nil
	}
}

func getOrCreateCounter(m pkg.Metric) error {
	var counter *prometheus.CounterVec
	var ok bool

	if counter, ok = CustomCounters[m.ID()]; !ok {
		counter = prometheus.V2.NewCounterVec(prometheus.CounterVecOpts{
			VariableLabels: prometheus.UnconstrainedLabels(m.LabelNames()),
			CounterOpts: prometheus.CounterOpts{
				Name: m.Name,
			},
		})
		CustomCounters[m.ID()] = counter
	}

	if metric, err := counter.GetMetricWith(m.Labels); err != nil {
		return err
	} else {
		metric.Add(m.Value)
		return nil
	}
}

func getOrCreateHistogram(m pkg.Metric) error {
	var histogram *prometheus.HistogramVec
	var ok bool
	if histogram, ok = CustomHistograms[m.ID()]; !ok {
		histogram = prometheus.V2.NewHistogramVec(prometheus.HistogramVecOpts{
			VariableLabels: prometheus.UnconstrainedLabels(m.LabelNames()),
			HistogramOpts: prometheus.HistogramOpts{
				Name: m.Name,
			},
		})
		CustomHistograms[m.ID()] = histogram
	}
	if metric, err := histogram.GetMetricWith(m.Labels); err != nil {
		return err
	} else {
		metric.Observe(m.Value)
		return nil
	}
}

func FillLatencies(checkKey string, duration string, latency *types.Latency) error {
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

func FillUptime(checkKey, duration string, uptime *types.Uptime) error {
	if runner.Prometheus == nil || duration == "" {
		return nil
	}
	value, err := runner.Prometheus.GetUptime(checkKey, duration)
	if err != nil {
		return err
	}
	uptime.P100 = lo.ToPtr(value)
	return nil
}

func UnregisterGauge(ctx context.Context, checkIDs []string) {
	for _, checkID := range checkIDs {
		ctx.Debugf("Unregistering gauge for checkID %s", checkID)
		Gauge.DeletePartialMatch(prometheus.Labels{"key": checkID})
	}
}
