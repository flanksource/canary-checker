package checks

import (
	"errors"
	"fmt"
	"strings"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	utils "github.com/flanksource/commons/collections"
	"github.com/flanksource/duty/connection"
)

type ArgoChecker struct{}

func (c *ArgoChecker) Type() string {
	return "argo"
}

func (c *ArgoChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.Argo {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

func (c *ArgoChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.ArgoCheck)
	result := pkg.Success(check, ctx.Canary)
	results := pkg.Results{result}

	if err := check.ArgoConnection.Hydrate(ctx); err != nil {
		var validationErr *connection.ArgoConnectionValidationError
		if errors.As(err, &validationErr) {
			return results.Invalidf("%v", validationErr)
		}
		return results.Failf("%v", err)
	}

	client, err := check.ArgoConnection.Client(ctx.Context)
	if err != nil {
		var validationErr *connection.ArgoConnectionValidationError
		if errors.As(err, &validationErr) {
			return results.Invalidf("%v", validationErr)
		}
		return results.Failf("failed creating argo client: %v", err)
	}

	var (
		repositories []connection.ArgoRepository
		clusters     []connection.ArgoCluster
		repoFailures []string
		cluFailures  []string
	)

	if len(check.Repositories) > 0 {
		repositories, err = client.ListRepositories(ctx.Context)
		if err != nil {
			return results.Failf("failed listing repositories: %v", err)
		}
		repoFailures = verifyRepositories(check.Repositories, repositories)
		result.AddData(map[string]any{
			"repositories": map[string]any{
				"selectors": repositorySelectorDetails(check.Repositories),
				"items":     repositoryDetails(repositories),
				"all":       hasRepositoryWildcard(check.Repositories),
			},
		})
	} else {
		result.AddData(map[string]any{"repositories": map[string]any{"skipped": true}})
	}

	if len(check.Clusters) > 0 {
		clusters, err = client.ListClusters(ctx.Context)
		if err != nil {
			return results.Failf("failed listing clusters: %v", err)
		}
		cluFailures = verifyClusters(check.Clusters, clusters)
		result.AddData(map[string]any{
			"clusters": map[string]any{
				"selectors": clusterSelectorDetails(check.Clusters),
				"items":     clusterDetails(clusters),
				"all":       hasClusterWildcard(check.Clusters),
			},
		})
	} else {
		result.AddData(map[string]any{"clusters": map[string]any{"skipped": true}})
	}

	allFailures := append(repoFailures, cluFailures...)
	if len(allFailures) > 0 {
		return results.Failf("%s", strings.Join(allFailures, "; "))
	}

	switch {
	case len(check.Repositories) == 0 && len(check.Clusters) == 0:
		result.ResultMessage("argocd authentication succeeded; repository and cluster checks skipped")
	case len(check.Repositories) == 0:
		result.ResultMessage("validated %d clusters", countValidatedClusters(check.Clusters, clusters))
	case len(check.Clusters) == 0:
		result.ResultMessage("validated %d repositories", countValidatedRepositories(check.Repositories, repositories))
	default:
		result.ResultMessage("validated %d repositories and %d clusters", countValidatedRepositories(check.Repositories, repositories), countValidatedClusters(check.Clusters, clusters))
	}

	return results
}

func verifyRepositories(selectors []v1.ArgoRequiredRepository, repositories []connection.ArgoRepository) []string {
	if len(selectors) == 0 {
		return nil
	}

	failures := make([]string, 0)
	if hasRepositoryWildcard(selectors) {
		if len(repositories) == 0 {
			return []string{"no repositories returned by argocd"}
		}
		for _, repo := range repositories {
			if !isSuccessfulConnection(repo.ConnectionState.Status) {
				failures = append(failures, fmt.Sprintf("repository %s connection status=%q message=%q", firstNonEmpty(repo.Repo, repo.Name), repo.ConnectionState.Status, repo.ConnectionState.Message))
			}
		}
		return failures
	}

	for _, required := range selectors {
		if required.Name == "" && required.Repo == "" {
			failures = append(failures, "repository selector must define name or repo")
			continue
		}

		matchedRepo, found := findRepository(required, repositories)
		if !found {
			failures = append(failures, fmt.Sprintf("required repository not found (name=%q repo=%q)", required.Name, required.Repo))
			continue
		}

		if !isSuccessfulConnection(matchedRepo.ConnectionState.Status) {
			failures = append(failures, fmt.Sprintf("repository %s connection status=%q message=%q", firstNonEmpty(matchedRepo.Repo, matchedRepo.Name), matchedRepo.ConnectionState.Status, matchedRepo.ConnectionState.Message))
		}
	}

	return failures
}

func verifyClusters(selectors []v1.ArgoRequiredCluster, clusters []connection.ArgoCluster) []string {
	if len(selectors) == 0 {
		return nil
	}

	failures := make([]string, 0)
	if hasClusterWildcard(selectors) {
		if len(clusters) == 0 {
			return []string{"no clusters returned by argocd"}
		}
		for _, cluster := range clusters {
			if !isSuccessfulConnection(cluster.ConnectionState.Status) {
				failures = append(failures, fmt.Sprintf("cluster %s connection status=%q message=%q", firstNonEmpty(cluster.Server, cluster.Name), cluster.ConnectionState.Status, cluster.ConnectionState.Message))
			}
		}
		return failures
	}

	for _, required := range selectors {
		if required.Name == "" && required.Server == "" {
			failures = append(failures, "cluster selector must define name or server")
			continue
		}

		matchedCluster, found := findCluster(required, clusters)
		if !found {
			failures = append(failures, fmt.Sprintf("required cluster not found (name=%q server=%q)", required.Name, required.Server))
			continue
		}

		if !isSuccessfulConnection(matchedCluster.ConnectionState.Status) {
			failures = append(failures, fmt.Sprintf("cluster %s connection status=%q message=%q", firstNonEmpty(matchedCluster.Server, matchedCluster.Name), matchedCluster.ConnectionState.Status, matchedCluster.ConnectionState.Message))
		}
	}

	return failures
}

func hasRepositoryWildcard(selectors []v1.ArgoRequiredRepository) bool {
	for _, selector := range selectors {
		if selector.Name == "*" || selector.Repo == "*" {
			return true
		}
	}
	return false
}

func hasClusterWildcard(selectors []v1.ArgoRequiredCluster) bool {
	for _, selector := range selectors {
		if selector.Name == "*" || selector.Server == "*" {
			return true
		}
	}
	return false
}

func countValidatedRepositories(selectors []v1.ArgoRequiredRepository, repositories []connection.ArgoRepository) int {
	if hasRepositoryWildcard(selectors) {
		return len(repositories)
	}
	return len(selectors)
}

func countValidatedClusters(selectors []v1.ArgoRequiredCluster, clusters []connection.ArgoCluster) int {
	if hasClusterWildcard(selectors) {
		return len(clusters)
	}
	return len(selectors)
}

func repositorySelectorDetails(selectors []v1.ArgoRequiredRepository) []map[string]any {
	items := make([]map[string]any, 0, len(selectors))
	for _, selector := range selectors {
		items = append(items, map[string]any{
			"name": selector.Name,
			"repo": selector.Repo,
		})
	}
	return items
}

func clusterSelectorDetails(selectors []v1.ArgoRequiredCluster) []map[string]any {
	items := make([]map[string]any, 0, len(selectors))
	for _, selector := range selectors {
		items = append(items, map[string]any{
			"name":   selector.Name,
			"server": selector.Server,
		})
	}
	return items
}

func isSuccessfulConnection(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), "successful")
}

