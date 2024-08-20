package checks

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/api/context"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

// TCPChecker checks if the given port is open on the given host
type TCPChecker struct{}

// NewTCPChecker creates and returns a pointer to a TCPChecker
func NewTCPChecker() *TCPChecker {
	return &TCPChecker{}
}

// Run executes tcp checks for the given config, returning results
func (t *TCPChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, c := range ctx.Canary.Spec.TCP {
		results = append(results, t.Check(ctx, c)...)
	}
	return results
}

// Check performs a single tcp check, returning a checkResult
func (t *TCPChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	c := extConfig.(v1.TCPCheck)
	result := pkg.Success(c, ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	addr, port, err := extractAddrAndPort(c.Endpoint)
	if err != nil {
		return results.ErrorMessage(err)
	}

	timeout := time.Millisecond * time.Duration(c.ThresholdMillis)
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(addr, port), timeout)
	if err != nil {
		return results.Failf("Connection error: %s", err)
	}
	if conn != nil {
		defer conn.Close()
	}
	return results
}

func extractAddrAndPort(e string) (string, string, error) {
	s := strings.Split(e, ":")
	if len(s) != 2 {
		return "", "", errors.New(formatErrorMsg(e))
	}
	return s[0], s[1], nil
}

func formatErrorMsg(f string) string {
	return fmt.Sprintf("Incorrect endpoint format: %s should be ADDRESS:PORT", f)
}

// Type returns the type
func (t *TCPChecker) Type() string {
	return "tcp"
}
