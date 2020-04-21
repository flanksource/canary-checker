package main

import (
	"fmt"
	"os"

	"github.com/flanksource/canary-checker/cmd"
	log "github.com/sirupsen/logrus"
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
			level, _ := cmd.Flags().GetCount("loglevel")
			switch {
			case level > 1:
				log.SetLevel(log.TraceLevel)
			case level > 0:
				log.SetLevel(log.DebugLevel)
			default:
				log.SetLevel(log.InfoLevel)
			}
		},
	}

	root.AddCommand(cmd.Run, cmd.Serve)

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
				log.Fatal(err)
			}
		},
	})

	docs.AddCommand(cmd.APIDocs)
	root.AddCommand(docs)

	root.PersistentFlags().CountP("loglevel", "v", "Increase logging level")
	root.PersistentFlags().StringP("configfile", "c", "", "Specify configfile")
	root.SetUsageTemplate(root.UsageTemplate() + fmt.Sprintf("\nversion: %s\n ", version))
	// root.MarkPersistentFlagRequired("configfile")
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
