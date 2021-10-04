package cmd

import (
	"fmt"
	"io/fs"
	nethttp "net/http"
	_ "net/http/pprof" // required by serve

	"github.com/flanksource/canary-checker/pkg/details"
	"github.com/flanksource/canary-checker/pkg/runner"

	"github.com/flanksource/canary-checker/pkg/push"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/api"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/canary-checker/pkg/prometheus"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/flanksource/canary-checker/ui"
	"github.com/flanksource/commons/logger"
	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"
)

var schedule, configFile string

var Serve = &cobra.Command{
	Use:   "serve",
	Short: "Start a server to execute checks ",
	Run:   serverRun,
}

func serverRun(cmd *cobra.Command, args []string) {

	configs, err := pkg.ParseConfig(configFile)
	if err != nil {
		logger.Fatalf("could not parse %s: %v", configFile, err)
	}
	kommonsClient, err := pkg.NewKommonsClient()
	if err != nil {
		logger.Warnf("Failed to get kommons client, features that read kubernetes config will fail: %v", err)
	}
	cron := cron.New()
	cron.Start()

	for _, canary := range configs {
		// if canary.Spec.Interval == 0 && canary.Spec.Schedule == "" {
		// 	canary.Spec.Schedule = schedule
		// }
		if canary.Spec.Schedule != "" {
			schedule = canary.Spec.Schedule
		} else {
			schedule = fmt.Sprintf("@every %ds", canary.Spec.Interval)
		}
		for _, _c := range checks.All {
			c := _c
			if !checks.Checks(canary.Spec.GetAllChecks()).Includes(c) {
				continue
			}
			cron.AddFunc(schedule, func() { // nolint: errcheck
				go func() {
					for _, result := range checks.RunChecks(context.New(kommonsClient, canary)) {
						if logPass && result.Pass || logFail && !result.Pass {
							logger.Infof(result.String())
						}
						cache.AddCheck(canary, result)
						metrics.Record(canary, result)
					}
				}()
			})
		}
		fmt.Println("added checks to the serve")
	}
	serve()
}

func serve() {
	var staticRoot nethttp.FileSystem
	var allowedCors string

	if dev {
		staticRoot = nethttp.Dir("./ui/build")
		allowedCors = fmt.Sprintf("http://localhost:%d", devGuiPort)
	} else {
		fs, err := fs.Sub(ui.StaticContent, "build")
		if err != nil {
			logger.Errorf("Error: %v", err)
		}
		staticRoot = nethttp.FS(fs)
		allowedCors = ""
	}

	nethttp.Handle("/metrics", promhttp.HandlerFor(prom.DefaultGatherer, promhttp.HandlerOpts{}))

	push.AddServers(pushServers)
	go push.Start()

	runner.Prometheus, _ = prometheus.NewPrometheusAPI(prometheusURL)

	nethttp.Handle("/", nethttp.FileServer(staticRoot))
	nethttp.HandleFunc("/api", simpleCors(api.Handler, allowedCors))
	nethttp.HandleFunc("/api/triggerCheck", simpleCors(api.TriggerCheckHandler, allowedCors))
	nethttp.HandleFunc("/api/prometheus/graph", simpleCors(api.PrometheusGraphHandler, allowedCors))
	nethttp.HandleFunc("/api/push", simpleCors(push.Handler, allowedCors))
	nethttp.HandleFunc("/api/details", simpleCors(details.Handler, allowedCors))
	addr := fmt.Sprintf("0.0.0.0:%d", httpPort)
	logger.Infof("Starting health dashboard at http://%s", addr)
	logger.Infof("Metrics can be accessed at http://%s/metrics", addr)

	if err := nethttp.ListenAndServe(addr, nil); err != nil {
		logger.Fatalf("failed to start server: %v", err)
	}
}

// simpleCors is minimal middleware for injecting an Access-Control-Allow-Origin header value.
// If an empty allowedOrigin is specified, then no header is added.
func simpleCors(f nethttp.HandlerFunc, allowedOrigin string) nethttp.HandlerFunc {
	// if not set return a no-op middleware
	if allowedOrigin == "" {
		return func(w nethttp.ResponseWriter, r *nethttp.Request) {
			f(w, r)
		}
	}
	return func(w nethttp.ResponseWriter, r *nethttp.Request) {
		(w).Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		f(w, r)
	}
}

func init() {
	ServerFlags(Serve.Flags())
	Serve.Flags().StringVarP(&configFile, "configfile", "c", "canary-checker.yaml", "Path to the config file")
	// Serve.MarkFlagRequired("configfile") // nolint: errcheck
	Serve.Flags().StringP("schedule", "s", "", "schedule to run checks on. Supports all cron expression and golang duration support in format: '@every duration'")
}
