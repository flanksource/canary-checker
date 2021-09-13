package output

import (
	"strconv"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/console"
	"github.com/flanksource/commons/logger"
)

func GetJunitReport(results []*pkg.CheckResult) string {
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
