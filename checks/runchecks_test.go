package checks

import (
	"strings"
	"testing"

	canaryContext "github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	dutyContext "github.com/flanksource/duty/context"
)

func TestSortChecksByDependency(t *testing.T) {
	tests := []struct {
		name        string
		checks      []external.Check
		wantOrder   []string
		wantErr     bool
		errContains string
	}{
		{
			name: "no dependencies - all checks returned",
			checks: []external.Check{
				v1.HTTPCheck{Description: v1.Description{Name: "a"}},
				v1.HTTPCheck{Description: v1.Description{Name: "b"}},
				v1.HTTPCheck{Description: v1.Description{Name: "c"}},
			},
			wantOrder: []string{"a", "b", "c"}, // order may vary, test just checks all are present
			wantErr:   false,
		},
		{
			name: "simple chain - a -> b -> c",
			checks: []external.Check{
				v1.HTTPCheck{Description: v1.Description{Name: "c", DependsOn: []string{"b"}}},
				v1.HTTPCheck{Description: v1.Description{Name: "a"}},
				v1.HTTPCheck{Description: v1.Description{Name: "b", DependsOn: []string{"a"}}},
			},
			wantOrder: []string{"a", "b", "c"},
			wantErr:   false,
		},
		{
			name: "diamond dependency - d depends on b and c, both depend on a",
			checks: []external.Check{
				v1.HTTPCheck{Description: v1.Description{Name: "d", DependsOn: []string{"b", "c"}}},
				v1.HTTPCheck{Description: v1.Description{Name: "b", DependsOn: []string{"a"}}},
				v1.HTTPCheck{Description: v1.Description{Name: "c", DependsOn: []string{"a"}}},
				v1.HTTPCheck{Description: v1.Description{Name: "a"}},
			},
			wantOrder: []string{"a", "b", "c", "d"},
			wantErr:   false,
		},
		{
			name: "missing dependency",
			checks: []external.Check{
				v1.HTTPCheck{Description: v1.Description{Name: "a", DependsOn: []string{"nonexistent"}}},
			},
			wantErr:     true,
			errContains: "non-existent check 'nonexistent'",
		},
		{
			name: "circular dependency - a -> b -> a",
			checks: []external.Check{
				v1.HTTPCheck{Description: v1.Description{Name: "a", DependsOn: []string{"b"}}},
				v1.HTTPCheck{Description: v1.Description{Name: "b", DependsOn: []string{"a"}}},
			},
			wantErr:     true,
			errContains: "circular dependency",
		},
		{
			name: "unnamed checks run first",
			checks: []external.Check{
				v1.HTTPCheck{Description: v1.Description{Name: "b", DependsOn: []string{"a"}}},
				v1.HTTPCheck{Description: v1.Description{Name: ""}},
				v1.HTTPCheck{Description: v1.Description{Name: "a"}},
			},
			wantOrder: []string{"", "a", "b"},
			wantErr:   false,
		},
		{
			name: "unnamed check with dependsOn should error",
			checks: []external.Check{
				v1.HTTPCheck{Description: v1.Description{Name: "", DependsOn: []string{"a"}}},
				v1.HTTPCheck{Description: v1.Description{Name: "a"}},
			},
			wantErr:     true,
			errContains: "must have a name",
		},
		{
			name: "duplicate check names should error",
			checks: []external.Check{
				v1.HTTPCheck{Description: v1.Description{Name: "a"}},
				v1.HTTPCheck{Description: v1.Description{Name: "a"}},
				v1.HTTPCheck{Description: v1.Description{Name: "b", DependsOn: []string{"a"}}},
			},
			wantErr:     true,
			errContains: "duplicate check name 'a'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sorted, err := sortChecksByDependency(tt.checks)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errContains)
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(sorted) != len(tt.wantOrder) {
				t.Errorf("expected %d checks, got %d", len(tt.wantOrder), len(sorted))
				return
			}

			gotOrder := make([]string, len(sorted))
			for i, check := range sorted {
				gotOrder[i] = check.GetName()
			}

			if tt.name == "diamond dependency - d depends on b and c, both depend on a" {
				if gotOrder[0] != "a" || gotOrder[3] != "d" {
					t.Errorf("expected first='a' and last='d', got first=%q last=%q", gotOrder[0], gotOrder[3])
				}
				if !((gotOrder[1] == "b" && gotOrder[2] == "c") || (gotOrder[1] == "c" && gotOrder[2] == "b")) {
					t.Errorf("expected b and c in positions 1 and 2, got %v", gotOrder)
				}
				return
			}

			if tt.name == "no dependencies - all checks returned" {
				gotSet := make(map[string]bool)
				for _, name := range gotOrder {
					gotSet[name] = true
				}
				for _, want := range tt.wantOrder {
					if !gotSet[want] {
						t.Errorf("expected check %q to be present, got %v", want, gotOrder)
						return
					}
				}
				return
			}

			for i, want := range tt.wantOrder {
				if gotOrder[i] != want {
					t.Errorf("position %d: expected %q, got %q (full order: %v)", i, want, gotOrder[i], gotOrder)
					return
				}
			}
		})
	}
}

