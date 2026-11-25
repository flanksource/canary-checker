package checks

import (
	"context"
	"net"
	"testing"

	v1 "github.com/flanksource/canary-checker/api/v1"
)

func TestSrvInfo(t *testing.T) {
	tests := []struct {
		query   string
		name    string
		wantErr bool
	}{
		{"_ldap._tcp.example.com", "example.com", false},
		{"_ldap._tcp.example.com.", "example.com.", false},
		{"_ldap._tcp.sub.example.com", "sub.example.com", false},
		{"_ldap._tcp.localhost", "localhost", false},
		{"_ldap._tcp", "", true},
		{"", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			service, proto, name, err := srvInfo(tt.query)
			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error state: %v", err)
			}
			if tt.wantErr {
				return
			}
			if service != "ldap" || proto != "tcp" || name != tt.name {
				t.Errorf("got (%q, %q, %q), want (%q, %q, %q)", service, proto, name, "ldap", "tcp", tt.name)
			}
		})
	}
}

func TestGetDialerIPv6Server(t *testing.T) {
	pc, err := net.ListenPacket("udp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback unavailable: %v", err)
	}
	defer pc.Close()
	port := pc.LocalAddr().(*net.UDPAddr).Port

	for _, server := range []string{"::1", "[::1]"} {
		t.Run(server, func(t *testing.T) {
			dial, err := getDialer(v1.DNSCheck{Server: server, Port: port}, 2)
			if err != nil {
				t.Fatal(err)
			}
			conn, err := dial(context.Background(), "udp", "ignored")
			if err != nil {
				t.Fatalf("dialing an IPv6 DNS server failed: %v", err)
			}
			defer conn.Close()
			if got := conn.RemoteAddr().(*net.UDPAddr).Port; got != port {
				t.Errorf("got port %d, want %d", got, port)
			}
		})
	}
}
