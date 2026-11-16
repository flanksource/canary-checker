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

func TestElasticsearchTotalHits(t *testing.T) {
	for _, tt := range []struct {
		name      string
		response  string
		expected  int
		wantError bool
	}{
		{"one hit", `{"hits":{"total":{"value":1},"hits":[{}]}}`, 1, false},
		{"zero hits", `{"hits":{"total":{"value":0},"hits":[]}}`, 0, false},
		{"tracking disabled", `{"hits":{"hits":[{}]}}`, 0, true},
		{"null total", `{"hits":{"total":null,"hits":[]}}`, 0, true},
		{"missing hits", `{}`, 0, true},
		{"missing value", `{"hits":{"total":{}}}`, 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Elastic-Product", "Elasticsearch")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()
			ctx := checkcontext.New(dutycontext.New(), v1.Canary{})
			check := v1.ElasticsearchCheck{
				Connection: v1.Connection{URL: server.URL},
				Query:      `{"track_total_hits":false}`, Results: tt.expected,
			}
			results := (&ElasticsearchChecker{}).Check(ctx, check)
			if len(results) != 1 {
				t.Fatalf("got %d results, want 1", len(results))
			}
			if tt.wantError {
				if results[0].Pass || results[0].ErrorObject == nil || !strings.Contains(results[0].ErrorObject.Error(), "hits.total.value") {
					t.Fatalf("expected explicit missing total error, got %+v", results[0])
				}
			} else if !results[0].Pass || results[0].ErrorObject != nil {
				t.Fatalf("expected successful count comparison, got %+v", results[0])
			}
		})
	}
}
