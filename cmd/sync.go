package cmd

import (
	"github.com/spf13/cobra"

	"github.com/flanksource/canary-checker/pkg/db"
	configSync "github.com/flanksource/canary-checker/pkg/sync"
	"github.com/flanksource/commons/logger"
)

var Sync = &cobra.Command{
	Use: "sync",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if err := db.Init(db.ConnectionString); err != nil {
			logger.Fatalf("error connecting with postgres %v", err)
		}
	},
}

var AddCanary = &cobra.Command{
	Use:   "canary <system.yaml>",
	Short: "Add a new canary spec",
	Run: func(cmd *cobra.Command, configFiles []string) {
		if err := configSync.SyncCanary(dataFile, configFiles...); err != nil {
			logger.Fatalf("Could not sync canaries: %v", err)
		}
	},
}

func init() {
	AddTopology.PersistentFlags().StringVarP(&namespace, "namespace", "n", "default", "Namespace to query")
	Sync.AddCommand(AddCanary)
	Root.AddCommand(Sync)
}
