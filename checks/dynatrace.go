package checks

import (
	"github.com/dynatrace-ace/dynatrace-go-api-client/api/v2/environment/dynatrace"
	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

type DynatraceChecker struct{}

func (t *DynatraceChecker) Type() string {
	return "dynatrace"
}

func (t *DynatraceChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.Dynatrace {
		results = append(results, t.Check(ctx, conf)...)
	}
	return results
}

func (t *DynatraceChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.DynatraceCheck)

	var results pkg.Results
	result := pkg.Success(check, ctx.Canary)
	results = append(results, result)

	_, apiKey, err := ctx.Kommons.GetEnvValue(check.APIKey, check.Namespace)
	if err != nil {
		return results.Failf("error getting Dynatrace API key: %v", err)
	}

	config := dynatrace.NewConfiguration()
	config.Host = check.Host
	config.Scheme = check.Scheme
	config.DefaultHeader = map[string]string{
		"Authorization": "Api-Token " + apiKey,
	}

	apiClient := dynatrace.NewAPIClient(config)
	problems, apiResponse, err := apiClient.ProblemsApi.GetProblems(ctx).Execute()
	if err != nil {
		return results.Failf("error getting Dynatrace problems: %s", err.Error())
	}
	defer apiResponse.Body.Close()

	var problemDetails []map[string]any
	for _, problem := range *problems.Problems {
		data := map[string]any{
			"title":            problem.Title,
			"status":           problem.Status,
			"impactLevel":      problem.ImpactLevel,
			"severity":         problem.SeverityLevel,
			"impactedEntities": problem.ImpactedEntities,
			"affectedEntities": problem.AffectedEntities,
		}

		if problem.EntityTags != nil && len(*problem.EntityTags) != 0 {
			var labels = make(map[string]string, len(*problem.EntityTags))
			for _, entity := range *problem.EntityTags {
				if entity.Key == nil {
					continue
				}

				labels[*entity.Key] = *entity.Value
			}

			data["labels"] = labels
			data["entityTags"] = problem.EntityTags
		}

		if problem.EvidenceDetails != nil {
			data["totalEvidences"] = problem.EvidenceDetails.TotalCount
			data["evidences"] = problem.EvidenceDetails.Details
		}

		problemDetails = append(problemDetails, data)
	}

	result.AddDetails(problemDetails)
	return results
}
