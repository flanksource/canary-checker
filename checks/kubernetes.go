package checks

import (
	"fmt"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/is-healthy/pkg/health"
	"github.com/gobwas/glob"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type KubernetesChecker struct{}

func (c *KubernetesChecker) Type() string {
	return "kubernetes"
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *KubernetesChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.Kubernetes {
		results = append(results, c.Check(*ctx, conf)...)
	}

	return results
}

func (c *KubernetesChecker) Check(ctx context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.KubernetesCheck)
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	ctx = ctx.WithKubernetesConnection(check.KubernetesConnection)
	k8sClient, err := ctx.Kubernetes()
	if err != nil {
		return results.Failf("Kubernetes is not initialized: %v", err)
	}
	var namespaces = []string{}
	var nsSelector = check.Namespace.ToDutySelector()
	if !nsSelector.IsEmpty() && !nsSelector.Wildcard() {
		list, err := k8sClient.QueryResources(ctx, check.Namespace.ToDutySelector().Type("Namespace").MetadataOnly())
		if err != nil {
			return results.Failf("Failed to get namespaces: %v", err)
		}
		for _, v := range list {
			namespaces = append(namespaces, v.GetName())
		}
	}

	var allResources []unstructured.Unstructured

	for _, namespace := range namespaces {
		selector := check.Resource.ToDutySelector()
		if namespace != "" {
			selector.Namespace = namespace
		}
		selector.Types = []string{check.Kind}
		resources, err := k8sClient.QueryResources(ctx, selector)

		if err != nil {
			return results.Failf("failed to get resources (%s): %v", selector, err)
		}
		for _, filter := range check.Ignore {
			resources, err = filterResources(resources, filter)
			if err != nil {
				results.Failf("failed to filter resources: %v, filter: %v", err, filter)
				return results
			}
		}

		for _, resource := range resources {
			_resource := resource
			resourceHealth, err := health.GetResourceHealth(&_resource, nil)
			if err != nil {
				results.Failf("error getting resource health (%s/%s/%s): %v",
					resource.GetKind(), resource.GetNamespace(), resource.GetName(), err)
			} else {
				resource.Object["healthStatus"] = resourceHealth

				if check.Healthy && resourceHealth.Health != health.HealthHealthy {
					results.Failf("%s/%s/%s is not healthy (health: %s, status: %s): %s\n",
						resource.GetKind(), resource.GetNamespace(), resource.GetName(), resourceHealth.Health, resourceHealth.Status, resourceHealth.Message)
				}

				if check.Ready && !resourceHealth.Ready {
					results.Failf("%s/%s/%s is not ready (status: %s): %s\n", resource.GetKind(),
						resource.GetNamespace(), resource.GetName(), resourceHealth.Status, resourceHealth.Message)
				}
			}
		}

		allResources = append(allResources, resources...)
	}

	if check.Test.IsEmpty() && len(allResources) == 0 {
		return results.Failf("no resources found")
	}

	result.AddDetails(allResources)
	return results
}

func filterResources(resources []unstructured.Unstructured, filter string) ([]unstructured.Unstructured, error) {
	var filtered []unstructured.Unstructured
	ignoreGlob, err := glob.Compile(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to compile glob: %v", err)
	}
	for _, resource := range resources {
		if ignoreGlob.Match(resource.GetName()) {
			continue
		}
		filtered = append(filtered, resource)
	}
	return filtered, nil
}
