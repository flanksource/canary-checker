package checks

import (
    "fmt"
    "log"
    "time"

    "github.com/jasonlvhit/gocron"
    "github.com/jinzhu/copier"
    "github.com/sparrc/go-ping"

    "github.com/flanksource/canary-checker/pkg"
    "github.com/prometheus/client_golang/prometheus"
)

var (
    icmpDnsFailed = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "canary_check_icmp_dns_failed",
        Help: "The total number of dns requests failed",
    })

    packetLoss = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "canary_check_icmp_packetloss",
            Help: "Packet loss percentage in ICMP check",
        },
        []string{"endpoint","ip"},
    )

    icmpLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "canary_check_icmp_latency",
            Help:    "ICMP latency in milliseconds",
            Buckets: []float64{25, 50, 100, 200, 400, 800, 1000, 1200, 1500, 2000},
            },
            []string{"endpoint", "ip"},
    )
)



func init() {
    prometheus.MustRegister(icmpDnsFailed, packetLoss, icmpLatency)
}

type IcmpChecker struct{}

// Type: returns checker type
func (c *IcmpChecker) Type() string {
    return "icmp"
}

// Schedule: Add every check as a cron job, calls MetricProcessor with the set of metrics
func (c *IcmpChecker) Schedule(config pkg.Config, interval uint64, mp MetricProcessor) {
    for _, conf := range config.ICMP {
        icmpCheck := pkg.ICMPCheck{}
        if err := copier.Copy(&icmpCheck, &conf.ICMPCheck); err != nil {
            log.Printf("error copying %v", err)
        }
        gocron.Every(interval).Seconds().Do(func() {
            metrics := c.Check(icmpCheck)
            mp(metrics)
        })
    }
}


// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *IcmpChecker) Run(config pkg.Config) []*pkg.CheckResult {
    var checks []*pkg.CheckResult
    for _, conf := range config.ICMP {
        for _, result := range c.Check(conf.ICMPCheck) {
            checks = append(checks, result)
            fmt.Println(result)
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
            log.Printf("Failed to resolve DNS for %s", endpoint)
            icmpDnsFailed.Inc()
            checkResult := &pkg.CheckResult{
                    Pass:     false,
                    Invalid:  true,
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
                m := []pkg.Metric{
                    {
                        Name: "latency",
                        Type: pkg.HistogramType,
                        Labels: map[string]string{
                            "endpoint": endpoint,
                            "ip":       urlObj.IP,
                        },
                        Value: float64(checkResults.Latency),
                    },
                    {
                        Name: "packetLoss",
                        Type: pkg.GaugeType,
                        Labels: map[string]string{
                            "endpoint": endpoint,
                            "ip":       urlObj.IP,
                        },
                        Value: float64(checkResults.PacketLoss),
                    },
                }
                checkResult := &pkg.CheckResult{
                    Pass:     pass,
                    Invalid:  false,
                    Duration: int64(checkResults.Latency),
                    Endpoint: endpoint,
                    Metrics:  m,
                }
                result = append(result, checkResult)
                
                packetLoss.WithLabelValues(endpoint,urlObj.IP).Set(float64(checkResults.PacketLoss))
                icmpLatency.WithLabelValues(endpoint,urlObj.IP).Observe(float64(checkResults.Latency))

            } else {
                checkResult := &pkg.CheckResult{
                    Pass:     false,
                    Invalid:  true,
                    Endpoint: endpoint,
                    Metrics:  []pkg.Metric{},
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
        //fmt.Println("ERROR: %s\n", err.Error())
        return nil, err
    }
    pinger.Count = packetCount
    pinger.Timeout = time.Second * 10       // 10 seconds
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

