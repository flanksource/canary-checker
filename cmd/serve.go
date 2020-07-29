package cmd

import (
	"fmt"
	nethttp "net/http"
	_ "net/http/pprof"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/aggregate"
	"github.com/flanksource/canary-checker/pkg/api"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/canary-checker/statuspage"
	"github.com/flanksource/commons/logger"
	"github.com/go-co-op/gocron"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
)

var Serve = &cobra.Command{
	Use:   "serve",
	Short: "Start a server to execute checks ",
	Run: func(cmd *cobra.Command, args []string) {
		configfile, _ := cmd.Flags().GetString("configfile")
		config := pkg.ParseConfig(configfile)

		interval, _ := cmd.Flags().GetUint64("interval")

		config.Interval = interval
		logger.Infof("Running checks every %d seconds", config.Interval)

		scheduler := gocron.NewScheduler(time.UTC)

		canary := v1.Canary{}
		for _, _c := range checks.All {
			c := _c
			scheduler.Every(interval).Seconds().StartImmediately().Do(func() {
				go func() {
					for _, result := range c.Run(config) {
						cache.AddCheck(canary, result)
						metrics.Record(canary, result)
					}
				}()
			})
		}

		scheduler.StartAsync()
		serve(cmd)
	},
}

func serve(cmd *cobra.Command) {
	httpPort, _ := cmd.Flags().GetInt("httpPort")
	dev, _ := cmd.Flags().GetBool("dev")
	nethttp.Handle("/metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{}))
	var staticRoot nethttp.FileSystem
	if dev {
		staticRoot = nethttp.Dir("./statuspage")
	} else {
		staticRoot = statuspage.FS(false)
	}
	nethttp.Handle("/", nethttp.FileServer(staticRoot))
	nethttp.HandleFunc("/api", api.Handler)
	nethttp.HandleFunc("/api/triggerCheck", api.TriggerCheckHandler)
	nethttp.HandleFunc("/api/aggregate", aggregate.Handler)

	addr := fmt.Sprintf("0.0.0.0:%d", httpPort)
	logger.Infof("Starting health dashboard at http://%s", addr)
	logger.Infof("Metrics dashboard can be accessed at http://%s/metrics", addr)

	if err := nethttp.ListenAndServe(addr, nil); err != nil {
		logger.Fatalf("failed to start server: %v", err)
	}
}

func init() {
	Serve.Flags().StringP("configfile", "c", "", "Specify configfile")
	Serve.Flags().Int("httpPort", 8080, "Port to expose a health dashboard ")
	Serve.Flags().Uint64("interval", 30, "Default interval (in seconds) to run checks on")
	Serve.Flags().Int("failureThreshold", 2, "Default Number of consecutive failures required to fail a check")
	Serve.Flags().Bool("dev", false, "Run in development mode")
	Serve.Flags().IntVar(&cache.Size, "maxStatusCheckCount", 5, "Maximum number of past checks in the status page")
	Serve.Flags().StringSliceVar(&aggregate.Servers, "aggregateServers", []string{}, "Aggregate check results from multiple servers in the status page")
	Serve.Flags().StringVar(&api.ServerName, "name", "local", "Server name shown in aggregate dashboard")
}
