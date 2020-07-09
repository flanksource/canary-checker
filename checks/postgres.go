package checks

import (
	"database/sql"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
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
		results = append(results, c.Check(conf)...)
	}
	return results
}

// CheckConfig : Attempts to connect to a DB using the specified
//               driver and connection string
// Returns check result and metrics
func (c *PostgresChecker) Check(check v1.PostgresCheck) []*pkg.CheckResult {
	var result []*pkg.CheckResult

	start := time.Now()
	queryResult, err := connectWithDriver(check.Driver, check.Connection, check.Query)
	elapsed := time.Since(start)
	if (err != nil) || (queryResult != check.Result) {
		checkResult := &pkg.CheckResult{
			Pass:     false,
			Invalid:  false,
			Duration: elapsed.Milliseconds(),
			Metrics:  []pkg.Metric{},
		}
		if err != nil {
			logger.Errorf(err.Error())
		}
		if queryResult != check.Result {
			logger.Errorf("Query '%s', did not return '%d', but '%d'", check.Query, check.Result, queryResult)
		}
		result = append(result, checkResult)
		return result
	}

	checkResult := &pkg.CheckResult{
		Check:    check,
		Pass:     true,
		Invalid:  false,
		Duration: elapsed.Milliseconds(),
		Metrics:  []pkg.Metric{},
	}
	result = append(result, checkResult)
	logger.Debugf("Duration %f", float64(elapsed.Milliseconds()))
	return result

}

// Connects to a db using the specified `driver` and `connectionstring`
// Performs the test query given in `query`.
// Gives the single row test query result as result.
func connectWithDriver(driver string, connectionSting string, query string) (int, error) {
	db, err := sql.Open(driver, connectionSting)
	if err != nil {
		logger.Errorf(err.Error())
		return 0, err
	}
	defer db.Close()

	var resultValue int
	err = db.QueryRow(query).Scan(&resultValue)
	if err != nil {
		logger.Errorf(err.Error())
		return 0, err
	}
	logger.Debugf("Connection test query result of %d", resultValue)

	return resultValue, nil
}
