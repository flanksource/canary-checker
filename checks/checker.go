package checks

import "github.com/flanksource/canary-checker/pkg"

type MetricProcessor func([]*pkg.CheckResult)

type Checker interface {
	Schedule(config pkg.Config, interval uint64, mp MetricProcessor)
	Run(config pkg.Config) []*pkg.CheckResult
	Type() string
}
