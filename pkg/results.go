package pkg

import (
	"fmt"
	"time"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
)

func Fail(check external.Check, canary v1.Canary) *CheckResult {
	return &CheckResult{
		Check:  check,
		Data:   make(map[string]interface{}),
		Start:  time.Now(),
		Pass:   false,
		Canary: canary,
	}
}

func SetupError(canary v1.Canary, err error) []*CheckResult {
	var results []*CheckResult
	for _, check := range canary.Spec.GetAllChecks() {
		results = append(results, &CheckResult{
			Start:   time.Now(),
			Pass:    false,
			Invalid: true,
			Error:   err.Error(),
			Check:   check,
			Data:    make(map[string]interface{}),
		})
	}
	return results
}

func Success(check external.Check, canary v1.Canary) *CheckResult {
	return &CheckResult{
		Start:  time.Now(),
		Pass:   true,
		Check:  check,
		Data:   make(map[string]interface{}),
		Canary: canary,
	}
}

func (result *CheckResult) ErrorMessage(err error) *CheckResult {
	if err == nil {
		return result
	}
	return result.Failf(err.Error())
}

func (result *CheckResult) ResultMessage(message string, args ...interface{}) *CheckResult {
	result.Message = fmt.Sprintf(message, args...)
	return result
}

func (result *CheckResult) StartTime(start time.Time) *CheckResult {
	result.Start = start
	result.Duration = time.Since(start).Milliseconds()
	return result
}

func (result *CheckResult) GetDuration() int64 {
	if result.Duration > 0 {
		return result.Duration
	}
	if !result.Start.IsZero() {
		return time.Since(result.Start).Milliseconds()
	}
	return 0
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
	result.Error += fmt.Sprintf(message, args...)
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
