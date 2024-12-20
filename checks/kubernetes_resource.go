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
	"github.com/flanksource/is-healthy/pkg/health"
	"github.com/flanksource/is-healthy/pkg/lua"
	"github.com/flanksource/kommons"
	"github.com/patrickmn/go-cache"
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
)

const (
	// maximum number of static & non static resources a canary can have
	defaultMaxResourcesAllowed = 10

	resourceWaitTimeoutDefault  = time.Minute * 10
	resourceWaitIntervalDefault = time.Second * 5
	waitForExprDefault          = `dyn(resources).all(r, k8s.isReady(r))`
)

func resourceLabelKey(key string) string {
	return fmt.Sprintf("canaries.flanksource.com/%s", key)
}

func getName(o unstructured.Unstructured) string {
	return fmt.Sprintf("%s/%s/%s", o.GetNamespace(), o.GetKind(), o.GetName())
}

type KubernetesResourceChecker struct{}

func (c *KubernetesResourceChecker) Type() string {
	return "kubernetes_resource"
}

func (c *KubernetesResourceChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.DeepCopy().Spec.KubernetesResource {
		results = append(results, c.Check(*ctx, conf)...)
	}
	return results
}

func (c *KubernetesResourceChecker) Check(ctx context.Context, check v1.KubernetesResourceCheck) pkg.Results {
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	// We do this before virtual check run in case the check times out
	// and returns an err, the default templating requires 'display' in env
	result.AddData(map[string]any{
		"display": make(map[string]any),
	})

	if err := c.validate(ctx, check); err != nil {
		return results.Failf("validation: %v", err)
	}

	if check.Kubeconfig != nil {
		var err error
		ctx, err = ctx.WithKubeconfig(*check.Kubeconfig)
		if err != nil {
			return results.WithError(err).Invalidf("Cannot connect to kubernetes")
		}
	}

	if check.HasResourcesWithMissingNamespace() {
		namespacedResources, err := fetchNamespacedResources(ctx)
		if err != nil {
			return results.Failf("failed to get api resources: %w", err)
		}

		check.SetMissingNamespace(ctx.Canary, namespacedResources)
	}

	check.SetCanaryOwnerReference(ctx.Canary)

	if err := templateKubernetesResourceCheck(ctx.Canary.GetPersistedID(), ctx.Canary.GetCheckID(check.GetName()), &check); err != nil {
		return results.Failf("templating error: %v", err)
	}

	kClient := kommons.NewClient(ctx.KubernetesRestConfig(), ctx.Logger)

	for i := range check.StaticResources {
		resource := check.StaticResources[i]
		if err := kClient.ApplyUnstructured(resource.GetNamespace(), &resource); err != nil {
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
		if err := kClient.ApplyUnstructured(resource.GetNamespace(), &resource); err != nil {
			return results.Failf("failed to apply resource (%s/%s/%s): %v", resource.GetKind(), resource.GetNamespace(), resource.GetName(), err)
		}
	}

	resourceMap := map[string]map[string]any{}
	var healthMap map[string]health.HealthStatus
	if !check.WaitFor.Disable {
		var err error
		var resourceObjs []unstructured.Unstructured
		if healthMap, resourceObjs, err = c.evalWaitFor(ctx, check); err != nil {
			return results.Failf("error in evaluating wait for: %v", err)
		}

		for _, r := range resourceObjs {
			r.SetManagedFields(nil)
			resourceMap[getName(r)] = r.Object
		}
	}

	ctx.Tracef("found %d checks to run", len(check.Checks))

	displayPerCheck := map[string]string{}
	for _, c := range check.Checks {
		virtualCanary := v1.Canary{
			ObjectMeta: metav1.ObjectMeta{
				Name:      ctx.Canary.ObjectMeta.Name,
				Namespace: ctx.Canary.ObjectMeta.Namespace,
				Labels:    ctx.Canary.ObjectMeta.Labels,
			},
			Spec: c.CanarySpec,
		}

		templater := gomplate.StructTemplater{
			Values: map[string]any{
				"staticResources": check.StaticResources,
				"resources":       check.Resources,
			},
			ValueFunctions: true,
			IgnoreFields: map[string]string{
				"URL": "string", // Avoid templating URL which might have non-templated username, password etc.
			},
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
			ctx.Tracef("running check: %s", virtualCanary.Name)

			ctx = _ctx.(context.Context)
			checkCtx := context.New(ctx.Context, virtualCanary)
			res, err := Exec(checkCtx)
			if err != nil {
				return fmt.Errorf("error executing check: %w", err)
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

			for _, r := range res {
				displayPerCheck[r.Check.GetName()] = r.Message
			}
			return nil
		})
		if retryErr != nil {
			return results.Failf(retryErr.Error())
		}
	}

	result.AddData(map[string]any{
		"health":    healthMap,
		"resources": resourceMap,
		"display":   displayPerCheck,
	})
	return results
}

func (c *KubernetesResourceChecker) evalWaitFor(ctx context.Context, check v1.KubernetesResourceCheck) (map[string]health.HealthStatus, []unstructured.Unstructured, error) {
	healthMap := make(map[string]health.HealthStatus)
	waitTimeout := resourceWaitTimeoutDefault
	if wt, _ := check.WaitFor.GetTimeout(); wt > 0 {
		waitTimeout = wt
	}

	waitInterval := resourceWaitIntervalDefault
	if wt, _ := check.WaitFor.GetInterval(); wt > 0 {
		waitInterval = wt
	}
	var resourceObjs []unstructured.Unstructured
	var attempts int
	backoff := retry.WithMaxDuration(waitTimeout, retry.NewConstant(waitInterval))
	retryErr := retry.Do(ctx, backoff, func(_ctx gocontext.Context) error {
		ctx = _ctx.(context.Context)
		attempts++

		ctx.Tracef("waiting for %d resources, attempt=%d, timeout=%s", check.TotalResources(), attempts, waitTimeout)

		resourceObjs, err := ctx.KubernetesClient().FetchResources(ctx, append(check.StaticResources, check.Resources...)...)
		if err != nil {
			if apiErrors.IsNotFound(err) {
				return retry.RetryableError(err)
			}

			return fmt.Errorf("wait for evaluation. fetching resources: %w", err)
		} else if len(resourceObjs) != check.TotalResources() {
			var got []string
			for _, r := range resourceObjs {
				got = append(got, fmt.Sprintf("%s/%s/%s", r.GetKind(), r.GetNamespace(), r.GetName()))
			}

			return fmt.Errorf("unexpected error. expected %d resources, got %d (%s)", check.TotalResources(), len(resourceObjs), strings.Join(got, ","))
		}

		waitForExpr := check.WaitFor.Expr
		if waitForExpr == "" {
			waitForExpr = waitForExprDefault
		}

		var templateVar = map[string]any{
			"resources": resourceObjs,
		}
		if response, err := gomplate.RunTemplate(templateVar, gomplate.Template{Expression: waitForExpr}); err != nil {
			return fmt.Errorf("wait for expression evaluation: %w", err)
		} else if passed, err := strconv.ParseBool(response); err != nil {
			return fmt.Errorf("wait for expression (%q) didn't evaluate to a boolean", check.WaitFor.Expr)
		} else {
			notReadyResources := []string{}
			for _, r := range resourceObjs {
				rh, _ := health.GetResourceHealth(&r, lua.ResourceHealthOverrides{})
				name := getName(r)
				healthMap[name] = *rh
				ctx.Tracef("health for %s: %s ready=%v", name, rh, rh.Ready)
				if rh.Health.IsWorseThan(health.HealthWarning) {
					notReadyResources = append(notReadyResources, fmt.Sprintf("%s: %s", name, rh))
				} else if !rh.Ready {
					notReadyResources = append(notReadyResources, fmt.Sprintf("%s: not ready", name))
				}
			}
			if passed {
				return nil
			}
			return retry.RetryableError(fmt.Errorf("%s", strings.Join(notReadyResources, ", ")))
		}
	})

	return healthMap, resourceObjs, retryErr
}

func (c *KubernetesResourceChecker) validate(ctx context.Context, check v1.KubernetesResourceCheck) error {
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

func DeleteResources(ctx context.Context, check v1.KubernetesResourceCheck, deleteStatic bool) error {
	ctx.Tracef("deleting resources")

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

		rc, err := ctx.KubernetesClient().GetClientByGroupVersionKind(resource.GroupVersionKind().Group, resource.GroupVersionKind().Version, resource.GetKind())
		if err != nil {
			return fmt.Errorf("failed to get rest client for (%s/%s/%s): %w", resource.GetKind(), resource.GetNamespace(), resource.GetName(), err)
		}

		clients.Store(gvk, rc.Namespace(resource.GetNamespace()))
	}

	eg, _ := errgroup.WithContext(ctx)
	for i := range resources {
		resource := resources[i]

		eg.Go(func() error {
			cachedClient, _ := clients.Load(resource.GetObjectKind().GroupVersionKind())
			rc := cachedClient.(dynamic.NamespaceableResourceInterface)

			deleteOpt := metav1.DeleteOptions{
				GracePeriodSeconds: lo.ToPtr(int64(0)),
				PropagationPolicy:  lo.ToPtr(lo.Ternary(check.WaitFor.Delete, metav1.DeletePropagationForeground, metav1.DeletePropagationBackground)),
			}

			switch resource.GetKind() {
			case "Namespace", "Service":
				// NOTE: namespace cannot be deleted with `.DeleteCollection()`
				//
				// FIXME: Even though Service can be deleted with `.DeleteCollection()`
				// it failed on the CI. It's probably due to an older kubernetes
				// version we're using on the CI (v1.20.7).
				// Delete it by name for now while we wait upgrade the kubernetes version
				// on our CI.
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
				ctx.Tracef("all the resources have been deleted")
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
					ctx.Tracef("all (%v) have been deleted", gvk)
					deleted[gvk] = struct{}{}
				} else {
					ctx.Tracef("all (%v) have not been deleted", gvk)
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

var apiResourceCache = cache.New(time.Hour*24, time.Hour*24)

func fetchNamespacedResources(ctx context.Context) (map[schema.GroupVersionKind]bool, error) {
	kubeconfigCacheKey := ctx.KubernetesRestConfig().Host + ctx.KubernetesRestConfig().APIPath
	if val, ok := apiResourceCache.Get(kubeconfigCacheKey); ok {
		return val.(map[schema.GroupVersionKind]bool), nil
	}

	_, resourceList, err := ctx.Kubernetes().Discovery().ServerGroupsAndResources()
	if err != nil {
		return nil, fmt.Errorf("failed to get server groups & resources: %w", err)
	}

	namespaceResources := make(map[schema.GroupVersionKind]bool)
	for _, apiResourceList := range resourceList {
		for _, resource := range apiResourceList.APIResources {
			if resource.Namespaced {
				gv, _ := schema.ParseGroupVersion(apiResourceList.GroupVersion)
				key := schema.GroupVersionKind{
					Group:   gv.Group,
					Version: gv.Version,
					Kind:    resource.Kind,
				}

				namespaceResources[key] = true
			}
		}
	}

	apiResourceCache.SetDefault(kubeconfigCacheKey, namespaceResources)
	return namespaceResources, nil
}
