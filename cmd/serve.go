package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	nethttp "net/http"
	_ "net/http/pprof" // required by serve
	"os"
	"os/signal"

	"github.com/flanksource/canary-checker/pkg/controllers"
	"github.com/flanksource/canary-checker/pkg/db"

	"github.com/flanksource/canary-checker/pkg/details"
	"github.com/flanksource/canary-checker/pkg/runner"

	"github.com/flanksource/canary-checker/pkg/push"

	"github.com/flanksource/canary-checker/pkg/api"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/prometheus"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/flanksource/canary-checker/ui"
	"github.com/flanksource/commons/logger"
	"github.com/spf13/cobra"
)

var schedule, configFile string
var executor bool

var Serve = &cobra.Command{
	Use:   "serve config.yaml",
	Short: "Start a server to execute checks",
	Run: func(cmd *cobra.Command, configFiles []string) {
		setup()
		controllers.StartScanCanaryConfigs(dataFile, configFiles)
		if executor {
			controllers.Start()
		}
		serve()
	},
}

func setup() {
	if err := db.Init(); err != nil {
		logger.Fatalf("error connecting to db %v", err)
	}
	cache.PostgresCache = cache.NewPostgresCache(db.Pool)
	push.AddServers(pushServers)
	go push.Start()

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

	if db.ConnectionString != "" {
		cache.PostgresCache = cache.NewPostgresCache(db.Pool)
	}

	push.AddServers(pushServers)
	go push.Start()

	runner.Prometheus, _ = prometheus.NewPrometheusAPI(prometheusURL)

	mux := http.NewServeMux()
	mux.HandleFunc("/", stripQuery(nethttp.FileServer(staticRoot).ServeHTTP))
	mux.HandleFunc("/about", simpleCors(api.About, allowedCors))
	mux.HandleFunc("/api", simpleCors(api.CheckSummary, allowedCors))
	mux.HandleFunc("/api/graph", simpleCors(api.CheckDetails, allowedCors))
	mux.HandleFunc("/api/triggerCheck", simpleCors(api.TriggerCheckHandler, allowedCors))
	mux.HandleFunc("/api/prometheus/graph", simpleCors(api.PrometheusGraphHandler, allowedCors))
	mux.HandleFunc("/api/push", simpleCors(push.Handler, allowedCors))
	mux.HandleFunc("/api/details", simpleCors(details.Handler, allowedCors))
	mux.HandleFunc("/api/changes", simpleCors(api.Changes, allowedCors))
	mux.HandleFunc("/api/topology", simpleCors(api.Topology, allowedCors))
	mux.Handle("/metrics", promhttp.HandlerFor(prom.DefaultGatherer, promhttp.HandlerOpts{}))
	addr := fmt.Sprintf("0.0.0.0:%d", httpPort)

	var srv = nethttp.Server{
		Addr:    addr,
		Handler: mux,
	}

	logger.Infof("Starting health dashboard at http://%s", addr)
	logger.Infof("Metrics can be accessed at http://%s/metrics", addr)

	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint

		// We received an interrupt signal, shut down.
		if err := srv.Shutdown(context.Background()); err != nil {
			// Error from closing listeners, or context timeout:
			logger.Infof("HTTP server Shutdown: %v", err)
		}
		close(idleConnsClosed)
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		// Error starting or closing listener:
		log.Fatalf("HTTP server ListenAndServe: %v", err)
	}

	<-idleConnsClosed

	if err := db.StopServer(); err != nil {
		logger.Errorf("Error stopping embedded postgres: %v", err)
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
	Serve.Flags().BoolVar(&executor, "executor", true, "If false, only serve the UI and sync the configs")
	Serve.Flags().StringVarP(&configFile, "configfile", "c", "", "Specify configfile")
	Serve.Flags().StringVarP(&schedule, "schedule", "s", "", "schedule to run checks on. Supports all cron expression and golang duration support in format: '@every duration'")
}
