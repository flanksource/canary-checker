package checks

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/commons/text"
	"github.com/prometheus/common/model"
)

type PrometheusChecker struct{}

func (c *PrometheusChecker) Type() string {
	return "prometheus"
}

func (c *PrometheusChecker) Run(canary v1.Canary) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range canary.Spec.Prometheus {
		results = append(results, c.Check(canary, conf))
	}
	return results
}

func (c *PrometheusChecker) Check(canary v1.Canary, extConfig external.Check) *pkg.CheckResult {
	start := time.Now()
	check := extConfig.(v1.PrometheusCheck)
	template := check.GetDisplayTemplate()
	textResults := template != ""
	promClient, err := utils.NewPrometheusAPI(check.Host)
	if err != nil {
		pkg.Fail(check).TextResults(textResults).ErrorMessage(err).StartTime(start).ResultMessage(prometheusTemplateResult(template, nil))
	}
	modelValue, warning, err := promClient.Query(context.TODO(), check.Query, time.Now())
	if err != nil {
		pkg.Fail(check).TextResults(textResults).ErrorMessage(err).StartTime(start).ResultMessage(prometheusTemplateResult(template, nil))
	}
	if warning != nil {
		logger.Debugf("warnings when running the query: %v", warning)
	}
	var result = make([]interface{}, 0)
	if modelValue != nil {
		for _, value := range modelValue.(model.Vector) {
			result = append(result, value.Metric)
		}
	}
	var results = map[string]interface{}{"results": result}
	if check.ResultsFunction != "" {
		success, err := text.TemplateWithDelims(check.ResultsFunction, "[[", "]]", results)
		if err != nil {
			return pkg.Fail(check).TextResults(textResults).ResultMessage(prometheusTemplateResult(template, result)).ErrorMessage(err).StartTime(start)
		}
		if strings.ToLower(success) != "true" {
			return pkg.Fail(check).TextResults(textResults).ResultMessage(prometheusTemplateResult(template, result)).ErrorMessage(fmt.Errorf("result function returned %v", success)).StartTime(start)
		}
	}
	message, err := text.TemplateWithDelims(template, "[[", "]]", results)
	if err != nil {
		return pkg.Fail(check).TextResults(textResults).ResultMessage(prometheusTemplateResult(template, result)).ErrorMessage(err).StartTime(start)
	}
	return pkg.Success(check).TextResults(textResults).ResultMessage(message).StartTime(start)
}

func prometheusTemplateResult(template string, result interface{}) string {
	var results = map[string]interface{}{"results": result}
	message, err := text.TemplateWithDelims(template, "[[", "]]", results)
	if err != nil {
		message = message + "\n" + err.Error()
	}
	return message
}
