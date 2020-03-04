package test

import (
	"testing"

	"github.com/flanksource/canary-checker/cmd"
	"github.com/flanksource/canary-checker/pkg"
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
						run.Endpoint != tt.want[i].Endpoint ||
						run.Message != tt.want[i].Message {
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
