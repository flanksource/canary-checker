package checks

import (
	"time"

	"github.com/flanksource/canary-checker/api/context"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/prometheus"
	"github.com/flanksource/commons/logger"
	"github.com/prometheus/common/model"
)

type PrometheusChecker struct{}

func (c *PrometheusChecker) Type() string {
	return "prometheus"
}

func (c *PrometheusChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.Prometheus {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

func (c *PrometheusChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.PrometheusCheck)
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	//nolint:staticcheck
	if check.Host != "" {
		return results.Failf("host field is deprecated, use url field instead")
	}

	// Use global prometheus url if check's url is empty
	if check.URL == "" {
		check.URL = prometheus.PrometheusURL
	}

	connection, err := ctx.GetConnection(check.Connection)
	if err != nil {
		return results.Failf("error getting connection: %v", err)
	}

	if connection.URL == "" {
		return results.Failf("Must specify a URL")
	}

	promClient, err := prometheus.NewPrometheusAPI(connection.URL)
	if err != nil {
		return results.ErrorMessage(err)
	}
	modelValue, warning, err := promClient.Query(ctx.Context, check.Query, time.Now())
	if err != nil {
		return results.ErrorMessage(err)
	}
	if warning != nil {
		logger.Debugf("warnings when running the query: %v", warning)
	}
	var prometheusResults = make([]map[string]interface{}, 0)
	var data = map[string]interface{}{
		"value":       0,
		"firstResult": make(map[string]string),
	}
	if modelValue != nil {
		for i, value := range modelValue.(model.Vector) {
			val := make(map[string]interface{})
			val["value"] = float64(value.Value)
			if i == 0 {
				data["firstResult"] = val
				data["value"] = float64(value.Value)
			}
			for k, v := range value.Metric {
				val[string(k)] = string(v)
			}
			prometheusResults = append(prometheusResults, val)
		}
	}
	if len(prometheusResults) != 0 {
		check.Labels = check.Labels.AddLabels(data["firstResult"].(map[string]interface{}))
	}
	result.UpdateCheck(check)
	data["results"] = prometheusResults
	result.AddData(data)
	return results
}
