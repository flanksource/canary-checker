package pkg

import (
	"fmt"
	"time"

	"github.com/flanksource/canary-checker/api/external"
)

func Fail(check external.Check) *CheckResult {
	return &CheckResult{
		Check: check,
		Pass:  false,
	}
}

func Success(check external.Check) *CheckResult {
	return &CheckResult{
		Pass:  true,
		Check: check,
		Data:  make(map[string]interface{}),
	}
}

func (result *CheckResult) ErrorMessage(err error) *CheckResult {
	result.Error = err.Error()
	result.Pass = false
	return result
}

func (result *CheckResult) ResultMessage(message string, args ...interface{}) *CheckResult {
	result.Message = fmt.Sprintf(message, args...)
	return result
}

func (result *CheckResult) StartTime(start time.Time) *CheckResult {
	result.Duration = time.Since(start).Milliseconds()
	return result
}

func (result *CheckResult) ResultDescription(description string) *CheckResult {
	result.Description = description
	return result
}

func (result *CheckResult) TextResults(textResults bool) *CheckResult {
	if textResults {
		result.DisplayType = "Text"
	}
	return result
}

func (result *CheckResult) Failf(message string, args ...interface{}) *CheckResult {
	if result.Error != "" {
		result.Error += ", "
	}
	result.Pass = false
	result.Error = result.Error + fmt.Sprintf(message, args...)
	return result
}

func (result *CheckResult) AddDetails(detail interface{}) *CheckResult {
	result.Detail = detail
	if result.Data == nil {
		result.Data = make(map[string]interface{})
	}
	result.Data["results"] = detail
	return result
}

func (result *CheckResult) AddMetric(metric Metric) *CheckResult {
	result.Metrics = append(result.Metrics, metric)
	return result
}

func (result *CheckResult) AddData(data map[string]interface{}) *CheckResult {
	if result.Data == nil {
		result.Data = data
		return result
	}
	for k, v := range data {
		result.Data[k] = v
	}
	return result
}
