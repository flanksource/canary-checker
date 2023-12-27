package checks

import (
	canaryContext "github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	dutyConfig "github.com/flanksource/duty/config"
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

	if ctx.IsDebugEnabled() {
		ctx.Infof("query: %v", check.Query)
	}

	var results pkg.Results
	results = append(results, result)

	res, err := dutyConfig.Query(ctx, ctx.Pool(), check.Query)
	if err != nil {
		return results.Failf("failed running query: %v", err)
	}

	queryResult := ConfigDBQueryResult{Results: res}
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
