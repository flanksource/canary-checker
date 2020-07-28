package checks

import (
	"database/sql"
	"fmt"
	"time"

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

// CheckConfig : Attempts to connect to a DB using the specified
//               driver and connection string
// Returns check result and metrics
func (c *PostgresChecker) Check(extConfig external.Check) *pkg.CheckResult {
	check := extConfig.(v1.PostgresCheck)
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

// Connects to a db using the specified `driver` and `connectionstring`
// Performs the test query given in `query`.
// Gives the single row test query result as result.
func connectWithDriver(driver string, connectionSting string, query string) (int, error) {
	db, err := sql.Open(driver, connectionSting)
	if err != nil {
		return 0, fmt.Errorf("failed to connect to db: %s", err.Error())

	}
	defer db.Close()

	var resultValue int
	err = db.QueryRow(query).Scan(&resultValue)
	if err != nil {
		return 0, fmt.Errorf("failed to query db: %s", err.Error())
	}
	return resultValue, nil
}
