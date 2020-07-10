package metrics

import (
	"strconv"

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
		[]string{"type", "endpoint", "name", "namespace"},
	)

	OpsSuccessCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "canary_check_success_count",
			Help: "The total number of successful checks",
		},
		[]string{"type", "endpoint", "name", "namespace"},
	)

	OpsFailedCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "canary_check_failed_count",
			Help: "The total number of failed checks",
		},
		[]string{"type", "endpoint", "name", "namespace"},
	)

	RequestLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "canary_check_duration",
			Help:    "A histogram of the response latency in milliseconds.",
			Buckets: []float64{5, 10, 25, 50, 200, 500, 1000, 3000, 10000, 30000},
		},
		[]string{"type", "endpoint", "name", "namespace"},
	)

	Guage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "canary_check",
			Help: "A gauge representing the canaries success (0) or failure (1)",
		},
		[]string{"type", "endpoint", "name", "namespace"},
	)

	GenericGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "canary_check_gauge",
			Help: "A gauge representing duration",
		},
		[]string{"type", "name", "metric", "namespace"},
	)

	GenericCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "canary_check_counter",
			Help: "A gauge representing counters",
		},
		[]string{"type", "name", "metric", "value", "namespace"},
	)

	GenericHistogram = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "canary_check_histogram",
			Help:    "A histogram representing durations",
			Buckets: []float64{5, 10, 25, 50, 200, 500, 1000, 2500, 5000, 10000, 20000},
		},
		[]string{"type", "name", "metric", "namespace"},
	)
)

func init() {
	prometheus.MustRegister(Guage, OpsCount, OpsSuccessCount, OpsFailedCount, RequestLatency, GenericGauge, GenericCounter, GenericHistogram)
}

func Record(namespace, name string, result *pkg.CheckResult) {
	if result == nil || result.Check == nil {
		logger.Warnf("%s/%s returned a nil result", namespace, name)
		return
	}
	checkType := result.Check.GetType()
	endpoint := result.Check.GetDescription()
	if endpoint == "" {
		endpoint = result.Check.GetEndpoint()
	}
	if logger.IsTraceEnabled() {
		logger.Tracef(result.String())
	}
	OpsCount.WithLabelValues(checkType, endpoint, name, namespace).Inc()
	if result.Pass {
		Guage.WithLabelValues(checkType, endpoint, name, namespace).Set(0)
		OpsSuccessCount.WithLabelValues(checkType, endpoint, name, namespace).Inc()
		if result.Duration > 0 {
			RequestLatency.WithLabelValues(checkType, endpoint, name, namespace).Observe(float64(result.Duration))
		}

		for _, m := range result.Metrics {
			switch m.Type {
			case CounterType:
				GenericCounter.WithLabelValues(checkType, endpoint, m.Name, strconv.Itoa(int(m.Value)), namespace).Inc()
			case GaugeType:
				GenericGauge.WithLabelValues(checkType, endpoint, m.Name, namespace).Set(m.Value)
			case HistogramType:
				GenericHistogram.WithLabelValues(checkType, endpoint, m.Name, namespace).Observe(m.Value)
			}
		}
	} else {
		Guage.WithLabelValues(checkType, endpoint, name, namespace).Set(1)
		OpsFailedCount.WithLabelValues(checkType, endpoint, name, namespace).Inc()
	}
}
