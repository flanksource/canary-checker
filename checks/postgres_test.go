package checks

import (
	sql "database/sql"
	"database/sql/driver"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/stretchr/testify/assert"
)

func TestObfuscatePassword(t *testing.T) {
	tests := []struct {
		name             string
		connectionString string
		want             string
	}{
		{
			"no password in connectionstring",
			"user=postgres password=mysecretpassword host=192.168.0.103 port=15432 dbname=postgres sslmode=disable",
			"user=postgres password=### host=192.168.0.103 port=15432 dbname=postgres sslmode=disable",
		},
	}
	for _, tt := range tests {
		got := obfuscateConnectionStringPassword(tt.connectionString)
		if tt.want != got {
			t.Errorf("Test Case '%s', want '%v', got '%v'", tt.name, tt.want, got)
		}
	}
}

func TestExecuteSimpleQuery(t *testing.T) {
	// create a mock db
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	var queryTests = []struct {
		description string
		query       string
		want        int
	}{
		{
			"constant 1",
			"SELECT 1",
			1,
		},
		{
			"Select single column with one result",
			"SELECT t.col1 FROM t",
			2,
		},
	}

	for _, tt := range queryTests {
		t.Run(tt.description, func(t *testing.T) {
			// This is the result we expect
			rows := sqlmock.NewRows([]string{"column"}).
				AddRow(tt.want)

			// declare our expectation
			mock.ExpectQuery("^" + tt.query + "$").WillReturnRows(rows)

			got, err := executeSimpleQuery(db, tt.query)

			if err != nil {
				t.Errorf("Test scenario '%s' failed with error: %v", tt.description, err)
			}

			if got != tt.want {
				t.Errorf("Test scenario '%s' failed. Wanted result of '%v', but got '%v'", tt.description, tt.want, got)
			}

			expectationErr := mock.ExpectationsWereMet()
			if expectationErr != nil {
				t.Errorf("Test scenario '%s' failed. Expected queries not made: %v", tt.description, expectationErr)
			}
		})

	}
}

func TestExecuteComplexQuery(t *testing.T) {
	// create a mock db
	//TODO NewWithDSN
	//db, mock, err := sqlmock.NewWithDSN()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	var queryTests = []struct {
		description string
		query       string
		wantColumns []string
		values      [][]interface{}
	}{
		{
			description: "Have colA and colB",
			query:       "SELECT * FROM a_table",
			wantColumns: []string{"colA", "colB"},
			values: [][]interface{}{
				{"valA1", time.Now()},
				{"valA2", "valB2"},
				{"valA3", "valB3"},
			},
		},
	}

	for _, tt := range queryTests {
		t.Run(tt.description, func(t *testing.T) {

			for _, rowVals := range tt.values {
				if len(tt.wantColumns) != len(rowVals) {
					t.Fatalf("Invalid table test value, columns and values misaligned: cols(%v), values(%v)", tt.wantColumns, rowVals)
				}
			}

			// This is the query result we expect
			rows := sqlmock.NewRows(tt.wantColumns)
			for _, rowVals := range tt.values {
				var values []driver.Value = make([]driver.Value, len(rowVals))
				for col, val := range rowVals {

					values[col] = val
				}
				rows.AddRow(values...)
			}

			// declare our expectation
			mock.ExpectQuery("^" + strings.ReplaceAll(tt.query, "*", "\\*") + "$").WillReturnRows(rows)

			got, err := executeComplexQuery(db, tt.query)

			if err != nil {
				t.Errorf("Test scenario '%s' failed with error: %v", tt.description, err)
			}

			var want []pkg.PostgresResults = make([]pkg.PostgresResults, 0)

			for _, rowVals := range tt.values {

				wantedResultMap := make(map[string]string)
				for i, col := range tt.wantColumns {

					value := fmt.Sprintf("%v", rowVals[i])

					wantedResultMap[col] = value
				}
				wantedResult := pkg.PostgresResults{}
				wantedResult.Values = wantedResultMap
				want = append(want, wantedResult)
			}

			t.Logf("Want:\n%+v", want)
			t.Logf("Got:\n%+v", got)
			assert.Equal(t, want, got)

			expectationErr := mock.ExpectationsWereMet()
			if expectationErr != nil {
				t.Errorf("Test scenario '%s' failed. Expected queries not made: %v", tt.description, expectationErr)
			}
		})

	}
}

func TestLocalComplexQuery(t *testing.T) {
	var conString string = "user=postgres password=mysecretpassword host=192.168.0.103 port=15432 dbname=postgres sslmode=disable"

	db, err := sql.Open("postgres", conString)
	if err != nil {
		t.Error(err.Error())
	}
	defer db.Close()

	got, err := executeComplexQuery(db, "SELECT * FROM b_table")
	t.Log(got)

}
