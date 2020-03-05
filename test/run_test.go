package test

import (
	"testing"

	"github.com/flanksource/canary-checker/cmd"
	"github.com/flanksource/canary-checker/pkg"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRunChecks(t *testing.T) {
	type args struct {
		config pkg.Config
	}
	tests := []struct {
		name string
		args args
		want []pkg.CheckResult
	}{
		{
			name: "http_pass",
			args: args{
				pkg.ParseConfig("../fixtures/http_pass.yaml"),
			},
			want: []pkg.CheckResult{
				{
					Pass:     true,
					Invalid:  false,
					Endpoint: "https://httpstat.us/200",
					Metrics:  []pkg.Metric{},
				},
			},
		},
		{
			name: "http_fail",
			args: args{
				pkg.ParseConfig("../fixtures/http_fail.yaml"),
			},
			want: []pkg.CheckResult{
				{
					Pass:     false,
					Invalid:  true,
					Endpoint: "https://ttpstat.us/500",
					Metrics:  []pkg.Metric{},
				},
				{
					Pass:     false,
					Invalid:  false,
					Endpoint: "https://httpstat.us/500",
					Metrics:  []pkg.Metric{},
				},
			},
		},
		{
			name: "postgres_fail",
			args: args{
				pkg.ParseConfig("../fixtures/postgres_fail.yaml"),
			},
			want: []pkg.CheckResult{
				{
					Pass:     false,
					Invalid:  false,
					Endpoint: "user=pqgotest dbname=pqgotest sslmode=verify-full",
					Metrics:  []pkg.Metric{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runs := cmd.RunChecks(tt.args.config)

			for i, run := range runs {
				if i > len(tt.want)-1 {
					t.Errorf("Test %s failed. Found unexpected extra result is %v", tt.name, run)
				} else {
					/* Not checking durations we don't want equality*/
					/* TODO: test metrics? */
					if run.Invalid != tt.want[i].Invalid ||
						run.Pass != tt.want[i].Pass ||
						(tt.want[i].Endpoint != "" && run.Endpoint != tt.want[i].Endpoint) ||
						(tt.want[i].Message != "" && run.Message != tt.want[i].Message) {
						t.Errorf("Test %s failed. Expected result is %v, but found %v", tt.name, tt.want, run)
					}
				}
			}
			if len(tt.want) > len(runs) {
				t.Errorf("Test %s failed. Expected %d results, but found %d ", tt.name, len(tt.want), len(runs))
				for i := len(runs); i <= len(tt.want)-1; i++ {
					t.Errorf("Did not find %s %v", tt.name, tt.want[i])
				}

			}
		})
	}
}

// a successful case
func TestPostgresCheckWithDbMock(t *testing.T) {

	// create a mock db
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	// This is the result we expect
	rows := sqlmock.NewRows([]string{"column"}).
		AddRow(1)

	// declare our expectation
	mock.ExpectQuery("^SELECT 1$").WillReturnRows(rows)

	config := pkg.ParseConfig("../fixtures/postgres_succeed.yaml")

	results := cmd.RunChecks(config)

	expectationErr := mock.ExpectationsWereMet()
	if expectationErr != nil {
		t.Errorf("Test %s failed. Expected queries not made: %v", "postgres_succeed", expectationErr)
	}

	for _, result := range results {
		if result.Invalid {
			t.Errorf("Test %s failed. Expected valid result, but found %v", "postgres_succeed", result.Invalid)
		}
		if !result.Pass {
			t.Errorf("Test %s failed. Expected PASS result, but found %v", "postgres_succeed", result.Pass)
		}

	}

}
