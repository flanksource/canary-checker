package checks

import (
	"net"
	"time"

	"github.com/flanksource/canary-checker/api/context"

	"github.com/flanksource/canary-checker/api/external"
	"github.com/flanksource/canary-checker/pkg/dns"
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
func (c *IcmpChecker) Run(ctx *context.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range ctx.Canary.Spec.ICMP {
		results = append(results, c.Check(ctx, conf))
	}
	return results
}

// CheckConfig : Check every record of DNS name against config information
// Returns check result and metrics
func (c *IcmpChecker) Check(ctx *context.Context, extConfig external.Check) *pkg.CheckResult {
	check := extConfig.(v1.ICMPCheck)
	endpoint := check.Endpoint

	result := pkg.Success(check)

	ips, err := dns.Lookup("A", endpoint)
	if err != nil {
		return result.ErrorMessage(err)
	}

	for _, urlObj := range ips {
		pingerStats, err := c.checkICMP(urlObj, check.PacketCount)
		if err != nil {
			return result.ErrorMessage(err)
		}
		if pingerStats.PacketsSent == 0 {
			return result.Failf("Failed to check icmp, no packets sent")
		}
		latency := pingerStats.AvgRtt.Milliseconds()
		if latency == 0 && pingerStats.AvgRtt.Microseconds() > 0 {
			// For submillisecond response times, round up to 1ms
			latency = 1
		}
		result.Duration = latency
		loss := pingerStats.PacketLoss

		if check.ThresholdMillis < latency {
			return result.Failf("timeout after %d ", latency)
		}
		if check.PacketLossThreshold < int64(loss*100) {
			return result.Failf("packet loss of %0.0f > than threshold of %d", loss, check.PacketLossThreshold)
		}

		packetLoss.WithLabelValues(endpoint, ips[0].String()).Set(loss)
		return result //nolint
	}
	return result.Failf("no IP found for %s", endpoint)
}

func (c *IcmpChecker) checkICMP(ip net.IP, packetCount int) (*ping.Statistics, error) {
	pinger, err := ping.NewPinger(ip.String())
	if err != nil {
		return nil, err
	}
	// this requires running as root or with NET_RAW privileges, this is easier than the alternative
	// sysctl -w net.ipv4.ping_group_range="0   2147483647" which doesn't require root, but does require kubelet changes
	// whitelist the sysctl's for use
	pinger.SetPrivileged(true)
	pinger.Count = packetCount
	pinger.Timeout = time.Second * 10
	pinger.Run()
	return pinger.Statistics(), nil
}
