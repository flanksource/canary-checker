package checks

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	dutyCtx "github.com/flanksource/duty/context"
)

func TestAlertManagerCheckNon2xxMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("Unauthorized: invalid token"))
	}))
	defer server.Close()

	ctx := &context.Context{
		Context:   dutyCtx.New(),
		Namespace: "default",
		Canary:    v1.Canary{},
	}
	check := v1.AlertManagerCheck{
		Connection: v1.Connection{URL: server.URL},
	}

	results := (&AlertManagerChecker{}).Check(ctx, check)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	result := results[0]
	if result.Pass {
		t.Fatalf("expected the check to fail on a 401 response")
	}
	if strings.Contains(result.Error, "<nil>") {
		t.Errorf("error should not contain <nil>, got %q", result.Error)
	}
	for _, want := range []string{"401", "Unauthorized: invalid token"} {
		if !strings.Contains(result.Error, want) {
			t.Errorf("expected error to contain %q, got %q", want, result.Error)
		}
	}
}
