package checks

import (
	"database/sql"
	"fmt"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

// Package contains common function used by SQL Checks (Currently Postgresql and Mssql)

// Connects to a db using the specified `driver` and `connectionstring`
// Performs the test query given in `query`.
// Gives the single row test query result as result.
func querySQL(driver string, connectionSting string, query string) (int, error) {
	db, err := sql.Open(driver, connectionSting)
	if err != nil {
		return 0, fmt.Errorf("failed to connect to db: %s", err.Error())
	}
	defer db.Close()

	var count int
	rows, err := db.Query(query)
	if err != nil || rows.Err() != nil {
		return 0, fmt.Errorf("failed to query db: %s", err.Error())
	}
	for rows.Next() {
		count++
	}

	return count, nil
}

// CheckConfig : Attempts to connect to a DB using the specified
//               driver and connection string
// Returns check result and metrics
func CheckSQL(check v1.SQLCheck) *pkg.CheckResult { // nolint: golint
	start := time.Now()
	queryResult, err := querySQL(check.GetDriver(), check.GetConnection(), check.GetQuery())
	if err != nil {
		return Failf(check, "failed to execute query %s", err)
	}
	if queryResult != check.Result {
		return Failf(check, "expected %d results, got %d", check.GetResult(), queryResult)
	}
	return Success(check, start)
}
