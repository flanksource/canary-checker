package cmd

import (
	"github.com/spf13/cobra"
)

var Run = &cobra.Command{
	Use:   "run",
	Short: "Execute checks and return",
}

func init() {
}
