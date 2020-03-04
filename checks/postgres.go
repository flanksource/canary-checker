package checks

import (
	"github.com/prometheus/client_golang/prometheus"

	"github.com/flanksource/canary-checker/pkg"
)

var (
	postgresFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "canary_check_postgres_connectivity_failed",
		Help: "The total number of postgres connectivity checks failed",
	})
)

func init() {
	prometheus.MustRegister(postgresFailed)
}

type PostgresChecker struct{}

// Type: returns checker type
func (c *PostgresChecker) Type() string {
	return "postgres"
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *PostgresChecker) Run(config pkg.Config) []*pkg.CheckResult {
	var checks []*pkg.CheckResult
	for _, conf := range config.Postgres {
		for _, result := range c.Check(conf.PostgresCheck) {
			checks = append(checks, result)
		}
	}
	return checks
}

// CheckConfig : Gives failure result
// Returns check result and metrics
func (c *PostgresChecker) Check(check pkg.PostgresCheck) []*pkg.CheckResult {
	var result []*pkg.CheckResult
	checkResult := &pkg.CheckResult{
		Pass:     false,
		Invalid:  true,
		Endpoint: check.Connection,
		Metrics:  []pkg.Metric{},
	}
	result = append(result, checkResult)
	return result
}
