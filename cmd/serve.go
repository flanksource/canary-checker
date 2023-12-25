package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof" // required by serve
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	echopprof "github.com/sevennt/echo-pprof"
	"go.opentelemetry.io/otel"

	apicontext "github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/jobs"
	canaryJobs "github.com/flanksource/canary-checker/pkg/jobs/canary"
	"github.com/flanksource/duty"

	"github.com/flanksource/canary-checker/pkg/runner"

	"github.com/flanksource/canary-checker/pkg/api"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/prometheus"
	commonsCtx "github.com/flanksource/commons/context"
	"github.com/flanksource/commons/logger"
	dutyContext "github.com/flanksource/duty/context"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
)

var schedule, configFile string
var executor, debug bool
var propertiesFile = "canary-checker.properties"

var Serve = &cobra.Command{
	Use:   "serve config.yaml",
	Short: "Start a server to execute checks",
	Run: func(cmd *cobra.Command, configFiles []string) {
		logger.ParseFlags(cmd.Flags())
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

	kommonsClient, k8s, err := pkg.NewKommonsClient()
	if err != nil {
		logger.Warnf("failed to get kommons client, checks that read kubernetes configs will fail: %v", err)
	}

	apicontext.DefaultContext = dutyContext.NewContext(context.Background(), commonsCtx.WithTracer(otel.GetTracerProvider().Tracer("canary-checker"))).
		WithDB(db.Gorm, db.Pool).
		WithKubernetes(k8s).
		WithKommons(kommonsClient).
		WithNamespace(runner.WatchNamespace)

	if err := duty.UpdatePropertiesFromFile(apicontext.DefaultContext, propertiesFile); err != nil {
		logger.Fatalf("Error setting properties in database: %v", err)
	}
}

func postgrestResponseModifier(r *http.Response) error {
	shouldPersistCanary := r.Request.Method == http.MethodPost &&
		strings.TrimSuffix(r.Request.URL.Path, "/") == "/canaries" &&
		r.StatusCode == http.StatusCreated

	// If a new canary is inserted via postgrest, we need to persist the canary
	// again so that all the checks of that canary are created in the database
	if shouldPersistCanary {
		var canaries []pkg.Canary
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return fmt.Errorf("error reading response body: %w", err)
		}
		if err := json.Unmarshal(body, &canaries); err != nil {
			return fmt.Errorf("error unmarshaling response body to json: %w", err)
		}
		for _, c := range canaries {
			if _, err := db.PersistCanaryModel(apicontext.DefaultContext.DB(), c); err != nil {
				logger.Errorf("Error persisting canary[%s]: %v", c.ID, err)
			}
		}
	}

	return nil
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

	runner.Prometheus, _ = prometheus.NewPrometheusAPI(prometheus.PrometheusURL)

	if debug {
		logger.Infof("Starting pprof at /debug")
		echopprof.Wrap(e)
	}

	if !disablePostgrest {
		go db.StartPostgrest()
		forward(e, "/db", db.PostgRESTEndpoint(), postgrestResponseModifier)
	}

	e.Use(middleware.Logger())

	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.SetRequest(c.Request().WithContext(apicontext.DefaultContext.Wrap(c.Request().Context())))
			return next(c)
		}
	})

	e.GET("/api/summary", api.HealthSummary) // Deprecated: Use Post request for filtering
	e.POST("/api/summary", api.HealthSummary)
	e.GET("/about", api.About)
	e.GET("/api/graph", api.CheckDetails)
	e.POST("/api/push", api.PushHandler)
	e.GET("/api/details", api.DetailsHandler)
	e.GET("/api/topology", api.Topology)

	e.POST("/webhook/:id", api.WebhookHandler)

	e.GET("/metrics", echo.WrapHandler(promhttp.HandlerFor(prom.DefaultGatherer, promhttp.HandlerOpts{})))
	e.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	e.POST("/run/canary/:id", api.RunCanaryHandler)
	e.POST("/run/topology/:id", api.RunTopologyHandler)

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

func forward(e *echo.Echo, prefix string, target string, respModifierFunc func(*http.Response) error) {
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
		ModifyResponse: respModifierFunc,
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
