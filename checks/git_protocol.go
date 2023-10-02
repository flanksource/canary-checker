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
	DefaultFileName = "test.txt"
)

type GitProtocolChecker struct{}

func (c *GitProtocolChecker) Type() string {
	return "gitProtocol"
}

func (c *GitProtocolChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.GitProtocol {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

func pushChanges(repoURL, username, password, filename string) error {
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

	filePath := path.Join(dir, filename)
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

	if _, err := w.Add(filename); err != nil {
		return fmt.Errorf("failed to add changes to staging area: %v", err)
	}

	commitMsg := fmt.Sprintf("Updated %s with time %s", filename, currentTime)
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

func (c *GitProtocolChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.GitProtocolCheck)
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	filename := check.FileName

	// Fetching Git Username
	username, err := ctx.GetEnvValueFromCache(check.Username)
	if err != nil {
		return results.Failf("error fetching git user from env cache: %v", err)
	}
	// Fetching Git Password
	password, err := ctx.GetEnvValueFromCache(check.Password)
	if err != nil {
		return results.Failf("error fetching git password from env cache: %v", err)
	}

	if len(filename) == 0 {
		filename = DefaultFileName
	}

	// Push Changes
	if err := pushChanges(check.Repository, username, password, filename); err != nil {
		return results.Failf("error pushing changes: %v", err)
	}

	details := map[string]string{
		"msg": "GitLab pull/push command succeeded.",
	}
	result.AddDetails(details)

	return results
}
