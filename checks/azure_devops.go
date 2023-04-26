package checks

import (
	"fmt"
	"regexp"
	"time"

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
		return results.ErrorMessage(fmt.Errorf("failed to get pipeline (project=%s): %w", check.Project, err))
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
		runs, err := pipelineClient.ListRuns(ctx, pipelines.ListRunsArgs{PipelineId: p.Id, Project: &projectID})
		if err != nil {
			return results.ErrorMessage(fmt.Errorf("failed to get run (pipeline=%s): %w", check.Pipeline, err))
		}

		if len(*runs) == 0 {
			continue
		}

		latestRun := (*runs)[0]

		if !matchPipelineVariables(check.Variables, latestRun.Variables) {
			continue
		}

		if check.ThresholdMillis != nil {
			runDuration := latestRun.FinishedDate.Time.Sub(latestRun.CreatedDate.Time)
			if runDuration > time.Duration(*check.ThresholdMillis)*time.Millisecond {
				return results.Failf("Runtime:%v was over the threshold:%v", runDuration, *check.ThresholdMillis)
			}
		}

		if *latestRun.Result != pipelines.RunResultValues.Succeeded {
			return results.Failf("Runtime completed with unsuccessful result: %s", latestRun.Result)
		}

		matchedPipelines = append(matchedPipelines, p)
	}

	return results
}

func matchPipelineVariables(want map[string]string, got *map[string]pipelines.Variable) bool {
	if len(want) == 0 {
		return true
	}

	if len(*got) == 0 && len(want) != 0 {
		return false
	}

	for k, v := range want {
		if _, ok := (*got)[k]; !ok {
			return false
		}

		if *(*got)[k].Value != v {
			return false
		}
	}

	return true
}
