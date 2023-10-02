package checks

import (
	"fmt"
	"os"
	"path"
	"time"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

const (
	TestFileName = "test.txt"
)

type GitLabChecker struct{}

func (c *GitLabChecker) Type() string {
	return "gitlab"
}

func (c *GitLabChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.GitLab {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

func pushChanges(repoURL, username, password string) error {
	dir, err := os.MkdirTemp("", "repo-clone-")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(dir)

	r, err := git.PlainClone(dir, false, &git.CloneOptions{
		URL:  repoURL,
		Auth: &http.BasicAuth{Username: username, Password: password},
	})
	if err != nil {
		return fmt.Errorf("failed to clone repo: %v", err)
	}

	w, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %v", err)
	}

	filePath := path.Join(dir, TestFileName)
	currentTime := time.Now().Format("2006-01-02-04-05")

	// Check if the file exists and if not we will create it
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		err = os.WriteFile(filePath, []byte(currentTime), 0644)
		if err != nil {
			return fmt.Errorf("failed to write to file: %v", err)
		}
	}

	updatedContent := fmt.Sprintf("Updated at: %s", currentTime)
	err = os.WriteFile(filePath, []byte(updatedContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to update file: %v", err)
	}

	if _, err := w.Add(TestFileName); err != nil {
		return fmt.Errorf("failed to add changes to staging area: %v", err)
	}

	commitMsg := fmt.Sprintf("Updated %s with time %s", TestFileName, currentTime)
	if _, err := w.Commit(commitMsg, &git.CommitOptions{
		Author: &object.Signature{
			Name:  username,
			Email: fmt.Sprintf("%s@canary-checker.org", username),
			When:  time.Now(),
		},
	}); err != nil {
		return fmt.Errorf("failed to commit changes: %v", err)
	}

	if err := r.Push(&git.PushOptions{
		Auth: &http.BasicAuth{Username: username, Password: password},
	}); err != nil {
		return fmt.Errorf("failed to push changes: %v", err)
	}

	return nil
}

func (c *GitLabChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.GitLabCheck)
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	// Fetching GitLab Token
	password, err := ctx.GetEnvValueFromCache(check.Password)
	if err != nil {
		return results.Failf("error fetching gitlab token from env cache: %v", err)
	}

	// Push Changes
	if err := pushChanges(check.Repository, check.Username, password); err != nil {
		return results.Failf("error pushing changes: %v", err)
	}

	details := map[string]string{
		"msg": "GitLab pull/push command succeeded.",
	}
	result.AddDetails(details)

	return results
}
