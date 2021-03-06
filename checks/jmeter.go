package checks

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"time"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/exec"
	"github.com/flanksource/kommons"
	csvmap "github.com/recursionpharma/go-csv-map"
	"k8s.io/apimachinery/pkg/util/rand"
)

func init() {
	//register metrics here
}

type JmeterChecker struct {
	kommons *kommons.Client `yaml:"-" json:"-"`
}

func (c *JmeterChecker) SetClient(client *kommons.Client) {
	c.kommons = client
}

func (c JmeterChecker) GetClient() *kommons.Client {
	return c.kommons
}

func (c *JmeterChecker) Type() string {
	return "jmeter"
}

func (c *JmeterChecker) Run(config v1.CanarySpec) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range config.Jmeter {
		results = append(results, c.Check(conf))
	}
	return results
}

func (c *JmeterChecker) Check(extConfig external.Check) *pkg.CheckResult {
	start := time.Now()
	jmeterCheck := extConfig.(v1.JmeterCheck)
	client := c.GetClient()
	_, value, err := client.GetEnvValue(jmeterCheck.Jmx, jmeterCheck.GetNamespace())
	if err != nil {
		return Failf(jmeterCheck, "Failed to parse the jmx plan: %v", err)
	}
	testPlanFilename := fmt.Sprintf("/tmp/jmx-%s-%s-%d.jmx", jmeterCheck.GetNamespace(), jmeterCheck.Jmx.Name, rand.Int())
	logFilename := fmt.Sprintf("/tmp/jmx-%s-%s-%d.jtl", jmeterCheck.GetNamespace(), jmeterCheck.Jmx.Name, rand.Int())
	err = ioutil.WriteFile(testPlanFilename, []byte(value), 0755)
	defer os.Remove(testPlanFilename) // nolint: errcheck
	if err != nil {
		return Failf(jmeterCheck, "unable to write test plan file")
	}
	var host string
	var port string
	if jmeterCheck.Host != "" {
		host = "-H " + jmeterCheck.Host
	}
	if jmeterCheck.Port != 0 {
		port = "-P " + string(jmeterCheck.Port)
	}
	jmeterCmd := fmt.Sprintf("jmeter -n %s %s -t %s %s %s -l %s", getProperties(jmeterCheck.Properties), getSystemProperties(jmeterCheck.SystemProperties), testPlanFilename, host, port, logFilename)
	_, ok := exec.SafeExec(jmeterCmd)
	defer os.Remove(logFilename) // nolint: errcheck
	if !ok {
		return Failf(jmeterCheck, "error running the jmeter command: %v", jmeterCmd)
	}
	f, err := os.Open(logFilename)
	if err != nil {
		return Failf(jmeterCheck, "error opening the log file: %v", err)
	}
	defer f.Close() // nolint: errcheck
	elapsedTime, err := checkLogs(f)
	if err != nil {
		return Failf(jmeterCheck, "check failed: %v", err)
	}
	totalDuration := time.Duration(elapsedTime) * time.Millisecond
	if jmeterCheck.ResponseDuration != "" {
		resDuration, err := time.ParseDuration(jmeterCheck.ResponseDuration)
		if err != nil {
			return Failf(jmeterCheck, "error parsing response duration: %v", err)
		}
		if totalDuration > resDuration {
			return Failf(jmeterCheck, "the response took %v longer than specified", (totalDuration - resDuration).String())
		}
	}
	return Success(jmeterCheck, start)
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

func checkLogs(r io.Reader) (int64, error) {
	var err error
	var elapsedTime int64
	var failMessage string
	csvReader := csvmap.NewReader(r)
	csvReader.Columns, err = csvReader.ReadHeader()
	if err != nil {
		return 0, err
	}
	records, err := csvReader.ReadAll()
	if err != nil {
		return 0, err
	}

	for i := range records {
		tempElapsed, err := strconv.ParseInt(records[i]["elapsed"], 10, 64)
		if err != nil {
			return elapsedTime, err
		}
		elapsedTime += tempElapsed
		if records[i]["success"] == "false" {
			failMessage += records[i]["failureMessage"]
		}
	}
	if failMessage != "" {
		return elapsedTime, fmt.Errorf(failMessage)
	}
	return elapsedTime, err
}
