package checks

import (
	"fmt"
	"regexp"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/kommons"
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
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

// CheckConfig : Check every ldap entry for lookup and auth
// Returns check result and metrics
func (c *KubernetesChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.KubernetesCheck)
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)
	client, err := ctx.Kommons.GetClientByKind(check.Kind)
	if err != nil {
		return results.Failf("Failed to get client for kind %s: %v", check.Kind, err)
	}
	namespaces, err := getNamespaces(ctx, check)
	if err != nil {
		return results.Failf("Failed to get namespaces: %v", err)
	}
	var allResources []unstructured.Unstructured

	message := ""

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
		logger.Debugf("Found %d resources in namespace %s with label=%s field=%s", len(resources), namespace, check.Resource.LabelSelector, check.Resource.FieldSelector)
		if check.CheckReady() {
			for _, resource := range resources {
				ready, msg := ctx.Kommons.IsReady(&resource)
				if !ready {
					if message != "" {
						message += ", "
					}
					message += fmt.Sprintf("%s is not ready: %v", kommons.GetName(resource), msg)
				}
			}
		}
		allResources = append(allResources, resources...)
	}
	if check.Test.IsEmpty() && len(allResources) == 0 {
		return results.Failf("no resources found")
	}
	result.AddDetails(allResources)
	if message != "" {
		return results.Failf(message)
	}
	return results
}

func getResourcesFromNamespace(ctx *context.Context, client dynamic.NamespaceableResourceInterface, check v1.KubernetesCheck, namespace string) ([]unstructured.Unstructured, error) {
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

func getNamespaces(ctx *context.Context, check v1.KubernetesCheck) ([]string, error) {
	var namespaces []string
	if check.Namespace.Name != "" {
		return []string{check.Namespace.Name}, nil
	}
	k8sClient, err := ctx.Kommons.GetClientset()
	if err != nil {
		return nil, err
	}

	if check.Namespace.FieldSelector == "" && check.Namespace.LabelSelector == "" {
		return []string{""}, err
	}
	namespeceList, err := k8sClient.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
		LabelSelector: check.Namespace.LabelSelector,
		FieldSelector: check.Namespace.FieldSelector,
	})
	if err != nil {
		return nil, err
	}
	for _, namespace := range namespeceList.Items {
		namespaces = append(namespaces, namespace.Name)
	}
	return namespaces, nil
}

func filterResources(resources []unstructured.Unstructured, filter string) ([]unstructured.Unstructured, error) {
	var filtered []unstructured.Unstructured
	ignoreRegexp, err := regexp.Compile(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to compile glob: %v", err)
	}
	for _, resource := range resources {
		if ignoreRegexp.MatchString(resource.GetName()) {
			continue
		}
		filtered = append(filtered, resource)
	}
	return filtered, nil
}
