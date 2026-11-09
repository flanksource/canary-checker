package checks

import "testing"

func TestExtractAddrAndPort(t *testing.T) {
	tests := []struct {
		endpoint string
		addr     string
		port     string
		wantErr  bool
	}{
		{"example.com:443", "example.com", "443", false},
		{"127.0.0.1:80", "127.0.0.1", "80", false},
		{"[::1]:443", "::1", "443", false},
		{"[2001:db8::1]:8080", "2001:db8::1", "8080", false},
		{"example.com", "", "", true},
		{"::1:443", "", "", true},
		{"", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.endpoint, func(t *testing.T) {
			addr, port, err := extractAddrAndPort(tt.endpoint)
			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error state: %v", err)
			}
			if addr != tt.addr || port != tt.port {
				t.Errorf("got (%q, %q), want (%q, %q)", addr, port, tt.addr, tt.port)
			}
		})
	}
}
