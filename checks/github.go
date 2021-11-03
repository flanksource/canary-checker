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

func (c *GitHubChecker) Run(ctx *context.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range ctx.Canary.Spec.GitHub {
		result := c.Check(ctx, conf)
		if result != nil {
			results = append(results, result)
		}
	}
	return results
}

func (c *GitHubChecker) Check(ctx *context.Context, extConfig external.Check) *pkg.CheckResult {
	updated, err := Contextualise(extConfig, ctx)
	if err != nil {
		return pkg.Fail(extConfig, ctx.Canary)
	}
	check := updated.(v1.GitHubCheck)
	checkResult := pkg.Success(check, ctx.Canary)
	_, githubToken, err := ctx.Kommons.GetEnvValue(*check.GithubToken, ctx.Canary.GetNamespace())
	if err != nil {
		return checkResult.Failf("error fetching github token: %v", err)
	}
	askGitCmd := fmt.Sprintf("GITHUB_TOKEN=%v askgit \"%v\" --format json", githubToken, check.Query)
	cmd := osExec.Command("bash", "-c", askGitCmd)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return checkResult.Failf("error executing askgit command: %v", err)
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
			return checkResult.Failf("error parsing askgit result: %v", err)
		}

		rowResults = append(rowResults, rowResult)
	}
	checkResult.AddDetails(rowResults)
	return checkResult
}
