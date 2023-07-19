package cmd

import (
	"os"

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

		// Run in embedded mode once to download the postgres binary
		databaseDir := "temp-database-dir"
		if err := os.Mkdir(databaseDir, 0755); err != nil {
			logger.Fatalf("Failed to create database directory[%s]: %+v", err)
		}
		defer os.RemoveAll(databaseDir)

		db.ConnectionString = "embedded://" + databaseDir
		if err := db.Init(); err != nil {
			logger.Fatalf("Failed to run in embedded mode: %+v", err)
		}
		if err := db.PostgresServer.Stop(); err != nil {
			logger.Fatalf("Failed to stop embedded postgres: %+v", err)
		}

		// Intentionally exit with code 0 for Docker
		os.Exit(0)
	},
}
