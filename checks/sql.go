package checks

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

type SQLChecker interface {
	GetCheck() external.Check
	GetDriver() string
	GetSQLCheck() v1.SQLCheck
	GetType() string
}

type SQLDetails struct {
	Rows  []map[string]interface{} `json:"rows,omitempty"`
	Count int                      `json:"count,omitempty"`
}

// Package contains common function used by SQL Checks (Currently Postgresql and Mssql)

// Connects to a db using the specified `driver` and `connectionstring`
// Performs the test query given in `query`.
// Gives the single row test query result as result.
func querySQL(driver string, connection string, query string) (*SQLDetails, error) {
	db, err := sql.Open(driver, connection)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to db: %w", err)
	}
	defer db.Close()

	rows, err := db.Query(query)
	result := SQLDetails{}
	if err != nil || rows.Err() != nil {
		return nil, fmt.Errorf("failed to query db: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	for rows.Next() {
		var rowValues = make([]interface{}, len(columns))
		for i := range rowValues {
			var s sql.NullString
			rowValues[i] = &s
		}
		if err := rows.Scan(rowValues...); err != nil {
			return nil, err
		}

		var row = make(map[string]interface{})
		for i, val := range rowValues {
			v := *val.(*sql.NullString)
			if v.Valid {
				row[columns[i]] = v.String
			} else {
				row[columns[i]] = nil
			}
		}

		result.Rows = append(result.Rows, row)
	}

	result.Count = len(result.Rows)
	return &result, nil
}

// CheckSQL : Attempts to connect to a DB using the specified
//
//	driver and connection string
//
// Returns check result and metrics
func CheckSQL(ctx *context.Context, checker SQLChecker) pkg.Results { // nolint: golint
	check := checker.GetSQLCheck()
	result := pkg.Success(checker.GetCheck(), ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	if check.Connection.Connection != "" && !strings.HasPrefix(check.Connection.Connection, "connection://") {
		check.URL = check.Connection.Connection
		check.Connection.Connection = ""
	}

	connection, err := ctx.GetConnection(check.Connection)
	if err != nil {
		return results.Failf("error getting connection: %v", err)
	}

	query := check.GetQuery()

	if ctx.Canary.Annotations["template"] != "false" {
		query, err = template(ctx.WithCheck(checker.GetCheck()), v1.Template{
			Template: query,
		})
		if err != nil {
			return results.ErrorMessage(err)
		}
		if ctx.IsDebug() {
			ctx.Infof("query: %s", query)
		}
	}

	details, err := querySQL(checker.GetDriver(), connection.URL, query)
	if err != nil {
		return results.ErrorMessage(err)
	}

	result.AddDetails(details)
	if details.Count < check.Result {
		return results.Failf("Query returned %d rows, expected %d", details.Count, check.Result)
	}

	return results
}