type retryTestChecker struct {
	attempts      int
	passOnAttempt int
}

type internalRetryTestChecker struct {
	retryTestChecker
}

func (c *internalRetryTestChecker) HandlesRetriesInternally() bool { return true }

func (c *retryTestChecker) Type() string { return "http" }
func (c *retryTestChecker) Run(ctx *canaryContext.Context) pkg.Results {
	return nil
}
func (c *retryTestChecker) Check(ctx *canaryContext.Context, check external.Check) pkg.Results {
	c.attempts++
	result := pkg.Success(check, ctx.Canary)
	if c.passOnAttempt == 0 || c.attempts < c.passOnAttempt {
		result.Failf("attempt %d failed", c.attempts)
	}
	return pkg.Results{result}
}

func newRetryTestContext(checkRetries *v1.CheckRetries) *canaryContext.Context {
	return &canaryContext.Context{
		Context:     dutyContext.New(),
		Namespace:   "default",
		Environment: map[string]interface{}{},
		Outputs:     map[string]*pkg.CheckResult{},
		Canary: v1.Canary{
			Spec: v1.CanarySpec{
				CheckRetries: checkRetries,
			},
		},
	}
}

func TestRunCheckWithRetriesRetriesUntilSuccess(t *testing.T) {
	interval := v1.Duration("1ms")
	maxRetries := 2
	ctx := newRetryTestContext(&v1.CheckRetries{Interval: &interval, MaxRetries: &maxRetries})
	check := v1.HTTPCheck{Description: v1.Description{Name: "retry-me"}}
	checker := &retryTestChecker{passOnAttempt: 2}

	results, skipped := runCheckWithRetries(ctx, checker, check)
	if skipped != 0 {
		t.Fatalf("expected no skipped results, got %d", skipped)
	}
	if checker.attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", checker.attempts)
	}
	if len(results) != 1 || !results[0].Pass {
		t.Fatalf("expected final result to pass, got %#v", results)
	}

	retryData, ok := results[0].Data["checkRetries"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected checkRetries data, got %#v", results[0].Data["checkRetries"])
	}
	if retryData["attempts"] != 2 || retryData["retries"] != 1 {
		t.Fatalf("unexpected retry data: %#v", retryData)
	}
}

func TestRunCheckWithRetriesDisabledPerCheck(t *testing.T) {
	interval := v1.Duration("1ms")
	maxRetries := 2
	ctx := newRetryTestContext(&v1.CheckRetries{Interval: &interval, MaxRetries: &maxRetries})
	check := v1.HTTPCheck{Description: v1.Description{
		Name:         "no-retry",
		CheckRetries: &v1.CheckRetries{Disabled: true},
	}}
	checker := &retryTestChecker{passOnAttempt: 2}

	results, _ := runCheckWithRetries(ctx, checker, check)
	if checker.attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", checker.attempts)
	}
	if len(results) != 1 || results[0].Pass {
		t.Fatalf("expected final result to fail, got %#v", results)
	}
}

func TestInternalRetryHandlerSkipsOuterRetry(t *testing.T) {
	interval := v1.Duration("1ms")
	maxRetries := 2
	ctx := newRetryTestContext(&v1.CheckRetries{Interval: &interval, MaxRetries: &maxRetries})
	check := v1.HTTPCheck{Description: v1.Description{Name: "internal-retry"}}
	checker := &internalRetryTestChecker{retryTestChecker: retryTestChecker{passOnAttempt: 2}}

	results, _ := runCheckWithRetries(ctx, checker, check)
	if checker.attempts != 1 {
		t.Fatalf("expected outer retry wrapper to skip internal retry handler, got %d attempts", checker.attempts)
	}
	if len(results) != 1 || results[0].Pass {
		t.Fatalf("expected single failed result, got %#v", results)
	}
}

func TestKubernetesResourceCheckRetriesUseDescriptionField(t *testing.T) {
	maxRetries := 2
	check := v1.KubernetesResourceCheck{
		Description: v1.Description{
			Name:         "kubernetes-resource",
			CheckRetries: &v1.CheckRetries{MaxRetries: &maxRetries, Disabled: true},
		},
	}

	retries := check.GetCheckRetries()
	if retries == nil {
		t.Fatalf("expected retry config")
	}
	if !retries.Disabled {
		t.Fatalf("expected disabled flag to be returned")
	}
	if retries.MaxRetries == nil || *retries.MaxRetries != maxRetries {
		t.Fatalf("expected maxRetries=%d, got %#v", maxRetries, retries.MaxRetries)
	}
}

func TestAllCheckersImplementSingleCheckRunner(t *testing.T) {
	for _, checker := range All {
		if _, ok := checker.(SingleCheckRunner); !ok {
			t.Errorf("checker %s does not implement SingleCheckRunner", checker.Type())
		}
	}
}
