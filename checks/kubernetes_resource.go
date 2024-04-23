package checks

import (
	gocontext "context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/flanksource/gomplate/v3"
	"github.com/samber/lo"
	"github.com/sethvargo/go-retry"
	"golang.org/x/sync/errgroup"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/collections"
	"github.com/flanksource/commons/utils"
	"github.com/flanksource/duty/types"
)

const (
	// maximum number of static & non static resources a canary can have
	defaultMaxResourcesAllowed = 10

	resourceWaitTimeoutDefault  = time.Minute * 10
	resourceWaitIntervalDefault = time.Second * 5
	waitForExprDefault          = `dyn(resources).all(r, k8s.isHealthy(r))`
)

func resourceLabelKey(key string) string {
	return fmt.Sprintf("canaries.flanksource.com/%s", key)
}

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

	if err := templateKubernetesResourceCheck(ctx.Canary.GetPersistedID(), ctx.Canary.GetCheckID(check.GetName()), &check); err != nil {
		return results.Failf("templating error: %v", err)
	}

	for i := range check.StaticResources {
		resource := check.StaticResources[i]
		if err := ctx.Kommons().ApplyUnstructured(utils.Coalesce(resource.GetNamespace(), ctx.Namespace), &resource); err != nil {
			return results.Failf("failed to apply static resource %s: %v", resource.GetName(), err)
		}
	}

	defer func() {
		if err := DeleteResources(ctx, check, false); err != nil {
			results.Failf(err.Error())
		}
	}()

	if check.ClearResources {
		if err := DeleteResources(ctx, check, false); err != nil {
			results.Failf(err.Error())
		}
	}

	for i := range check.Resources {
		resource := check.Resources[i]
		if err := ctx.Kommons().ApplyUnstructured(utils.Coalesce(resource.GetNamespace(), ctx.Namespace), &resource); err != nil {
			return results.Failf("failed to apply resource (%s/%s/%s): %v", resource.GetKind(), resource.GetNamespace(), resource.GetName(), err)
		}
	}

	if !check.WaitFor.Disable {
		if err := c.evalWaitFor(ctx, check); err != nil {
			return results.Failf("%v", err)
		}
	}

	ctx.Logger.V(4).Infof("found %d checks to run", len(check.Checks))
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

		if wt, _ := check.CheckRetries.GetDelay(); wt > 0 {
			time.Sleep(wt)
		}

		var backoff retry.Backoff
		backoff = retry.BackoffFunc(func() (time.Duration, bool) {
			return 0, true // don't retry by default
		})

		if retryInterval, _ := check.CheckRetries.GetInterval(); retryInterval > 0 {
			backoff = retry.NewConstant(retryInterval)
		}

		if maxRetryTimeout, _ := check.CheckRetries.GetTimeout(); maxRetryTimeout > 0 {
			backoff = retry.WithMaxDuration(maxRetryTimeout, backoff)
		}

		retryErr := retry.Do(ctx, backoff, func(_ctx gocontext.Context) error {
			ctx.Logger.V(4).Infof("running check: %s", virtualCanary.Name)

			ctx = _ctx.(*context.Context)
			checkCtx := context.New(ctx.Context, virtualCanary)
			res, err := Exec(checkCtx)
			if err != nil {
				return err
			} else {
				for _, r := range res {
					if !r.Pass {
						if r.Error != "" {
							return retry.RetryableError(fmt.Errorf("check (name:%s) failed with error: %v", r.GetName(), r.Error))
						} else {
							return retry.RetryableError(fmt.Errorf("check (name:%s) failed", r.GetName()))
						}
					}
				}
			}

			return nil
		})
		if retryErr != nil {
			return results.Failf(retryErr.Error())
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

	kClient := pkg.NewKubeClient(ctx.Kommons().GetRESTConfig)

	var attempts int
	backoff := retry.WithMaxDuration(waitTimeout, retry.NewConstant(waitInterval))
	retryErr := retry.Do(ctx, backoff, func(_ctx gocontext.Context) error {
		ctx = _ctx.(*context.Context)
		attempts++
		ctx.Logger.V(4).Infof("waiting for %d resources to be ready. (attempts: %d)", check.TotalResources(), attempts)

		var templateVar = map[string]any{}
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
		return fmt.Errorf("invalid wait timeout (%s): %w", check.WaitFor.Timeout, err)
	}

	if _, err := check.WaitFor.GetInterval(); err != nil {
		return fmt.Errorf("invalid wait interval (%s): %w", check.WaitFor.Interval, err)
	}

	if _, err := check.CheckRetries.GetTimeout(); err != nil {
		return fmt.Errorf("invalid check timeout (%s): %w", check.CheckRetries.Timeout, err)
	}

	if _, err := check.CheckRetries.GetInterval(); err != nil {
		return fmt.Errorf("invalid check retry interval (%s): %w", check.CheckRetries.Interval, err)
	}

	if _, err := check.CheckRetries.GetDelay(); err != nil {
		return fmt.Errorf("invalid check initial delay (%s): %w", check.CheckRetries.Delay, err)
	}

	maxResourcesAllowed := ctx.Properties().Int("checks.kubernetesResource.maxResources", defaultMaxResourcesAllowed)
	if check.TotalResources() > maxResourcesAllowed {
		return fmt.Errorf("too many resources (%d). only %d supported", check.TotalResources(), maxResourcesAllowed)
	}

	return nil
}

func DeleteResources(ctx *context.Context, check v1.KubernetesResourceCheck, deleteStatic bool) error {
	ctx.Logger.V(4).Infof("deleting resources")

	resources := check.Resources
	if deleteStatic {
		resources = append(resources, check.StaticResources...)
	}

	// firstly, cache dynamic clients by gvk
	clients := sync.Map{}
	for i := range resources {
		resource := resources[i]

		gvk := resource.GetObjectKind().GroupVersionKind()
		if _, ok := clients.Load(gvk); ok {
			continue // client already cached
		}

		namespace := utils.Coalesce(resource.GetNamespace(), ctx.Namespace)
		rc, _, _, err := ctx.Kommons().GetDynamicClientFor(namespace, &resource)
		if err != nil {
			return fmt.Errorf("failed to get rest client for (%s/%s/%s): %w", resource.GetKind(), resource.GetNamespace(), resource.GetName(), err)
		}
		clients.Store(gvk, rc)
	}

	eg, _ := errgroup.WithContext(ctx)
	for i := range resources {
		resource := resources[i]

		eg.Go(func() error {
			cachedClient, _ := clients.Load(resource.GetObjectKind().GroupVersionKind())
			rc := cachedClient.(dynamic.ResourceInterface)

			deleteOpt := metav1.DeleteOptions{
				GracePeriodSeconds: lo.ToPtr(int64(0)),
				PropagationPolicy:  lo.ToPtr(metav1.DeletePropagationOrphan),
			}

			switch resource.GetKind() {
			case "Namespace": // namespace cannot be deleted with `.DeleteCollection()`
				if err := rc.Delete(ctx, resource.GetName(), deleteOpt); err != nil {
					var statusErr *apiErrors.StatusError
					if errors.As(err, &statusErr) {
						switch statusErr.ErrStatus.Code {
						case 404:
							return nil
						}
					}

					return fmt.Errorf("failed to delete resource (%s/%s/%s): %w", resource.GetKind(), resource.GetNamespace(), resource.GetName(), err)
				}

			default:
				labelSelector := fmt.Sprintf("%s=%s", resourceLabelKey("canary-id"), ctx.Canary.GetPersistedID())
				if checkID := ctx.Canary.GetCheckID(check.GetName()); checkID != "" {
					labelSelector += fmt.Sprintf(",%s=%s", resourceLabelKey("check-id"), checkID)
				}
				if !deleteStatic {
					labelSelector += fmt.Sprintf(",!%s", resourceLabelKey("is-static"))
				}

				if err := rc.DeleteCollection(ctx, deleteOpt, metav1.ListOptions{LabelSelector: labelSelector}); err != nil {
					var statusErr *apiErrors.StatusError
					if errors.As(err, &statusErr) {
						switch statusErr.ErrStatus.Code {
						case 404:
							return nil
						}
					}

					return fmt.Errorf("failed to delete resource (%s/%s/%s): %w", resource.GetKind(), resource.GetNamespace(), resource.GetName(), err)
				}
			}

			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	if !check.WaitFor.Delete {
		return nil
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if len(resources) == 0 {
				ctx.Logger.V(4).Infof("all the resources have been deleted")
				return nil
			}

			deleted := make(map[schema.GroupVersionKind]struct{})
			for _, resource := range resources {
				gvk := resource.GetObjectKind().GroupVersionKind()
				cachedClient, _ := clients.Load(gvk)
				rc := cachedClient.(dynamic.ResourceInterface)

				labelSelector := fmt.Sprintf("%s=%s", resourceLabelKey("canary-id"), ctx.Canary.GetPersistedID())
				if checkID := ctx.Canary.GetCheckID(check.GetName()); checkID != "" {
					labelSelector += fmt.Sprintf(",%s=%s", resourceLabelKey("check-id"), checkID)
				}
				if !deleteStatic {
					labelSelector += fmt.Sprintf(",!%s", resourceLabelKey("is-static"))
				}

				if listResponse, err := rc.List(ctx, metav1.ListOptions{LabelSelector: labelSelector}); err != nil {
					return fmt.Errorf("error getting resource (%s/%s/%s) while polling: %w", resource.GetKind(), resource.GetNamespace(), resource.GetName(), err)
				} else if listResponse == nil || len(listResponse.Items) == 0 {
					ctx.Logger.V(4).Infof("all (%v) have been deleted", gvk)
					deleted[gvk] = struct{}{}
				} else {
					ctx.Logger.V(4).Infof("all (%v) have not been deleted", gvk)
				}
			}

			resources = lo.Filter(resources, func(item unstructured.Unstructured, _ int) bool {
				_, ok := deleted[item.GetObjectKind().GroupVersionKind()]
				return !ok
			})

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func templateKubernetesResourceCheck(canaryID, checkID string, check *v1.KubernetesResourceCheck) error {
	templater := gomplate.StructTemplater{
		ValueFunctions: true,
		DelimSets: []gomplate.Delims{
			{Left: "{{", Right: "}}"},
			{Left: "$(", Right: ")"},
		},
	}

	for i, r := range check.Resources {
		namespace, kind := r.GetNamespace(), r.GetObjectKind().GroupVersionKind()
		if err := templater.Walk(&r); err != nil {
			return fmt.Errorf("error templating resource: %w", err)
		}

		if r.GetNamespace() != namespace || r.GetObjectKind().GroupVersionKind() != kind {
			return fmt.Errorf("templating the namespace or group/version/kind of a resource is not allowed")
		}
		newLabels := collections.MergeMap(r.GetLabels(), map[string]string{
			resourceLabelKey("canary-id"): canaryID,
			resourceLabelKey("check-id"):  checkID,
		})
		r.SetLabels(newLabels)
		check.Resources[i] = r
	}

	for i, r := range check.StaticResources {
		namespace, kind := r.GetNamespace(), r.GetKind()
		if err := templater.Walk(&r); err != nil {
			return fmt.Errorf("error templating resource: %w", err)
		}

		if r.GetNamespace() != namespace || r.GetKind() != kind {
			return fmt.Errorf("templating the namespace or group/version/kind of a resource is not allowed")
		}

		newLabels := collections.MergeMap(r.GetLabels(), map[string]string{
			resourceLabelKey("canary-id"): canaryID,
			resourceLabelKey("check-id"):  checkID,
			resourceLabelKey("is-static"): "true",
		})
		r.SetLabels(newLabels)
		check.StaticResources[i] = r
	}

	return nil
}
