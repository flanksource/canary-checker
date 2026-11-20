package pkg

import "testing"

func TestTruncateStripsANSI(t *testing.T) {
	in := "\x1b[31mERROR\x1b[0m: \x1b[1;33mfailed\x1b[0m\x1b[K to run"
	want := "ERROR: failed to run"
	if got := TruncateError(in); got != want {
		t.Errorf("TruncateError() = %q, want %q", got, want)
	}
	if got := TruncateMessage(in); got != want {
		t.Errorf("TruncateMessage() = %q, want %q", got, want)
	}
}
