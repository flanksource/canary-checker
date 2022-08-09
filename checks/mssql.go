package checks

import (
	"github.com/flanksource/canary-checker/api/context"

	_ "github.com/denisenkom/go-mssqldb" // required by mssql
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

func init() {
	//register metrics here
}

type MssqlChecker struct{}

// Type: returns checker type
func (c *MssqlChecker) Type() string {
	return "mssql"
}

// Run - Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *MssqlChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.Mssql {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

// Check CheckConfig : Attempts to connect to a DB using the specified
//
//	driver and connection string
//
// Returns check result and metrics
func (c *MssqlChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	return CheckSQL(ctx, extConfig.(v1.MssqlCheck))
}
