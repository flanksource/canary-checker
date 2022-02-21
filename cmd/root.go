package cmd

import (
	"fmt"

	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/runner"
	"github.com/flanksource/commons/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var Root = &cobra.Command{
	Use: "canary-checker",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logger.UseZap(cmd.Flags())
	},
}

var dev bool
var httpPort, metricsPort, devGuiPort int
var namespace, includeCheck, prometheusURL string
var pushServers, pullServers []string
var exposeEnv bool
var logPass, logFail bool
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func ServerFlags(flags *pflag.FlagSet) {
	flags.IntVar(&httpPort, "httpPort", 8080, "Port to expose a health dashboard ")
	flags.IntVar(&devGuiPort, "devGuiPort", 3004, "Port used by a local npm server in development mode")
	flags.IntVar(&metricsPort, "metricsPort", 8081, "Port to expose a health dashboard ")
	flags.BoolVar(&dev, "dev", false, "Run in development mode")
	flags.BoolVar(&logFail, "log-fail", true, "Log every failing check")
	flags.BoolVar(&logPass, "log-pass", false, "Log every passing check")
	flags.StringVarP(&namespace, "namespace", "n", "", "Watch only specified namespaces, otherwise watch all")
	flags.StringVar(&includeCheck, "include-check", "", "Run matching canaries - useful for debugging")
	flags.IntVar(&cache.DefaultCacheCount, "maxStatusCheckCount", 5, "Maximum number of past checks in the in memory cache")
	flags.StringSliceVar(&pushServers, "push-servers", []string{}, "push check results to multiple canary servers")
	flags.StringSliceVar(&pullServers, "pull-servers", []string{}, "push check results to multiple canary servers")
	flags.StringVar(&runner.RunnerName, "name", "local", "Server name shown in aggregate dashboard")
	flags.StringVar(&prometheusURL, "prometheus", "", "URL of the prometheus server that is scraping this instance")
	flags.StringVar(&db.ConnectionString, "db", "DB_URL", "Connection string for the postgres database")
	flags.IntVar(&db.DefaultExpiryDays, "cache-timeout", 90, "Cache timeout in days")
	flags.StringVarP(&cache.DefaultWindow, "default-window", "", "1h", "Default search window")
}

func init() {
	logger.BindFlags(Root.PersistentFlags())

	if len(commit) > 8 {
		version = fmt.Sprintf("%v, commit %v, built at %v", version, commit[0:8], date)
	}
	Root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version of canary-checker",
		Args:  cobra.MinimumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version)
		},
	})
	runner.Version = version

	Root.PersistentFlags().BoolVar(&exposeEnv, "expose-env", false, "Expose environment variables for use in all templates. Note this has serious security implications with untrusted canaries")
	Root.AddCommand(Docs)
	Root.AddCommand(Run, Serve, Operator, Push)
}
