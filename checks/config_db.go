package checks

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	canaryContext "github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
)

type ConfigdbChecker struct{}

func (c *ConfigdbChecker) Type() string {
	return "configdb"
}

func (c *ConfigdbChecker) Run(ctx *canaryContext.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.ConfigDB {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

func (c *ConfigdbChecker) Check(ctx *canaryContext.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.ConfigDBCheck)
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)
	logger.Infof("Sending GET request to: %v with query param: %v", check.Host, check.Query)
	var url string
	if strings.HasSuffix(check.Host, "/") {
		url = fmt.Sprintf("%v?query=%v", check.Host, check.Query)
	} else {
		url = fmt.Sprintf("%v/?query=%v", check.Host, check.Query)
	}
	resp, err := http.Get(url)
	if err != nil {
		return results.Failf("Failed to send GET request, %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		return results.Failf("Failed to get response, %v", resp.Status)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return results.Failf("Failed to read the response body: %v", err)
	}

	result.AddDetails(body)
	return results
}
