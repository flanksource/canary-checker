package checks

import (
	"fmt"
	"regexp"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/pipelines"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
)

type AzureDevopsChecker struct {
}

func (t *AzureDevopsChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.AzureDevops {
		results = append(results, t.check(ctx, conf)...)
	}
	return results
}

func (t *AzureDevopsChecker) Type() string {
	return "azuredevops"
}

func (t *AzureDevopsChecker) check(ctx *context.Context, check v1.AzureDevopsCheck) pkg.Results {
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	connection := azuredevops.NewPatConnection(fmt.Sprintf("https://dev.azure.com/%s", check.Organization), check.PersonalAccessToken)
	coreClient, err := core.NewClient(ctx, connection)
	if err != nil {
		return results.ErrorMessage(fmt.Errorf("failed to create core client: %w", err))
	}

	project, err := coreClient.GetProject(ctx, core.GetProjectArgs{ProjectId: &check.Project})
	if err != nil {
		return results.ErrorMessage(fmt.Errorf("failed to get project (name=%s): %w", check.Project, err))
	}

	projectID := project.Id.String()
	pipelineClient := pipelines.NewClient(ctx, connection)
	allPipelines, err := pipelineClient.ListPipelines(ctx, pipelines.ListPipelinesArgs{Project: &projectID})
	if err != nil {
		return results.ErrorMessage(fmt.Errorf("failed to get pipeline (name=%s): %w", check.Pipeline, err))
	}

	var isRegexp bool
	pipelineRegex, err := regexp.Compile(check.Pipeline)
	if err != nil {
		// regexp compilation failed means we assume that it's a literal string.
		// However, even literal strings can be valid regexp, like "Hello world".
	} else {
		isRegexp = true
	}

	var matchedPipelines []pipelines.Pipeline
	for _, p := range *allPipelines {
		if isRegexp {
			matched := pipelineRegex.MatchString(*p.Name)
			if !matched {
				continue
			}
		} else {
			if *p.Name != check.Pipeline {
				continue
			}
		}

		logger.Infof("Checking pipeline %s", *p.Name)
		matchedPipelines = append(matchedPipelines, p)
	}

	return results
}
