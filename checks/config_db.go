package checks

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	logger.Tracef("[%v] query: %v", check.Host, check.Query)
	var endpoint string
	var query = url.QueryEscape(check.Query)
	if strings.HasSuffix(check.Host, "/") {
		endpoint = fmt.Sprintf("%v?query=%v", check.Host, query)
	} else {
		endpoint = fmt.Sprintf("%v/?query=%v", check.Host, query)
	}
	resp, err := http.Get(endpoint)
	if err != nil {
		return results.Failf("Failed to send GET request, %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusCreated {
		return results.Failf("Failed to get response, %v", resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return results.Failf("Failed to read the response body: %v", err)
	}
	queryResult := ConfigDBQueryResult{}
	if err := json.Unmarshal(body, &queryResult); err != nil {
		results.Failf("failed to unmarshal the response body into QueryResult: %v", err)
	}
	result.AddDetails(queryResult)
	return results
}

type ConfigDBQueryResult struct {
	Count   int                      `json:"count"`
	Columns []ConfigDBQueryColumn    `json:"columns"`
	Results []map[string]interface{} `json:"results"`
}

type ConfigDBQueryColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}
