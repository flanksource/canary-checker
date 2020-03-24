package checks

import "github.com/flanksource/canary-checker/pkg"

type Checker interface {
	Run(config pkg.Config, results chan *pkg.CheckResult)
	Type() string
}
