package checks

import (
	"github.com/dynatrace-ace/dynatrace-go-api-client/api/v2/environment/dynatrace"
	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
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

	apiKey, _, err := ctx.Kommons.GetEnvValue(check.APIKey, check.Namespace)
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

	logger.Infof("Found %d problems and %d warnings", len(*problems.Problems), len(*problems.Warnings))

	var problemDetails []map[string]any
	for _, problem := range *problems.Problems {
		problemDetails = append(problemDetails, map[string]any{
			"name": problem.Title,
			// "message": problem.
			"labels":   problem.EntityTags,
			"severity": problem.SeverityLevel,
		})
	}

	result.AddDetails(problemDetails)
	return results
}
