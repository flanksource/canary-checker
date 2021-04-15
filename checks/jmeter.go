package checks

import (
	"fmt"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/exec"
	"github.com/flanksource/kommons"
	"io/ioutil"
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
	start := time.Now()
	jmeterCheck := extConfig.(v1.JmeterCheck)
	client := c.GetClient()
	_, value, err := client.GetEnvValue(jmeterCheck.JmxFrom, jmeterCheck.GetNamespace())
	if err != nil {
		return Failf(jmeterCheck, "Failed to parse the jmx plan: %v", err)
	}

	filename := fmt.Sprintf("/tmp/jmx-%s-%s.jmx", jmeterCheck.GetNamespace(), jmeterCheck.JmxFrom.Name)
	err = ioutil.WriteFile(filename, []byte(value), 0755)
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
	jmeterCmd := fmt.Sprintf("jmeter -n %s %s -t %s %s %s", getProperties(jmeterCheck.Properties), getSystemProperties(jmeterCheck.SystemProperties), filename, host, port)
	err = exec.Exec(jmeterCmd)
	if err != nil {
		return Failf(jmeterCheck, "error running the jmeter command: %v", jmeterCmd)
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
