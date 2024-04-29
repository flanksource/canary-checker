package checks

import (
	"encoding/json"
	"fmt"
	osExec "os/exec"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

func init() {
	//register metrics here
}

type GitHubChecker struct {
}

func (c *GitHubChecker) Type() string {
	return "github"
}

func (c *GitHubChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.GitHub {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

func (c *GitHubChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.GitHubCheck)
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	var githubToken string
	if connection, err := ctx.HydrateConnectionByURL(check.ConnectionName); err != nil {
		return results.Errorf("failed to find connection for github token %q: %v", check.ConnectionName, err)
	} else if connection != nil {
		githubToken = connection.Password
	} else {
		githubToken, err = ctx.GetEnvValueFromCache(check.GithubToken)
		if err != nil {
			return results.Errorf("error fetching github token from env cache: %v", err)
		}
	}

	askGitCmd := fmt.Sprintf("mergestat \"%v\" --format json", check.Query)
	if ctx.IsTrace() {
		ctx.Tracef("Executing askgit command: %v", askGitCmd)
	}
	cmd := osExec.Command("bash", "-c", askGitCmd)
	cmd.Env = append(cmd.Env, "GITHUB_TOKEN="+githubToken)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return results.Errorf("error executing askgit command. output=%q: %v", output, err)
	}

	var rowResults = make([]map[string]any, 0)
	err = json.Unmarshal(output, &rowResults)
	if err != nil {
		return results.Errorf("error parsing mergestat result: %v", err)
	}

	result.AddDetails(rowResults)
	return results
}
