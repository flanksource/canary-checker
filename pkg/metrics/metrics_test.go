package metrics

import (
	"sync"
	"testing"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	dutycontext "github.com/flanksource/duty/context"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

var setupOnce sync.Once

func ensureMetricsSetup() {
	setupOnce.Do(func() {
		SetupMetrics()
	})
}

// checkMetricLabelsFor mirrors the label slice Record() builds for the
// check-scoped counters (OpsCount, OpsFailedCount, OpsSuccessCount,
// OpsErrorCount, CanaryCheckInfo) when no additional metric labels are
// configured.
func checkMetricLabelsFor(canary v1.Canary, check pkg.GenericCheck) []string {
	return []string{
		check.GetType(),
		canary.GetDescription(check),
		canary.Name,
		canary.Namespace,
		canary.Spec.Owner,
		canary.Spec.Severity,
		canary.GetCheckID(check.GetName()),
		check.GetName(),
	}
}

func newFixture(checkName, checkKey string) (v1.Canary, pkg.GenericCheck) {
	canary := v1.Canary{}
	canary.Name = "test-canary"
	canary.Namespace = "test-ns"
	canary.Status.Checks = map[string]string{checkName: checkKey}

	check := pkg.GenericCheck{
		Type:     "http",
		Endpoint: "http://example.com",
	}
	check.Name = checkName
	return canary, check
}

func resultFor(canary v1.Canary, check pkg.GenericCheck, pass, invalid, internalErr bool) *pkg.CheckResult {
	return &pkg.CheckResult{
		Pass:          pass,
		Invalid:       invalid,
		InternalError: internalErr,
		Duration:      10,
		Check:         check,
		Canary:        canary,
	}
}

// TestRecord_FailedCountNotDoubleRecorded asserts that a single check run
// increments the outcome counters exactly once and appends to the rolling
// uptime window exactly once.
//
// It also pins the uptime-ratio semantics: only genuine failures append to the
// fail window and only passes append to the pass window. Invalid and
// internal-error results drop out of the ratio entirely (neither pass nor
// fail), so the in-memory Uptime1H agrees with the Prometheus uptime, which is
// computed from canary_check_failed_count + canary_check_success_count only.
//
// Regression test for the double-count bug where canary_check_failed_count was
// incremented twice (and fail.Append called twice) for a normal failure.
func TestRecord_FailedCountNotDoubleRecorded(t *testing.T) {
	ensureMetricsSetup()
	ctx := dutycontext.New()

	cases := []struct {
		name             string
		pass             bool
		invalid          bool
		internalErr      bool
		wantFailedDelta  float64
		wantInvalidDelta float64
		wantErrorDelta   float64
		wantSuccessDelta float64
		wantUptimeFailed int
		wantUptimePassed int
	}{
		{
			name:             "normal failure",
			pass:             false,
			wantFailedDelta:  1,
			wantUptimeFailed: 1,
		},
		{
			name:             "invalid failure",
			pass:             false,
			invalid:          true,
			wantInvalidDelta: 1,
			// invalid results drop out of the uptime ratio: neither pass nor fail.
			wantUptimeFailed: 0,
			wantUptimePassed: 0,
		},
		{
			name:           "internal error failure",
			pass:           false,
			internalErr:    true,
			wantErrorDelta: 1,
			// internal errors drop out of the uptime ratio: neither pass nor fail.
			wantUptimeFailed: 0,
			wantUptimePassed: 0,
		},
		{
			name:             "pass",
			pass:             true,
			wantSuccessDelta: 1,
			wantUptimePassed: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			canary, check := newFixture(tc.name, tc.name)
			labels := checkMetricLabelsFor(canary, check)

			// The failed/passed rolling windows are global and keyed by check
			// key, so reset this case's key after it runs. Otherwise uptime
			// counts accumulate across `go test -count=N` runs.
			t.Cleanup(func() { RemoveCheckByKey(canary.GetCheckID(check.GetName())) })

			beforeFailed := testutil.ToFloat64(OpsFailedCount.WithLabelValues(labels...))
			beforeInvalid := testutil.ToFloat64(OpsInvalidCount.WithLabelValues(labels...))
			beforeError := testutil.ToFloat64(OpsErrorCount.WithLabelValues(labels...))
			beforeSuccess := testutil.ToFloat64(OpsSuccessCount.WithLabelValues(labels...))

			uptime, _ := Record(ctx, canary, resultFor(canary, check, tc.pass, tc.invalid, tc.internalErr))

			if got := testutil.ToFloat64(OpsFailedCount.WithLabelValues(labels...)) - beforeFailed; got != tc.wantFailedDelta {
				t.Errorf("canary_check_failed_count delta = %v, want %v", got, tc.wantFailedDelta)
			}
			if got := testutil.ToFloat64(OpsInvalidCount.WithLabelValues(labels...)) - beforeInvalid; got != tc.wantInvalidDelta {
				t.Errorf("canary_check_invalid_count delta = %v, want %v", got, tc.wantInvalidDelta)
			}
			if got := testutil.ToFloat64(OpsErrorCount.WithLabelValues(labels...)) - beforeError; got != tc.wantErrorDelta {
				t.Errorf("canary_check_error_count delta = %v, want %v", got, tc.wantErrorDelta)
			}
			if got := testutil.ToFloat64(OpsSuccessCount.WithLabelValues(labels...)) - beforeSuccess; got != tc.wantSuccessDelta {
				t.Errorf("canary_check_success_count delta = %v, want %v", got, tc.wantSuccessDelta)
			}

			if uptime.Failed != tc.wantUptimeFailed {
				t.Errorf("uptime.Failed = %v, want %v", uptime.Failed, tc.wantUptimeFailed)
			}
			if uptime.Passed != tc.wantUptimePassed {
				t.Errorf("uptime.Passed = %v, want %v", uptime.Passed, tc.wantUptimePassed)
			}
		})
	}
}
