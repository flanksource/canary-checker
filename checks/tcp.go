package checks

import (
	"fmt"
	"net"
	"strings"
	"time"

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
func (t *TCPChecker) Run(config v1.CanarySpec) []*pkg.CheckResult {
	results := []*pkg.CheckResult{}
	for _, c := range config.TCP {
		results = append(results, t.Check(c))
	}
	return results
}

// Check performs a single tcp check, returning a checkResult
func (t *TCPChecker) Check(extConfig external.Check) *pkg.CheckResult {
	c := extConfig.(v1.TCPCheck)
	addr, port, err := extractAddrAndPort(c.Endpoint)
	if err != nil {
		return Failf(c, err.Error())
	}

	timeout := time.Millisecond * time.Duration(c.ThresholdMillis)
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(addr, port), timeout)
	if err != nil {
		return Failf(c, "Connection error: %s", err.Error())
	}
	if conn != nil {
		defer conn.Close()
	}
	return Passf(c, "Successfully opened: %s", net.JoinHostPort(addr, port))
}

func extractAddrAndPort(e string) (string, string, error) {
	s := strings.Split(e, ":")
	if len(s) != 2 {
		return "", "", fmt.Errorf(formatErrorMsg(e))
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
