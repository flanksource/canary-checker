package pkg

import (
	"fmt"
	"time"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/duty/db"
)

type Results []*CheckResult

func Invalid(check external.Check, canary v1.Canary, reason string) Results {
	return Results{&CheckResult{
		Start:   time.Now(),
		Pass:    false,
		Invalid: true,
		Error:   reason,
		Check:   check,
		Data: map[string]interface{}{
			"results": make(map[string]interface{}),
		},
		Canary: canary,
	}}
}

func SetupError(canary v1.Canary, err error) Results {
	var results Results
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
	result := New(check, canary)
	result.Pass = true
	return result
}

func New(check external.Check, canary v1.Canary) *CheckResult {
	return &CheckResult{
		Start: time.Now(),
		Check: check,
		Data: map[string]interface{}{
			"results": make(map[string]interface{}),
		},
		Canary: canary,
	}
}

func (result *CheckResult) ErrorMessage(err error) *CheckResult {
	if err == nil {
		return result
	}
	result.ErrorObject = err
	return result.Failf(err.Error())
}

func (result *CheckResult) UpdateCheck(check external.Check) *CheckResult {
	result.Check = check
	return result
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

	result.InternalError = db.IsDBError(fmt.Errorf(message, args...))

	result.Pass = false
	result.Error += fmt.Sprintf(message, args...)
	return result
}

func (result *CheckResult) Invalidf(message string, args ...interface{}) Results {
	result = result.Failf(message, args...)
	result.Invalid = true
	return Results{result}
}

func (result *CheckResult) AddDetails(detail interface{}) *CheckResult {
	result.Detail = detail
	if result.Data == nil {
		result.Data = make(map[string]interface{})
	}
	result.Data["results"] = detail
	return result
}

func (result *CheckResult) ToSlice() Results {
	return Results{result}
}

func (result *CheckResult) AddMetric(metric Metric) *CheckResult {
	result.Metrics = append(result.Metrics, metric)
	return result
}

func (result *CheckResult) AddDataStruct(data interface{}) *CheckResult {
	if m, err := utils.ToJSONMap(data); err != nil {
		result.Invalidf(err.Error())
		return result
	} else {
		return result.AddData(m)
	}
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

func (r Results) Failf(msg string, args ...interface{}) Results {
	r[0].Failf(msg, args...)
	return r
}

func (r Results) Invalidf(msg string, args ...interface{}) Results {
	r[0].Invalidf(msg, args...)
	return r
}

func (r Results) WithError(err error) Results {
	r[0].ErrorObject = err
	return r
}

func (r Results) ErrorMessage(err error) Results {
	r[0].ErrorMessage(err)
	return r
}

func (r Results) TotalArtifacts() int {
	var total int
	for _, result := range r {
		total += len(result.Artifacts)
	}
	return total
}
