package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/jobs/canary"
	"github.com/flanksource/canary-checker/pkg/prometheus"
	"github.com/flanksource/canary-checker/pkg/runner"
	"github.com/flanksource/canary-checker/pkg/telemetry"
	"github.com/flanksource/commons/http"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/connection"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/query"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel"
)

func InitContext() (context.Context, error) {
	kommonsClient, k8s, err := pkg.NewKommonsClient()
	if err != nil {
		logger.Warnf("Failed to get kubernetes client: %v", err)
	}

	var ctx context.Context

	runner.AddShutdownHook(func() {
		if err := db.StopServer(); err != nil {
			logger.Errorf("failed to stop db, %v", err)
		}
	})

	if ctx, err = db.Init(); err != nil {
		logger.Warnf("error connecting to db %v", err)
		ctx = context.New()
	} else {
		if err := context.LoadPropertiesFromFile(ctx, propertiesFile); err != nil {
			return ctx, errors.Wrap(err, "Error loading properties")
		}
	}

	ctx.WithTracer(otel.GetTracerProvider().Tracer("canary-checker"))
	return ctx.
		WithKubernetes(k8s).
		WithKommons(kommonsClient), nil
}

var Root = &cobra.Command{
	Use: "canary-checker",
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		runner.Shutdown()
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logger.UseZap()

		canary.LogFail = logFail || logger.IsLevelEnabled(3)
		canary.LogPass = logPass || logger.IsLevelEnabled(4)

		db.ConnectionString = readFromEnv(db.ConnectionString)
		if db.ConnectionString == "DB_URL" {
			db.ConnectionString = ""
		}

		if canary.UpstreamConf.Valid() {
			canary.UpstreamConf.Options = append(canary.UpstreamConf.Options, func(c *http.Client) {
				c.UserAgent(fmt.Sprintf("canary-checker %s", runner.Version))
			})
			logger.Infof("Pushing checks %s", canary.UpstreamConf)
		} else if partial, err := canary.UpstreamConf.IsPartiallyFilled(); partial && err != nil {
			logger.Warnf("Upstream not fully configured: %s", canary.UpstreamConf)
		}

		if otelcollectorURL != "" {
			logger.Infof("Sending traces to %s", otelcollectorURL)

			runner.AddShutdownHook(telemetry.InitTracer(otelServiceName, otelcollectorURL, true))
		}
		if prometheus.PrometheusURL != "" {
			logger.Infof("Setting default prometheus: %s", prometheus.PrometheusURL)
			runner.Prometheus, _ = prometheus.NewPrometheusAPI(context.New(), connection.HTTPConnection{URL: prometheus.PrometheusURL})
		}

		go func() {
			quit := make(chan os.Signal, 1)
			signal.Notify(quit, os.Interrupt)
			<-quit
			logger.Infof("Caught Ctrl+C")
			// call shutdown hooks explicitly, post-run cleanup hooks will be a no-op
			runner.Shutdown()
		}()
	},
}

var (
	httpPort         = 8080
	publicEndpoint   = "http://localhost:8080"
	logPass, logFail bool

	otelcollectorURL string
	otelServiceName  string
)

func ServerFlags(flags *pflag.FlagSet) {
	flags.IntVar(&httpPort, "httpPort", httpPort, "Port to expose a health dashboard ")

	_ = flags.MarkDeprecated("devGuiPort", "")
	_ = flags.MarkDeprecated("metricsPort", "Extra metrics server removed")
	_ = flags.MarkDeprecated("dev", "")
	_ = flags.MarkDeprecated("push-servers", "")
	_ = flags.MarkDeprecated("pull-servers", "")
	_ = flags.MarkDeprecated("expose-env", "")
	_ = flags.MarkDeprecated("shared-library", "")
	_ = flags.MarkDeprecated("maxStatusCheckCount", "")
	_ = flags.MarkDeprecated("check-retention-period", "")
	_ = flags.MarkDeprecated("component-retention-period", "")
	_ = flags.MarkDeprecated("canary-retention-period", "")
	_ = flags.MarkDeprecated("check-status-retention-period", "")

	flags.StringVar(&publicEndpoint, "public-endpoint", publicEndpoint, "Host on which the health dashboard is exposed. Could be used for generting-links, redirects etc.")
	flags.StringSliceVar(&runner.IncludeCanaries, "include-check", []string{}, "Run matching canaries - useful for debugging")
	flags.StringSliceVar(&runner.IncludeTypes, "include-type", []string{}, "Check type to disable")
	flags.StringSliceVar(&runner.IncludeNamespaces, "include-namespace", []string{}, "Check type to disable")
	flags.StringVar(&runner.RunnerName, "name", "local", "Server name shown in aggregate dashboard")

	flags.IntVar(&db.DefaultExpiryDays, "cache-timeout", 90, "Cache timeout in days")
	flags.StringVarP(&query.DefaultCheckQueryWindow, "default-window", "", "1h", "Default search window")
	flags.StringVar(&checks.DefaultArtifactConnection, "artifact-connection", "", "Specify the default connection to use for artifacts")

	flags.IntVar(&canary.ReconcilePageSize, "upstream-page-size", 500, "upstream reconciliation page size")
	flags.DurationVar(&canary.ReconcileMaxAge, "upstream-max-age", time.Hour*48, "upstream reconciliation max age")
	flags.StringVar(&canary.UpstreamConf.Host, "upstream-host", os.Getenv("UPSTREAM_HOST"), "central canary checker instance to push/pull canaries")
	flags.StringVar(&canary.UpstreamConf.Username, "upstream-user", os.Getenv("UPSTREAM_USER"), "upstream username")
	flags.StringVar(&canary.UpstreamConf.Password, "upstream-password", os.Getenv("UPSTREAM_PASSWORD"), "upstream password")
	flags.StringVar(&canary.UpstreamConf.AgentName, "agent-name", os.Getenv("AGENT_NAME"), "name of this agent")
	flags.BoolVar(&canary.UpstreamConf.InsecureSkipVerify, "upstream-insecure-skip-verify", os.Getenv("UPSTREAM_INSECURE_SKIP_VERIFY") == "true", "Skip TLS verification on the upstream servers certificate")
}

func readFromEnv(v string) string {
	val := os.Getenv(v)
	if val != "" {
		return val
	}
	return v
}

func init() {
	logger.BindFlags(Root.PersistentFlags())
	duty.BindFlags(Root.PersistentFlags())

	Root.PersistentFlags().StringVar(&db.ConnectionString, "db", "DB_URL", "Connection string for the postgres database")
	Root.PersistentFlags().BoolVar(&db.RunMigrations, "db-migrations", false, "Run database migrations")
	Root.PersistentFlags().BoolVar(&db.DBMetrics, "db-metrics", false, "Expose db metrics")
	Root.PersistentFlags().BoolVar(&logFail, "log-fail", false, "Log every failing check")
	Root.PersistentFlags().BoolVar(&logPass, "log-pass", false, "Log every passing check")
	Root.PersistentFlags().StringVar(&otelcollectorURL, "otel-collector-url", "", "OpenTelemetry gRPC Collector URL in host:port format")
	Root.PersistentFlags().StringVar(&otelServiceName, "otel-service-name", "canary-checker", "OpenTelemetry service name for the resource")
	Root.PersistentFlags().StringVar(&prometheus.PrometheusURL, "prometheus", "", "URL of the prometheus server that is scraping this instance")
	Root.AddCommand(Docs)
	Root.AddCommand(Run, Serve, Operator)
	Root.AddCommand(Serve, GoOffline)
}
