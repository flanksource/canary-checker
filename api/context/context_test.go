package context

import (
	"testing"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
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

func TestWithCheckResultExposesResultFields(t *testing.T) {
	check := v1.FolderCheck{Description: v1.Description{Name: "folder"}}
	canary := v1.Canary{}
	result := pkg.Success(check, canary)
	result.Duration = 123
	result.Message = "display message"
	result.Failf("failure reason")

	ctx := New(dutyCtx.New(), canary).WithCheckResult(result)

	want := map[string]any{
		"duration": int64(123),
		"error":    "failure reason",
		"message":  "display message",
		"pass":     false,
	}
	for key, expected := range want {
		if actual := ctx.Environment[key]; actual != expected {
			t.Errorf("Environment[%q] = %#v, want %#v", key, actual, expected)
		}
	}
}

func TestWithCheckResultDataTakesPrecedence(t *testing.T) {
	check := v1.FolderCheck{Description: v1.Description{Name: "folder"}}
	canary := v1.Canary{}
	data := map[string]any{
		"error":   "data error",
		"message": "data message",
		"pass":    "data pass",
	}
	result := pkg.Success(check, canary).AddData(data)

	ctx := New(dutyCtx.New(), canary).WithCheckResult(result)

	for key, expected := range data {
		if actual := ctx.Environment[key]; actual != expected {
			t.Errorf("Environment[%q] = %#v, want data value %#v", key, actual, expected)
		}
	}
}
