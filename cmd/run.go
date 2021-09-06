package cmd

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/flanksource/commons/console"
	"github.com/flanksource/commons/timer"

	"github.com/spf13/cobra"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
)

var Run = &cobra.Command{
	Use:   "run <canary.yaml>",
	Short: "Execute checks and return",
	Run: func(cmd *cobra.Command, configFiles []string) {
		namespace, _ := cmd.Flags().GetString("namespace")
		junitFile, _ := cmd.Flags().GetString("junit-file")
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
			logger.Infof("Checking %s", configfile)
			configs, err := pkg.ParseConfig(configfile)
			if err != nil {
				logger.Errorf("Could not parse %s: %v", configfile, err)
				continue
			}
			for _, config := range configs {
				if namespace != "" {
					config.Namespace = namespace
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

		if junitFile != "" {
			report := getJunitReport(results)
			err := ioutil.WriteFile(junitFile, []byte(report), 0755)
			if err != nil {
				logger.Fatalf("%d checks failed", failed)
			}
		}

		logger.Infof("%d passed, %d failed in %s", passed, failed, timer)

		if failed > 0 {
			os.Exit(1)
		}
	},
}

func init() {
	Run.Flags().StringP("namespace", "n", "", "Specify namespace")
	Run.Flags().StringP("junit", "j", "", "Export JUnit XML formatted results to this file e.g: junit.xml")
}

func getJunitReport(results []*pkg.CheckResult) string {
	var testCases []console.JUnitTestCase
	var failed int
	var totalTime int64
	for _, result := range results {
		totalTime += result.Duration
		testCase := console.JUnitTestCase{
			Classname: result.Check.GetType(),
			Name:      result.Check.GetDescription(),
			Time:      strconv.Itoa(int(result.Duration)),
		}
		if !result.Pass {
			failed++
			testCase.Failure = &console.JUnitFailure{
				Message: result.Message,
			}
		}
		testCases = append(testCases, testCase)
	}
	testSuite := console.JUnitTestSuite{
		Tests:     len(results),
		Failures:  failed,
		Time:      strconv.Itoa(int(totalTime)),
		Name:      "canary-checker-run",
		TestCases: testCases,
	}
	testSuites := console.JUnitTestSuites{
		Suites: []console.JUnitTestSuite{
			testSuite,
		},
	}
	report, err := testSuites.ToXML()
	if err != nil {
		logger.Fatalf("error creating junit results: %v", err)
	}
	return report
}

func CleanupFilename(fileName string) string {
	removeSuffix := fileName[:len(fileName)-len(filepath.Ext(fileName))]
	return strings.Replace(removeSuffix, "_", "", -1)
}
