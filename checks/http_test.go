package checks

import (
	"strings"
	"testing"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
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

