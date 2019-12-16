package cmd

import (
	"github.com/spf13/cobra"
)

var Serve = &cobra.Command{
	Use:   "serve",
	Short: "Start a server to execute checks ",
}

func init() {
	Serve.Flags().Int("metrics-port", 0, "Port to export prometheus metrics on, use 0 to disable ")
	Serve.Flags().Int("http-port", 0, "Port to expose a health dashboard ")
	Serve.Flags().Int("interval", 0, "Default interval (in seconds) to run checks on")
	Serve.Flags().Int("failureThreshold", 2, "Default Number of consecutive failures required to fail a check")
}
