package checks

import (
	canaryContext "github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/query"
)

type CatalogChecker struct{}

func (c *CatalogChecker) Type() string {
	return "catalog"
}

func (c *CatalogChecker) Run(ctx *canaryContext.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.Catalog {
		results = append(results, c.Check(ctx, conf)...)
	}

	return results
}

func (c *CatalogChecker) Check(ctx *canaryContext.Context, check v1.CatalogCheck) pkg.Results {
	result := pkg.Success(check, ctx.Canary)

	var results pkg.Results
	results = append(results, result)

	items, err := query.FindConfigsByResourceSelector(ctx.Context, check.Selector...)
	if err != nil {
		return results.Failf("failed to fetch catalogs: %v", err)
	}

	queryResult := CatalogResult{Catalogs: items}
	result.AddDetails(queryResult)
	return results
}

type CatalogResult struct {
	Catalogs []models.ConfigItem `json:"catalogs"`
}
