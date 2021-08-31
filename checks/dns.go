package checks

import (
	"fmt"
	"net"
	"reflect"
	"sort"
	"strings"
	"time"

	canaryContext "github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"golang.org/x/net/context"
)

type DNSChecker struct{}

// Type: returns checker type
func (c *DNSChecker) Type() string {
	return "dns"
}

func (c *DNSChecker) Run(ctx *canaryContext.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range ctx.Canary.Spec.DNS {
		results = append(results, c.Check(ctx, conf))
	}
	return results
}

func (c *DNSChecker) Check(canaryCtx *canaryContext.Context, extConfig external.Check) *pkg.CheckResult {
	check := extConfig.(v1.DNSCheck)
	ctx := context.Background()
	result := pkg.Success(check)
	dialer, err := getDialer(check, check.Timeout)
	if err != nil {
		return Failf(check, "Failed to get dialer, %v", err)
	}
	r := net.Resolver{
		PreferGo: true,
		Dial:     dialer,
	}

	timeout := check.Timeout
	if timeout == 0 {
		timeout = 10
	}

	resultCh := make(chan *pkg.CheckResult, 1)

	switch qs := check.QueryType; qs {
	case "PTR":
		go checkPTR(ctx, &r, check, resultCh)
	case "CNAME":
		go checkCNAME(ctx, &r, check, resultCh)
	case "SRV":
		go checkSRV(ctx, &r, check, resultCh)
	case "MX":
		go checkMX(ctx, &r, check, resultCh)
	case "TXT":
		go checkTXT(ctx, &r, check, resultCh)
	case "NS":
		go checkNS(ctx, &r, check, resultCh)
	default:
		go checkA(ctx, &r, check, resultCh)
	}

	select {
	case res := <-resultCh:
		if res.Duration > int64(check.ThresholdMillis) {
			return result.Failf("%dms > %dms", res.Duration, check.ThresholdMillis)
		}
		if res.Duration == 0 {
			// round up submillisecond response times to 1ms
			res.Duration = 1
		}
		return res
	case <-time.After(time.Second * time.Duration(timeout)):
		return result.Failf(fmt.Sprintf("timed out after %d seconds", timeout))
	}
}

func checkA(ctx context.Context, r *net.Resolver, check v1.DNSCheck, resultCh chan *pkg.CheckResult) {
	start := time.Now()
	result, err := r.LookupHost(ctx, check.Query)
	if err != nil {
		resultCh <- Failf(check, "Failed to lookup: %v", err)
	}

	elapsed := time.Since(start)

	pass, message := checkResult(result, check)

	resultCh <- &pkg.CheckResult{
		Check:    check,
		Pass:     pass,
		Invalid:  false,
		Duration: elapsed.Milliseconds(),
		Message:  message,
	}
}

func checkPTR(ctx context.Context, r *net.Resolver, check v1.DNSCheck, resultCh chan *pkg.CheckResult) {
	start := time.Now()
	result, err := r.LookupAddr(ctx, check.Query)
	if err != nil {
		resultCh <- Failf(check, "Failed to lookup: %v", err)
	}

	elapsed := time.Since(start)

	pass, message := checkResult(result, check)
	resultCh <- &pkg.CheckResult{
		Check:    check,
		Pass:     pass,
		Invalid:  false,
		Duration: elapsed.Milliseconds(),
		Message:  message,
	}
}

func checkCNAME(ctx context.Context, r *net.Resolver, check v1.DNSCheck, resultCh chan *pkg.CheckResult) {
	start := time.Now()
	result, err := r.LookupCNAME(ctx, check.Query)
	if err != nil {
		resultCh <- Failf(check, "Failed to lookup: %v", err)
	}
	elapsed := time.Since(start)

	pass, message := checkResult([]string{result}, check)
	resultCh <- &pkg.CheckResult{
		Check:    check,
		Pass:     pass,
		Invalid:  false,
		Duration: elapsed.Milliseconds(),
		Message:  message,
	}
}

