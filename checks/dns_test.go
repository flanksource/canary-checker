package checks

import "testing"

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
