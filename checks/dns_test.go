package checks

import (
	"context"
	"net"
	"testing"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"golang.org/x/net/dns/dnsmessage"
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

func TestCheckAIPv4Only(t *testing.T) {
	for _, tt := range []struct {
		name     string
		ipv4     bool
		ipv6     bool
		wantPass bool
	}{
		{"ipv6-only", false, true, false},
		{"dual-stack", true, true, true},
		{"ipv4-only", true, false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := net.ListenPacket("udp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			done := make(chan struct{})
			t.Cleanup(func() { conn.Close(); <-done })
			go func() {
				defer close(done)
				buf := make([]byte, 4096)
				for {
					n, addr, err := conn.ReadFrom(buf)
					if err != nil {
						return
					}
					var request dnsmessage.Message
					if err := request.Unpack(buf[:n]); err != nil {
						t.Error(err)
						return
					}
					response := dnsmessage.Message{
						Header:    dnsmessage.Header{ID: request.ID, Response: true, Authoritative: true, RecursionAvailable: true},
						Questions: request.Questions,
					}
					for _, q := range request.Questions {
						header := dnsmessage.ResourceHeader{Name: q.Name, Class: dnsmessage.ClassINET, TTL: 60}
						if q.Type == dnsmessage.TypeA && tt.ipv4 {
							response.Answers = append(response.Answers, dnsmessage.Resource{Header: header, Body: &dnsmessage.AResource{A: [4]byte{192, 0, 2, 1}}})
						}
						if q.Type == dnsmessage.TypeAAAA && tt.ipv6 {
							response.Answers = append(response.Answers, dnsmessage.Resource{Header: header, Body: &dnsmessage.AAAAResource{AAAA: [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}}})
						}
					}
					data, err := response.Pack()
					if err != nil {
						t.Error(err)
						return
					}
					if _, err := conn.WriteTo(data, addr); err != nil {
						return
					}
				}
			}()
			resolver := &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, conn.LocalAddr().String())
			}}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			check := v1.DNSCheck{Query: tt.name + ".test.", QueryType: "A", MinRecords: 1}
			if tt.ipv4 {
				check.ExactReply = []string{"192.0.2.1"}
			}
			pass, message, err := checkA(ctx, resolver, check)
			if pass != tt.wantPass {
				t.Fatalf("pass=%v, want %v (message=%q, err=%v)", pass, tt.wantPass, message, err)
			}
			if tt.wantPass && err != nil {
				t.Fatal(err)
			}
		})
	}
}
