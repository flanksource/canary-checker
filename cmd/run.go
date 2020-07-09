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
func RunChecks(config v1.CanarySpec) []*pkg.CheckResult {

	var results []*pkg.CheckResult

	for _, c := range checks.All {
		results = append(results, c.Run(config)...)
		}

	return results
}
