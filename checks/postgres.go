package checks

import (
	"database/sql"
	"time"

	"github.com/flanksource/canary-checker/pkg"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
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
func (c *PostgresChecker) Run(config pkg.Config, results chan *pkg.CheckResult) {
	for _, conf := range config.Postgres {
		for _, result := range c.Check(conf.PostgresCheck) {
			results <- result
		}
	}
}

// CheckConfig : Attempts to connect to a DB using the specified
//               driver and connection string
// Returns check result and metrics
func (c *PostgresChecker) Check(check pkg.PostgresCheck) []*pkg.CheckResult {

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
			log.Errorf(err.Error())
		}
		if queryResult != check.Result {
			log.Errorf("Query '%s', did not return '%d', but '%d'", check.Query, check.Result, queryResult)
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
	log.Debugf("Duration %f", float64(elapsed.Milliseconds()))
	return result

}

// Connects to a db using the specified `driver` and `connectionstring`
// Performs the test query given in `query`.
// Gives the single row test query result as result.
func connectWithDriver(driver string, connectionSting string, query string) (int, error) {
	db, err := sql.Open(driver, connectionSting)
	if err != nil {
		log.Error(err.Error())
		return 0, err
	}
	defer db.Close()

	var resultValue int
	err = db.QueryRow(query).Scan(&resultValue)
	if err != nil {
		log.Error(err.Error())
		return 0, err
	}
	log.Debugf("Connection test query result of %d", resultValue)

	return resultValue, nil
}
