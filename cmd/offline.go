package cmd

import (
	"os"

	"github.com/flanksource/canary-checker/pkg/runner"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/api"
	"github.com/flanksource/duty/postgrest"
	"github.com/spf13/cobra"
)

var GoOffline = &cobra.Command{
	Use:  "go-offline",
	Long: "Download all dependencies.",
	Run: func(cmd *cobra.Command, args []string) {
		if err := postgrest.GoOffline(); err != nil {
			logger.Fatalf("Failed to go offline: %+v", err)
		}

		// Run in embedded mode once to download the postgres binary
		databaseDir := "temp-database-dir"
		if err := os.Mkdir(databaseDir, 0755); err != nil {
			logger.Fatalf("Failed to create database directory[%s]: %+v", err)
		}
		defer os.RemoveAll(databaseDir)

		api.DefaultConfig.ConnectionString = "embedded://" + databaseDir
		_, closer, err := duty.Start("embedded-temp")
		runner.AddShutdownHook(closer)
		if err != nil {
			logger.Fatalf("Failed to run in embedded mode: %+v", err)
		}

		// Intentionally exit with code 0 for Docker
		runner.ShutdownAndExit(0, "Finished downloading dependencies")
	},
}
