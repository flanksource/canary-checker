package checks

import (
	"github.com/flanksource/canary-checker/api/context"

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
func (c *PostgresChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.Postgres {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

func (c *PostgresChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	return CheckSQL(ctx, extConfig.(v1.PostgresCheck).SQLCheck)
}
