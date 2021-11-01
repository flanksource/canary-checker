package cmd

import (
	"fmt"
	"io/fs"
	nethttp "net/http"
	_ "net/http/pprof" // required by serve

	"github.com/flanksource/canary-checker/pkg/changes"

	"github.com/flanksource/canary-checker/pkg/details"
	"github.com/flanksource/canary-checker/pkg/runner"
	"github.com/flanksource/canary-checker/pkg/spec"

	"github.com/flanksource/canary-checker/pkg/push"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
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
	Use:   "serve config.yaml",
	Short: "Start a server to execute checks ",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, configFiles []string) {
		var canaries []v1.Canary
		for _, configfile := range configFiles {
			configs, err := pkg.ParseConfig(configfile, dataFile)
			if err != nil {
				logger.Fatalf("could not parse %s: %v", configfile, err)
			}
			canaries = append(canaries, configs...)
		}
		kommonsClient, err := pkg.NewKommonsClient()
		if err != nil {
			logger.Warnf("Failed to get kommons client, features that read kubernetes config will fail: %v", err)
		}
		cron := cron.New()

		for _, canary := range canaries {
			if schedule == "" {
				if canary.Spec.Schedule != "" {
					schedule = canary.Spec.Schedule
				} else if canary.Spec.Interval > 0 {
					schedule = fmt.Sprintf("@every %ds", canary.Spec.Interval)
				}
			}
			canary.SetRunnerName(runner.RunnerName)
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
							cache.CacheChain.Add(pkg.FromV1(canary, result.Check), pkg.FromResult(*result))
							metrics.Record(canary, result)
							push.Queue(pkg.FromV1(canary, result.Check), pkg.FromResult(*result))
						}
					}()
				})
			}
		}
		cron.Start()
		serve()
	},
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
	if cache.PostgresConnectionString != "" {
		conn, err := cache.InitPostgres(cache.PostgresConnectionString)
		if err != nil {
			logger.Debugf("error connecting with postgres. Only using in-memory cache: %v", err)
		}
		if conn != nil {
			// needs to be implemented
			cache.PostgresCache.Conn = conn
			cache.CacheChain.Chain = append(cache.CacheChain.Chain, cache.PostgresCache)
			for _, check := range cache.PostgresCache.GetChecks() {
				cache.InMemoryCache.Checks[check.Key] = check
			}
		}
	}
	push.AddServers(pushServers)
	go push.Start()

	runner.Prometheus, _ = prometheus.NewPrometheusAPI(prometheusURL)

	nethttp.HandleFunc("/", stripQuery(nethttp.FileServer(staticRoot).ServeHTTP))
	nethttp.HandleFunc("/api", simpleCors(api.Handler, allowedCors))
	nethttp.HandleFunc("/api/triggerCheck", simpleCors(api.TriggerCheckHandler, allowedCors))
	nethttp.HandleFunc("/api/prometheus/graph", simpleCors(api.PrometheusGraphHandler, allowedCors))
	nethttp.HandleFunc("/api/push", simpleCors(push.Handler, allowedCors))
	nethttp.HandleFunc("/api/details", simpleCors(details.Handler, allowedCors))
	nethttp.HandleFunc("/api/spec", simpleCors(spec.CheckHandler, allowedCors))
	nethttp.HandleFunc("/api/spec/canary", simpleCors(spec.CanaryHandler, allowedCors))
	nethttp.HandleFunc("/api/changes", simpleCors(changes.Handler, allowedCors))
	addr := fmt.Sprintf("0.0.0.0:%d", httpPort)
	logger.Infof("Starting health dashboard at http://%s", addr)
	logger.Infof("Metrics can be accessed at http://%s/metrics", addr)

	if err := nethttp.ListenAndServe(addr, nil); err != nil {
		logger.Fatalf("failed to start server: %v", err)
	}
}

// stripQuery removes query parameters for static sites
func stripQuery(f nethttp.HandlerFunc) nethttp.HandlerFunc {
	return func(w nethttp.ResponseWriter, r *nethttp.Request) {
		r.URL.RawQuery = ""
		f(w, r)
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
	Serve.Flags().StringVarP(&configFile, "configfile", "c", "", "Specify configfile")
	Serve.Flags().StringVarP(&schedule, "schedule", "s", "", "schedule to run checks on. Supports all cron expression and golang duration support in format: '@every duration'")
}
