package cmd

import (
	"fmt"
	"io/fs"
	nethttp "net/http"
	_ "net/http/pprof" // required by serve
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

		canaryName, _ := cmd.Flags().GetString("canary-name")
		canaryNamespace, _ := cmd.Flags().GetString("canary-namespace")
		canary := v1.Canary{
			ObjectMeta: metav1.ObjectMeta{
				Name:      canaryName,
				Namespace: canaryNamespace,
			},
		}
		kommonsClient, err := pkg.NewKommonsClient()
		if err != nil {
			logger.Warnf("Failed to get kommons client, features that read kubernetes config will fail: %v", err)
		}

		config.SetNamespaces(canaryNamespace)
		for _, _c := range checks.All {
			c := _c
			switch cs := c.(type) {
			case checks.SetsClient:
				cs.SetClient(kommonsClient)
			}
			scheduler.Every(interval).Seconds().StartImmediately().Do(func() { // nolint: errcheck
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
	devGuiHTTPPort, _ := cmd.Flags().GetInt("devGuiHTTPPort")

	var staticRoot nethttp.FileSystem
	var allowedCors string

	if dev {
		staticRoot = nethttp.Dir("./statuspage/dist")
		allowedCors = fmt.Sprintf("http://localhost:%d", devGuiHTTPPort)
		logger.Infof("Starting in local development mode")
		logger.Infof("Allowing access from a GUI on %s", allowedCors)
		logger.Infof("   (it can be started with:")
		logger.Infof("    npm run serve -- --port %d ", devGuiHTTPPort)
		logger.Infof("   )")
	} else {
		fs, err := fs.Sub(statuspage.StaticContent, "dist")
		if err != nil {
			logger.Errorf("Error: %v", err)
		}
		staticRoot = nethttp.FS(fs)
		allowedCors = ""
	}

	nethttp.Handle("/metrics", promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{}))

	prometheusHost, _ := cmd.Flags().GetString("prometheus")

	nethttp.Handle("/", nethttp.FileServer(staticRoot))
	nethttp.HandleFunc("/api", simpleCors(api.Handler, allowedCors))
	nethttp.HandleFunc("/api/triggerCheck", simpleCors(api.TriggerCheckHandler, allowedCors))
	nethttp.HandleFunc("/api/prometheus/graph", simpleCors(api.PrometheusGraphHandler(prometheusHost), allowedCors))
	nethttp.HandleFunc("/api/aggregate", simpleCors(aggregate.Handler, allowedCors))

	addr := fmt.Sprintf("0.0.0.0:%d", httpPort)
	logger.Infof("Starting health dashboard at http://%s", addr)
	logger.Infof("Metrics dashboard can be accessed at http://%s/metrics", addr)

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
	Serve.Flags().StringP("configfile", "c", "", "Specify configfile")
	Serve.Flags().Int("httpPort", 8080, "Port to expose a health dashboard ")
	Serve.Flags().Int("devGuiHttpPort", 8081, "Port used by a local npm server in development mode")
	Serve.Flags().Uint64("interval", 30, "Default interval (in seconds) to run checks on")
	Serve.Flags().Int("failureThreshold", 2, "Default Number of consecutive failures required to fail a check")
	Serve.Flags().Bool("dev", false, "Run in development mode")
	Serve.Flags().String("prometheus", "http://localhost:8080", "Prometheus address")
	Serve.Flags().IntVar(&cache.Size, "maxStatusCheckCount", 5, "Maximum number of past checks in the status page")
	Serve.Flags().StringSliceVar(&aggregate.Servers, "aggregateServers", []string{}, "Aggregate check results from multiple servers in the status page")
	Serve.Flags().StringVar(&api.ServerName, "name", "local", "Server name shown in aggregate dashboard")

	Serve.Flags().String("canary-name", "", "Canary name")
	Serve.Flags().String("canary-namespace", "", "Canary namespace")
}
