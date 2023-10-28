package cmd

import (
	"os"
	"time"

	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/jobs/canary"
	"github.com/flanksource/canary-checker/pkg/prometheus"
	"github.com/flanksource/canary-checker/pkg/runner"
	"github.com/flanksource/commons/logger"
	gomplate "github.com/flanksource/gomplate/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var Root = &cobra.Command{
	Use: "canary-checker",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logger.UseZap(cmd.Flags())
		for _, script := range sharedLibrary {
			if err := gomplate.LoadSharedLibrary(script); err != nil {
				logger.Errorf("Failed to load shared library %s: %v", script, err)
			}
		}
		db.ConnectionString = readFromEnv(db.ConnectionString)
		if db.ConnectionString == "DB_URL" {
			db.ConnectionString = ""
		}

		if canary.UpstreamConf.Valid() {
			logger.Infof("Pushing checks to %s with name=%s user=%s", canary.UpstreamConf.Host, canary.UpstreamConf.AgentName, canary.UpstreamConf.Username)
		}
	},
}

var httpPort = 8080
var publicEndpoint = "http://localhost:8080"
var pushServers, pullServers []string
var sharedLibrary []string
var exposeEnv bool
var logPass, logFail bool

func ServerFlags(flags *pflag.FlagSet) {
	flags.IntVar(&httpPort, "httpPort", httpPort, "Port to expose a health dashboard ")

	flags.Bool("dev", false, "")
	flags.Int("devGuiPort", 3004, "Port used by a local npm server in development mode")
	flags.Int("metricsPort", 8081, "Port to expose a health dashboard ")

	_ = flags.MarkDeprecated("devGuiPort", "")
	_ = flags.MarkDeprecated("metricsPort", "Extra metrics server removed")
	_ = flags.MarkDeprecated("dev", "")

	flags.StringVar(&publicEndpoint, "public-endpoint", publicEndpoint, "Host on which the health dashboard is exposed. Could be used for generting-links, redirects etc.")
	flags.StringSliceVar(&runner.IncludeCanaries, "include-check", []string{}, "Run matching canaries - useful for debugging")
	flags.StringSliceVar(&runner.IncludeTypes, "include-type", []string{}, "Check type to disable")
	flags.StringSliceVar(&runner.IncludeNamespaces, "include-namespace", []string{}, "Check type to disable")
	flags.IntVar(&cache.DefaultCacheCount, "maxStatusCheckCount", 5, "Maximum number of past checks in the in memory cache")
	flags.StringSliceVar(&pushServers, "push-servers", []string{}, "push check results to multiple canary servers")
	flags.StringSliceVar(&pullServers, "pull-servers", []string{}, "push check results to multiple canary servers")
	flags.StringVar(&runner.RunnerName, "name", "local", "Server name shown in aggregate dashboard")
	flags.StringVar(&prometheus.PrometheusURL, "prometheus", "", "URL of the prometheus server that is scraping this instance")
	flags.StringVar(&db.ConnectionString, "db", "DB_URL", "Connection string for the postgres database. Use embedded:///path/to/dir to use the embedded database")
	flags.IntVar(&db.DefaultExpiryDays, "cache-timeout", 90, "Cache timeout in days")
	flags.StringVarP(&cache.DefaultWindow, "default-window", "", "1h", "Default search window")
	flags.IntVar(&db.CheckStatusRetentionDays, "check-status-retention-period", db.DefaultCheckStatusRetentionDays, "Check status retention period in days")
	flags.IntVar(&db.CheckRetentionDays, "check-retention-period", db.DefaultCheckRetentionDays, "Check retention period in days")
	flags.IntVar(&db.CanaryRetentionDays, "canary-retention-period", db.DefaultCanaryRetentionDays, "Canary retention period in days")

	flags.IntVar(&canary.ReconcilePageSize, "upstream-page-size", 500, "upstream reconciliation page size")
	flags.DurationVar(&canary.ReconcileMaxAge, "upstream-max-age", time.Hour*48, "upstream reconciliation max age")
	flags.StringVar(&canary.UpstreamConf.Host, "upstream-host", os.Getenv("UPSTREAM_HOST"), "central canary checker instance to push/pull canaries")
	flags.StringVar(&canary.UpstreamConf.Username, "upstream-user", os.Getenv("UPSTREAM_USER"), "upstream username")
	flags.StringVar(&canary.UpstreamConf.Password, "upstream-password", os.Getenv("UPSTREAM_PASSWORD"), "upstream password")
	flags.StringVar(&canary.UpstreamConf.AgentName, "agent-name", os.Getenv("UPSTREAM_NAME"), "name of this agent")
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

	Root.PersistentFlags().StringVar(&db.ConnectionString, "db", "DB_URL", "Connection string for the postgres database")
	Root.PersistentFlags().BoolVar(&db.RunMigrations, "db-migrations", false, "Run database migrations")
	Root.PersistentFlags().BoolVar(&db.DBMetrics, "db-metrics", false, "Expose db metrics")
	Root.PersistentFlags().BoolVar(&logFail, "log-fail", true, "Log every failing check")
	Root.PersistentFlags().BoolVar(&logPass, "log-pass", false, "Log every passing check")
	Root.PersistentFlags().StringArrayVar(&sharedLibrary, "shared-library", []string{}, "Add javascript files to be shared by all javascript templates")
	Root.PersistentFlags().BoolVar(&exposeEnv, "expose-env", false, "Expose environment variables for use in all templates. Note this has serious security implications with untrusted canaries")
	Root.AddCommand(Docs)
	Root.AddCommand(Run, Serve, Operator)
	Root.AddCommand(Serve, GoOffline)
}
