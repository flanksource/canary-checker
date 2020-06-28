package checks

import (
	"fmt"
	"net"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/pkg"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

type DNSChecker struct{}

// Type: returns checker type
func (c *DNSChecker) Type() string {
	return "dns"
}

func (c *DNSChecker) Run(config pkg.Config, results chan *pkg.CheckResult) {
	for _, conf := range config.DNS {
		results <- c.Check(conf.DNSCheck)
	}
}

func (c *DNSChecker) Check(check pkg.DNSCheck) *pkg.CheckResult {
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
			Metrics:  getDNSMetrics(check, elapsed, result),
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
			Metrics:  getDNSMetrics(check, elapsed, result),
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
			Metrics:  getDNSMetrics(check, elapsed, []string{result}),
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
		fmt.Println(cname, addr)
		return &pkg.CheckResult{}
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
			Metrics:  getDNSMetrics(check, elapsed, resultString),
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
			Metrics:  getDNSMetrics(check, elapsed, result),
		}
	}

	if check.QueryType == "NS" {
		result, err := r.LookupNS(ctx, check.Query)
		elapsed := time.Since(start)
		if err != nil {
			// log.Errorf("Failed to lookup: %v", err)
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
			Metrics:  getDNSMetrics(check, elapsed, resultString),
		}
	}

	return &pkg.CheckResult{}
}

func getDialer(check pkg.DNSCheck, timeout int) (func(ctx context.Context, network, address string) (net.Conn, error), error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		d := net.Dialer{
			Timeout: time.Second * time.Duration(timeout),
		}
		return d.DialContext(ctx, "udp", fmt.Sprintf("%s:%d", check.Server, check.Port))
	}, nil
}

func checkResult(got []string, check pkg.DNSCheck) (result bool, message string) {
	expected := check.ExactReply
	count := len(got)
	var pass = true
	var errMessage string
	if count < check.MinRecords {
		pass = false
		errMessage = "Records count is less then minrecords"
	}

	if len(check.ExactReply) != 0 {
		sort.Sort(sort.StringSlice(got))
		sort.Sort(sort.StringSlice(expected))
		if !reflect.DeepEqual(got, expected) {
			pass = false
			errMessage = fmt.Sprintf("Got %s, expected %s", got, check.ExactReply)
		}
	}
	log.Tracef("DNS Result: %s", got)
	log.Tracef("Expected Result %s", check.ExactReply)

	if pass {
		message = fmt.Sprintf("Successful check on %s. Got %v", check.Server, got)
	} else {
		message = fmt.Sprintf("Check failed: %s %s on %s. %s", check.QueryType, check.Query, check.Server, errMessage)
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

func getDNSMetrics(check pkg.DNSCheck, lookupTime time.Duration, records []string) []pkg.Metric {
	return []pkg.Metric{
		{
			Name: "dns_lookup_time",
			Type: pkg.HistogramType,
			Labels: map[string]string{
				"dnsCheckQuery":  check.Query,
				"dnsCheckServer": check.Server,
				"dnsCheckPort":   strconv.Itoa(check.Port),
			},
			Value: float64(lookupTime.Milliseconds()),
		},
		{
			Name: "dns_records",
			Type: pkg.GaugeType,
			Labels: map[string]string{
				"dnsCheckQuery":  check.Query,
				"dnsCheckServer": check.Server,
				"dnsCheckPort":   strconv.Itoa(check.Port),
			},
			Value: float64(len(records)),
		},
	}
}
