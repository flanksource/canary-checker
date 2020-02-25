package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
)

var Run = &cobra.Command{
	Use:   "run",
	Short: "Execute checks and return",
	Run: func(cmd *cobra.Command, args []string) {
		configfile, _ := cmd.Flags().GetString("configfile")
		config := pkg.ParseConfig(configfile)
		RunChecks(config)
	},
}

func init() {

}
func RunChecks(config pkg.Config) []*pkg.CheckResult {
	var checks = []checks.Checker{
		&checks.HttpChecker{},
		&checks.IcmpChecker{},
		&checks.DockerPullChecker{},
		&checks.S3Checker{},
	}

	var results []*pkg.CheckResult

	for _, c := range checks {
		for _, result := range c.Run(config) {
			results = append(results, result)
			fmt.Println(result)
		}
	}

	return results
}
