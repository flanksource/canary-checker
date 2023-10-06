package checks

import (
	"net"
	"os"
	"time"

	"github.com/flanksource/canary-checker/api/context"

	"github.com/flanksource/canary-checker/api/external"
	"github.com/flanksource/canary-checker/pkg/dns"
	ping "github.com/prometheus-community/pro-bing"
	"github.com/prometheus/client_golang/prometheus"

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

var PRIVILEGED = os.Getenv("PING_MODE") == "privileged"

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
func (c *IcmpChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.ICMP {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

// CheckConfig : Check every record of DNS name against config information
// Returns check result and metrics
func (c *IcmpChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.ICMPCheck)
	var results pkg.Results
	result := pkg.Success(check, ctx.Canary)
	results = append(results, result)

	endpoint := check.Endpoint
	ips, err := dns.Lookup("A", endpoint)
	if err != nil {
		return results.ErrorMessage(err)
	}

	for _, urlObj := range ips {
		pingerStats, err := c.checkICMP(urlObj, check.PacketCount)
		if err != nil {
			return results.ErrorMessage(err)
		}
		if pingerStats.PacketsSent == 0 {
			return results.Failf("Failed to check icmp, no packets sent")
		}
		latency := pingerStats.AvgRtt.Milliseconds()
		if latency == 0 && pingerStats.AvgRtt.Microseconds() > 0 {
			// For submillisecond response times, round up to 1ms
			latency = 1
		}
		result.Duration = latency
		loss := pingerStats.PacketLoss

		if check.ThresholdMillis < latency {
			return results.Failf("timeout after %d ", latency)
		}
		if check.PacketLossThreshold < int64(loss*100) {
			return results.Failf("%s packet loss of %0.0f%% > than threshold of %d%%", urlObj.To4(), loss, check.PacketLossThreshold)
		}

		packetLoss.WithLabelValues(endpoint, ips[0].String()).Set(loss)
		return results //nolint
	}

	return results.Failf("no IP found for %s", endpoint)
}

func (c *IcmpChecker) checkICMP(ip net.IP, packetCount int) (*ping.Statistics, error) {
	pinger, err := ping.NewPinger(ip.String())
	if err != nil {
		return nil, err
	}
	pinger.SetNetwork("ip4")
	pinger.SetPrivileged(PRIVILEGED)
	if packetCount == 0 {
		packetCount = 5
	}
	pinger.Count = packetCount
	pinger.Timeout = time.Second * 10
	err = pinger.Run()
	return pinger.Statistics(), err
}
