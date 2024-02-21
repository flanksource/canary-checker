package checks

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/pipelines"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/duty/models"
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

	var err error
	var c *models.Connection
	if check.PersonalAccessToken.ValueStatic != "" {
		c = &models.Connection{Password: check.PersonalAccessToken.ValueStatic}
	} else if c, err = ctx.HydrateConnectionByURL(check.ConnectionName); err != nil {
		return results.Failf("failed to hydrate connection: %v", err)
	} else if c != nil {
		if c, err = c.Merge(ctx, check); err != nil {
			return results.Failf("failed to merge connection: %v", err)
		}
	}

	connection := azuredevops.NewPatConnection(fmt.Sprintf("https://dev.azure.com/%s", check.Organization), c.Password)
	coreClient, err := core.NewClient(ctx, connection)
	if err != nil {
		return results.ErrorMessage(fmt.Errorf("failed to create core client: %w", err))
	}

	project, err := coreClient.GetProject(ctx, core.GetProjectArgs{ProjectId: &check.Project})
	if err != nil {
		return results.ErrorMessage(fmt.Errorf("failed to get project (name=%s): %w", check.Project, err))
	}
	projectID := project.Id.String()

	pipelineRegexp, err := regexp.Compile(check.Pipeline)
	isRegexp := nil == err
	// regexp compilation failed means we assume that it's a literal string.
	// However, even literal strings can be valid regexp, like "Hello world".

	pipelineClient := pipelines.NewClient(ctx, connection)
	allPipelines, err := pipelineClient.ListPipelines(ctx, pipelines.ListPipelinesArgs{Project: &projectID})
	if err != nil {
		return results.ErrorMessage(fmt.Errorf("failed to get pipeline (project=%s): %w", check.Project, err))
	}

	for _, pipeline := range *allPipelines {
		if isRegexp {
			matched := pipelineRegexp.MatchString(*pipeline.Name)
			if !matched {
				continue
			}
		} else if *pipeline.Name != check.Pipeline {
			continue
		}

		// This fetches top 10,000 runs for a particular pipeline.
		// Unfortunately, there's no way to fetch the latest run for a pipeline.
		// Additionally, this endpoint doesn't support the filters we require
		// - fetching only X amount of runs
		// - fetching only those runs that have completed
		// https://learn.microsoft.com/en-us/rest/api/azure/devops/pipelines/runs/list?view=azure-devops-rest-7.1
		runs, err := pipelineClient.ListRuns(ctx, pipelines.ListRunsArgs{PipelineId: pipeline.Id, Project: &projectID})
		if err != nil {
			return results.ErrorMessage(fmt.Errorf("failed to get runs (pipeline=%s): %w", check.Pipeline, err))
		}

		latestRun := getLatestCompletedRun(*runs)
		if latestRun == nil {
			continue
		}

		if !matchPipelineVariables(check.Variables, latestRun.Variables) {
			continue
		}

		// Need to query the Run API to get more details about it
		// because the ListRuns API doesn't return Resources.
		latestRun, err = pipelineClient.GetRun(ctx, pipelines.GetRunArgs{Project: &projectID, PipelineId: pipeline.Id, RunId: (*runs)[0].Id})
		if err != nil {
			return results.ErrorMessage(fmt.Errorf("failed to get run (pipeline=%s): %w", check.Pipeline, err))
		}

		if !matchBranchNames(check.Branches, latestRun.Resources) {
			continue
		}

		if check.ThresholdMillis != nil {
			runDuration := latestRun.FinishedDate.Time.Sub(latestRun.CreatedDate.Time)
			threhold := time.Duration(*check.ThresholdMillis) * time.Millisecond
			if runDuration > threhold {
				return results.Failf("Runtime:%v was over the threshold:%v", runDuration, threhold)
			}
		}

		if *latestRun.Result != pipelines.RunResultValues.Succeeded {
			return results.Failf("Runtime completed with unsuccessful result: %s", *latestRun.Result)
		}
	}

	return results
}

// getLatestCompletedRun returns the latest completed pipeline run.
// It assumes that the runs are ordered by completion date in descending order.
func getLatestCompletedRun(runs []pipelines.Run) *pipelines.Run {
	if len(runs) == 0 {
		return nil
	}

	for _, run := range runs {
		if *run.State != pipelines.RunStateValues.Completed {
			continue
		}

		return &run
	}

	return nil
}

func matchPipelineVariables(want map[string]string, got *map[string]pipelines.Variable) bool {
	if len(want) == 0 {
		return true
	}

	if len(want) != 0 && len(*got) == 0 {
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

func matchBranchNames(branches []string, resources *pipelines.RunResources) bool {
	if len(branches) == 0 {
		return true
	}

	if len(branches) != 0 && resources == nil {
		return false
	}

	branchName := extractBranchName(resources)
	for _, w := range branches {
		if branchName == w {
			return true
		}
	}

	return false
}

// extractBranchName extracts the name of the branch from a RunResources object.
// The branch name is extracted from the refname which is of the form "refs/heads/2pm".
func extractBranchName(got *pipelines.RunResources) string {
	repo, ok := (*got.Repositories)["self"]
	if !ok {
		return ""
	}

	if repo.RefName == nil {
		return ""
	}

	sections := strings.Split(*repo.RefName, "/")
	if len(sections) == 0 {
		return ""
	}

	return sections[len(sections)-1]
}
