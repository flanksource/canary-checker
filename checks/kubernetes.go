package checks

import (
	"fmt"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
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
func (c *KubernetesChecker) Run(ctx *context.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range ctx.Canary.Spec.Kubernetes {
		results = append(results, c.Check(ctx, conf))
	}
	return results
}

// CheckConfig : Check every ldap entry for lookup and auth
// Returns check result and metrics
func (c *KubernetesChecker) Check(ctx *context.Context, extConfig external.Check) *pkg.CheckResult {
	check := extConfig.(v1.KubernetesCheck)
	result := pkg.Success(check, ctx.Canary)
	client, err := ctx.Kommons.GetClientByKind(check.Kind)
	if err != nil {
		return result.Failf("Failed to get client for kind %s: %v", check.Kind, err)
	}
	namespaces, err := getNamespaces(ctx, check)
	if err != nil {
		return result.Failf("Failed to get namespaces: %v", err)
	}
	var allResources []unstructured.Unstructured
	differentReadyStatus := make(map[string]string)
	for _, namespace := range namespaces {
		resources, err := getResourcesFromNamespace(ctx, client, check, namespace)
		if err != nil {
			return result.Failf("failed to get resources: %v. namespace: %v", err, namespace)
		}
		for _, resource := range resources {
			ready, msg := ctx.Kommons.IsReady(&resource)
			if ready != check.CheckReady() {
				differentReadyStatus[fmt.Sprintf("The resource %v-%v-%v is expected Ready: %v but Ready is %v", resource.GetName(), resource.GetNamespace(), resource.GetKind(), check.CheckReady(), ready)] = msg
			}
		}
		allResources = append(allResources, resources...)
	}
	if allResources == nil {
		return result.Failf("no resources found")
	}
	result.AddDetails(allResources)
	if len(differentReadyStatus) > 0 {
		message := "The following resources found with different ready status\n"
		for key, value := range differentReadyStatus {
			message += fmt.Sprintf("%v: %v\n", key, value)
		}
		return result.Failf(message)
	}
	return result
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
