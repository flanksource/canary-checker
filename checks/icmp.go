package checks

import (
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/prometheus/client_golang/prometheus"
)

type IcmpChecker struct{}

// Type: returns checker type
func (c *IcmpChecker) Type() string {
	return "icmp"
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *IcmpChecker) Run(config pkg.Config) []*pkg.CheckResult {
	var checks []*pkg.CheckResult
	for _, conf := range config.ICMP {
		for _, result := range c.Check(conf.ICMPCheck) {
			checks = append(checks, result)
			fmt.Println(result)
		}
	}
	return checks
}