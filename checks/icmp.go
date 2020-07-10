package checks

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sparrc/go-ping"

	v1 "github.com/flanksource/canary-checker/api/v1"
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
func (c *IcmpChecker) Run(config v1.CanarySpec) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range config.ICMP {
		results = append(results, c.Check(conf))
	}
	return results
}

// CheckConfig : Check every record of DNS name against config information
// Returns check result and metrics
func (c *IcmpChecker) Check(check v1.ICMPCheck) *pkg.CheckResult {
	endpoint := check.Endpoint

	lookupResult, err := DNSLookup(endpoint)
	if err != nil {
		return invalidErrorf(check, err, "unable to resolve dns")
	}
	for _, urlObj := range lookupResult {
		pingerStats, err := c.checkICMP(urlObj, check.PacketCount)
		if err != nil {
			return Failf(check, "Failed to check icmp: %v", err)
		}
		if pingerStats.PacketsSent == 0 {
			return Failf(check, "Failed to check icmp, no packets sent")
		}
		latency := float64(pingerStats.AvgRtt.Milliseconds())
		loss := pingerStats.PacketLoss

		if check.ThresholdMillis < int64(latency) {
			return Failf(check, "timeout after %d ", latency)
		}
		if check.PacketLossThreshold < int64(loss*100) {
			return Failf(check, "packet loss of %d > than threshold of %d", loss, check.PacketLossThreshold)
		}

		packetLoss.WithLabelValues(endpoint, urlObj.IP).Set(loss)

		return &pkg.CheckResult{
			Pass:     true,
			Check:    check,
			Duration: int64(latency),
		}
	}

	return Failf(check, "No results found")

}

func (c *IcmpChecker) checkICMP(urlObj pkg.URL, packetCount int) (*ping.Statistics, error) {
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
	return pinger.Statistics(), nil
}
