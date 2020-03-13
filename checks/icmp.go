package checks

import (
	"fmt"
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
func (c *IcmpChecker) Run(config pkg.Config) []*pkg.CheckResult {
	var checks []*pkg.CheckResult
	for _, conf := range config.ICMP {
		for _, result := range c.Check(conf.ICMPCheck) {
			checks = append(checks, result)
		}
	}
	return checks
}

// CheckConfig : Check every record of DNS name against config information
// Returns check result and metrics
func (c *IcmpChecker) Check(check pkg.ICMPCheck) []*pkg.CheckResult {
	var result []*pkg.CheckResult
	for _, endpoint := range check.Endpoints {
		timeOK, packetOK := false, false
		lookupResult, err := DNSLookup(endpoint)
		if err != nil {
			checkResult := &pkg.CheckResult{
				Pass:     false,
				Invalid:  true,
				Message:  fmt.Sprintf("failed to resolve dns for %s: %v", endpoint, err),
				Endpoint: endpoint,
				Metrics:  []pkg.Metric{},
			}
			result = append(result, checkResult)
			continue
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
				pass := timeOK && packetOK

				checkResult := &pkg.CheckResult{
					Pass:     pass,
					Invalid:  false,
					Duration: int64(checkResults.Latency),
					Endpoint: endpoint,
				}
				result = append(result, checkResult)

				packetLoss.WithLabelValues(endpoint, urlObj.IP).Set(float64(checkResults.PacketLoss))

			} else {
				checkResult := &pkg.CheckResult{
					Pass:     false,
					Invalid:  true,
					Endpoint: endpoint,
					Duration: int64(checkResults.Latency),
				}
				result = append(result, checkResult)
			}
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
	pinger.SetPrivileged(false)
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
