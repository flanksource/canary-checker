package checks

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	dutyCtx "github.com/flanksource/duty/context"
)

// TestLDAPCheckClosesConnection dials a local listener and rejects the bind
// before any LDAP response is required. The socket from DialURL must be closed
// before Check returns; otherwise the read on the accepted connection times out.
func TestLDAPCheckClosesConnection(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	accepted := make(chan net.Conn, 1)
	acceptErr := make(chan error, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			acceptErr <- err
			return
		}
		accepted <- c
	}()

	addr := ln.Addr().(*net.TCPAddr)
	ctx := &context.Context{
		Context:     dutyCtx.New(),
		Namespace:   "default",
		Canary:      v1.Canary{},
		Environment: map[string]interface{}{},
	}
	check := v1.LDAPCheck{
		Connection: v1.Connection{
			URL: fmt.Sprintf("ldap://127.0.0.1:%d", addr.Port),
		},
	}

	results := (&LdapChecker{}).Check(ctx, check)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Pass {
		t.Fatal("expected the check to fail")
	}
	if !strings.Contains(results[0].Error, "Failed to bind") {
		t.Fatalf("expected dial to succeed and bind to fail, got %q", results[0].Error)
	}

	var conn net.Conn
	select {
	case conn = <-accepted:
	case err := <-acceptErr:
		t.Fatalf("accept: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("LDAP dial never connected")
	}
	t.Cleanup(func() { _ = conn.Close() })

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var buf [1]byte
	_, err = conn.Read(buf[:])
	if err == nil {
		t.Fatal("LDAP client wrote to the socket and left it open")
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		t.Fatal("LDAP connection still open after the check returned")
	}
}
