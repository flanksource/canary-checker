package cmd

import (
	"fmt"
	"os"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg/jobs/canary"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/canary-checker/pkg/prometheus"
	"github.com/flanksource/canary-checker/pkg/runner"
	"github.com/flanksource/canary-checker/pkg/telemetry"
	"github.com/flanksource/commons/http"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/commons/properties"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/connection"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/db"
	"github.com/flanksource/duty/query"
	"github.com/flanksource/duty/shutdown"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel"
)

const app = "canary-checker"

func InitContext() (context.Context, error) {
	ctx, closer, err := duty.Start(app, duty.SkipChangelogMigration, duty.SkipMigrationByDefaultMode)
	if err != nil {
		return ctx, fmt.Errorf("failed to initialize db: %v", err.Error())
	}
	shutdown.AddHook(closer)

	if err := properties.LoadFile(propertiesFile); err != nil {
		return ctx, fmt.Errorf("failed to load properties: %v", err)
	}

	ctx.WithTracer(otel.GetTracerProvider().Tracer(app))
	if ctx.DB() != nil {
		if err := ctx.DB().Use(db.NewOopsPlugin()); err != nil {
			return ctx, fmt.Errorf("failed to use oops gorm plugin: %w", err)
		}
	}

	return ctx, nil
}

var Root = &cobra.Command{
	Use: app,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logger.UseSlog()
		shutdown.WaitForSignal()

		canary.LogFail = logFail || logger.IsLevelEnabled(3)
		canary.LogPass = logPass || logger.IsLevelEnabled(4)

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

			shutdown.AddHook(telemetry.InitTracer(otelServiceName, otelcollectorURL, true))
		}
		if prometheus.PrometheusURL != "" {
			logger.Infof("Setting default prometheus: %s", prometheus.PrometheusURL)
			runner.Prometheus, _ = prometheus.NewPrometheusAPI(
				context.New(),
				connection.HTTPConnection{URL: prometheus.PrometheusURL},
			)
		}

		// Setup metrics since v1.AdditionalCheckMetricLabels is set now
		metrics.SetupMetrics()
	},
}

var (
	httpPort         = 8080
	publicEndpoint   = "http://localhost:8080"
	logPass, logFail bool

	otelcollectorURL string
	otelServiceName  string
)

func deprecatedFlags(flags *pflag.FlagSet) {
	_ = flags.Int("devGuiPort", 3004, "Port used by a local npm server in development mode")
	if err := flags.MarkDeprecated("devGuiPort", "the flag used to be a no-op"); err != nil {
		panic(err)
	}

	_ = flags.Int("metricsPort", 8081, "Port to expose a health dashboard ")
	if err := flags.MarkDeprecated("metricsPort", "Extra metrics server removed"); err != nil {
		panic(err)
	}

	_ = flags.Bool("dev", false, "")
	if err := flags.MarkDeprecated("dev", "the flag used to be a no-op"); err != nil {
		panic(err)
	}

	_ = flags.StringSlice("push-servers", []string{}, "push check results to multiple canary servers")
	if err := flags.MarkDeprecated("push-servers", "this feature has been deprecated."); err != nil {
		panic(err)
	}

	_ = flags.StringSlice("pull-servers", []string{}, "pull check results from multiple canary servers")
	if err := flags.MarkDeprecated("pull-servers", "this feature has been deprecated."); err != nil {
		panic(err)
	}

	_ = flags.Bool(
		"expose-env",
		false,
		"Expose environment variables for use in all templates. Note this has serious security implications with untrusted canaries",
	)
	if err := flags.MarkDeprecated("expose-env", "the flag used to be a no-op"); err != nil {
		panic(err)
	}

	flags.StringArray("shared-library", []string{}, "Add javascript files to be shared by all javascript templates")
	if err := flags.MarkDeprecated("shared-library", "running custom scripts isn't allowed"); err != nil {
		panic(err)
	}

	_ = flags.Int("maxStatusCheckCount", 5, "Maximum number of past checks in the in memory cache")
	if err := flags.MarkDeprecated("maxStatusCheckCount", "use the `start` and `until` query params"); err != nil {
		panic(err)
	}

	_ = flags.Int("check-retention-period", 7, "Check retention period in days")
	if err := flags.MarkDeprecated("check-retention-period", "Use property check.retention.age"); err != nil {
		panic(err)
	}

	_ = flags.Int("canary-retention-period", 7, "Canary retention period in days")
	if err := flags.MarkDeprecated("canary-retention-period", "use property canary.retention.age"); err != nil {
		panic(err)
	}

	_ = flags.Int("check-status-retention-period", 30, "Check status retention period in days")
	if err := flags.MarkDeprecated("check-status-retention-period", "use property check.status.retention.days"); err != nil {
		panic(err)
	}

	_ = flags.Int("cache-timeout", 90, "Cache timeout in days")
	if err := flags.MarkDeprecated("cache-timeout", "the flag used to be a no-op"); err != nil {
		panic(err)
	}
}

