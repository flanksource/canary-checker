package checks

import (
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	"github.com/flanksource/canary-checker/pkg"

	"database/sql"

	_ "github.com/lib/pq"
)

var (
	postgresFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "canary_check_postgres_connectivity_failed",
		Help: "The total number of postgres connectivity checks failed",
	})
)

func init() {
	prometheus.MustRegister(postgresFailed)
}

type PostgresChecker struct{}

// Type: returns checker type
func (c *PostgresChecker) Type() string {
	return "postgres"
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *PostgresChecker) Run(config pkg.Config) []*pkg.CheckResult {
	var checks []*pkg.CheckResult
	for _, conf := range config.Postgres {
		for _, result := range c.Check(conf.PostgresCheck) {
			checks = append(checks, result)
		}
	}
	return checks
}

// CheckConfig : Attempts to connect to a DB using the specified
//               driver and connection string
// Returns check result and metrics
func (c *PostgresChecker) Check(check pkg.PostgresCheck) []*pkg.CheckResult {

	var result []*pkg.CheckResult

	queryResult, err := connectWithDriver(check.Driver, check.Connection, check.Query)
	if (err != nil) || (queryResult != check.Result) {
		checkResult := &pkg.CheckResult{
			Pass:     false,
			Invalid:  false,
			Endpoint: check.Connection,
			Metrics:  []pkg.Metric{},
		}
		if err != nil {
			log.Error(err.Error())
		}
		if queryResult != check.Result {
			log.Error("Query '%s', did not return '%d', but '%d'", check.Query, check.Result, queryResult)
		}
		result = append(result, checkResult)
		return result
	}

	checkResult := &pkg.CheckResult{
		Pass:     true,
		Invalid:  false,
		Endpoint: check.Connection,
		Metrics:  []pkg.Metric{},
	}
	result = append(result, checkResult)
	return result

}

// Connect to a  db using the specified driver and connectionstring
// `connectionString`.
// Performs a `SELECT 1` test query.
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
	log.Debugf("Connection test query result of %i", resultValue)

	return resultValue, nil
}
