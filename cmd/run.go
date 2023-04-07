package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/flanksource/commons/timer"

	"github.com/flanksource/canary-checker/cmd/output"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/spf13/cobra"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
)

var outputFile, dataFile, runNamespace string
var junit, csv bool

var Run = &cobra.Command{
	Use:   "run <canary.yaml>",
	Short: "Execute checks and return",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		db.ConnectionString = readFromEnv(db.ConnectionString)
		if err := db.Init(); err != nil {
			logger.Fatalf("error connecting with postgres %v", err)
		}
	},
	Run: func(cmd *cobra.Command, configFiles []string) {
		timer := timer.NewTimer()
		if len(configFiles) == 0 {
			log.Fatalln("Must specify at least one canary")
		}
		kommonsClient, err := pkg.NewKommonsClient()
		if err != nil {
			logger.Warnf("Failed to get kommons client, features that read kubernetes configs will fail: %v", err)
		}
		var results = []*pkg.CheckResult{}

		wg := sync.WaitGroup{}
		queue := make(chan []*pkg.CheckResult, 1)

		for _, configfile := range configFiles {
			configs, err := pkg.ParseConfig(configfile, dataFile)
			if err != nil {
				logger.Errorf("Could not parse %s: %v", configfile, err)
				continue
			}
			logger.Infof("Checking %s, %d checks found", configfile, len(configs))
			for _, config := range configs {
				if runNamespace != "" {
					config.Namespace = runNamespace
				}
				if config.Name == "" {
					config.Name = CleanupFilename(configfile)
				}
				wg.Add(1)
				_config := config
				go func() {
					queue <- checks.RunChecks(context.New(kommonsClient, _config))
					wg.Done()
				}()
			}
		}
		failed := 0
		passed := 0

		go func() {
			wg.Wait()
			close(queue)
		}()

		for item := range queue {
			for _, result := range item {
				if !result.Pass {
					failed++
				} else {
					passed++
				}
				fmt.Printf("%s \t%s\t\n", time.Now().Format(time.RFC3339), result.String())
				results = append(results, result)
			}
		}

		if junit {
			report := output.GetJunitReport(results)
			if err := output.HandleOutput(report, outputFile); err != nil {
				logger.Fatalf("error writing output file: %v", err)
			}
		}
		if csv {
			report, err := output.GetCSVReport(results)
			if err != nil {
				logger.Fatalf("error generating CSV file: %v", err)
			}
			if err := output.HandleOutput(report, outputFile); err != nil {
				logger.Fatalf("error writing output file: %v", err)
			}
		}

		logger.Infof("%d passed, %d failed in %s", passed, failed, timer)

		if failed > 0 {
			os.Exit(1)
		}
	},
}

func init() {
	Run.PersistentFlags().StringVarP(&dataFile, "data", "d", "", "Template out each spec using the JSON or YAML data in this file")
	Run.PersistentFlags().StringVarP(&outputFile, "output-file", "o", "", "file to output the results in")
	Run.Flags().StringVarP(&runNamespace, "namespace", "n", "", "Namespace to run canary checks in")
	Run.Flags().BoolVarP(&junit, "junit", "j", false, "output results in junit format")
	Run.Flags().BoolVar(&csv, "csv", false, "output results in csv format")
}

func CleanupFilename(fileName string) string {
	removeSuffix := fileName[:len(fileName)-len(filepath.Ext(fileName))]
	return strings.Replace(removeSuffix, "_", "", -1)
}
