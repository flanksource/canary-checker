package checks

import (
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"

	goctx "context"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type KubernetesCreatorChecker struct{}

func (c *KubernetesCreatorChecker) Type() string {
	return "kubernetescreator"
}

var (
	k8sCreatePrometheusCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "canary_check_k8s_create_total",
			Help: "Number of times the kubernetes check has run",
		},
		[]string{"name"},
	)
	k8sCreatePrometheusFailCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "canary_check_k8s_create_failed",
			Help: "Number of times the kubernetes check has failed",
		},
		[]string{"region"},
	)
	k8sCreatePrometheusPassCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "canary_check_k8s_create_passed",
			Help: "Number of times the kubernetes check has passed",
		},
		[]string{"region"},
	)
)

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *KubernetesCreatorChecker) Run(ctx *context.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range ctx.Canary.Spec.KubernetesCreator {
		results = append(results, c.Check(ctx, conf))
	}
	return results
}

// CheckConfig : Check every ldap entry for lookup and auth
// Returns check result and metrics
func (c *KubernetesCreatorChecker) Check(ctx *context.Context, extConfig external.Check) *pkg.CheckResult {
	updated, err := ctx.Contextualise(extConfig)
	if err != nil {
		return pkg.Fail(extConfig, ctx.Canary)
	}
	check := updated.(v1.KubernetesCreatorCheck)
	k8sCreatePrometheusCount.WithLabelValues(check.GetEndpoint()).Inc()
	result := pkg.Success(check, ctx.Canary)
	namespace := ctx.Canary.Namespace
	var created []*unstructured.Unstructured
	for _, resource := range check.ResourceSpec {
		client, err := ctx.Kommons.GetClientByKind(resource.GetKind())
		if err != nil {
			return result.Failf("Failed to get client for kind %s: %v", resource.GetKind(), err)
		}
		new, err := client.Create(goctx.Background(), &resource, metav1.CreateOptions{})
		if err != nil {
			return result.Failf("Could not create resource: %v", err)
		}
		created = append(created, new)
	}
	type M map[string]interface{}
	var templateInfo map[int]M
	for i, resource := range created {
		templateInfo[i] = resource.UnstructuredContent()
	}
	templateResources := map[string]interface{}{
		"resources": templateInfo,
	}
	innerFail := false
	innerCanaries, innerMessage, err := ctx.GetCanaries(namespace, check.CanaryRef)
	if err != nil {
		innerFail = true
	}

	for _, inner := range innerCanaries {
		if len(inner.Spec.Kubernetes) > 0 {
			return Error(check, fmt.Errorf("checks may not be nested with checks of the same type to avoid potential recursion.  Skipping inner kubernetes"))
		}
		innerResults := RunChecks(ctx.New(templateResources))
		for _, result := range innerResults {
			if !result.Pass {
				innerFail = true
				innerMessage = append(innerMessage, result.Message)
			}
		}
	}

	if innerFail {
		return c.HandleFail(check, fmt.Sprintf("referenced canaries failed: %v", strings.Join(innerMessage, ", ")))
	}

	k8sCreatePrometheusPassCount.WithLabelValues(check.GetEndpoint()).Inc()
	return result
}

func (c KubernetesCreatorChecker) HandleFail(check v1.KubernetesCreatorCheck, message string) *pkg.CheckResult {
	k8sCreatePrometheusFailCount.WithLabelValues(check.GetEndpoint()).Inc()
	return &pkg.CheckResult{ // nolint: staticcheck
		Check:       check,
		Pass:        false,
		Duration:    0,
		Invalid:     false,
		DisplayType: "Text",
		Message:     message,
	}
}
