package checks

import (
	"strings"
	"testing"

	canaryContext "github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	dutyContext "github.com/flanksource/duty/context"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sTypes "k8s.io/apimachinery/pkg/types"
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
	disabled := true
	ctx := newRetryTestContext(&v1.CheckRetries{Interval: &interval, MaxRetries: &maxRetries})
	check := v1.HTTPCheck{Description: v1.Description{
		Name:         "no-retry",
		CheckRetries: &v1.CheckRetries{Disabled: &disabled},
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

func TestRunCheckWithRetriesCanReenablePerCheck(t *testing.T) {
	interval := v1.Duration("1ms")
	maxRetries := 2
	disabled := true
	enabled := false
	ctx := newRetryTestContext(&v1.CheckRetries{Interval: &interval, MaxRetries: &maxRetries, Disabled: &disabled})
	check := v1.HTTPCheck{Description: v1.Description{
		Name:         "retry-me",
		CheckRetries: &v1.CheckRetries{Disabled: &enabled},
	}}
	checker := &retryTestChecker{passOnAttempt: 2}

	results, _ := runCheckWithRetries(ctx, checker, check)
	if checker.attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", checker.attempts)
	}
	if len(results) != 1 || !results[0].Pass {
		t.Fatalf("expected final result to pass, got %#v", results)
	}
}

func TestRunCheckWithRetriesRejectsZeroIntervalWithTimeout(t *testing.T) {
	interval := v1.Duration("0s")
	timeout := v1.Duration("1s")
	ctx := newRetryTestContext(&v1.CheckRetries{Interval: &interval, Timeout: &timeout})
	check := v1.HTTPCheck{Description: v1.Description{Name: "zero-interval"}}
	checker := &retryTestChecker{passOnAttempt: 2}

	results, _ := runCheckWithRetries(ctx, checker, check)
	if checker.attempts != 0 {
		t.Fatalf("expected no attempts for invalid retry policy, got %d", checker.attempts)
	}
	if len(results) != 1 || !results[0].Invalid {
		t.Fatalf("expected invalid result, got %#v", results)
	}
	if !strings.Contains(results[0].Error, "interval must be greater than zero") {
		t.Fatalf("expected interval validation error, got %q", results[0].Error)
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
	disabled := true
	check := v1.KubernetesResourceCheck{
		Description: v1.Description{
			Name:         "kubernetes-resource",
			CheckRetries: &v1.CheckRetries{MaxRetries: &maxRetries, Disabled: &disabled},
		},
	}

	retries := check.GetCheckRetries()
	if retries == nil {
		t.Fatalf("expected retry config")
	}
	if retries.Disabled == nil || !*retries.Disabled {
		t.Fatalf("expected disabled flag to be returned")
	}
	if retries.MaxRetries == nil || *retries.MaxRetries != maxRetries {
		t.Fatalf("expected maxRetries=%d, got %#v", maxRetries, retries.MaxRetries)
	}
}

func TestKubernetesResourceValidateAllowsNilCheckRetries(t *testing.T) {
	ctx := newRetryTestContext(nil)
	if err := (&KubernetesResourceChecker{}).validate(*ctx, v1.KubernetesResourceCheck{}); err != nil {
		t.Fatalf("expected nil checkRetries to validate, got %v", err)
	}
}

func TestKubernetesResourceCheckRejectsInvalidMergedRetriesBeforeKubernetesAccess(t *testing.T) {
	interval := v1.Duration("not-a-duration")
	ctx := newRetryTestContext(&v1.CheckRetries{Interval: &interval})

	resource := unstructured.Unstructured{}
	resource.SetAPIVersion("v1")
	resource.SetKind("ConfigMap")
	resource.SetName("cm")
	resource.SetNamespace("default")

	check := v1.KubernetesResourceCheck{
		Description: v1.Description{Name: "kubernetes-resource"},
		Resources:   []unstructured.Unstructured{resource},
		WaitFor:     v1.KubernetesResourceCheckWaitFor{Disable: true},
	}

	results := (&KubernetesResourceChecker{}).Check(ctx, check)
	if len(results) != 1 {
		t.Fatalf("expected one result, got %#v", results)
	}
	if results[0].Pass {
		t.Fatalf("expected validation failure, got passing result: %#v", results[0])
	}
	if !strings.Contains(results[0].Error, "validation:") || !strings.Contains(results[0].Error, "checkRetries: interval") {
		t.Fatalf("expected merged retry validation error before kubernetes access, got %q", results[0].Error)
	}
}

func TestKubernetesResourceCheckDoesNotMutateOriginalSpec(t *testing.T) {
	ctx := newRetryTestContext(nil)
	ctx.Canary.Name = "canary"
	ctx.Canary.Namespace = "default"
	ctx.Canary.UID = k8sTypes.UID("canary-id")
	ctx.Canary.Status.Checks = map[string]string{"kubernetes-resource": "check-id"}

	resource := unstructured.Unstructured{}
	resource.SetAPIVersion("v1")
	resource.SetKind("ConfigMap")
	resource.SetName("cm")
	resource.SetNamespace("default")
	resource.SetLabels(map[string]string{"existing": "true"})

	check := v1.KubernetesResourceCheck{
		Description: v1.Description{Name: "kubernetes-resource"},
		Resources:   []unstructured.Unstructured{resource},
		WaitFor:     v1.KubernetesResourceCheckWaitFor{Disable: true},
	}

	_ = (&KubernetesResourceChecker{}).Check(ctx, check)

	labels := check.Resources[0].GetLabels()
	if len(labels) != 1 || labels["existing"] != "true" {
		t.Fatalf("expected original labels to remain unchanged, got %#v", labels)
	}
	if ownerRefs := check.Resources[0].GetOwnerReferences(); len(ownerRefs) != 0 {
		t.Fatalf("expected original owner references to remain unchanged, got %#v", ownerRefs)
	}
}

func TestAllCheckersImplementSingleCheckRunner(t *testing.T) {
	for _, checker := range All {
		if _, ok := checker.(SingleCheckRunner); !ok {
			t.Errorf("checker %s does not implement SingleCheckRunner", checker.Type())
		}
	}
}
