package checks

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	checkcontext "github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	dutycontext "github.com/flanksource/duty/context"
)

func TestDynatraceCheckOptionalFields(t *testing.T) {
	for _, tt := range []struct {
		name       string
		response   string
		wantLabels []map[string]string
	}{
		{
			name:       "tag without value",
			response:   `{"totalCount":1,"problems":[{"problemId":"p1","title":"t","entityTags":[{"key":"production"},{"key":"team","value":"sre"}]}]}`,
			wantLabels: []map[string]string{{"production": "", "team": "sre"}},
		},
		{
			name:     "no problems field",
			response: `{"totalCount":0}`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()

			ctx := checkcontext.New(dutycontext.New(), v1.Canary{})
			check := v1.DynatraceCheck{Scheme: "http", Host: strings.TrimPrefix(server.URL, "http://")}
			check.APIKey.ValueStatic = "token"
			results := (&DynatraceChecker{}).Check(ctx, check)
			if len(results) != 1 || !results[0].Pass {
				t.Fatalf("expected one successful result, got %+v", results)
			}

			var gotLabels []map[string]string
			details, _ := results[0].Detail.([]map[string]any)
			for _, detail := range details {
				labels, _ := detail["labels"].(map[string]string)
				gotLabels = append(gotLabels, labels)
			}
			if !reflect.DeepEqual(gotLabels, tt.wantLabels) {
				t.Fatalf("expected labels %v, got %v (detail %#v)", tt.wantLabels, gotLabels, results[0].Detail)
			}
		})
	}
}
