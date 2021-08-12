package checks

import (
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
func (c *MssqlChecker) Run(canary v1.Canary) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range canary.Spec.Mssql {
		results = append(results, c.Check(canary, conf))
	}
	return results
}

// Check CheckConfig : Attempts to connect to a DB using the specified
//               driver and connection string
// Returns check result and metrics
func (c *MssqlChecker) Check(canary v1.Canary, extConfig external.Check) *pkg.CheckResult {
	return CheckSQL(extConfig.(v1.MssqlCheck).SQLCheck)
}
