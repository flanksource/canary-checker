package checks

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	checkcontext "github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	dutycontext "github.com/flanksource/duty/context"
)

func TestOpenSearchTotalHits(t *testing.T) {
	for _, tt := range []struct {
		name      string
		response  string
		wantError bool
	}{
		{"zero hits", `{"hits":{"total":{"value":0},"hits":[]}}`, false},
		{"missing total", `{"hits":{"hits":[]}}`, true},
		{"missing value", `{"hits":{"total":{}}}`, true},
		{"null value", `{"hits":{"total":{"value":null}}}`, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()

			ctx := checkcontext.New(dutycontext.New(), v1.Canary{})
			check := v1.OpenSearchCheck{
				Connection: v1.Connection{URL: server.URL},
				Query:      `{"query":{"match_all":{}}}`,
				Results:    0,
			}
			results := (&OpenSearchChecker{}).Check(ctx, check)
			if len(results) != 1 {
				t.Fatalf("got %d results, want 1", len(results))
			}
			result := results[0]
			if tt.wantError {
				if result.Pass || result.ErrorObject == nil || !strings.Contains(result.ErrorObject.Error(), "hits.total.value") {
					t.Fatalf("expected explicit missing total error, got %+v", result)
				}
			} else if !result.Pass || result.ErrorObject != nil {
				t.Fatalf("expected successful count comparison, got %+v", result)
			}
		})
	}
}
