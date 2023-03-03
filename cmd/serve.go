package cmd

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof" // required by serve
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	echopprof "github.com/sevennt/echo-pprof"

	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/jobs"
	canaryJobs "github.com/flanksource/canary-checker/pkg/jobs/canary"

	"github.com/flanksource/canary-checker/pkg/runner"

	"github.com/flanksource/canary-checker/pkg/push"

	"github.com/flanksource/canary-checker/pkg/api"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/prometheus"
	"github.com/flanksource/commons/logger"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
)

var schedule, configFile string
var executor, debug bool

var Serve = &cobra.Command{
	Use:   "serve config.yaml",
	Short: "Start a server to execute checks",
	Run: func(cmd *cobra.Command, configFiles []string) {
		setup()
		canaryJobs.StartScanCanaryConfigs(dataFile, configFiles)
		if executor {
			jobs.Start()
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
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{allowedCors},
	}))
	if db.ConnectionString != "" {
		cache.PostgresCache = cache.NewPostgresCache(db.Pool)
	}

	// PostgREST needs to know how it is exposed to create the correct links
	db.HTTPEndpoint = publicEndpoint + "/db"

	push.AddServers(pushServers)
	go push.Start()

	runner.Prometheus, _ = prometheus.NewPrometheusAPI(prometheusURL)

	if debug {
		logger.Infof("Starting pprof at /debug")
		echopprof.Wrap(e)
	}

	if !disablePostgrest {
		go db.StartPostgrest()
		forward(e, "/db", db.PostgRESTEndpoint())
	}

	e.Use(middleware.Logger())

	e.GET("/api", api.CheckSummary)
	e.GET("/api/summary", api.HealthSummary)
	e.GET("/about", api.About)
	e.GET("/api/graph", api.CheckDetails)
	e.POST("/api/push", api.PushHandler)
	e.GET("/api/details", api.DetailsHandler)
	e.GET("/api/topology", api.Topology)
	e.GET("/metrics", echo.WrapHandler(promhttp.HandlerFor(prom.DefaultGatherer, promhttp.HandlerOpts{})))
	e.GET("/api/changes", api.Changes)
	e.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	// Start server
	go func() {
		if err := e.Start(fmt.Sprintf(":%d", httpPort)); err != nil && err != http.ErrServerClosed {
			e.Logger.Fatal(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	logger.Infof("Shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()
	if err := db.StopServer(); err != nil {
		e.Logger.Fatal("Error stopping embedded postgres: %v", err)
	}
	if err := e.Shutdown(ctx); err != nil {
		e.Logger.Fatal(err)
	}
}

func forward(e *echo.Echo, prefix string, target string) {
	targetURL, err := url.Parse(target)
	if err != nil {
		e.Logger.Fatal(err)
	}
	e.Group(prefix).Use(middleware.ProxyWithConfig(middleware.ProxyConfig{
		Rewrite: map[string]string{
			fmt.Sprintf("^%s/*", prefix): "/$1",
		},
		Balancer: middleware.NewRoundRobinBalancer([]*middleware.ProxyTarget{
			{
				URL: targetURL,
			},
		}),
	}))
}

func init() {
	ServerFlags(Serve.Flags())
	debugDefault := os.Getenv("DEBUG") == "true"
	Serve.Flags().BoolVar(&executor, "executor", true, "If false, only serve the UI and sync the configs")
	Serve.Flags().BoolVar(&debug, "debug", debugDefault, "If true, start pprof at /debug")
	Serve.Flags().StringVarP(&configFile, "configfile", "c", "", "Specify configfile")
	Serve.Flags().StringVarP(&schedule, "schedule", "s", "", "schedule to run checks on. Supports all cron expression and golang duration support in format: '@every duration'")
	Serve.Flags().BoolVar(&disablePostgrest, "disable-postgrest", false, "Disable the postgrest server")
}
