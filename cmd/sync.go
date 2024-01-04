package cmd

import (
	"github.com/spf13/cobra"

	configSync "github.com/flanksource/canary-checker/pkg/sync"
	"github.com/flanksource/commons/logger"
)

var Sync = &cobra.Command{
	Use: "sync",
}

var AddCanary = &cobra.Command{
	Use:   "canary <system.yaml>",
	Short: "Add a new canary spec",
	Run: func(cmd *cobra.Command, configFiles []string) {

		if ctx, err := InitContext(); err != nil {
			logger.Fatalf("error connecting with postgres %v", err)
		} else {
			if err := configSync.SyncCanary(ctx, dataFile, configFiles...); err != nil {
				logger.Fatalf("Could not sync canaries: %v", err)
			}
		}
	},
}

func init() {
	Sync.AddCommand(AddCanary)
	Root.AddCommand(Sync)
}
