package checks

import (
	"fmt"
	"time"

	"github.com/joshdk/go-junit"
)

const failTestCount = 10

// Test represents the results of a single test run.
type JunitTest struct {
	// Name is a descriptor given to the test.
	Name string `json:"name" yaml:"name"`

	// Classname is an additional descriptor for the hierarchy of the test.
	Classname string `json:"classname" yaml:"classname"`

	// Duration is the total time taken to run the tests.
	Duration float64 `json:"duration" yaml:"duration"`

	// Status is the result of the test. Status values are passed, skipped,
	// failure, & error.
	Status junit.Status `json:"status" yaml:"status"`

	// Message is an textual description optionally included with a skipped,
	// failure, or error test case.
	Message string `json:"message,omitempty" yaml:"message,omitempty"`

	// Error is a record of the failure or error of a test, if applicable.
	//
	// The following relations should hold true.
	//   Error == nil && (Status == Passed || Status == Skipped)
	//   Error != nil && (Status == Failed || Status == Error)
	Error error `json:"error,omitempty" yaml:"error,omitempty"`

	// Additional properties from XML node attributes.
	// Some tools use them to store additional information about test location.
	Properties map[string]string `json:"properties,omitempty" yaml:"properties,omitempty"`

	// SystemOut is textual output for the test case. Usually output that is
	// written to stdout.
	SystemOut string `json:"stdout,omitempty" yaml:"stdout,omitempty"`

	// SystemErr is textual error output for the test case. Usually output that is
	// written to stderr.
	SystemErr string `json:"stderr,omitempty" yaml:"stderr,omitempty"`
}

type JunitTestSuite struct {
	Name   string `json:"name"`
	Totals `json:",inline"`
	Tests  []JunitTest `json:"tests"`
}

func (suites JunitTestSuites) GetMessages() string {
	var message string
	count := 0
	for _, suite := range suites.Suites {
		for _, test := range suite.Tests {
			if test.Status == junit.StatusFailed {
				message = message + "\n" + test.Name
				count++
			}
			if count >= failTestCount {
				return message
			}
		}
	}
	return message
}

type JunitTestSuites struct {
	Suites []JunitTestSuite `json:"suites,omitempty"`
	Totals `json:",inline"`
}

type Totals struct {
	Passed   int     `json:"passed"`
	Failed   int     `json:"failed"`
	Skipped  int     `json:"skipped,omitempty"`
	Error    int     `json:"error,omitempty"`
	Duration float64 `json:"duration"`
}

func (t Totals) String() string {
	s := ""
	if t.Passed > 0 {
		s += fmt.Sprintf("%d passed", t.Passed)
	}
	if t.Failed > 0 {
		if s != "" {
			s += ", "
		}
		s += fmt.Sprintf("%d failed", t.Failed)
	}

	if t.Error > 0 {
		if s != "" {
			s += ", "
		}
		s += fmt.Sprintf("%d errors", t.Error)
	}

	if t.Skipped > 0 {
		if s != "" {
			s += ", "
		}
		s += fmt.Sprintf("%d skipped", t.Skipped)
	}

	if t.Duration > 0 {
		if s != "" {
			s += " "
		}
		s += fmt.Sprintf(" in %s", time.Duration(t.Duration)*time.Second)
	}

	return s
}

func FromTotals(t junit.Totals) Totals {
	return Totals{
		Passed:   t.Passed,
		Error:    t.Error,
		Failed:   t.Failed,
		Skipped:  t.Skipped,
		Duration: t.Duration.Seconds(),
	}
}

func FromTest(test junit.Test) JunitTest {
	if test.Classname != "" {
		delete(test.Properties, "classname")
	}
	if test.Name != "" {
		delete(test.Properties, "name")
	}
	if test.Duration.String() != "" {
		delete(test.Properties, "time")
	}

	return JunitTest{
		Name:       test.Name,
		Classname:  test.Classname,
		Status:     test.Status,
		Duration:   test.Duration.Seconds(),
		Properties: test.Properties,
		Message:    test.Message,
		Error:      test.Error,
		SystemOut:  test.SystemOut,
		SystemErr:  test.SystemErr,
	}
}

func (t Totals) Add(other Totals) Totals {
	t.Duration += other.Duration
	t.Passed += other.Passed
	t.Error += other.Error
	t.Failed += other.Failed
	t.Skipped += other.Skipped
	return t
}

func (suites JunitTestSuites) Append(suite junit.Suite) JunitTestSuites {
	suite.Aggregate()
	_suite := JunitTestSuite{
		Name:   suite.Name,
		Totals: FromTotals(suite.Totals),
	}
	for _, test := range suite.Tests {
		_suite.Tests = append(_suite.Tests, FromTest(test))
	}
	suites.Suites = append(suites.Suites, _suite)
	suites.Totals = suites.Totals.Add(_suite.Totals)
	return suites
}

func (suites JunitTestSuites) Ingest(xml string) (JunitTestSuites, error) {
	testSuite, err := junit.Ingest([]byte(xml))
	if err != nil {
		return suites, err
	}
	for _, suite := range testSuite {
		suites = suites.Append(suite)
	}
	return suites, nil
}
