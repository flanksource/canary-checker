package checks

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"testing"

	"github.com/mdelapenya/tlscert"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	checkContext "github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	dutyCtx "github.com/flanksource/duty/context"
	"github.com/flanksource/duty/types"
)

func TestRedisChecker_Ping(t *testing.T) {
	ctx := context.Background()

	redisContainer, err := tcredis.Run(ctx, "redis:7-alpine")
	if err != nil {
		t.Fatalf("failed to start redis container: %v", err)
	}
	t.Cleanup(func() {
		if err := redisContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate redis container: %v", err)
		}
	})

	host, err := redisContainer.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get redis host: %v", err)
	}

	port, err := redisContainer.MappedPort(ctx, "6379")
	if err != nil {
		t.Fatalf("failed to get redis mapped port: %v", err)
	}

	endpoint := fmt.Sprintf("%s:%s", host, port.Port())

	checker := &RedisChecker{}

	canary := v1.Canary{
		Spec: v1.CanarySpec{
			Redis: []v1.RedisCheck{
				{
					Description: v1.Description{Name: "test-redis"},
					Connection: v1.Connection{
						URL: endpoint,
					},
				},
			},
		},
	}

	dCtx := dutyCtx.New()
	checkCtx := checkContext.New(dCtx, canary)

	results := checker.Run(checkCtx)
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}

	for _, result := range results {
		if result.Error != "" {
			t.Errorf("expected no error, got: %s", result.Error)
		}
		if !result.Pass {
			t.Error("expected check to pass")
		}
	}
}

// generateRedisTLSCerts creates a CA and a server certificate (signed by that
// CA, valid for "localhost") and returns their PEM encodings. The CA PEM is
// what a client must trust to verify the server; the server cert/key are mounted
// into the redis container.
func generateRedisTLSCerts(t *testing.T) (caPEM, serverCertPEM, serverKeyPEM []byte) {
	t.Helper()

	ca := tlscert.SelfSignedCA("redis-test-ca")
	if ca == nil {
		t.Fatalf("failed to generate CA certificate")
	}

	req := tlscert.NewRequest("localhost")
	req.Parent = ca
	server := tlscert.SelfSignedFromRequest(req)
	if server == nil {
		t.Fatalf("failed to generate server certificate")
	}

	return ca.Bytes, server.Bytes, server.KeyBytes
}

// startRedisTLS brings up a redis:7 container that accepts TLS only (plaintext
// port disabled) and does not require client certificates (--tls-auth-clients
// no). It returns the host:port the client should connect to.
func startRedisTLS(t *testing.T) (addr string, caPEM []byte) {
	t.Helper()
	ctx := context.Background()

	caPEM, serverCertPEM, serverKeyPEM := generateRedisTLSCerts(t)

	redisC, err := tcredis.Run(ctx, "redis:7",
		testcontainers.WithFiles(
			testcontainers.ContainerFile{
				Reader:            bytes.NewReader(caPEM),
				ContainerFilePath: "/tls/ca.crt",
				FileMode:          0o644,
			},
			testcontainers.ContainerFile{
				Reader:            bytes.NewReader(serverCertPEM),
				ContainerFilePath: "/tls/server.crt",
				FileMode:          0o644,
			},
			testcontainers.ContainerFile{
				Reader:            bytes.NewReader(serverKeyPEM),
				ContainerFilePath: "/tls/server.key",
				FileMode:          0o644,
			},
		),
		// TLS only: disable the plaintext port and serve TLS on 6379. We do not
		// require client certificates, so the client only needs to trust the CA.
		testcontainers.WithCmdArgs(
			"--tls-port", "6379",
			"--port", "0",
			"--tls-cert-file", "/tls/server.crt",
			"--tls-key-file", "/tls/server.key",
			"--tls-ca-cert-file", "/tls/ca.crt",
			"--tls-auth-clients", "no",
		),
	)
	if err != nil {
		t.Fatalf("failed to start redis tls container: %v", err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(redisC); err != nil {
			t.Errorf("failed to terminate container: %v", err)
		}
	})

	connStr, err := redisC.ConnectionString(context.Background())
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}
	u, err := url.Parse(connStr)
	if err != nil {
		t.Fatalf("failed to parse connection string %q: %v", connStr, err)
	}

	return u.Host, caPEM // u.Host is host:port
}

// TestRedisCheckerTLS verifies that the redis check can talk to a TLS-only
// redis server: the check must negotiate TLS to succeed.
func TestRedisCheckerTLS(t *testing.T) {
	addr, caPEM := startRedisTLS(t)

	canaryCtx := &checkContext.Context{
		Context:     dutyCtx.New(),
		Namespace:   "default",
		Canary:      v1.Canary{},
		Environment: map[string]any{},
	}

	check := v1.RedisCheck{
		Connection: v1.Connection{
			URL: addr,
		},
		TLSConfig: &v1.SwitchableTLSConfig{
			TLSConfig: v1.TLSConfig{
				CA: types.EnvVar{ValueStatic: string(caPEM)},
			},
		},
	}

	results := (&RedisChecker{}).Check(canaryCtx, check)
	if len(results) == 0 {
		t.Fatalf("expected at least one result, got none")
	}
	if !results[0].Pass {
		t.Fatalf("expected redis TLS check to pass, but it failed: %s", results[0].Error)
	}
}

// TestRedisCheckerTLSRejectedWithoutTLSConfig confirms the container really is
// TLS-only: with no TLS config supplied, the check must fail to talk to it.
// This guards against the TLS test passing for the wrong reason (e.g. the
// container silently falling back to plaintext).
func TestRedisCheckerTLSRejectedWithoutTLSConfig(t *testing.T) {
	addr, _ := startRedisTLS(t)

	canaryCtx := &checkContext.Context{
		Context:     dutyCtx.New(),
		Namespace:   "default",
		Canary:      v1.Canary{},
		Environment: map[string]any{},
	}

	check := v1.RedisCheck{
		Connection: v1.Connection{
			URL: addr,
		},
		// No TLSConfig: a plaintext dial against a TLS-only server must fail.
	}

	results := (&RedisChecker{}).Check(canaryCtx, check)
	if len(results) == 0 {
		t.Fatalf("expected at least one result, got none")
	}
	if results[0].Pass {
		t.Fatalf("expected redis check without TLS to fail against a TLS-only server, but it passed")
	}
}

// TestRedisCheckerTLSInsecureSkipVerify verifies that the check can talk to a
// TLS-only server when TLS is enabled with certificate verification skipped. No
// CA is needed because the server certificate is not validated.
func TestRedisCheckerTLSInsecureSkipVerify(t *testing.T) {
	addr, _ := startRedisTLS(t)

	canaryCtx := &checkContext.Context{
		Context:     dutyCtx.New(),
		Namespace:   "default",
		Canary:      v1.Canary{},
		Environment: map[string]any{},
	}

	check := v1.RedisCheck{
		Connection: v1.Connection{
			URL: addr,
		},
		TLSConfig: &v1.SwitchableTLSConfig{
			TLSConfig: v1.TLSConfig{
				InsecureSkipVerify: true,
			},
		},
	}

	results := (&RedisChecker{}).Check(canaryCtx, check)
	if len(results) == 0 {
		t.Fatalf("expected at least one result, got none")
	}
	if !results[0].Pass {
		t.Fatalf("expected redis TLS check with insecureSkipVerify to pass, but it failed: %s", results[0].Error)
	}
}
