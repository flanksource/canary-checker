package checks

import (
	"encoding/json"
	"fmt"
	"os"
	osExec "os/exec"
	"path/filepath"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/deps"
)

// GitHubChecker runs SQL queries against GitHub repositories using mergestat.
// Deprecated: This check type is deprecated and will be removed in a future release.
type GitHubChecker struct{}

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
		return results.Failf("failed to find connection for github token %q: %v", check.ConnectionName, err)
	} else if connection != nil {
		githubToken = connection.Password
	} else {
		githubToken, err = ctx.GetEnvValueFromCache(check.GithubToken, ctx.GetNamespace())
		if err != nil {
			return results.Failf("error fetching github token from env cache: %v", err)
		}
	}

	mergestatPath, err := installMergestat(ctx)
	if err != nil {
		return results.Failf("failed to install mergestat: %v", err)
	}

	askGitCmd := fmt.Sprintf("%s \"%v\" --format json", mergestatPath, check.Query)
	if ctx.IsTrace() {
		ctx.Tracef("Executing mergestat command: %v", askGitCmd)
	}
	cmd := osExec.Command("bash", "-c", askGitCmd)
	cmd.Env = append(os.Environ(), "GITHUB_TOKEN="+githubToken)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return results.Failf("error executing mergestat command. output=%q: %v", output, err)
	}

	var rowResults = make([]map[string]any, 0)
	err = json.Unmarshal(output, &rowResults)
	if err != nil {
		return results.Failf("error parsing mergestat result: %v", err)
	}

	result.AddDetails(rowResults)
	return results
}

func installMergestat(ctx *context.Context) (string, error) {
	binDir := filepath.Join(os.TempDir(), "canary-checker", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create bin directory: %w", err)
	}

	result, err := deps.InstallWithContext(ctx, "mergestat/mergestat-lite", "any",
		deps.WithBinDir(binDir),
	)
	if err != nil {
		return "", fmt.Errorf("failed to install mergestat: %w", err)
	}

	if ctx.IsDebug() && result != nil {
		ctx.Debugf("%s", result.Pretty().ANSI())
	}

	return filepath.Join(binDir, "mergestat"), nil
}
