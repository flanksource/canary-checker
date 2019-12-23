package cmd

import (
	"github.com/flanksource/canary-checker/pkg"
	"github.com/spf13/cobra"
)

var Run = &cobra.Command{
	Use:   "run",
	Short: "Execute checks and return",
	Run: func(cmd *cobra.Command, args []string) {
		configfile, _ := cmd.Flags().GetString("configfile")
		pkg.ReadConfig(configfile)
	},
}

func init() {

}
