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
		want []pkg.CheckResult // each config can result in multiple checks
	}{
		// {
		// 	name: "http_pass",
		// 	args: args{
		// 		discardConfigError(pkg.ParseConfig("../fixtures/http_pass.yaml")),
		// 	},
		// 	want: []pkg.CheckResult{
		// 		{
		// 			Pass:     true,
		// 			Invalid:  false,
		// 			Endpoint: "https://httpstat.us/200",
		// 			Metrics:  []pkg.Metric{},
		// 		},
		// 	},
		// },
		// {
		// 	name: "http_fail",
		// 	args: args{
		// 		discardConfigError(pkg.ParseConfig("../fixtures/http_fail.yaml")),
		// 	},
		// 	want: []pkg.CheckResult{
		// 		{
		// 			Pass:     false,
		// 			Invalid:  true,
		// 			Endpoint: "https://ttpstat.us/500",
		// 			Metrics:  []pkg.Metric{},
		// 		},
		// 		{
		// 			Pass:     false,
		// 			Invalid:  false,
		// 			Endpoint: "https://httpstat.us/500",
		// 			Metrics:  []pkg.Metric{},
		// 		},
		// 	},
		// },
		{
			name: "postgres_fail",
			args: args{
				discardConfigError(pkg.ParseConfig("../fixtures/postgres_fail.yaml")),
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
			checkResults := cmd.RunChecks(tt.args.config)

			for i, res := range checkResults {
				// check if this result is extra
				if i > len(tt.want)-1 {
					t.Errorf("Test %s failed. Found unexpected extra result is %v", tt.name, res)
				} else {
					/* Not checking durations we don't want equality*/
					if res.Invalid != tt.want[i].Invalid ||
						res.Pass != tt.want[i].Pass ||
						(tt.want[i].Endpoint != "" && res.Endpoint != tt.want[i].Endpoint) ||
						(tt.want[i].Message != "" && res.Message != tt.want[i].Message) {
						t.Errorf("Test %s failed. Expected result is %v, but found %v", tt.name, tt.want, res)
					}

				}
			}
			// check if we have more expected results than were found
			if len(tt.want) > len(checkResults) {
				t.Errorf("Test %s failed. Expected %d results, but found %d ", tt.name, len(tt.want), len(checkResults))
				for i := len(checkResults); i <= len(tt.want)-1; i++ {
					t.Errorf("Did not find %s %v", tt.name, tt.want[i])
				}

			}

		})
	}
}

// Test the connectivity with a mock DB
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

	config := discardConfigError(pkg.ParseConfig("../fixtures/postgres_succeed.yaml"))

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

// This test is validating a piece of test infrastructure
//
func TestPostgresConfigErrorDrop(t *testing.T) {
	//assignment helper
	c := func(s int) *int {
		return &s
	}
	var test = struct {
		description  string
		yamlFixture  string
		error        bool
		errorMessage string
		wantConfig   pkg.Config
	}{
		description:  "we can parse single value result for postgres",
		yamlFixture:  "../fixtures/postgres_yaml_single_result.yaml",
		error:        false,
		errorMessage: "",
		wantConfig: pkg.Config{
			Postgres: []pkg.Postgres{
				{
					pkg.PostgresCheck{
						Driver:     "someDriver",
						Connection: "someConnection",
						Query:      "someQuery",
						Result:     c(1),
					},
				},
			},
		},
	}

	gotConfig := discardConfigError(pkg.ParseConfig(test.yamlFixture))

	if !cmp.Equal(test.wantConfig, gotConfig, cmpopts.EquateEmpty()) {
		t.Errorf("Test '%s': want %v, got %v", test.description, test.wantConfig, gotConfig)
	}

}

func discardConfigError(c pkg.Config, e error) pkg.Config {
	return c
}
