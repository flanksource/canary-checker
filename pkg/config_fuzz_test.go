package pkg

import (
	"os"
	"testing"
)

func FuzzParseConfig(f *testing.F) {
	for _, seed := range []string{
		`apiVersion: canaries.flanksource.com/v1
kind: Canary
metadata:
  name: http-checks
spec:
  schedule: "@every 5m"
  http:
    - name: http-pass
      url: https://example.com
      responseCodes: [200, 204]
`,
		`schedule: "@every 1m"
http:
  - name: spec-only
    url: https://example.com/health
    responseContent: ok
`,
		`apiVersion: canaries.flanksource.com/v1
kind: Canary
metadata:
  name: multi-doc-one
spec:
  exec:
    - name: echo
      script: echo ok
---
apiVersion: canaries.flanksource.com/v1
kind: Canary
metadata:
  name: multi-doc-two
spec:
  webhook:
    transform:
      expr: 'true'
`,
		`http:
  - name: templated-body
    url: https://example.com
    body: '{{ .payload }}'
    templateBody: true
`,
		`---
not: [valid
`,
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// Keep fuzz inputs bounded so parser/resource regressions are attributable and CI-friendly.
		if len(data) > 1<<20 {
			return
		}

		configFile := t.TempDir() + "/canary.yaml"
		if err := os.WriteFile(configFile, data, 0o600); err != nil {
			t.Fatalf("write fuzz config: %v", err)
		}

		_, _ = ParseConfig(configFile, "")
	})
}