func ServerFlags(flags *pflag.FlagSet) {
	flags.IntVar(&httpPort, "httpPort", httpPort, "Port to expose a health dashboard ")

	flags.StringSliceVar(&v1.AdditionalCheckMetricLabels,
		"metric-labels-allowlist",
		nil,
		"comma-separated list of additional check label keys that should be included in the check metrics",
	)

	flags.StringVar(
		&publicEndpoint,
		"public-endpoint",
		publicEndpoint,
		"Host on which the health dashboard is exposed. Could be used for generting-links, redirects etc.",
	)
	flags.StringVar(&runner.RunnerName, "name", "local", "Server name shown in aggregate dashboard")

	flags.StringSliceVar(
		&runner.IncludeCanaries,
		"include-check",
		[]string{},
		"(Deprecated: use --include-canary) Run matching canaries - useful for debugging",
	)
	flags.StringSliceVar(
		&runner.IncludeCanaries,
		"include-canary",
		[]string{},
		"Only run canaries matching the given names",
	)
	flags.StringSliceVar(
		&runner.IncludeLabels,
		"include-labels",
		nil,
		"Only run canaries matching the given label selector",
	)
	flags.StringSliceVar(
		&runner.IncludeNamespaces,
		"include-namespace",
		[]string{},
		"a comma separated list of namespaces whose canary should be run",
	)

	flags.StringVarP(&query.DefaultCheckQueryWindow, "default-window", "", "1h", "Default search window")
	flags.StringVar(
		&checks.DefaultArtifactConnection,
		"artifact-connection",
		"",
		"Specify the default connection to use for artifacts",
	)

	flags.IntVar(&canary.ReconcilePageSize, "upstream-page-size", 500, "upstream reconciliation page size")
	flags.DurationVar(&canary.ReconcileMaxAge, "upstream-max-age", time.Hour*48, "upstream reconciliation max age")
	flags.StringVar(
		&canary.UpstreamConf.Host,
		"upstream-host",
		os.Getenv("UPSTREAM_HOST"),
		"central canary checker instance to push/pull canaries",
	)
	flags.StringVar(&canary.UpstreamConf.Username, "upstream-user", os.Getenv("UPSTREAM_USER"), "upstream username")
	flags.StringVar(
		&canary.UpstreamConf.Password,
		"upstream-password",
		os.Getenv("UPSTREAM_PASSWORD"),
		"upstream password",
	)
	flags.StringVar(&canary.UpstreamConf.AgentName, "agent-name", os.Getenv("AGENT_NAME"), "name of this agent")
	flags.BoolVar(
		&canary.UpstreamConf.InsecureSkipVerify,
		"upstream-insecure-skip-verify",
		os.Getenv("UPSTREAM_INSECURE_SKIP_VERIFY") == "true",
		"Skip TLS verification on the upstream servers certificate",
	)

	duty.BindPFlags(flags, duty.SkipMigrationByDefaultMode)

	deprecatedFlags(flags)
}

func init() {
	logger.BindFlags(Root.PersistentFlags())
	_ = properties.LoadFile("canary-checker.properties")
	logger.UseSlog()
	Root.PersistentFlags().BoolVar(&logFail, "log-fail", false, "Log every failing check")
	Root.PersistentFlags().BoolVar(&logPass, "log-pass", false, "Log every passing check")
	Root.PersistentFlags().
		StringVar(&otelcollectorURL, "otel-collector-url", "", "OpenTelemetry gRPC Collector URL in host:port format")
	Root.PersistentFlags().
		StringVar(&otelServiceName, "otel-service-name", "canary-checker", "OpenTelemetry service name for the resource")
	Root.PersistentFlags().
		StringVar(&prometheus.PrometheusURL, "prometheus", "", "URL of the prometheus server that is scraping this instance")
	Root.AddCommand(Docs)
	Root.AddCommand(Run, Serve, Operator)
	Root.AddCommand(Serve, GoOffline)
}
