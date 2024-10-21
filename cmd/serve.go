package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof" // required by serve
	"os"
	"strings"
	"time"

	apicontext "github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/echo"
	"github.com/flanksource/canary-checker/pkg/jobs"
	canaryJobs "github.com/flanksource/canary-checker/pkg/jobs/canary"
	echov4 "github.com/labstack/echo/v4"

	"github.com/flanksource/canary-checker/pkg/runner"

	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/commons/logger"
	dutyApi "github.com/flanksource/duty/api"
	dutyEcho "github.com/flanksource/duty/echo"
	"github.com/flanksource/duty/postgrest"
	"github.com/flanksource/duty/shutdown"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
)

var (
	schedule, configFile string
	executor             bool
	propertiesFile       = "canary-checker.properties"
)

var Serve = &cobra.Command{
	Use:   "serve config.yaml",
	Short: "Start a server to execute checks",
	Run: func(cmd *cobra.Command, configFiles []string) {
		if err := setupDefaultContext(); err != nil {
			shutdown.ShutdownAndExit(1, err.Error())
		}

		canaryJobs.StartScanCanaryConfigs(apicontext.DefaultContext, dataFile, configFiles)
		if executor {
			jobs.Start()
		}

		serve()
	},
}

func setupDefaultContext() error {
	var err error

	if apicontext.DefaultContext, err = InitContext(); err != nil {
		return err
	}

	apicontext.DefaultContext = apicontext.DefaultContext.WithNamespace(runner.WatchNamespace)

	cache.PostgresCache = cache.NewPostgresCache(apicontext.DefaultContext)

	return nil
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
			if _, _, err := db.PersistCanaryModel(apicontext.DefaultContext, c); err != nil {
				logger.Errorf("Error persisting canary[%s]: %v", c.ID, err)
			}
		}
	}

	return nil
}

func serve() {
	e := echo.New(apicontext.DefaultContext)

	dutyEcho.AddDebugHandlers(apicontext.DefaultContext, e, func(next echov4.HandlerFunc) echov4.HandlerFunc { return next })

	e.GET("/metrics", echov4.WrapHandler(promhttp.HandlerFor(prom.DefaultGatherer, promhttp.HandlerOpts{})))

	if dutyApi.DefaultConfig.Postgrest.URL != "" {
		echo.Forward(e, "/db", postgrest.PostgRESTEndpoint(dutyApi.DefaultConfig), postgrestResponseModifier)
	} else {
		logger.Tracef("No PostgREST endpoint configured, skipping proxy")
	}

	shutdown.AddHook(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()

		if err := e.Shutdown(ctx); err != nil {
			e.Logger.Fatal(err)
		}
	})

	if err := e.Start(fmt.Sprintf(":%d", httpPort)); err != nil && err != http.ErrServerClosed {
		e.Logger.Fatal(err)
	}
}

func init() {
	ServerFlags(Serve.Flags())
	debugDefault := os.Getenv("DEBUG") == "true"
	Serve.Flags().BoolVar(&executor, "executor", true, "If false, only serve the UI and sync the configs")
	Serve.Flags().BoolVar(&echo.Debug, "debug", debugDefault, "If true, start pprof at /debug")
	Serve.Flags().StringVar(&echo.AllowedCORS, "allowed-cors", "", "Allowed CORS origin headers")
	Serve.Flags().StringVarP(&configFile, "configfile", "c", "", "Specify configfile")
	Serve.Flags().StringVarP(&schedule, "schedule", "s", "", "schedule to run checks on. Supports all cron expression and golang duration support in format: '@every duration'")
}
