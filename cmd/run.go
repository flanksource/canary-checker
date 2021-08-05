package cmd

import (
	"fmt"
	"io/ioutil"
	"strconv"

	"github.com/flanksource/commons/console"

	"github.com/spf13/cobra"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
)

var Run = &cobra.Command{
	Use:   "run",
	Short: "Execute checks and return",
	Run: func(cmd *cobra.Command, args []string) {
		configfile, _ := cmd.Flags().GetString("configfile")
		namespace, _ := cmd.Flags().GetString("namespace")
		junitFile, _ := cmd.Flags().GetString("junit-file")
		config := pkg.ParseConfig(configfile)
		failed := 0
		results := RunChecks(config, namespace)
		if junitFile != "" {
			report := getJunitReport(results)
			err := ioutil.WriteFile(junitFile, []byte(report), 0755)
			if err != nil {
				logger.Fatalf("%d checks failed", failed)
			}
		}
		for _, result := range results {
			fmt.Println(result)
			if !result.Pass {
				failed++
			}
		}
		if failed > 0 {
			logger.Fatalf("%d checks failed", failed)
		}
	},
}

func init() {
	Run.Flags().StringP("configfile", "c", "", "Specify configfile")
	Run.Flags().StringP("namespace", "n", "", "Specify namespace")
	Run.Flags().StringP("junit-file", "j", "", "path to file where junit results will be published. E.g: junit.xml")
}
func RunChecks(config v1.CanarySpec, namespace string) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	kommonsClient, err := pkg.NewKommonsClient()
	if err != nil {
		logger.Warnf("Failed to get kommons client, features that read kubernetes configs will fail: %v", err)
	}

	config.SetNamespaces(namespace)
	config.SetSQLDrivers()
	for _, c := range checks.All {
		switch cs := c.(type) {
		case checks.SetsClient:
			cs.SetClient(kommonsClient)
		}
		result := c.Run(config)
		for _, r := range result {
			if r != nil {
				results = append(results, r)
			}
		}
	}
	return results
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
