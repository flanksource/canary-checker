package checks

import (
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/gomplate/v3"
	"github.com/flanksource/is-healthy/pkg/health"
	"golang.org/x/sync/errgroup"

	"github.com/flanksource/commons/duration"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/types"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

const (
	// maximum number of static & non static resources a canary can have
	maxResourcesAllowed = 10

	// resourceWaitTimeout is the default timeout to wait for all resources
	// to be ready. Timeout on the spec will take precedence over this.
	resourceWaitTimeout = time.Minute * 10

	annotationkey = "flanksource.canary-checker/kubernetes-resource-canary"
)

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

func (c *KubernetesResourceChecker) applyKubeconfig(ctx *context.Context, kubeConfig types.EnvVar) (*context.Context, error) {
	val, err := ctx.GetEnvValueFromCache(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig from env: %w", err)
	}

	if strings.HasPrefix(val, "/") {
		kClient, kube, err := pkg.NewKommonsClientWithConfigPath(val)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize kubernetes client from the provided kubeconfig: %w", err)
		}

		ctx = ctx.WithDutyContext(ctx.WithKommons(kClient))
		ctx = ctx.WithDutyContext(ctx.WithKubernetes(kube))
	} else {
		kClient, kube, err := pkg.NewKommonsClientWithConfig(val)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize kubernetes client from the provided kubeconfig: %w", err)
		}

		ctx = ctx.WithDutyContext(ctx.WithKommons(kClient))
		ctx = ctx.WithDutyContext(ctx.WithKubernetes(kube))
	}

	return ctx, nil
}

func (c *KubernetesResourceChecker) Check(ctx *context.Context, check v1.KubernetesResourceCheck) pkg.Results {
	result := pkg.Success(check, ctx.Canary)
	var err error
	var results pkg.Results
	results = append(results, result)

	if check.Timeout != "" {
		if d, err := duration.ParseDuration(check.Timeout); err != nil {
			return results.Failf("failed to parse timeout: %v", err)
		} else {
			ctx2, cancel := ctx.WithTimeout(time.Duration(d))
			defer cancel()

			ctx = ctx.WithDutyContext(ctx2)
		}
	}

	totalResources := len(check.StaticResources) + len(check.Resources)
	if totalResources > maxResourcesAllowed {
		return results.Failf("too many resources (%d). only %d allowed", totalResources, maxResourcesAllowed)
	}

	if check.Kubeconfig != nil {
		ctx, err = c.applyKubeconfig(ctx, *check.Kubeconfig)
		if err != nil {
			return results.Failf("failed to apply kube config: %v", err)
		}
	}

	for i := range check.StaticResources {
		resource := check.StaticResources[i]

		// annotate the resource with the canary ID so we can easily clean it up later
		// TODO: see if this is actually needed
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
				results.ErrorMessage(fmt.Errorf("failed to delete resource %s: %v", resource.GetName(), err))
			}
		}()
	}

	if check.WaitForReady {
		timeout := resourceWaitTimeout
		if deadline, ok := ctx.Deadline(); ok {
			timeout = time.Until(deadline)
		}

		logger.Debugf("waiting for %s for %d resources to be ready.", timeout, totalResources)

		kClient := pkg.NewKubeClient(ctx.Kommons().GetRESTConfig)
		errG, _ := errgroup.WithContext(ctx)
		for _, r := range append(check.StaticResources, check.Resources...) {
			r := r
			errG.Go(func() error {
				if status, err := kClient.WaitForResource(ctx, r.GetKind(), r.GetNamespace(), r.GetName()); err != nil {
					return fmt.Errorf("error waiting for resource(%s/%s/%s) to be ready: %w", r.GetKind(), r.GetNamespace(), r.GetName(), err)
				} else if status.Status != health.HealthStatusHealthy {
					return fmt.Errorf("resource(%s/%s/%s) didn't become healthy. message (%s)", r.GetKind(), r.GetNamespace(), r.GetName(), status.Message)
				}

				return nil
			})
		}

		if err := errG.Wait(); err != nil {
			return results.Failf("%v", err)
		}
	}

	logger.Debugf("found %d checks to run", len(check.Checks))
	for _, c := range check.Checks {
		virtualCanary := v1.Canary{
			ObjectMeta: ctx.Canary.ObjectMeta,
			Spec:       c.CanarySpec,
		}

		templater := gomplate.StructTemplater{
			Values: map[string]any{
				"staticResource": check.StaticResources,
				"resources":      check.Resources,
			},
			ValueFunctions: true,
			DelimSets: []gomplate.Delims{
				{Left: "{{", Right: "}}"},
				{Left: "$(", Right: ")"},
			},
		}
		if err := templater.Walk(&virtualCanary); err != nil {
			return results.Failf("error templating checks %v", err)
		}

		checkCtx := context.New(ctx.Context, virtualCanary)
		res, err := Exec(checkCtx)
		if err != nil {
			return results.Failf("%v", err)
		} else {
			for _, r := range res {
				if r.Error != "" {
					results.Failf("check (name:%s) failed with error: %v", r.GetName(), r.Error)
				}
			}
		}
	}

	return results
}
