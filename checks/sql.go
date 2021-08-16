package checks

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/commons/text"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

// Package contains common function used by SQL Checks (Currently Postgresql and Mssql)

// Connects to a db using the specified `driver` and `connectionstring`
// Performs the test query given in `query`.
// Gives the single row test query result as result.
func querySQL(driver string, connectionSting string, query string) (count int, result []map[string]interface{}, err error) {
	db, err := sql.Open(driver, connectionSting)
	if err != nil {
		return 0, result, fmt.Errorf("failed to connect to db: %s", err.Error())
	}
	defer db.Close()
	rows, err := db.Query(query)
	if err != nil || rows.Err() != nil {
		return 0, result, fmt.Errorf("failed to query db: %s", err.Error())
	}
	columns, err := rows.Columns()
	if err != nil {
		return 0, result, fmt.Errorf("failed to get columns")
	}
	for rows.Next() {
		var rowValues = make([]interface{}, len(columns))
		for i := range rowValues {
			s := ""
			rowValues[i] = &s
		}
		err = rows.Scan(rowValues...)
		var row = make(map[string]interface{})
		for i, val := range rowValues {
			row[columns[i]] = *val.(*string)
		}
		result = append(result, row)
		count++
	}
	return count, result, nil
}

// CheckSQL : Attempts to connect to a DB using the specified
//               driver and connection string
// Returns check result and metrics
func CheckSQL(check v1.SQLCheck) *pkg.CheckResult { // nolint: golint
	start := time.Now()
	var textResults bool
	if check.DisplayTemplate != "" {
		textResults = true
	}
	template := check.GetDisplayTemplate()
	count, result, err := querySQL(check.GetDriver(), check.GetConnection(), check.GetQuery())
	if err != nil {
		return pkg.Fail(check).TextResults(textResults).ResultMessage(sqlTemplateResult(template)).ErrorMessage(err).StartTime(start)
	}
	if count == 0 {
		return pkg.Fail(check).TextResults(textResults).ResultMessage(sqlTemplateResult(template)).ErrorMessage(fmt.Errorf("0 rows returned from the query")).StartTime(start)
	}
	results := map[string]interface{}{"results": result}
	if check.ResultsFunction != "" {
		success, err := text.TemplateWithDelims(check.ResultsFunction, "[[", "]]", results)
		if err != nil {
			return pkg.Fail(check).TextResults(textResults).ResultMessage(sqlTemplateResult(template)).ErrorMessage(err).StartTime(start)
		}
		if strings.ToLower(success) != "true" {
			return pkg.Fail(check).TextResults(textResults).ResultMessage(sqlTemplateResult(template)).ErrorMessage(fmt.Errorf("result function returned %v", success)).StartTime(start)
		}
	}
	message, err := text.TemplateWithDelims(template, "[[", "]]", results)
	if err != nil {
		return pkg.Fail(check).TextResults(textResults).ResultMessage(sqlTemplateResult(template)).ErrorMessage(err).StartTime(start)
	}
	return pkg.Success(check).TextResults(textResults).ResultMessage(message).StartTime(start)
}

func sqlTemplateResult(template string) (message string) {
	var results = map[string]interface{}{"results": []string{"null"}}
	message, err := text.TemplateWithDelims(template, "[[", "]]", results)
	if err != nil {
		message = message + "\n" + err.Error()
	}
	return message
}
