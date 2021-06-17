package checks

import (
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	_ "github.com/godror/godror" // module required for OracleDB connection
)

type OracleDBChecker struct{}

// Type: returns checker type
func (c *OracleDBChecker) Type() string {
	return "oracleDB"
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *OracleDBChecker) Run(config v1.CanarySpec) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range config.OracleDB {
		results = append(results, c.Check(conf))
	}
	return results
}

// CheckConfig : Attempts to connect to a DB using the specified
//               driver and connection string
// Returns check result and metrics
func (c *OracleDBChecker) Check(extConfig external.Check) *pkg.CheckResult {
	return CheckSQL(extConfig.(v1.OracleDBCheck).SQLCheck)
}
