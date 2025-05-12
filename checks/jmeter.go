package checks

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/flanksource/canary-checker/api/context"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/exec"
	"github.com/jszwec/csvutil"
	"k8s.io/apimachinery/pkg/util/rand"
)

func init() {
	//register metrics here
}

type JmeterChecker struct {
}

func (c *JmeterChecker) Type() string {
	return "jmeter"
}

func (c *JmeterChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.Jmeter {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

func (c *JmeterChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.JmeterCheck)
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	namespace := ctx.Canary.Namespace
	//FIXME: the jmx file should not be cached
	value, err := ctx.GetEnvValueFromCache(check.Jmx, ctx.GetNamespace())
	if err != nil {
		return results.Failf("Failed to parse the jmx plan: %v", err)
	}

	testPlanFilename := fmt.Sprintf("/tmp/jmx-%s-%s-%d.jmx", namespace, check.Jmx.Name, rand.Int())
	logFilename := fmt.Sprintf("/tmp/jmx-%s-%s-%d.jtl", namespace, check.Jmx.Name, rand.Int())
	err = os.WriteFile(testPlanFilename, []byte(value), 0755)
	defer os.Remove(testPlanFilename) // nolint: errcheck
	if err != nil {
		return results.Failf("unable to write test plan file")
	}

	var host string
	var port string
	if check.Host != "" {
		host = "-H " + check.Host
	}
	if check.Port != 0 {
		port = "-P " + string(check.Port)
	}
	jmeterCmd := fmt.Sprintf("jmeter -n %s %s -t %s %s %s -l %s", getProperties(check.Properties), getSystemProperties(check.SystemProperties), testPlanFilename, host, port, logFilename)
	_, ok := exec.SafeExec("%s", jmeterCmd)
	defer os.Remove(logFilename) // nolint: errcheck
	if !ok {
		return results.Failf("error running the jmeter command: %v", jmeterCmd)
	}
	raw, err := os.ReadFile(logFilename)
	if err != nil {
		return results.Failf("error opening the log file: %v", err)
	}
	elapsedTime, err := checkLogs(raw)
	if err != nil {
		return results.Failf("check failed: %v", err)
	}
	totalDuration := time.Duration(elapsedTime) * time.Millisecond
	if check.ResponseDuration != "" {
		resDuration, err := time.ParseDuration(check.ResponseDuration)
		if err != nil {
			return results.Failf("error parsing response duration: %v", err)
		}
		if totalDuration > resDuration {
			return results.Failf("the response took %v longer than specified", (totalDuration - resDuration).String())
		}
	}

	return results
}

func getProperties(properties []string) string {
	var props string
	for _, prop := range properties {
		props += " -J" + prop
	}
	return props
}
func getSystemProperties(properties []string) string {
	var props string
	for _, prop := range properties {
		props += " -D" + prop
	}
	return props
}

type JMeterRecord struct {
	Elapsed        int64  `csv:"elapsed"`
	Success        bool   `csv:"success"`
	FailureMessage string `csv:"failureMessage,omitempty"`
}

func checkLogs(r []byte) (int64, error) {
	var err error
	var elapsedTime int64
	var failMessage string
	var records []JMeterRecord
	failure := false

	err = csvutil.Unmarshal(r, &records)
	if err != nil {
		return elapsedTime, err
	}

	for i := range records {
		elapsedTime += records[i].Elapsed
		if !records[i].Success {
			failure = true
			failMessage += "\n"
			failMessage += records[i].FailureMessage
		}
	}
	if failure {
		return elapsedTime, errors.New(failMessage)
	}
	return elapsedTime, nil
}
