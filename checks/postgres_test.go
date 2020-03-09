package checks

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
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
