package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
)

var Run = &cobra.Command{
	Use:   "run",
	Short: "Execute checks and return",
	Run: func(cmd *cobra.Command, args []string) {
		configfile, _ := cmd.Flags().GetString("configfile")
		config := pkg.ParseConfig(configfile)
		failed := 0
		for _, result := range RunChecks(config) {
			fmt.Println(result)
			if !result.Pass {
				failed++
			}
		}
		if failed > 0 {
			logger.Fatalf("%d checks failed", failed)
		}
	},
}

func init() {
	Run.Flags().StringP("configfile", "c", "", "Specify configfile")
}
func RunChecks(config v1.CanarySpec) []*pkg.CheckResult {

	var results []*pkg.CheckResult

	for _, c := range checks.All {
		result := c.Run(config)
		for _, r := range result {
			if r != nil {
				results = append(results, r)
			}
		}
	}

	return results
}
