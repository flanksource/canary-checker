package checks

import (
	"encoding/json"
	"strings"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	opensearch "github.com/opensearch-project/opensearch-go/v2"
)

// OpenSearchChecker checks whether the query returns the expected number of results.
type OpenSearchChecker struct{}

func (t *OpenSearchChecker) Type() string {
	return "opensearch"
}

func (t *OpenSearchChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.Opensearch {
		results = append(results, t.check(ctx, conf)...)
	}
	return results
}

func (t *OpenSearchChecker) check(ctx *context.Context, check v1.OpenSearchCheck) pkg.Results {
	result := pkg.Success(check, ctx.Canary)

	var results pkg.Results
	results = append(results, result)

	connection, err := ctx.GetConnection(check.Connection)
	if err != nil {
		return results.Failf("error getting connection: %v", err)
	}

	if connection.URL == "" {
		return results.Failf("Must specify a URL")
	}

	cfg := opensearch.Config{
		Username:  connection.Username,
		Password:  connection.Password,
		Addresses: []string{connection.URL},
	}

	osClient, err := opensearch.NewClient(cfg)
	if err != nil {
		return results.Failf("error creating the openSearch client: %v", err)
	}

	body := strings.NewReader(check.Query)
	res, err := osClient.Search(
		osClient.Search.WithContext(ctx),
		osClient.Search.WithIndex(check.Index),
		osClient.Search.WithBody(body),
	)
	if err != nil {
		return results.Failf("error searching: %v", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		var e OpenSearchErrorResponse
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			return results.Failf("[status=%s]: error parsing the response body: %s", res.Status(), err)
		} else {
			return results.Failf("[status=%s]: server responded with an error. type=%v, reason=%v", res.Status(), e.Error.Type, e.Error.Reason)
		}
	}

	var response OpenSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return results.Failf("error parsing the response body: %s", err)
	}

	if response.Hits.Total.Value == 0 && check.ShouldMarkFailOnEmpty() {
		return results.Failf("Query has returned empty value")
	}

	if response.Hits.Total.Value != check.Results {
		return results.Failf("Query returned %d rows, expected %d", response.Hits.Total.Value, check.Results)
	}

	result.AddDetails(response)
	return results
}

// OpenSearchResponse is a minimal struct representing a success response from open search.
type OpenSearchResponse struct {
	Hits struct {
		Total struct {
			Value int64 `json:"value"`
		} `json:"total"`
	} `json:"hits"`
}

// OpenSearchResponse is a minimal struct representing an error response from open search.
type OpenSearchErrorResponse struct {
	Error struct {
		Type   string `json:"type"`
		Reason string `json:"reason"`
	} `json:"error"`
}
