package checks

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/types"
	"github.com/google/uuid"
)

type SQLChecker interface {
	GetCheck() external.Check
	GetDriver() string
	GetSQLCheck() v1.SQLCheck
	GetType() string
}

type SQLDetails struct {
	Rows  []map[string]interface{} `json:"rows"`
	Count int                      `json:"count"`
}

// Connects to a db using the specified `driver` and `connectionstring`
// Performs the test query given in `query`.
// Gives the single row test query result as result.
func querySQL(ctx *context.Context, driver string, connection string, query string, timout time.Duration) (SQLDetails, error) {
	var result SQLDetails

	if driver == mysqlCheckType {
		// mysql driver expects a connection string in the format:
		// username:password@protocol(address)/dbname?param=value
		connection = strings.TrimPrefix(connection, "mysql://")
	}

	db, err := sql.Open(driver, connection)
	if err != nil {
		return result, fmt.Errorf("failed to connect to %s db: %w", driver, err)
	}
	defer db.Close()
	tctx, cancel := ctx.WithTimeout(timout * time.Second)
	defer cancel()
	rows, err := db.QueryContext(tctx, query)

	if err != nil || rows.Err() != nil {
		return result, fmt.Errorf("failed to query db: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return result, fmt.Errorf("failed to get columns: %w", err)
	}

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return result, fmt.Errorf("failed to get column types: %w", err)
	}

	for rows.Next() {
		rowValues := getRowValues(columnTypes)
		if err := rows.Scan(rowValues...); err != nil {
			return result, err
		}

		row := make(map[string]interface{})
		for i, val := range rowValues {
			if val == nil {
				row[columns[i]] = nil
				continue
			}

			switch v := val.(type) {
			case *sql.NullString:
				if v.Valid {
					row[columns[i]] = v.String
				} else {
					row[columns[i]] = nil
				}
			case *sql.NullInt32:
				if v.Valid {
					row[columns[i]] = v.Int32
				} else {
					row[columns[i]] = nil
				}
			case *sql.NullInt64:
				if v.Valid {
					row[columns[i]] = v.Int64
				} else {
					row[columns[i]] = nil
				}
			case *sql.NullFloat64:
				if v.Valid {
					row[columns[i]] = v.Float64
				} else {
					row[columns[i]] = nil
				}
			case *sql.NullBool:
				if v.Valid {
					row[columns[i]] = v.Bool
				} else {
					row[columns[i]] = nil
				}
			case *sql.NullTime:
				if v.Valid {
					row[columns[i]] = v.Time
				} else {
					row[columns[i]] = nil
				}
			case *types.JSON:
				jsonObj := make(map[string]any)
				var jsonArray []any
				if err := json.Unmarshal(*v, &jsonObj); err == nil {
					row[columns[i]] = jsonObj
				} else if err := json.Unmarshal(*v, &jsonArray); err == nil {
					row[columns[i]] = jsonArray
				} else {
					return result, fmt.Errorf("failed to parse json column: %s", *v)
				}
			default:
				row[columns[i]] = fmt.Sprint(val)
			}
		}

		result.Rows = append(result.Rows, row)
	}

	result.Count = len(result.Rows)
	return result, nil
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
	timout := check.GetQueryTimeout()

	if ctx.CanTemplate() {
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

	details, err := querySQL(ctx, checker.GetDriver(), connection.URL, query, timout)
	result.AddDetails(details)

	if err != nil {
		return results.ErrorMessage(err)
	}

	if details.Count == 0 && check.ShouldMarkFailOnEmpty() {
		return results.Failf("Query returned empty result")
	}

	if details.Count < check.Result {
		return results.Failf("Query returned %d rows, expected %d", details.Count, check.Result)
	}

	return results
}

// Note: maybe move to duty
func getRowValues(columnTypes []*sql.ColumnType) []interface{} {
	rowValues := make([]interface{}, len(columnTypes))

	for i, columnType := range columnTypes {
		switch columnType.DatabaseTypeName() {
		case "INT4", "INT8", "INT", "INT16", "INT32":
			var v sql.NullInt32
			rowValues[i] = &v
		case "BIGINT", "INT64":
			var v sql.NullInt64
			rowValues[i] = &v
		case "FLOAT", "FLOAT4", "FLOAT8", "DOUBLE", "DECIMAL", "NUMERIC":
			var v sql.NullFloat64
			rowValues[i] = &v
		case "BOOL":
			var v sql.NullBool
			rowValues[i] = &v
		case "DATETIME", "TIMESTAMP", "TIMESTAMPTZ":
			var v sql.NullTime
			rowValues[i] = &v
		case "JSONB", "JSON":
			var v types.JSON
			rowValues[i] = &v
		case "UUID":
			var v uuid.UUID
			rowValues[i] = &v
		case "TEXT", "VARCHAR", "CHAR", "NAME", "NVARCHAR", "NTEXT":
			var v sql.NullString
			rowValues[i] = &v
		default:
			logger.Warnf("unhandled database column type: %s", columnType.DatabaseTypeName())
			var v sql.NullString
			rowValues[i] = &v
		}
	}

	return rowValues
}
