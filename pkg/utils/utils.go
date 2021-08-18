package utils

import (
	"github.com/flanksource/canary-checker/api/external"
	"github.com/flanksource/canary-checker/checks"
)

func CheckerInChecks(allChecks []external.Check, checker checks.Checker) bool {
	for _, check := range allChecks {
		if check.GetType() == checker.Type() {
			return true
		}
	}
	return false
}
