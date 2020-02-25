package checks

import "github.com/flanksource/canary-checker/pkg"

type Checker interface {
	Run(config pkg.Config) []*pkg.CheckResult
	Type() string
}
