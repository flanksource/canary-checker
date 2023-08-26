package checks

import (
	"strings"
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
							Value: "result.code",
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

	histogramExpectedOutput := `
        # HELP histogram_metric histogram_metric
        # TYPE histogram_metric histogram

        histogram_metric_bucket{check_name="http-metrics-test",name="2xx_count",le="0.005"} 0
        histogram_metric_bucket{check_name="http-metrics-test",name="2xx_count",le="0.01"} 0
        histogram_metric_bucket{check_name="http-metrics-test",name="2xx_count",le="0.025"} 0
        histogram_metric_bucket{check_name="http-metrics-test",name="2xx_count",le="0.05"} 0
        histogram_metric_bucket{check_name="http-metrics-test",name="2xx_count",le="0.1"} 0
        histogram_metric_bucket{check_name="http-metrics-test",name="2xx_count",le="0.25"} 0
        histogram_metric_bucket{check_name="http-metrics-test",name="2xx_count",le="0.5"} 0
        histogram_metric_bucket{check_name="http-metrics-test",name="2xx_count",le="1"} 0
        histogram_metric_bucket{check_name="http-metrics-test",name="2xx_count",le="2.5"} 0
        histogram_metric_bucket{check_name="http-metrics-test",name="2xx_count",le="5"} 0
        histogram_metric_bucket{check_name="http-metrics-test",name="2xx_count",le="10"} 0
        histogram_metric_bucket{check_name="http-metrics-test",name="2xx_count",le="+Inf"} 5
        histogram_metric_sum{check_name="http-metrics-test",name="2xx_count"} 1000
        histogram_metric_count{check_name="http-metrics-test",name="2xx_count"} 5
    `

	assert.NoError(testutil.CollectAndCompare(histogram, strings.NewReader(histogramExpectedOutput)))
}
