package checks

import (
	"fmt"
	"net"
	"reflect"
	"sort"
	"strings"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"golang.org/x/net/context"
)

type DNSChecker struct{}

// Type: returns checker type
func (c *DNSChecker) Type() string {
	return "dns"
}

func (c *DNSChecker) Run(config v1.CanarySpec) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range config.DNS {
		results = append(results, c.Check(conf))
	}
	return results
}

func (c *DNSChecker) Check(check v1.DNSCheck) *pkg.CheckResult {
	start := time.Now()
	ctx := context.Background()
	dialer, err := getDialer(check, check.Timeout)
	if err != nil {
		return Failf(check, "Failed to get dialer, %v", err)
	}
	r := net.Resolver{
		PreferGo: true,
		Dial:     dialer,
	}
	if check.QueryType == "A" {
		result, err := r.LookupHost(ctx, check.Query)
		if err != nil {
			return Failf(check, "Failed to lookup: %v", err)

		}

		elapsed := time.Since(start)

		pass, message := checkResult(result, check)

		return &pkg.CheckResult{
			Check:    check,
			Pass:     pass,
			Invalid:  false,
			Duration: elapsed.Milliseconds(),
			Message:  message,
		}
	}
	if check.QueryType == "PTR" {
		result, err := r.LookupAddr(ctx, check.Query)
		if err != nil {
			return Failf(check, "Failed to lookup: %v", err)
		}

		elapsed := time.Since(start)

		pass, message := checkResult(result, check)
		return &pkg.CheckResult{
			Check:    check,
			Pass:     pass,
			Invalid:  false,
			Duration: elapsed.Milliseconds(),
			Message:  message,
		}
	}

	if check.QueryType == "CNAME" {
		result, err := r.LookupCNAME(ctx, check.Query)
		if err != nil {
			return Failf(check, "Failed to lookup: %v", err)
		}
		elapsed := time.Since(start)

		pass, message := checkResult([]string{result}, check)
		return &pkg.CheckResult{
			Check:    check,
			Pass:     pass,
			Invalid:  false,
			Duration: elapsed.Milliseconds(),
			Message:  message,
		}
	}

	if check.QueryType == "SRV" {
		service, proto, name, err := srvInfo(check.Query)
		if err != nil {
			return Failf(check, "Wrong SRV query %s", check.Query)
		}
		cname, addr, err := r.LookupSRV(ctx, service, proto, name)
		if err != nil {
			return Failf(check, "Failed to lookup: %v", err)
		}
		return Passf(check, "got: %s %v", cname, addr)
	}

	if check.QueryType == "MX" {
		result, err := r.LookupMX(ctx, check.Query)
		if err != nil {
			return Failf(check, "Failed to lookup: %v", err)
		}
		elapsed := time.Since(start)
		var resultString []string
		for _, reply := range result {
			resultString = append(resultString, fmt.Sprintf("%s %d", reply.Host, reply.Pref))
		}
		pass, message := checkResult(resultString, check)
		return &pkg.CheckResult{
			Pass:     pass,
			Invalid:  false,
			Duration: elapsed.Milliseconds(),
			Check:    check,
			Message:  message,
		}
	}

	if check.QueryType == "TXT" {
		result, err := r.LookupTXT(ctx, check.Query)
		if err != nil {
			return Failf(check, "Failed to lookup: %v", err)
		}
		elapsed := time.Since(start)
		pass, message := checkResult(result, check)
		return &pkg.CheckResult{
			Check:    check,
			Pass:     pass,
			Invalid:  false,
			Duration: elapsed.Milliseconds(),
			Message:  message,
		}
	}

	if check.QueryType == "NS" {
		result, err := r.LookupNS(ctx, check.Query)
		elapsed := time.Since(start)
		if err != nil {
			return &pkg.CheckResult{
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
		return &pkg.CheckResult{
			Check:    check,
			Pass:     pass,
			Invalid:  false,
			Duration: elapsed.Milliseconds(),
			Message:  message,
		}
	}

	return Failf(check, "unknown query type: %s", check.QueryType)
}

func getDialer(check v1.DNSCheck, timeout int) (func(ctx context.Context, network, address string) (net.Conn, error), error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		d := net.Dialer{
			Timeout: time.Second * time.Duration(timeout),
		}
		return d.DialContext(ctx, "udp", fmt.Sprintf("%s:%d", check.Server, check.Port))
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
		sort.Sort(sort.StringSlice(got))
		sort.Sort(sort.StringSlice(expected))
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
