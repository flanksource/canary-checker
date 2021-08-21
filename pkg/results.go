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
	}
}

func (result *CheckResult) ErrorMessage(err error) *CheckResult {
	result.Error = err.Error()
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

func (result *CheckResult) AddDetails(detail interface{}) *CheckResult {
	result.Detail = detail
	return result
}
