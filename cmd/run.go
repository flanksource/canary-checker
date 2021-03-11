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
		namespace, _ := cmd.Flags().GetString("namespace")
		config := pkg.ParseConfig(configfile)
		failed := 0
		for _, result := range RunChecks(config, namespace) {
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
	Run.Flags().StringP("namespace", "n", "", "Specify namespace")
}
func RunChecks(config v1.CanarySpec, namespace string) []*pkg.CheckResult {

	var results []*pkg.CheckResult
	kommonsClient, err := pkg.NewKommonsClient()
	if err != nil {
		logger.Warnf("Failed to get kommons client, features that read kubernetes configs will fail: %v", err)
	}

	config.SetNamespaces(namespace)

	for _, c := range checks.All {
		switch cs := c.(type) {
		case checks.SetsClient:
			cs.SetClient(kommonsClient)
		}
		result := c.Run(config)
		for _, r := range result {
			if r != nil {
				results = append(results, r)
			}
		}
	}

	return results
}
