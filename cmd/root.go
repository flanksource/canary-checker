package cmd

import (
	"os"

	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/jobs/canary"
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
	},
}

var httpPort = 8080
var publicEndpoint = "http://localhost:8080"
var includeCheck, prometheusURL string
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
	flags.StringVar(&includeCheck, "include-check", "", "Run matching canaries - useful for debugging")
	flags.IntVar(&cache.DefaultCacheCount, "maxStatusCheckCount", 5, "Maximum number of past checks in the in memory cache")
	flags.StringSliceVar(&pushServers, "push-servers", []string{}, "push check results to multiple canary servers")
	flags.StringSliceVar(&pullServers, "pull-servers", []string{}, "push check results to multiple canary servers")
	flags.StringVar(&runner.RunnerName, "name", "local", "Server name shown in aggregate dashboard")
	flags.StringVar(&prometheusURL, "prometheus", "", "URL of the prometheus server that is scraping this instance")
	flags.StringVar(&db.ConnectionString, "db", "DB_URL", "Connection string for the postgres database. Use embedded:///path/to/dir to use the embedded database")
	flags.IntVar(&db.DefaultExpiryDays, "cache-timeout", 90, "Cache timeout in days")
	flags.StringVarP(&cache.DefaultWindow, "default-window", "", "1h", "Default search window")
	flags.IntVar(&db.CheckStatusRetentionDays, "check-status-retention-period", db.DefaultCheckStatusRetentionDays, "Check status retention period in days")
	flags.IntVar(&db.CheckRetentionDays, "check-retention-period", db.DefaultCheckRetentionDays, "Check retention period in days")
	flags.IntVar(&db.CanaryRetentionDays, "canary-retention-period", db.DefaultCanaryRetentionDays, "Canary retention period in days")

	// Flags for push/pull
	flags.StringVar(&canary.UpstreamConf.Host, "upstream-host", "", "central canary checker instance to push/pull canaries")
	flags.StringVar(&canary.UpstreamConf.Username, "upstream-user", "", "upstream username")
	flags.StringVar(&canary.UpstreamConf.Password, "upstream-password", "", "upstream password")
	flags.StringVar(&canary.UpstreamConf.AgentName, "agent-name", "", "name of this agent")
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
	Root.PersistentFlags().BoolVar(&logFail, "log-fail", true, "Log every failing check")
	Root.PersistentFlags().BoolVar(&logPass, "log-pass", false, "Log every passing check")
	Root.PersistentFlags().StringArrayVar(&sharedLibrary, "shared-library", []string{}, "Add javascript files to be shared by all javascript templates")
	Root.PersistentFlags().BoolVar(&exposeEnv, "expose-env", false, "Expose environment variables for use in all templates. Note this has serious security implications with untrusted canaries")
	Root.AddCommand(Docs)
	Root.AddCommand(Run, Serve, Operator)
	Root.AddCommand(Serve, GoOffline)
}
