package checks

import (
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	_ "github.com/lib/pq"
)

func init() {
	//register metrics here
}

type PostgresChecker struct{}

// Type: returns checker type
func (c *PostgresChecker) Type() string {
	return "postgres"
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *PostgresChecker) Run(config v1.CanarySpec) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range config.Postgres {
		results = append(results, c.Check(conf))
	}
	return results
}

func (c *PostgresChecker) Check(extConfig external.Check) *pkg.CheckResult {
	return CheckSql(extConfig.(v1.PostgresCheck).SqlCheck)
}
