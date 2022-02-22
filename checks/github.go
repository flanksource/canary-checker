package checks

import (
	"encoding/json"
	"fmt"
	osExec "os/exec"
	"strings"

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
	_, githubToken, err := ctx.Kommons.GetEnvValue(*check.GithubToken, ctx.Canary.GetNamespace())
	if err != nil {
		return results.Failf("error fetching github token: %v", err)
	}
	askGitCmd := fmt.Sprintf("GITHUB_TOKEN=%v askgit \"%v\" --format json", githubToken, check.Query)
	cmd := osExec.Command("bash", "-c", askGitCmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return results.Failf("error executing askgit command: %v", err)
	}
	rows := string(output)
	var rowResults = make([]map[string]string, 0)
	for _, row := range strings.Split(rows, "\n") {
		if row == "" {
			continue
		}
		var rowResult map[string]string
		err := json.Unmarshal([]byte(row), &rowResult)
		if err != nil {
			return results.Failf("error parsing askgit result: %v", err)
		}

		rowResults = append(rowResults, rowResult)
	}
	result.AddDetails(rowResults)
	return results
}
