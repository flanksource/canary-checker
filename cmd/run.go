package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
)

var Run = &cobra.Command{
	Use:   "run",
	Short: "Execute checks and return",
	Run: func(cmd *cobra.Command, args []string) {
		configfile, _ := cmd.Flags().GetString("configfile")
		config, err := pkg.ParseConfig(configfile)
		if err != nil {
			log.Fatalf("Error parsing config: %v", err)
		}
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
		&checks.PostgresChecker{},
		&checks.LdapChecker{},
		&checks.S3BucketChecker{},
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