func findRepository(required v1.ArgoRequiredRepository, repositories []connection.ArgoRepository) (connection.ArgoRepository, bool) {
	for _, repo := range repositories {
		if !matchesRepository(required, repo) {
			continue
		}
		return repo, true
	}
	return connection.ArgoRepository{}, false
}

func findCluster(required v1.ArgoRequiredCluster, clusters []connection.ArgoCluster) (connection.ArgoCluster, bool) {
	for _, cluster := range clusters {
		if !matchesCluster(required, cluster) {
			continue
		}
		return cluster, true
	}
	return connection.ArgoCluster{}, false
}

func matchesRepository(required v1.ArgoRequiredRepository, repo connection.ArgoRepository) bool {
	if required.Name != "" && !utils.MatchItems(repo.Name, required.Name) {
		return false
	}
	if required.Repo != "" && !utils.MatchItems(repo.Repo, required.Repo) {
		return false
	}
	return true
}

func matchesCluster(required v1.ArgoRequiredCluster, cluster connection.ArgoCluster) bool {
	if required.Name != "" && !utils.MatchItems(cluster.Name, required.Name) {
		return false
	}
	if required.Server != "" && !utils.MatchItems(cluster.Server, required.Server) {
		return false
	}
	return true
}

func firstNonEmpty(values ...string) string {
	for _, item := range values {
		if item != "" {
			return item
		}
	}
	return "unknown"
}

func repositoryDetails(repositories []connection.ArgoRepository) []map[string]any {
	items := make([]map[string]any, 0, len(repositories))
	for _, repo := range repositories {
		items = append(items, map[string]any{
			"name":    repo.Name,
			"repo":    repo.Repo,
			"status":  repo.ConnectionState.Status,
			"message": repo.ConnectionState.Message,
			"labels":  repo.Labels,
		})
	}
	return items
}

func clusterDetails(clusters []connection.ArgoCluster) []map[string]any {
	items := make([]map[string]any, 0, len(clusters))
	for _, cluster := range clusters {
		items = append(items, map[string]any{
			"name":    cluster.Name,
			"server":  cluster.Server,
			"status":  cluster.ConnectionState.Status,
			"message": cluster.ConnectionState.Message,
			"labels":  cluster.Labels,
		})
	}
	return items
}
