package context

import (
	"testing"

	v1 "github.com/flanksource/canary-checker/api/v1"
	dutyCtx "github.com/flanksource/duty/context"
)

func TestGetConnectionTemplatesOutputs(t *testing.T) {
	ctx := &Context{
		Context:   dutyCtx.New(),
		Namespace: "default",
		Canary:    v1.Canary{},
		Environment: map[string]any{
			"outputs": map[string]any{
				"startPg": map[string]any{
					"results": map[string]any{
						"stdout": "5432",
					},
				},
			},
		},
	}

	conn := v1.Connection{
		URL: "postgres://postgres:postgres@localhost:$(outputs.startPg.results.stdout)/embedded?sslmode=disable",
	}

	result, err := ctx.GetConnection(conn)
	if err != nil {
		t.Fatalf("GetConnection() error: %v", err)
	}

	expected := "postgres://postgres:postgres@localhost:5432/embedded?sslmode=disable"
	if result.URL != expected {
		t.Errorf("got URL %q, want %q", result.URL, expected)
	}
}

func TestGetConnectionPreservesConnectionKeys(t *testing.T) {
	ctx := &Context{
		Context:   dutyCtx.New(),
		Namespace: "test-ns",
		Canary:    v1.Canary{},
		Environment: map[string]any{
			"namespace": "env-ns",
		},
	}

	conn := v1.Connection{
		URL: "http://$(namespace)/api",
	}

	result, err := ctx.GetConnection(conn)
	if err != nil {
		t.Fatalf("GetConnection() error: %v", err)
	}

	// Connection-specific "namespace" (test-ns) should take precedence over environment's
	if result.URL != "http://test-ns/api" {
		t.Errorf("got URL %q, want %q", result.URL, "http://test-ns/api")
	}
}
