package checks

import (
	"fmt"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/is-healthy/pkg/health"
	"github.com/gobwas/glob"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
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

	if check.KubeConfig != nil {
		var err error
		ctx, err = ctx.WithKubeconfig(*check.KubeConfig)
		if err != nil {
			return results.WithError(err).Invalidf("Cannot connect to kubernetes")
		}
	}

	if ctx.KubernetesRestConfig() == nil {
		return results.Failf("Kubernetes is not initialized")
	}

	client, err := ctx.KubernetesDynamicClient().GetClientByKind(check.Kind)
	if err != nil {
		return results.Failf("Failed to get client for kind %s: %v", check.Kind, err)
	}

	namespaces, err := getNamespaces(ctx, check)
	if err != nil {
		return results.Failf("Failed to get namespaces: %v", err)
	}
	var allResources []unstructured.Unstructured

	for _, namespace := range namespaces {
		resources, err := getResourcesFromNamespace(ctx, client, check, namespace)
		if err != nil {
			return results.Failf("failed to get resources: %v. namespace: %v", err, namespace)
		}
		for _, filter := range check.Ignore {
			resources, err = filterResources(resources, filter)
			if err != nil {
				results.Failf("failed to filter resources: %v. filter: %v", err, filter)
				return results
			}
		}

		ctx.Tracef("Found %d %s in namespace %s with label=%s field=%s", len(resources), check.Kind, namespace, check.Resource.LabelSelector, check.Resource.FieldSelector)
		for _, resource := range resources {
			_resource := resource
			resourceHealth, err := health.GetResourceHealth(&_resource, nil)
			if err != nil {
				results.Failf("error getting resource health (%s/%s/%s): %v",
					resource.GetKind(), resource.GetNamespace(), resource.GetName(), err)
			} else {
				resource.Object["healthStatus"] = resourceHealth

				if check.Healthy && resourceHealth.Health != health.HealthHealthy {
					results.Failf("%s/%s/%s is not healthy (health: %s, status: %s): %s",
						resource.GetKind(), resource.GetNamespace(), resource.GetName(), resourceHealth.Health, resourceHealth.Status, resourceHealth.Message)
				}

				if check.Ready && !resourceHealth.Ready {
					results.Failf("%s/%s/%s is not ready (status: %s): %s", resource.GetKind(),
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

func getResourcesFromNamespace(ctx context.Context, client dynamic.NamespaceableResourceInterface, check v1.KubernetesCheck, namespace string) ([]unstructured.Unstructured, error) {
	var resources []unstructured.Unstructured
	if check.Resource.Name != "" {
		resource, err := client.Namespace(namespace).Get(ctx, check.Resource.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		return []unstructured.Unstructured{*resource}, nil
	}
	resourceList, err := client.Namespace(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: check.Resource.LabelSelector,
		FieldSelector: check.Resource.FieldSelector,
	})
	if err != nil {
		return nil, err
	}
	resources = append(resources, resourceList.Items...)
	return resources, nil
}

func getNamespaces(ctx context.Context, check v1.KubernetesCheck) ([]string, error) {
	var namespaces []string
	if check.Namespace.Name != "" {
		return []string{check.Namespace.Name}, nil
	}

	if check.Namespace.FieldSelector == "" && check.Namespace.LabelSelector == "" {
		return []string{""}, nil
	}
	namespaceList, err := ctx.Kubernetes().CoreV1().Namespaces().List(ctx, metav1.ListOptions{
		LabelSelector: check.Namespace.LabelSelector,
		FieldSelector: check.Namespace.FieldSelector,
	})
	if err != nil {
		return nil, err
	}
	for _, namespace := range namespaceList.Items {
		namespaces = append(namespaces, namespace.Name)
	}
	return namespaces, nil
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
