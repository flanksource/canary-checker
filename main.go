package main

import (
	"fmt"
	"os"

	"github.com/flanksource/canary-checker/cmd"
	"github.com/flanksource/commons/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var root = &cobra.Command{
		Use: "canary-checker",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			logger.UseZap(cmd.Flags())
			logger.Infof("Starting %s", version)
		},
	}

	logger.BindFlags(root.PersistentFlags())

	if len(commit) > 8 {
		version = fmt.Sprintf("%v, commit %v, built at %v", version, commit[0:8], date)
	}

	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version of canary-checker",
		Args:  cobra.MinimumNArgs(0),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version)
		},
	})

	docs := &cobra.Command{
		Use:   "docs",
		Short: "generate documentation",
	}

	docs.AddCommand(&cobra.Command{
		Use:   "cli [PATH]",
		Short: "generate CLI documentation",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			err := doc.GenMarkdownTree(root, args[0])
			if err != nil {
				logger.Fatalf("error creating docs", err)
			}
		},
	})

	docs.AddCommand(cmd.APIDocs)
	root.AddCommand(docs)
	root.SetUsageTemplate(root.UsageTemplate() + fmt.Sprintf("\nversion: %s\n ", version))
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
