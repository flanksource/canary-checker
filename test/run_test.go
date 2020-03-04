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
		want *pkg.CheckResult
	}{
		{
			name: "http_pass",
			args: args{
				pkg.ParseConfig("../fixtures/http_pass.yaml"),
			},
			want: &pkg.CheckResult{
				Pass:    true,
				Invalid: false,
				Metrics: []pkg.Metric{},
			},
		},
		{
			name: "http_fail",
			args: args{
				pkg.ParseConfig("../fixtures/http_fail.yaml"),
			},
			want: &pkg.CheckResult{
				Pass:    false,
				Invalid: true,
				Metrics: []pkg.Metric{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := cmd.RunChecks(tt.args.config)[0]
			if run.Invalid != tt.want.Invalid ||
				run.Pass != tt.want.Pass {
				t.Errorf("Test %s failed. Expected result is %v", tt.name, tt.want)
			}
		})
	}
}
