package checks

import (
	"testing"

	v1 "github.com/flanksource/canary-checker/api/v1"
	dutyConnection "github.com/flanksource/duty/connection"
)

func TestVerifyRepositoriesSkippedWhenEmpty(t *testing.T) {
	failures := verifyRepositories(nil, []dutyConnection.ArgoRepository{{Repo: "https://github.com/flanksource/canary-checker"}})
	if len(failures) != 0 {
		t.Fatalf("expected no failures, got %v", failures)
	}
}

func TestVerifyRepositoriesRequired(t *testing.T) {
	repositories := []dutyConnection.ArgoRepository{
		{
			Name: "primary",
			Repo: "https://github.com/flanksource/canary-checker",
			ConnectionState: dutyConnection.ArgoConnectionState{
				Status: "Successful",
			},
		},
	}

	failures := verifyRepositories([]v1.ArgoRequiredRepository{
		{Repo: "https://github.com/flanksource/canary-checker"},
	}, repositories)

	if len(failures) != 0 {
		t.Fatalf("expected no failures, got %v", failures)
	}
}

func TestVerifyRepositoriesWildcardAll(t *testing.T) {
	repositories := []dutyConnection.ArgoRepository{
		{
			Name: "primary",
			Repo: "https://github.com/flanksource/canary-checker",
			ConnectionState: dutyConnection.ArgoConnectionState{
				Status: "Failed",
			},
		},
	}

	failures := verifyRepositories([]v1.ArgoRequiredRepository{{Repo: "*"}}, repositories)
	if len(failures) == 0 {
		t.Fatalf("expected at least one failure")
	}
}

func TestVerifyClustersWildcardAll(t *testing.T) {
	clusters := []dutyConnection.ArgoCluster{
		{
			Name:   "in-cluster",
			Server: "https://kubernetes.default.svc",
			ConnectionState: dutyConnection.ArgoConnectionState{
				Status: "Failed",
			},
		},
	}

	failures := verifyClusters([]v1.ArgoRequiredCluster{{Server: "*"}}, clusters)
	if len(failures) == 0 {
		t.Fatalf("expected at least one failure")
	}
}

func TestVerifyRepositoriesPatternMatch(t *testing.T) {
	repositories := []dutyConnection.ArgoRepository{
		{
			Name: "primary",
			Repo: "https://github.com/flanksource/canary-checker",
			ConnectionState: dutyConnection.ArgoConnectionState{
				Status: "Successful",
			},
		},
	}

	failures := verifyRepositories([]v1.ArgoRequiredRepository{{Repo: "https://github.com/flanksource/*"}}, repositories)
	if len(failures) != 0 {
		t.Fatalf("expected no failures, got %v", failures)
	}
}
