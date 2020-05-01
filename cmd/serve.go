package cmd

import (
	"fmt"
	nethttp "net/http"
	"strconv"
	"time"

	"github.com/jasonlvhit/gocron"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	_ "net/http/pprof"

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
			&checks.HelmChecker{},
			&checks.DNSChecker{},
			&checks.HttpChecker{},
			&checks.IcmpChecker{},
			&checks.S3Checker{},
			&checks.S3BucketChecker{},
			&checks.DockerPullChecker{},
			&checks.DockerPushChecker{},
			&checks.PostgresChecker{},
			&checks.LdapChecker{},
			checks.NewPodChecker(),
		}

		config.Interval = time.Duration(interval) * time.Second
		log.Infof("Running checks every %s", config.Interval)

		for _, _c := range checks {
			c := _c
			var results = make(chan *pkg.CheckResult)
			gocron.Every(interval).Seconds().From(gocron.NextTick()).Do(func() {
				go func() {
					c.Run(config, results)
				}()
			})
			go func() {
				for result := range results {
					processMetrics(c.Type(), result)
				}
			}()
		}

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

func processMetrics(checkType string, result *pkg.CheckResult) {
	description := ""
	switch result.Check.(type) {
	case pkg.Describable:
		description = result.Check.(pkg.Describable).GetDescription()
	}
	if log.IsLevelEnabled(log.InfoLevel) {
		fmt.Println(result)
	}
	pkg.OpsCount.WithLabelValues(checkType, result.Endpoint, description).Inc()
	if result.Pass {
		pkg.Guage.WithLabelValues(checkType, description).Set(0)
		pkg.OpsSuccessCount.WithLabelValues(checkType, result.Endpoint, description).Inc()
		if result.Duration > 0 {
			pkg.RequestLatency.WithLabelValues(checkType, result.Endpoint, description).Observe(float64(result.Duration))
		}

		for _, m := range result.Metrics {
			switch m.Type {
			case pkg.CounterType:
				pkg.GenericCounter.WithLabelValues(checkType, description, m.Name, strconv.Itoa(int(m.Value))).Inc()
			case pkg.GaugeType:
				pkg.GenericGauge.WithLabelValues(checkType, description, m.Name).Set(m.Value)
			case pkg.HistogramType:
				pkg.GenericHistogram.WithLabelValues(checkType, description, m.Name).Observe(m.Value)
			}
		}
	} else {
		pkg.Guage.WithLabelValues(checkType, description).Set(1)
		pkg.OpsFailedCount.WithLabelValues(checkType, result.Endpoint, description).Inc()
	}
}

func init() {
	Serve.Flags().Int("httpPort", 8080, "Port to expose a health dashboard ")
	Serve.Flags().Uint64("interval", 30, "Default interval (in seconds) to run checks on")
	Serve.Flags().Int("failureThreshold", 2, "Default Number of consecutive failures required to fail a check")
}
