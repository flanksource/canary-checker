package cmd

import (
	"github.com/flanksource/canary-checker/pkg/cache"
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

func ServerFlags(flags *pflag.FlagSet) {
	CommonFlags(flags)
	flags.IntVar(&devGuiPort, "devGuiPort", 3004, "Port used by a local npm server in development mode")
	flags.BoolVar(&dev, "dev", false, "Run in development mode")
	flags.StringSliceVar(&pullServers, "pull-servers", []string{}, "push check results to multiple canary servers")
	flags.StringVar(&prometheusURL, "prometheus", "", "URL of the prometheus server that is scraping this instance")
}

func CommonFlags(flags *pflag.FlagSet) {
	flags.IntVar(&httpPort, "httpPort", 8080, "Port to expose a health dashboard ")
	flags.IntVar(&metricsPort, "metricsPort", 8081, "Port to expose a health dashboard ")
	flags.BoolVar(&logFail, "log-fail", true, "Log every failing check")
	flags.BoolVar(&logPass, "log-pass", false, "Log every passing check")
	flags.StringVarP(&namespace, "namespace", "n", "", "Watch only specified namespaces, otherwise watch all")
	flags.StringVar(&includeCheck, "include-check", "", "Run matching canaries - useful for debugging")
	flags.IntVar(&cache.Size, "maxStatusCheckCount", 5, "Maximum number of past checks in the status page")
	flags.StringSliceVar(&pushServers, "push-servers", []string{}, "push check results to multiple canary servers")
	flags.StringVar(&runner.RunnerName, "name", "local", "Server name shown in aggregate dashboard")
}

func init() {
	logger.BindFlags(Root.PersistentFlags())

	Root.PersistentFlags().BoolVar(&exposeEnv, "expose-env", false, "Expose environment variables for use in all templates. Note this has serious security implications with untrusted canaries")
	Root.AddCommand(Docs)
	Root.AddCommand(Run, Serve, Operator, InstallService, UninstallService)
}
