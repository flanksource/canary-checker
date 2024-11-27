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
	"github.com/samber/lo"
	"golang.org/x/net/context"
)

type DNSChecker struct{}

// Type: returns checker type
func (c *DNSChecker) Type() string {
	return "dns"
}

func (c *DNSChecker) Run(ctx *canaryContext.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.DNS {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

var resolvers = map[string]func(ctx context.Context, r *net.Resolver, check v1.DNSCheck) (bool, string, error){
	"A":     checkA,
	"CNAME": checkCNAME,
	"SRV":   checkSRV,
	"MX":    checkMX,
	"PTR":   checkPTR,
	"TXT":   checkTXT,
	"NS":    checkNS,
}

func (c *DNSChecker) Check(ctx *canaryContext.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.DNSCheck)
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)
	timeout := check.Timeout
	if timeout == 0 {
		timeout = 10
	}

	var r net.Resolver
	if check.Server != "" {
		dialer, err := getDialer(check, timeout)
		if err != nil {
			return results.Failf("Failed to get dialer, %v", err)
		}
		r = net.Resolver{
			PreferGo: true,
			Dial:     dialer,
		}
	} else {
		r = net.Resolver{}
	}

	queryType := check.QueryType
	if queryType == "" {
		queryType = "A"
	}

	resultCh := make(chan *pkg.CheckResult, 1)
	if fn, ok := resolvers[strings.ToUpper(queryType)]; !ok {
		return results.Failf("unknown query type: %s", queryType)
	} else {
		go func() {
			pass, message, err := fn(ctx, &r, check)
			if err != nil {
				result.ErrorMessage(err)
			}
			if !pass {
				result.Failf(message)
			}
			resultCh <- result
		}()
	}

	select {
	case res := <-resultCh:
		res.Duration = res.GetDuration()
		if res.Duration == 0 {
			// round up submillisecond response times to 1ms
			res.Duration = 1
		}
		if check.ThresholdMillis > 0 && res.Duration > int64(check.ThresholdMillis) {
			return results.Failf("%dms > %dms", res.Duration, check.ThresholdMillis)
		}
		if res.Duration == 0 {
			// round up submillisecond response times to 1ms
			res.Duration = 1
		}
		return results
	case <-time.After(time.Second * time.Duration(timeout)):
		result.Duration = result.GetDuration()
		return results.Failf(fmt.Sprintf("timed out after %d seconds", timeout))
	}
}

func checkA(ctx context.Context, r *net.Resolver, check v1.DNSCheck) (pass bool, message string, err error) {
	result, err := r.LookupHost(ctx, check.Query)
	if err != nil {
		return pass, "", err
	}

	pass, message = checkResult(result, check)
	return
}

func checkPTR(ctx context.Context, r *net.Resolver, check v1.DNSCheck) (pass bool, message string, err error) {
	result, err := r.LookupAddr(ctx, check.Query)
	if err != nil {
		return pass, "", err
	}

	pass, message = checkResult(result, check)
	return
}

func checkCNAME(ctx context.Context, r *net.Resolver, check v1.DNSCheck) (pass bool, message string, err error) {
	result, err := r.LookupCNAME(ctx, check.Query)
	if err != nil {
		return pass, "", err
	}

	pass, message = checkResult([]string{result}, check)
	return
}

func checkSRV(ctx context.Context, r *net.Resolver, check v1.DNSCheck) (pass bool, message string, err error) {
	service, proto, name, err := srvInfo(check.Query)
	if err != nil {
		return false, "", err
	}
	cname, addr, err := r.LookupSRV(ctx, service, proto, name)
	if err != nil {
		return pass, "", err
	}
	pass = true
	message = fmt.Sprintf("got: %s %v", cname, addr)
	return
}

func checkMX(ctx context.Context, r *net.Resolver, check v1.DNSCheck) (pass bool, message string, err error) {
	result, err := r.LookupMX(ctx, check.Query)
	if err != nil {
		return pass, "", err
	}

	var resultString []string
	for _, reply := range result {
		resultString = append(resultString, fmt.Sprintf("%s %d", reply.Host, reply.Pref))
	}
	pass, message = checkResult(resultString, check)
	return
}

func checkTXT(ctx context.Context, r *net.Resolver, check v1.DNSCheck) (pass bool, message string, err error) {
	result, err := r.LookupTXT(ctx, check.Query)
	if err != nil {
		return pass, "", err
	}

	pass, message = checkResult(result, check)
	return
}

func checkNS(ctx context.Context, r *net.Resolver, check v1.DNSCheck) (pass bool, message string, err error) {
	result, err := r.LookupNS(ctx, check.Query)
	if err != nil {
		return pass, "", err
	}

	var resultString []string
	for _, reply := range result {
		resultString = append(resultString, reply.Host)
	}
	pass, message = checkResult(resultString, check)
	return
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
	expected := make([]string, len(check.ExactReply))
	copy(expected, check.ExactReply)
	count := len(got)
	var pass = true
	var errMessage string
	if count < check.MinRecords {
		pass = false
		errMessage = fmt.Sprintf("returned %d results, expecting %d", count, check.MinRecords)
	}

	if len(check.ExactReply) != 0 {
		got = lo.Map(got, func(s string, _ int) string { return strings.ToLower(s) })
		expected = lo.Map(expected, func(s string, _ int) string { return strings.ToLower(s) })
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
