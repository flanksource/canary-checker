package cmd

import (
	"fmt"
	nethttp "net/http"
	"strconv"

	"github.com/jasonlvhit/gocron"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
)

var Serve = &cobra.Command{
	Use:   "serve",
	Short: "Start a server to execute checks ",
	Run: func(cmd *cobra.Command, args []string) {
		configfile, _ := cmd.Flags().GetString("configfile")
		config := pkg.ParseConfig(configfile)
		httpPort, _ := cmd.Flags().GetInt("httpPort")
		interval, _ := cmd.Flags().GetUint64("interval")

		var checks = []checks.Checker{
			&checks.HttpChecker{},
			&checks.IcmpChecker{},
			&checks.S3Checker{},
			&checks.DockerPullChecker{},
			&checks.PostgresChecker{},
		}

		gocron.Every(interval).Seconds().Do(func() {
			for _, c := range checks {
				results := c.Run(config)
				processMetrics(c.Type(), results)
			}
		})

		gocron.Start()

		nethttp.Handle("/metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{}))

		addr := fmt.Sprintf("0.0.0.0:%d", httpPort)
		log.Infof("Starting health dashboard at http://%s", addr)
		log.Infof("Metrics dashboard can be accessed at http://%s/metrics", addr)

		if err := nethttp.ListenAndServe(addr, nil); err != nil {
			log.Fatal(errors.Wrap(err, "failed to start server"))
		}
	},
}

var counters map[string]prometheus.Counter

func processMetrics(checkType string, results []*pkg.CheckResult) {
	for _, result := range results {
		if log.IsLevelEnabled(log.DebugLevel) {
			fmt.Println(result)
		}
		pkg.OpsCount.WithLabelValues(checkType).Inc()
		if result.Pass {
			pkg.OpsSuccessCount.WithLabelValues(checkType).Inc()
			if result.Duration > 0 {
				pkg.RequestLatency.WithLabelValues(checkType, result.Endpoint).Observe(float64(result.Duration))
			}

			for _, m := range result.Metrics {
				switch m.Type {
				case pkg.CounterType:
					pkg.GenericCounter.WithLabelValues(checkType, m.Name, strconv.Itoa(int(m.Value))).Inc()
				case pkg.GaugeType:
					pkg.GenericGauge.WithLabelValues(checkType, m.Name).Set(m.Value)
				}
			}
		} else {
			pkg.OpsFailedCount.WithLabelValues(checkType).Inc()
		}
	}
}

func init() {
	Serve.Flags().Int("httpPort", 8080, "Port to expose a health dashboard ")
	Serve.Flags().Uint64("interval", 30, "Default interval (in seconds) to run checks on")
	Serve.Flags().Int("failureThreshold", 2, "Default Number of consecutive failures required to fail a check")
}
