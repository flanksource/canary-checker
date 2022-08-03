package cmd

import (
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/commons/logger"
	"github.com/spf13/cobra"
)

var GoOffline = &cobra.Command{
	Use:  "go-offline",
	Long: "Download all dependencies.",
	Run: func(cmd *cobra.Command, args []string) {
		if err := db.GoOffline(); err != nil {
			logger.Fatalf("Failed to go offline: %+v", err)
		}
	},
}
