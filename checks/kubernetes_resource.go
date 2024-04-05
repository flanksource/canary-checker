package checks

import (
	gocontext "context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/flanksource/gomplate/v3"
	"github.com/sethvargo/go-retry"

	"github.com/flanksource/commons/logger"
	"github.com/flanksource/commons/utils"
	"github.com/flanksource/duty/types"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

const (
	// maximum number of static & non static resources a canary can have
	defaultMaxResourcesAllowed = 10

	resourceWaitTimeoutDefault  = time.Minute * 10
	resourceWaitIntervalDefault = time.Second * 5
	waitForExprDefault          = `dyn(resources).all(r, k8s.isHealthy(r))`

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

func (c *KubernetesResourceChecker) Check(ctx *context.Context, check v1.KubernetesResourceCheck) pkg.Results {
	result := pkg.Success(check, ctx.Canary)
	var err error
	var results pkg.Results
	results = append(results, result)

	if err := c.validate(ctx, check); err != nil {
		return results.Failf("validation: %v", err)
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
		if err := ctx.Kommons().ApplyUnstructured(utils.Coalesce(resource.GetNamespace(), ctx.Namespace), &resource); err != nil {
			return results.Failf("failed to apply static resource %s: %v", resource.GetName(), err)
		}
	}

	for i := range check.Resources {
		resource := check.Resources[i]
		resource.SetAnnotations(map[string]string{annotationkey: ctx.Canary.ID()})
		if err := ctx.Kommons().ApplyUnstructured(utils.Coalesce(resource.GetNamespace(), ctx.Namespace), &resource); err != nil {
			return results.Failf("failed to apply resource %s: %v", resource.GetName(), err)
		}

		defer func() {
			if err := ctx.Kommons().DeleteUnstructured(utils.Coalesce(resource.GetNamespace(), ctx.Namespace), &resource); err != nil {
				logger.Errorf("failed to delete resource %s: %v", resource.GetName(), err)
				results.ErrorMessage(fmt.Errorf("failed to delete resource %s: %v", resource.GetName(), err))
			}
		}()
	}

	if !check.WaitFor.Disable {
		if err := c.evalWaitFor(ctx, check); err != nil {
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
				"staticResources": check.StaticResources,
				"resources":       check.Resources,
			},
			ValueFunctions: true,
			DelimSets: []gomplate.Delims{
				{Left: "{{", Right: "}}"},
				{Left: "$(", Right: ")"},
			},
		}
		if err := templater.Walk(&virtualCanary); err != nil {
			return results.Failf("error templating checks: %v", err)
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

func (c *KubernetesResourceChecker) evalWaitFor(ctx *context.Context, check v1.KubernetesResourceCheck) error {
	waitTimeout := resourceWaitTimeoutDefault
	if wt, _ := check.WaitFor.GetTimeout(); wt > 0 {
		waitTimeout = wt
	}

	waitInterval := resourceWaitIntervalDefault
	if wt, _ := check.WaitFor.GetInterval(); wt > 0 {
		waitInterval = wt
	}

	var attempts int
	backoff := retry.WithMaxDuration(waitTimeout, retry.NewConstant(waitInterval))
	retryErr := retry.Do(ctx, backoff, func(_ctx gocontext.Context) error {
		ctx = _ctx.(*context.Context)
		attempts++
		ctx.Tracef("waiting for %d resources to be ready. (attempts: %d)", check.TotalResources(), attempts)

		var templateVar = map[string]any{}
		kClient := pkg.NewKubeClient(ctx.Kommons().GetRESTConfig)
		if response, err := kClient.FetchResources(ctx, append(check.StaticResources, check.Resources...)...); err != nil {
			return fmt.Errorf("wait for evaluation. fetching resources: %w", err)
		} else if len(response) != check.TotalResources() {
			var got []string
			for _, r := range response {
				got = append(got, fmt.Sprintf("%s/%s/%s", r.GetKind(), r.GetNamespace(), r.GetName()))
			}

			return fmt.Errorf("unxpected error. expected %d resources, got %d (%s)", check.TotalResources(), len(response), strings.Join(got, ","))
		} else {
			templateVar["resources"] = response
		}

		waitForExpr := check.WaitFor.Expr
		if waitForExpr == "" {
			waitForExpr = waitForExprDefault
		}

		if response, err := gomplate.RunTemplate(templateVar, gomplate.Template{Expression: waitForExpr}); err != nil {
			return fmt.Errorf("wait for expression evaluation: %w", err)
		} else if parsed, err := strconv.ParseBool(response); err != nil {
			return fmt.Errorf("wait for expression (%q) didn't evaluate to a boolean", check.WaitFor.Expr)
		} else if !parsed {
			return retry.RetryableError(fmt.Errorf("not all resources are ready"))
		}

		return nil
	})

	return retryErr
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

func (c *KubernetesResourceChecker) validate(ctx *context.Context, check v1.KubernetesResourceCheck) error {
	if _, err := check.WaitFor.GetTimeout(); err != nil {
		return fmt.Errorf("failed to parse wait for timeout(%s): %w", check.WaitFor.Timeout, err)
	}

	if _, err := check.WaitFor.GetTimeout(); err != nil {
		return fmt.Errorf("failed to parse wait for timeout(%s): %w", check.WaitFor.Timeout, err)
	}

	maxResourcesAllowed := ctx.Properties().Int("checks.kubernetesResource.maxResources", defaultMaxResourcesAllowed)
	if check.TotalResources() > maxResourcesAllowed {
		return fmt.Errorf("too many resources (%d). only %d allowed", check.TotalResources(), maxResourcesAllowed)
	}

	return nil
}
