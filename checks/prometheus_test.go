package checks

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	checkcontext "github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	dutycontext "github.com/flanksource/duty/context"
)

func TestPrometheusCheckQueryResult(t *testing.T) {
	for _, tt := range []struct {
		name        string
		query       string
		data        string
		wantResults []map[string]interface{}
	}{
		{
			name:        "scalar",
			query:       "scalar(vector(0))",
			data:        `{"resultType":"scalar","result":[1700000000,"0"]}`,
			wantResults: []map[string]interface{}{{"value": float64(0)}},
		},
		{
			name:  "vector",
			query: "up",
			data:  `{"resultType":"vector","result":[{"metric":{"job":"api"},"value":[1700000000,"1"]},{"metric":{"job":"worker"},"value":[1700000000,"0"]}]}`,
			wantResults: []map[string]interface{}{
				{"job": "api", "value": float64(1)},
				{"job": "worker", "value": float64(0)},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/query" || r.FormValue("query") != tt.query {
					t.Errorf("unexpected Prometheus request: %s %s query=%q", r.Method, r.URL.Path, r.FormValue("query"))
					http.Error(w, "unexpected query", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"status":"success","data":` + tt.data + `}`))
			}))
			defer server.Close()

			ctx := checkcontext.New(dutycontext.New(), v1.Canary{})
			check := v1.PrometheusCheck{Query: tt.query}
			check.URL = server.URL
			results := (&PrometheusChecker{}).Check(ctx, check)
			if len(results) != 1 || !results[0].Pass {
				t.Fatalf("expected one successful result, got %+v", results)
			}
			wantData := map[string]interface{}{
				"value":       tt.wantResults[0]["value"],
				"firstResult": tt.wantResults[0],
				"results":     tt.wantResults,
			}
			if got := results[0].Data; !reflect.DeepEqual(got, wantData) {
				t.Errorf("result data = %#v, want %#v", got, wantData)
			}
		})
	}
}