func checkSRV(ctx context.Context, r *net.Resolver, check v1.DNSCheck, resultCh chan *pkg.CheckResult) {
	service, proto, name, err := srvInfo(check.Query)
	if err != nil {
		resultCh <- Failf(check, "Wrong SRV query %s", check.Query)
	}
	cname, addr, err := r.LookupSRV(ctx, service, proto, name)
	if err != nil {
		resultCh <- Failf(check, "Failed to lookup: %v", err)
	}
	resultCh <- Passf(check, "got: %s %v", cname, addr)
}

func checkMX(ctx context.Context, r *net.Resolver, check v1.DNSCheck, resultCh chan *pkg.CheckResult) {
	start := time.Now()
	result, err := r.LookupMX(ctx, check.Query)
	if err != nil {
		resultCh <- Failf(check, "Failed to lookup: %v", err)
	}
	elapsed := time.Since(start)
	var resultString []string
	for _, reply := range result {
		resultString = append(resultString, fmt.Sprintf("%s %d", reply.Host, reply.Pref))
	}
	pass, message := checkResult(resultString, check)
	resultCh <- &pkg.CheckResult{
		Pass:     pass,
		Invalid:  false,
		Duration: elapsed.Milliseconds(),
		Check:    check,
		Message:  message,
	}
}

func checkTXT(ctx context.Context, r *net.Resolver, check v1.DNSCheck, resultCh chan *pkg.CheckResult) {
	start := time.Now()
	result, err := r.LookupTXT(ctx, check.Query)
	if err != nil {
		resultCh <- Failf(check, "Failed to lookup: %v", err)
	}
	elapsed := time.Since(start)
	pass, message := checkResult(result, check)
	resultCh <- &pkg.CheckResult{
		Check:    check,
		Pass:     pass,
		Invalid:  false,
		Duration: elapsed.Milliseconds(),
		Message:  message,
	}
}

func checkNS(ctx context.Context, r *net.Resolver, check v1.DNSCheck, resultCh chan *pkg.CheckResult) {
	start := time.Now()
	result, err := r.LookupNS(ctx, check.Query)
	elapsed := time.Since(start)
	if err != nil {
		resultCh <- &pkg.CheckResult{
			Check:    check,
			Pass:     false,
			Invalid:  false,
			Duration: elapsed.Milliseconds(),
			Message:  err.Error(),
		}
	}
	var resultString []string
	for _, reply := range result {
		resultString = append(resultString, reply.Host)
	}
	pass, message := checkResult(resultString, check)
	resultCh <- &pkg.CheckResult{
		Check:    check,
		Pass:     pass,
		Invalid:  false,
		Duration: elapsed.Milliseconds(),
		Message:  message,
	}
}

func getDialer(check v1.DNSCheck, timeout int) (func(ctx context.Context, network, address string) (net.Conn, error), error) { // nolint: unparam
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		d := net.Dialer{
			Timeout: time.Second * time.Duration(timeout),
		}
		port := check.Port
		if port == 0 {
			port = 53
		}
		return d.DialContext(ctx, "udp", fmt.Sprintf("%s:%d", check.Server, port))
	}, nil
}

func checkResult(got []string, check v1.DNSCheck) (result bool, message string) {
	expected := check.ExactReply
	count := len(got)
	var pass = true
	var errMessage string
	if count < check.MinRecords {
		pass = false
		errMessage = fmt.Sprintf("returned %d results, expecting %d", count, check.MinRecords)
	}

	if len(check.ExactReply) != 0 {
		sort.Strings(got)
		sort.Strings(expected)
		if !reflect.DeepEqual(got, expected) {
			pass = false
			errMessage = fmt.Sprintf("Got %s, expected %s", got, check.ExactReply)
		}
	}

	if pass {
		message = fmt.Sprintf("got %v", got)
	} else {
		message = fmt.Sprintf("%s %s on %s: %s", check.QueryType, check.Query, check.Server, errMessage)
	}
	return pass, message
}

func srvInfo(srv string) (service string, proto string, name string, err error) {
	splited := strings.Split(srv, ".")
	if len(splited) < 3 {
		return "", "", "", fmt.Errorf("srvInfo: wrong srv string")
	}
	return strings.ReplaceAll(splited[0], "_", ""), strings.ReplaceAll(splited[1], "_", ""), splited[2], nil
}
