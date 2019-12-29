package cmd

import (
	"fmt"
	"github.com/flanksource/canary-checker/http"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/spf13/cobra"
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
	var checks []*pkg.CheckResult
	for _, conf := range config.HTTP {
		for _, result := range http.Check(conf.HTTPCheck) {
			checks = append(checks, result)
			fmt.Println(result)
		}
	}
	return checks
}