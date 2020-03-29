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
		config := pkg.ParseConfig(configfile)
		failed := 0
		for result := range RunChecks(config) {
			fmt.Println(result)
			if !result.Pass {
				failed++
			}
		}
		if failed > 0 {
			log.Fatalf("%d checks failed", failed)
		}
	},
}

func init() {

}
func RunChecks(config pkg.Config) chan *pkg.CheckResult {
	var checks = []checks.Checker{
		&checks.DNSChecker{},
		&checks.HttpChecker{},
		&checks.IcmpChecker{},
		&checks.DockerPullChecker{},
		&checks.S3Checker{},
		&checks.PostgresChecker{},
		&checks.LdapChecker{},
		&checks.S3BucketChecker{},
		checks.NewPodChecker(),
	}

	var results = make(chan *pkg.CheckResult)

	go func() {
		for _, c := range checks {
			c.Run(config, results)
		}
		close(results)
	}()

	return results
}
