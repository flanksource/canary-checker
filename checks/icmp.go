package checks

import (
	"fmt"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sparrc/go-ping"

	"github.com/flanksource/canary-checker/pkg"
)

var (
	packetLoss = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "canary_check_icmp_packetloss",
			Help: "Packet loss percentage in ICMP check",
		},
		[]string{"endpoint", "ip"},
	)
)

func init() {
	prometheus.MustRegister(packetLoss)
}

type IcmpChecker struct{}

// Type: returns checker type
func (c *IcmpChecker) Type() string {
	return "icmp"
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *IcmpChecker) Run(config pkg.Config, results chan *pkg.CheckResult) {
	for _, conf := range config.ICMP {
		for _, result := range c.Check(conf.ICMPCheck) {
			results <- result
		}
	}
}

// CheckConfig : Check every record of DNS name against config information
// Returns check result and metrics
func (c *IcmpChecker) Check(check pkg.ICMPCheck) []*pkg.CheckResult {
	var result []*pkg.CheckResult
	endpoint := check.Endpoint
	timeOK, packetOK := false, false
	lookupResult, err := DNSLookup(endpoint)
	if err != nil {
		result = append(result, invalidErrorf(check, err, "unable to resolve dns")...)
		return result
	}
	for _, urlObj := range lookupResult {
		checkResults, err := c.checkICMP(urlObj, check.PacketCount)
		if err == nil {
			if check.ThresholdMillis >= checkResults.Latency {
				timeOK = true
			}
			if check.PacketLossThreshold >= checkResults.PacketLoss {
				packetOK = true
			}
			msg := []string{}
			if !timeOK {
				msg = append(msg, "DNS Timeout")
			}
			if !packetOK {
				msg = append(msg, "Packet Invalid")
			}
			pass := timeOK && packetOK
			if pass {
				msg = append(msg, "Succesffully checked")
			}

			checkResult := &pkg.CheckResult{
				Check:    check,
				Pass:     pass,
				Invalid:  false,
				Duration: int64(checkResults.Latency),
				Message:  strings.Join(msg, ", "),
			}
			result = append(result, checkResult)

			packetLoss.WithLabelValues(endpoint, urlObj.IP).Set(float64(checkResults.PacketLoss))

		} else {
			checkResult := &pkg.CheckResult{
				Check:    check,
				Pass:     false,
				Invalid:  true,
				Duration: int64(checkResults.Latency),
				Message:  fmt.Sprintf("Failed to check icmp: %v", err),
			}
			result = append(result, checkResult)
		}
	}
	return result
}

func (c *IcmpChecker) checkICMP(urlObj pkg.URL, packetCount int) (*pkg.ICMPCheckResult, error) {
	ip := urlObj.IP
	pinger, err := ping.NewPinger(ip)
	if err != nil {
		return nil, err
	}
	// this requires running as root or with NET_RAW priveleges, this is easier than the alternativer
	// sysctl -w net.ipv4.ping_group_range="0   2147483647" which doesn't require root, but does require kubelet changes
	// whitelist the sysctl's for use
	pinger.SetPrivileged(true)
	pinger.Count = packetCount
	pinger.Timeout = time.Second * 10
	pinger.Run()
	pingerStats := pinger.Statistics()
	latency := pingerStats.AvgRtt.Milliseconds()
	packetLoss := pingerStats.PacketLoss
	checkResult := pkg.ICMPCheckResult{
		Endpoint:   urlObj.Host,
		Record:     urlObj.IP,
		Latency:    float64(latency),
		PacketLoss: packetLoss,
	}
	return &checkResult, nil
}
