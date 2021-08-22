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

func (c *PrometheusChecker) Run(ctx *context.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range ctx.Canary.Spec.Prometheus {
		results = append(results, c.Check(ctx, conf))
	}
	return results
}

func (c *PrometheusChecker) Check(ctx *context.Context, extConfig external.Check) *pkg.CheckResult {
	check := extConfig.(v1.PrometheusCheck)
	result := pkg.Success(check)

	promClient, err := prometheus.NewPrometheusAPI(check.Host)
	if err != nil {
		return result.ErrorMessage(err)
	}
	modelValue, warning, err := promClient.Query(ctx, check.Query, time.Now())
	if err != nil {
		return result.ErrorMessage(err)
	}
	if warning != nil {
		logger.Debugf("warnings when running the query: %v", warning)
	}
	var results = make([]interface{}, 0)
	if modelValue != nil {
		for _, value := range modelValue.(model.Vector) {
			results = append(results, value.Metric)
		}
	}
	return result.AddData(map[string]interface{}{"results": result})
}
