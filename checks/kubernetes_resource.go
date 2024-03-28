package checks

import (
	"fmt"
	"strings"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/types"
)

// maximum number of static & non static resources a canary can have
const maxResourcesAllowed = 10
const annotationkey = "flanksource.canary-checker/kubernetes-resource-canary"

type KubernetesResourceChecker struct{}

func (c *KubernetesResourceChecker) Type() string {
	return "kubernetes_resource"
}

func (c *KubernetesResourceChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.KubernetesResource {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

func (c *KubernetesResourceChecker) applyKubeconfig(ctx *context.Context, kubeConfig types.EnvVar) error {
	val, err := ctx.GetEnvValueFromCache(kubeConfig)
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig from env: %w", err)
	}

	if strings.HasPrefix(val, "/") {
		kClient, kube, err := pkg.NewKommonsClientWithConfigPath(val)
		if err != nil {
			return fmt.Errorf("failed to initialize kubernetes client from the provided kubeconfig: %w", err)
		}

		ctx = ctx.WithDutyContext(ctx.WithKommons(kClient))
		ctx = ctx.WithDutyContext(ctx.WithKubernetes(kube))
	} else {
		kClient, kube, err := pkg.NewKommonsClientWithConfig(val)
		if err != nil {
			return fmt.Errorf("failed to initialize kubernetes client from the provided kubeconfig: %w", err)
		}

		ctx = ctx.WithDutyContext(ctx.WithKommons(kClient))
		ctx = ctx.WithDutyContext(ctx.WithKubernetes(kube))
	}

	return nil
}

func (c *KubernetesResourceChecker) Check(ctx *context.Context, check v1.KubernetesResourceCheck) pkg.Results {
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	totalResources := len(check.StaticResources) + len(check.Resources)
	if totalResources > maxResourcesAllowed {
		return results.Failf("too many resources (%d). only %d allowed", totalResources, maxResourcesAllowed)
	}

	if check.Kubeconfig != nil {
		if err := c.applyKubeconfig(ctx, *check.Kubeconfig); err != nil {
			return results.Failf("failed to apply kube config: %v", err)
		}
	}

	for i := range check.StaticResources {
		resource := check.StaticResources[i]

		// annotate the resource with the canary ID so we can easily clean it up later
		resource.SetAnnotations(map[string]string{annotationkey: ctx.Canary.ID()})
		if err := ctx.Kommons().ApplyUnstructured(ctx.Namespace, &resource); err != nil {
			return results.Failf("failed to apply static resource %s: %v", resource.GetName(), err)
		}
	}

	for i := range check.Resources {
		resource := check.Resources[i]
		resource.SetAnnotations(map[string]string{annotationkey: ctx.Canary.ID()})
		if err := ctx.Kommons().ApplyUnstructured(ctx.Namespace, &resource); err != nil {
			return results.Failf("failed to apply resource %s: %v", resource.GetName(), err)
		}

		defer func() {
			if err := ctx.Kommons().DeleteUnstructured(ctx.Namespace, &resource); err != nil {
				logger.Errorf("failed to delete resource %s: %v", resource.GetName(), err)
			}
		}()
	}

	if check.WaitForReady {
		logger.Infof("waiting for resources to be ready.")
	}

	// run the actual check now

	return nil
}
