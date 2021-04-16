package checks

import (
	"fmt"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/exec"
	"github.com/flanksource/kommons"
	"github.com/recursionpharma/go-csv-map"
	"io"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/util/rand"
	"os"
	"strconv"
	"time"
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
	jmeterCheck := extConfig.(v1.JmeterCheck)
	client := c.GetClient()
	_, value, err := client.GetEnvValue(jmeterCheck.Jmx, jmeterCheck.GetNamespace())
	if err != nil {
		return Failf(jmeterCheck, "Failed to parse the jmx plan: %v", err)
	}
	testPlanFilename := fmt.Sprintf("/tmp/jmx-%s-%s-%d.jmx", jmeterCheck.GetNamespace(), jmeterCheck.Jmx.Name, rand.Int())
	logFilename := fmt.Sprintf("/tmp/jmx-%s-%s-%d.jtl", jmeterCheck.GetNamespace(), jmeterCheck.Jmx.Name, rand.Int())
	err = ioutil.WriteFile(testPlanFilename, []byte(value), 0755)
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
	if !ok {
		return Failf(jmeterCheck, "error running the jmeter command: %v", jmeterCmd)
	}
	f, err := os.Open(logFilename)
	if err != nil {
		return Failf(jmeterCheck, "error opening the log file: %v", err)
	}
	defer f.Close()
	defer os.Remove(logFilename)
	defer os.Remove(testPlanFilename)

	timestamp, err := checkLogs(f)
	if err != nil {
		return Failf(jmeterCheck, "check failed: %v", err)
	}
	unixTime, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return Failf(jmeterCheck, "failed to parse timestamp: %v", err)
	}

	return Success(jmeterCheck, time.Unix(unixTime, 0))
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

func checkLogs(r io.Reader) (timestamp string, err error) {
	var failMessage string
	csvReader := csvmap.NewReader(r)
	csvReader.Columns, err = csvReader.ReadHeader()
	if err != nil {
		return
	}
	records, err := csvReader.ReadAll()
	if err != nil {
		return
	}
	if records != nil {
		timestamp = records[0]["timeStamp"]
	}
	for i, _ := range records {
		if records[i]["success"] == "false" {
			failMessage += records[i]["failureMessage"]
		}
	}
	if failMessage != "" {
		return timestamp, fmt.Errorf(failMessage)
	}
	return
}
