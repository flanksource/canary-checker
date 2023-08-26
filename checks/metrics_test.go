package checks

import (
	"testing"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestPrometheusExportedMetrics(t *testing.T) {
	canary := v1.Canary{
		Spec: v1.CanarySpec{
			HTTP: []v1.HTTPCheck{{
				Description: v1.Description{
					Name: "http-metrics-test",
					Metrics: []external.Metrics{
						{
							Name:  "counter_metric",
							Type:  "counter",
							Value: "result.code == 200 ? 1 : 0",
							Labels: []external.MetricLabel{
								{Name: "name", Value: "2xx_count"},
								{Name: "check_name", ValueExpr: "check.name"},
							},
						},
						{
							Name:  "gauge_metric",
							Type:  "gauge",
							Value: "result.code",
							Labels: []external.MetricLabel{
								{Name: "name", Value: "2xx_count"},
								{Name: "check_name", ValueExpr: "check.name"},
							},
						},
						{
							Name:  "histogram_metric",
							Type:  "histogram",
							Value: "check.duration",
							Labels: []external.MetricLabel{
								{Name: "name", Value: "2xx_count"},
								{Name: "check_name", ValueExpr: "check.name"},
							},
						},
					},
				},
				Connection: v1.Connection{
					URL: "https://httpbin.demo.aws.flanksource.com/status/200",
				},
			}},
		},
	}

	// Run the check 5 times
	for i := 0; i < 5; i++ {
		_, err := RunChecks(context.New(nil, nil, nil, canary))
		if err != nil {
			t.Fatalf("metrics test failed: %v", err)
		}
	}

	counter := collectorMap["counter_metric"].(*prometheus.CounterVec)
	gauge := collectorMap["gauge_metric"].(*prometheus.GaugeVec)
	histogram := collectorMap["histogram_metric"].(*prometheus.HistogramVec)

	assert := assert.New(t)

	// Test collected metrics
	assert.Equal(1, testutil.CollectAndCount(counter))
	assert.Equal(1, testutil.CollectAndCount(gauge))
	assert.Equal(1, testutil.CollectAndCount(histogram))

	// Test the expected values using the ToFloat64 function
	assert.Equal(float64(5), testutil.ToFloat64(counter.WithLabelValues("2xx_count", "http-metrics-test")))
	assert.Equal(float64(200), testutil.ToFloat64(gauge.WithLabelValues("2xx_count", "http-metrics-test")))
}
