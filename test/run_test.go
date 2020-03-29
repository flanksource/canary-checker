package test

import (
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/flanksource/canary-checker/cmd"
	"github.com/flanksource/canary-checker/pkg"
)

type args struct {
	config pkg.Config
}

type test struct {
	name string
	args args
	want []pkg.CheckResult // each config can result in multiple checks
}

func TestRunChecks(t *testing.T) {
	tests := []test{
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
					Endpoint: "https://httpstat.us/500",
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
		{
			name: "dns_fail",
			args: args{
				pkg.ParseConfig("../fixtures/dns_fail.yaml"),
			},
			want: []pkg.CheckResult{
				{
					Pass:     false,
					Invalid:  false,
					Endpoint: "8.8.8.8:53",
					Metrics:  []pkg.Metric{},
					Message: "Check failed: A flanksource.com on 8.8.8.8. Got [34.65.228.161], expected [8.8.8.8]",
				},
				{
					Pass:     false,
					Invalid:  false,
					Endpoint: "8.8.8.8:53",
					Metrics:  []pkg.Metric{},
					Message: "Check failed: PTR 8.8.8.8 on 8.8.8.8. Records count is less then minrecords",
				},
				{
					Pass:     false,
					Invalid:  false,
					Endpoint: "8.8.8.8:53",
					Metrics:  []pkg.Metric{},
					Message: "Check failed: CNAME dns.google on 8.8.8.8. Got [dns.google.], expected [wrong.google.]",
				},
				{
					Pass:     false,
					Invalid:  false,
					Endpoint: "8.8.8.8:53",
					Metrics:  []pkg.Metric{},
					Message: "Check failed: MX flanksource.com on 8.8.8.8. Got [alt1.aspmx.l.google.com. 5 alt2.aspmx.l.google.com. 5 aspmx.l.google.com. 1 aspmx2.googlemail.com. 10 aspmx3.googlemail.com. 10], expected [alt1.aspmx.l.google.com. 5 alt2.aspmx.l.google.com. 5 aspmx.l.google.com. 1]",
				},
				{
					Pass:     false,
					Invalid:  false,
					Endpoint: "8.8.8.8:53",
					Metrics:  []pkg.Metric{},
					Message: "Check failed: TXT flanksource.com on 8.8.8.8. Records count is less then minrecords",
				},
				{
					Pass:     false,
					Invalid:  false,
					Endpoint: "8.8.8.8:53",
					Metrics:  []pkg.Metric{},
					Message: "Check failed: NS flanksource.com on 8.8.8.8. Got [ns-1450.awsdns-53.org. ns-1896.awsdns-45.co.uk. ns-908.awsdns-49.net. ns-91.awsdns-11.com.], expected [ns-91.awsdns-11.com.]",
				},
			},
		},
		{
			name: "dns_pass",
			args: args{
				pkg.ParseConfig("../fixtures/dns_pass.yaml"),
			},
			want: []pkg.CheckResult{
				{
					Pass:     true,
					Invalid:  false,
					Endpoint: "8.8.8.8:53",
					Metrics:  []pkg.Metric{},
					Message: "Successful check on 8.8.8.8. Got [34.65.228.161]",
				},
				{
					Pass:     true,
					Invalid:  false,
					Endpoint: "8.8.8.8:53",
					Metrics:  []pkg.Metric{},
					Message: "Successful check on 8.8.8.8. Got [dns.google.]",
				},
				{
					Pass:     true,
					Invalid:  false,
					Endpoint: "8.8.8.8:53",
					Metrics:  []pkg.Metric{},
					Message: "Successful check on 8.8.8.8. Got [dns.google.]",
				},
				{
					Pass:     true,
					Invalid:  false,
					Endpoint: "8.8.8.8:53",
					Metrics:  []pkg.Metric{},
					Message: "Successful check on 8.8.8.8. Got [alt1.aspmx.l.google.com. 5 alt2.aspmx.l.google.com. 5 aspmx.l.google.com. 1 aspmx2.googlemail.com. 10 aspmx3.googlemail.com. 10]",
				},
				{
					Pass:     true,
					Invalid:  false,
					Endpoint: "8.8.8.8:53",
					Metrics:  []pkg.Metric{},
					Message: "Successful check on 8.8.8.8. Got [google-site-verification=IIE1aJuvqseLUKSXSIhu2O2lgdU_d8csfJjjIQVc-q0]",
				},
				{
					Pass:     true,
					Invalid:  false,
					Endpoint: "8.8.8.8:53",
					Metrics:  []pkg.Metric{},
					Message: "Successful check on 8.8.8.8. Got [ns-1450.awsdns-53.org. ns-1896.awsdns-45.co.uk. ns-908.awsdns-49.net. ns-91.awsdns-11.com.]",
				},
			},
		},
	}
	runTests(t, tests)
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

	config := pkg.ParseConfig("../fixtures/postgres_succeed.yaml")

	results := cmd.RunChecks(config)
	foundResults := make([]*pkg.CheckResult, 0)
	for result := range results {
		foundResults = append(foundResults, result)
	}

	expectationErr := mock.ExpectationsWereMet()
	if expectationErr != nil {
		t.Errorf("Test %s failed. Expected queries not made: %v", "postgres_succeed", expectationErr)
	}

	for _, result := range foundResults {
		if result.Invalid {
			t.Errorf("Test %s failed. Expected valid result, but found %v", "postgres_succeed", result.Invalid)
		}
		if !result.Pass {
			t.Errorf("Test %s failed. Expected PASS result, but found %v", "postgres_succeed", result.Pass)
		}
	}
}

func runTests(t *testing.T, tests []test) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkResults := cmd.RunChecks(tt.args.config)

			i := 0

			foundResults := make([]*pkg.CheckResult, 0)

			for res := range checkResults {
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
				foundResults = append(foundResults, res)
				i++
			}
			// check if we have more expected results than were found
			if len(tt.want) > len(foundResults) {
				t.Errorf("Test %s failed. Expected %d results, but found %d ", tt.name, len(tt.want), len(foundResults))
				for i := len(foundResults); i <= len(tt.want)-1; i++ {
					t.Errorf("Did not find %s %v", tt.name, tt.want[i])
				}
			}
		})
	}
}
