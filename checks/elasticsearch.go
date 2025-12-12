package checks

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"

	"github.com/elastic/go-elasticsearch/v8"
)

type ElasticsearchChecker struct{}

func (c *ElasticsearchChecker) Type() string {
	return "elasticsearch"
}

func (c *ElasticsearchChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.Elasticsearch {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

func (c *ElasticsearchChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.ElasticsearchCheck)
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	connection, err := ctx.GetConnection(check.Connection)
	if err != nil {
		return results.Failf("error getting connection: %v", err)
	}

	cfg := elasticsearch.Config{
		Addresses: []string{connection.URL},
		Username:  connection.Username,
		Password:  connection.Password,
	}

	es, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return results.ErrorMessage(err)
	}

	body := strings.NewReader(check.Query)

	res, err := es.Search(
		es.Search.WithIndex(check.Index),
		es.Search.WithBody(body),
	)
	if err != nil {
		return results.ErrorMessage(err)
	}

	if res.IsError() {
		var e map[string]any
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			return results.ErrorMessage(fmt.Errorf("error parsing the response body: %w", err))
		}

		if errorResponse, ok := e["error"].(map[string]any); ok {
			return results.ErrorMessage(fmt.Errorf("error from elasticsearch [%s]: %v, %v",
				res.Status(),
				errorResponse["type"],
				errorResponse["reason"],
			))
		}

		return results.ErrorMessage(fmt.Errorf("error from elasticsearch [%s]: %v", res.Status(), e))
	}

	// We are closing the body after error as the Body object is not set in case of error
	// leading to nil pointer errors
	defer res.Body.Close()
	var r map[string]any
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return results.ErrorMessage(
			fmt.Errorf("error parsing the response body: %s", err),
		)
	}

	count := int(r["hits"].(map[string]any)["total"].(map[string]any)["value"].(float64))

	if count == 0 && check.ShouldMarkFailOnEmpty() {
		return results.Failf("Query has returned empty value")
	}

	if count != check.Results {
		return results.Failf("Query return %d rows, expected %d", count, check.Results)
	}

	result.AddDetails(r)
	return results
}
