package checks

import (
	"reflect"
	"testing"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

func Test_measureTestSeverity(t *testing.T) {
	type args struct {
		duration  int
		threshold *v1.TestThreshold
	}
	tests := []struct {
		name string
		args args
		want pkg.Severity
	}{
		{
			name: "simple - critical",
			args: args{
				duration: 2000,
				threshold: &v1.TestThreshold{
					Critical: "duration > 1500",
				},
			},
			want: pkg.SeverityCritical,
		},
		{
			name: "simple - high",
			args: args{
				duration: 1200,
				threshold: &v1.TestThreshold{
					Critical: "duration > 1500",
					High:     "duration > 1000",
				},
			},
			want: pkg.SeverityHigh,
		},
		{
			name: "simple - medium",
			args: args{
				duration: 760,
				threshold: &v1.TestThreshold{
					Critical: "duration > 1500",
					High:     "duration > 1000",
					Medium:   "duration > 750",
					Low:      "duration > 500",
				},
			},
			want: pkg.SeverityMedium,
		},
		{
			name: "simple - low",
			args: args{
				duration: 600,
				threshold: &v1.TestThreshold{
					Critical: "duration > 1500",
					High:     "duration > 1000",
					Low:      "duration > 500",
				},
			},
			want: pkg.SeverityLow,
		},
		{
			name: "complex expression",
			args: args{
				duration: 2100,
				threshold: &v1.TestThreshold{
					Critical: "duration > 1500 && duration < 2000",
					High:     "duration > 1000 && duration < 1500",
					Low:      "duration > 500 && duration < 1000",
				},
			},
			want: pkg.SeverityInfo,
		},
		{
			name: "no threshold defined",
			args: args{
				duration: 600,
			},
			want: pkg.SeverityInfo,
		},
		{
			name: "no severity match",
			args: args{
				duration: 400,
				threshold: &v1.TestThreshold{
					Critical: "duration > 1500",
					High:     "duration > 1000",
					Low:      "duration > 500",
				},
			},
			want: pkg.SeverityInfo,
		},
		{
			name: "invalid expression",
			args: args{
				duration: 400,
				threshold: &v1.TestThreshold{
					Critical: "duration >>> 1500",
					High:     "duration > 1000",
					Low:      "duration > 500",
				},
			},
			want: pkg.SeverityInfo,
		},
		{
			name: "use of undefined var",
			args: args{
				duration: 400,
				threshold: &v1.TestThreshold{
					Critical: "Duration > 1500",
					High:     "Duration > 1000",
					Low:      "Duration > 500",
				},
			},
			want: pkg.SeverityInfo,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]any{"duration": tt.args.duration}
			ctx := context.Context{}
			if got := measureTestSeverity(ctx.New(data), tt.args.threshold); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("measureTestSeverity() = %v, want %v", got, tt.want)
			}
		})
	}
}
