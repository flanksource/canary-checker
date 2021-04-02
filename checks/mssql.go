package checks

import (
	"time"

	_ "github.com/denisenkom/go-mssqldb"
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

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *MssqlChecker) Run(config v1.CanarySpec) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range config.Mssql {
		results = append(results, c.Check(conf))
	}
	return results
}

// CheckConfig : Attempts to connect to a DB using the specified
//               driver and connection string
// Returns check result and metrics
func (c *MssqlChecker) Check(extConfig external.Check) *pkg.CheckResult {
	check := extConfig.(v1.MssqlCheck)
	start := time.Now()
	queryResult, err := connectWithDriver(check.Driver, check.Connection, check.Query)
	if err != nil {
		return Failf(check, "failed to execute query %s", err)
	}
	if queryResult != check.Result {
		return Failf(check, "expected %d results, got %d", check.Result, queryResult)
	}
	return Success(check, start)
}
