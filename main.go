package main

import (
	"fmt"
	"github.com/flanksource/canary-checker/pkg/db"
	"os"

	"github.com/flanksource/canary-checker/cmd"
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
	defer func() {
		err := db.StopServer()
		if err != nil {
			os.Exit(1)
		}
	}()

	if err := cmd.Root.Execute(); err != nil {
		os.Exit(1)
	}
}
