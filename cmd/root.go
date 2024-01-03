package cmd

import (
	"os"
	"time"

	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/jobs/canary"
	"github.com/flanksource/canary-checker/pkg/prometheus"
	"github.com/flanksource/canary-checker/pkg/runner"
	"github.com/flanksource/canary-checker/pkg/telemetry"
	"github.com/flanksource/canary-checker/pkg/topology"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/query"
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

	if ctx, err = db.Init(); err != nil {
		logger.Warnf("error connecting to db %v", err)
		ctx = context.New()
	} else {
		if err := context.LoadPropertiesFromFile(ctx, propertiesFile); err != nil {
			logger.Fatalf("Error loading properties: %v", err)
		}
	}

	ctx.WithTracer(otel.GetTracerProvider().Tracer("canary-checker"))
	return ctx.
		WithKubernetes(k8s).
		WithKommons(kommonsClient), nil
}

var Root = &cobra.Command{
	Use: "canary-checker",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		canary.LogFail = logFail
		canary.LogPass = logPass

		logger.UseZap(cmd.Flags())

		db.ConnectionString = readFromEnv(db.ConnectionString)
		if db.ConnectionString == "DB_URL" {
			db.ConnectionString = ""
		}

		if canary.UpstreamConf.Valid() {
			logger.Infof("Pushing checks %s", canary.UpstreamConf)
		} else {
			logger.Debugf("Upstream not fully configured: %s", canary.UpstreamConf)
		}

		if otelcollectorURL != "" {
			telemetry.InitTracer(otelServiceName, otelcollectorURL, true)
		}
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

	flags.Bool("dev", false, "")
	flags.Int("devGuiPort", 3004, "Port used by a local npm server in development mode")
	flags.Int("metricsPort", 8081, "Port to expose a health dashboard ")

	_ = flags.MarkDeprecated("devGuiPort", "")
	_ = flags.MarkDeprecated("metricsPort", "Extra metrics server removed")
	_ = flags.MarkDeprecated("dev", "")
	_ = flags.MarkDeprecated("push-servers", "")
	_ = flags.MarkDeprecated("pull-servers", "")
	_ = flags.MarkDeprecated("expose-env", "")
	_ = flags.MarkDeprecated("shared-library", "")

	flags.StringVar(&publicEndpoint, "public-endpoint", publicEndpoint, "Host on which the health dashboard is exposed. Could be used for generting-links, redirects etc.")
	flags.StringSliceVar(&runner.IncludeCanaries, "include-check", []string{}, "Run matching canaries - useful for debugging")
	flags.StringSliceVar(&runner.IncludeTypes, "include-type", []string{}, "Check type to disable")
	flags.StringSliceVar(&runner.IncludeNamespaces, "include-namespace", []string{}, "Check type to disable")
	flags.IntVar(&query.DefaultCacheCount, "maxStatusCheckCount", 5, "Maximum number of past checks in the in memory cache")
	flags.StringVar(&runner.RunnerName, "name", "local", "Server name shown in aggregate dashboard")
	flags.StringVar(&prometheus.PrometheusURL, "prometheus", "", "URL of the prometheus server that is scraping this instance")
	flags.StringVar(&db.ConnectionString, "db", "DB_URL", "Connection string for the postgres database. Use embedded:///path/to/dir to use the embedded database")
	flags.IntVar(&db.DefaultExpiryDays, "cache-timeout", 90, "Cache timeout in days")
	flags.StringVarP(&query.DefaultCheckQueryWindow, "default-window", "", "1h", "Default search window")
	flags.IntVar(&db.CheckStatusRetention, "check-status-retention-period", db.CheckStatusRetention, "Check status retention period in days")
	flags.IntVar(&topology.CheckRetentionDays, "check-retention-period", topology.DefaultCheckRetentionDays, "Check retention period in days")
	flags.IntVar(&topology.CanaryRetentionDays, "canary-retention-period", topology.DefaultCanaryRetentionDays, "Canary retention period in days")
	flags.StringVar(&checks.DefaultArtifactConnection, "artifact-connection", "", "Specify the default connection to use for artifacts")

	flags.IntVar(&canary.ReconcilePageSize, "upstream-page-size", 500, "upstream reconciliation page size")
	flags.DurationVar(&canary.ReconcileMaxAge, "upstream-max-age", time.Hour*48, "upstream reconciliation max age")
	flags.StringVar(&canary.UpstreamConf.Host, "upstream-host", os.Getenv("UPSTREAM_HOST"), "central canary checker instance to push/pull canaries")
	flags.StringVar(&canary.UpstreamConf.Username, "upstream-user", os.Getenv("UPSTREAM_USER"), "upstream username")
	flags.StringVar(&canary.UpstreamConf.Password, "upstream-password", os.Getenv("UPSTREAM_PASSWORD"), "upstream password")
	flags.StringVar(&canary.UpstreamConf.AgentName, "agent-name", os.Getenv("UPSTREAM_NAME"), "name of this agent")
	flags.BoolVar(&canary.UpstreamConf.InsecureSkipVerify, "upstream-insecure-skip-verify", os.Getenv("UPSTREAM_INSECURE_SKIP_VERIFY") == "true", "Skip TLS verification on the upstream servers certificate")

	flags.StringVar(&otelcollectorURL, "otel-collector-url", "", "OpenTelemetry gRPC Collector URL in host:port format")
	flags.StringVar(&otelServiceName, "otel-service-name", "canary-checker", "OpenTelemetry service name for the resource")
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
	Root.AddCommand(Docs)
	Root.AddCommand(Run, Serve, Operator)
	Root.AddCommand(Serve, GoOffline)
}
