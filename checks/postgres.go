package checks

import (
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	_ "github.com/lib/pq" // Necessary for postgres
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
func (c *PostgresChecker) Run(canary v1.Canary) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range canary.Spec.Postgres {
		results = append(results, c.Check(canary, conf))
	}
	return results
}

func (c *PostgresChecker) Check(canary v1.Canary, extConfig external.Check) *pkg.CheckResult {
	return CheckSQL(extConfig.(v1.PostgresCheck).SQLCheck)
}
