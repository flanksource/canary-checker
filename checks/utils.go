package checks

import (
	"fmt"

	"github.com/flanksource/canary-checker/pkg"
)

func unexpectedErrorf(check pkg.Endpointer, err error, msg string, args ...interface{}) []*pkg.CheckResult {
	return []*pkg.CheckResult{&pkg.CheckResult{
		Check:    check,
		Pass:     false,
		Invalid:  false,
		Endpoint: check.GetEndpoint(),
		Message:  fmt.Sprintf("unexpected error %s: %v", fmt.Sprintf(msg, args...), err),
	}}
}

func invalidErrorf(check pkg.Endpointer, err error, msg string, args ...interface{}) []*pkg.CheckResult {
	return []*pkg.CheckResult{&pkg.CheckResult{
		Check:    check,
		Pass:     false,
		Invalid:  true,
		Endpoint: check.GetEndpoint(),
		Message:  fmt.Sprintf("%s: %v", fmt.Sprintf(msg, args...), err),
	}}
}

func Failf(check pkg.Endpointer, msg string, args ...interface{}) []*pkg.CheckResult {
	return []*pkg.CheckResult{&pkg.CheckResult{
		Check:    check,
		Pass:     false,
		Invalid:  false,
		Endpoint: check.GetEndpoint(),
		Message:  fmt.Sprintf(msg, args...),
	}}
}

func Passf(check pkg.Endpointer, msg string, args ...interface{}) []*pkg.CheckResult {
	return []*pkg.CheckResult{&pkg.CheckResult{
		Check:    check,
		Pass:     true,
		Invalid:  false,
		Endpoint: check.GetEndpoint(),
		Message:  fmt.Sprintf(msg, args...),
	}}
}
