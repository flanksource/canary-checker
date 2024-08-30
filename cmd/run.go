package cmd

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/flanksource/commons/timer"
	"github.com/flanksource/duty"

	"github.com/flanksource/canary-checker/cmd/output"
	"github.com/flanksource/canary-checker/pkg/runner"
	"github.com/spf13/cobra"

	apicontext "github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
)

var outputFile, dataFile, runNamespace string
var junit, csv, jsonExport bool

var Run = &cobra.Command{
	Use:   "run <canary.yaml>",
	Short: "Execute checks and return",
	Run: func(cmd *cobra.Command, configFiles []string) {
		timer := timer.NewTimer()
		if len(configFiles) == 0 {
			log.Fatalln("Must specify at least one canary")
		}

		ctx, closer, err := duty.Start("canary-checker", duty.ClientOnly, duty.SkipMigrationByDefaultMode)
		if err != nil {
			logger.Fatalf("Failed to initialize db: %v", err.Error())
		}
		runner.AddShutdownHook(closer)

		apicontext.DefaultContext = ctx

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
				log := logger.StandardLogger().Named(config.Name)
				wg.Add(1)
				_config := config
				go func() {
					defer wg.Done()

					res, err := checks.RunChecks(apicontext.New(apicontext.DefaultContext.WithName(_config.Name), _config))
					if err != nil {
						log.Errorf("error running checks: %v", err)
						return
					}

					queue <- res
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

				if result.Pass || result.ErrorObject == nil {
					logger.GetLogger(result.LoggerName()).Infof("%s", result.String())
				} else {
					logger.GetLogger(result.LoggerName()).Infof("%s %+v", result.String(), result.ErrorObject)
				}
				results = append(results, result)
			}
		}

		if junit {
			report := output.GetJunitReport(results)
			if err := output.HandleOutput(report, outputFile); err != nil {
				logger.Errorf("error writing output file: %v", err)
				os.Exit(1)
			}
		}
		if csv {
			report, err := output.GetCSVReport(results)
			if err != nil {
				logger.Errorf("error generating CSV file: %v", err)
				os.Exit(1)
			}
			if err := output.HandleOutput(report, outputFile); err != nil {
				logger.Errorf("error writing output file: %v", err)
				os.Exit(1)

			}
		}
		if jsonExport {
			for _, result := range results {
				result.Name = def(result.Name, result.Check.GetName(), result.Canary.Name)
				result.Description = def(result.Description, result.Check.GetDescription())
				result.Labels = merge(result.Check.GetLabels(), result.Labels)
			}

			data, err := json.Marshal(results)
			if err != nil {
				logger.Errorf("Failed to marshall json: %s", err)
				os.Exit(1)

			}
			_ = output.HandleOutput(string(data), outputFile)
		}

		logger.Infof("%d passed, %d failed in %s", passed, failed, timer)

		if failed > 0 {
			os.Exit(1)
		}
	},
}

func merge(m1, m2 map[string]string) map[string]string {
	out := make(map[string]string)
	for k, v := range m1 {
		out[k] = v
	}
	for k, v := range m2 {
		out[k] = v
	}
	return out
}

func def(a ...string) string {
	for _, s := range a {
		if s != "" {
			return s
		}
	}
	return ""
}

func init() {
	Run.PersistentFlags().StringVarP(&dataFile, "data", "d", "", "Template out each spec using the JSON or YAML data in this file")
	Run.PersistentFlags().StringVarP(&outputFile, "output-file", "o", "", "file to output the results in")
	duty.BindPFlags(Run.Flags(), duty.ClientOnly, duty.SkipMigrationByDefaultMode)
	Run.Flags().StringVarP(&runNamespace, "namespace", "n", "", "Namespace to run canary checks in")
	Run.Flags().BoolVar(&junit, "junit", false, "output results in junit format")
	Run.Flags().BoolVarP(&jsonExport, "json", "j", false, "output results in json format")
	Run.Flags().BoolVar(&csv, "csv", false, "output results in csv format")
}

func CleanupFilename(fileName string) string {
	removeSuffix := fileName[:len(fileName)-len(filepath.Ext(fileName))]
	return strings.Replace(removeSuffix, "_", "", -1)
}
