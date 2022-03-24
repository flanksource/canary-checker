package main

import (
	"fmt"
	"os"

	"github.com/flanksource/canary-checker/cmd"
	"github.com/flanksource/canary-checker/pkg/db"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(commit) > 8 {
		version = fmt.Sprintf("%v, commit %v, built at %v", version, commit[0:8], date)
	}
	cmd.Root.SetUsageTemplate(cmd.Root.UsageTemplate() + fmt.Sprintf("\nversion: %s\n ", version))
	defer db.StopServer()

	if err := cmd.Root.Execute(); err != nil {
		os.Exit(1)
	}
}
