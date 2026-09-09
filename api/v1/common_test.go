package v1

import (
	"encoding/json"
	"testing"
)

func TestDescriptionGetNamespace(t *testing.T) {
	for _, tt := range []struct {
		name      string
		namespace string
		want      string
	}{
		{name: "omitted"},
		{name: "empty object", namespace: `{}`},
		{name: "string", namespace: `"netpol-probe-a"`, want: "netpol-probe-a"},
		{name: "empty string", namespace: `""`},
		{name: "escaped string", namespace: `"netpol\u002dprobe-a"`, want: "netpol-probe-a"},
		{name: "legacy selector", namespace: `{"name":"netpol-probe-a"}`, want: "netpol-probe-a"},
		{name: "legacy selector with whitespace", namespace: " \n\t{\"name\":\"netpol-probe-a\"}", want: "netpol-probe-a"},
		{name: "invalid JSON", namespace: `"unterminated`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := Description{Namespace: json.RawMessage(tt.namespace)}
			if got := d.GetNamespace(); got != tt.want {
				t.Errorf("GetNamespace() = %q, want %q", got, tt.want)
			}
		})
	}
}
