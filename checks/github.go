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
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/duty"
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

	k8sClient, err := ctx.Kommons.GetClientset()
	if err != nil {
		return results.Failf("error getting k8s client from kommons client: %v", err)
	}

	var githubToken string
	if connection, err := duty.HydratedConnectionByURL(ctx, db.Gorm, k8sClient, ctx.Namespace, check.ConnectionName); err != nil {
		return results.Failf("failed to find connection for github token %q: %v", check.ConnectionName, err)
	} else if connection != nil {
		githubToken = connection.Password
	} else {
		githubToken, err = duty.GetEnvValueFromCache(k8sClient, check.GithubToken, ctx.Canary.GetNamespace())
		if err != nil {
			return results.Failf("error fetching github token from env cache: %v", err)
		}
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
