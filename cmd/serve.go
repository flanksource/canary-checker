package cmd

import (
	"context"
	"fmt"
	"net/http"
	nethttp "net/http"
	_ "net/http/pprof" // required by serve
	"os"
	"os/signal"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/flanksource/canary-checker/pkg/controllers"
	"github.com/flanksource/canary-checker/pkg/db"

	"github.com/flanksource/canary-checker/pkg/runner"

	"github.com/flanksource/canary-checker/pkg/push"

	"github.com/flanksource/canary-checker/pkg/api"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/prometheus"
	"github.com/flanksource/canary-checker/ui"
	"github.com/flanksource/commons/logger"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	var allowedCors string
	e := echo.New()
	if dev {
		e.Static("/*", "./ui/build")
		allowedCors = fmt.Sprintf("http://localhost:%d", devGuiPort)
	} else {
		contentHandler := echo.WrapHandler(http.FileServer(http.FS(ui.StaticContent)))
		var contentRewrite = middleware.Rewrite(map[string]string{"/*": "/build/$1"})
		e.GET("/", contentHandler, contentRewrite, stripQuery)
		allowedCors = ""
	}
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{allowedCors},
	}))
	if db.ConnectionString != "" {
		cache.PostgresCache = cache.NewPostgresCache(db.Pool)
	}

	push.AddServers(pushServers)
	go push.Start()

	runner.Prometheus, _ = prometheus.NewPrometheusAPI(prometheusURL)

	e.GET("/api", api.CheckSummary)
	e.GET("/about", api.About)
	e.GET("/api/graph", api.CheckDetails)
	e.POST("/api/push", api.PushHandler)
	e.GET("/api/details", api.DetailsHandler)
	e.GET("/api/topology", api.Topology)
	e.GET("/metrics", echo.WrapHandler(promhttp.HandlerFor(prom.DefaultGatherer, promhttp.HandlerOpts{})))
	e.GET("/api/changes", api.Changes)
	if err := e.Start(fmt.Sprintf(":%d", httpPort)); err != nil {
		e.Logger.Fatal(err)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()
	if err := db.StopServer(); err != nil {
		e.Logger.Fatal("Error stopping embedded postgres: %v", err)
	}
	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}
}

// stripQuery removes query parameters for static sites
func stripQuery(f echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Request().URL.RawQuery = ""
		return f(c)
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
