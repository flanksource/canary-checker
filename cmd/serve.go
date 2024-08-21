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
	"github.com/flanksource/canary-checker/pkg/jobs/topology"
	echov4 "github.com/labstack/echo/v4"

	"github.com/flanksource/canary-checker/pkg/runner"

	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/api"
	dutyContext "github.com/flanksource/duty/context"
	"github.com/flanksource/duty/job"
	"github.com/flanksource/duty/postgrest"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
)

var schedule, configFile string
var executor bool
var propertiesFile = "canary-checker.properties"

var Serve = &cobra.Command{
	Use:   "serve config.yaml",
	Short: "Start a server to execute checks",
	Run: func(cmd *cobra.Command, configFiles []string) {
		defer runner.Shutdown()
		canaryJobs.StartScanCanaryConfigs(setup(), dataFile, configFiles)
		if executor {
			jobs.Start()
		}
		serve()
	},
}

func setup() dutyContext.Context {
	var err error

	if apicontext.DefaultContext, err = InitContext(); err != nil {
		runner.ShutdownAndExit(1, err.Error())
		return apicontext.DefaultContext
	}

	apicontext.DefaultContext = apicontext.DefaultContext.WithNamespace(runner.WatchNamespace)

	cache.PostgresCache = cache.NewPostgresCache(apicontext.DefaultContext)

	return apicontext.DefaultContext
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

	// PostgREST needs to know how it is exposed to create the correct links
	db.HTTPEndpoint = publicEndpoint + "/db"

	e.GET("/metrics", echov4.WrapHandler(promhttp.HandlerFor(prom.DefaultGatherer, promhttp.HandlerOpts{})))

	if !api.DefaultConfig.Postgrest.Disable {
		echo.Forward(e, "/db", postgrest.PostgRESTEndpoint(api.DefaultConfig), postgrestResponseModifier)
	}

	e.GET("/jobs", job.CronDetailsHandler(
		canaryJobs.CanaryScheduler,
		canaryJobs.FuncScheduler,
		jobs.FuncScheduler,
		topology.TopologyScheduler,
	))

	runner.AddShutdownHook(func() {
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
