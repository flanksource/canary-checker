package checks

import (
	"testing"

	v1 "github.com/flanksource/canary-checker/api/v1"
)

func TestSortChecksByDependency(t *testing.T) {
	tests := []struct {
		name        string
		checks      []v1.HTTPCheck
		wantOrder   []string // expected order of check names
		wantErr     bool
		errContains string
	}{
		{
			name: "no dependencies - preserves order",
			checks: []v1.HTTPCheck{
				{Description: v1.Description{Name: "a"}},
				{Description: v1.Description{Name: "b"}},
				{Description: v1.Description{Name: "c"}},
			},
			wantOrder: []string{"a", "b", "c"},
			wantErr:   false,
		},
		{
			name: "simple chain - a -> b -> c",
			checks: []v1.HTTPCheck{
				{Description: v1.Description{Name: "c"}, DependsOn: []string{"b"}},
				{Description: v1.Description{Name: "a"}},
				{Description: v1.Description{Name: "b"}, DependsOn: []string{"a"}},
			},
			wantOrder: []string{"a", "b", "c"},
			wantErr:   false,
		},
		{
			name: "diamond dependency - d depends on b and c, both depend on a",
			checks: []v1.HTTPCheck{
				{Description: v1.Description{Name: "d"}, DependsOn: []string{"b", "c"}},
				{Description: v1.Description{Name: "b"}, DependsOn: []string{"a"}},
				{Description: v1.Description{Name: "c"}, DependsOn: []string{"a"}},
				{Description: v1.Description{Name: "a"}},
			},
			// a must be first, then b and c (order doesn't matter), then d
			wantOrder: []string{"a", "b", "c", "d"},
			wantErr:   false,
		},
		{
			name: "missing dependency",
			checks: []v1.HTTPCheck{
				{Description: v1.Description{Name: "a"}, DependsOn: []string{"nonexistent"}},
			},
			wantErr:     true,
			errContains: "non-existent check 'nonexistent'",
		},
		{
			name: "circular dependency - a -> b -> a",
			checks: []v1.HTTPCheck{
				{Description: v1.Description{Name: "a"}, DependsOn: []string{"b"}},
				{Description: v1.Description{Name: "b"}, DependsOn: []string{"a"}},
			},
			wantErr:     true,
			errContains: "circular dependency",
		},
		{
			name: "unnamed checks run first",
			checks: []v1.HTTPCheck{
				{Description: v1.Description{Name: "b"}, DependsOn: []string{"a"}},
				{Description: v1.Description{Name: ""}}, // unnamed
				{Description: v1.Description{Name: "a"}},
			},
			wantOrder: []string{"", "a", "b"}, // unnamed first
			wantErr:   false,
		},
		{
			name: "mixed named and unnamed",
			checks: []v1.HTTPCheck{
				{Description: v1.Description{Name: ""}},          // unnamed 1
				{Description: v1.Description{Name: "api-check"}}, // named, no deps
				{Description: v1.Description{Name: ""}},          // unnamed 2
			},
			wantOrder: []string{"", "", "api-check"},
			wantErr:   false,
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
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
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
				gotOrder[i] = check.Name
			}

			// For diamond dependency, we need to be flexible about b/c order
			if tt.name == "diamond dependency - d depends on b and c, both depend on a" {
				if gotOrder[0] != "a" || gotOrder[3] != "d" {
					t.Errorf("expected first='a' and last='d', got first=%q last=%q", gotOrder[0], gotOrder[3])
				}
				// b and c can be in any order as long as they're in positions 1 and 2
				if !((gotOrder[1] == "b" && gotOrder[2] == "c") || (gotOrder[1] == "c" && gotOrder[2] == "b")) {
					t.Errorf("expected b and c in positions 1 and 2, got %v", gotOrder)
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

func TestExtractValue(t *testing.T) {
	tests := []struct {
		name string
		data map[string]interface{}
		path string
		want interface{}
	}{
		{
			name: "simple key",
			data: map[string]interface{}{"token": "abc123"},
			path: "token",
			want: "abc123",
		},
		{
			name: "nested path",
			data: map[string]interface{}{
				"json": map[string]interface{}{
					"access_token": "secret",
				},
			},
			path: "json.access_token",
			want: "secret",
		},
		{
			name: "deeply nested",
			data: map[string]interface{}{
				"json": map[string]interface{}{
					"data": map[string]interface{}{
						"user": map[string]interface{}{
							"id": "12345",
						},
					},
				},
			},
			path: "json.data.user.id",
			want: "12345",
		},
		{
			name: "missing key",
			data: map[string]interface{}{"token": "abc123"},
			path: "nonexistent",
			want: nil,
		},
		{
			name: "missing nested key",
			data: map[string]interface{}{
				"json": map[string]interface{}{},
			},
			path: "json.missing.path",
			want: nil,
		},
		{
			name: "integer value",
			data: map[string]interface{}{"code": 200},
			path: "code",
			want: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractValue(tt.data, tt.path)
			if got != tt.want {
				t.Errorf("extractValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
