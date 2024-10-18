package checks

import (
	"fmt"
	"io"
	"net/http"

	gocontext "context"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/api/context"

	"github.com/flanksource/canary-checker/api/external"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/client-go/kubernetes"

	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/yaml"

	canaryv1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/commons/logger"
	perrors "github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"golang.org/x/sync/semaphore"
)

type NamespaceChecker struct {
	lock *semaphore.Weighted
	ng   *NameGenerator
	ctx  *context.Context
	k8s  kubernetes.Interface
}

func NewNamespaceChecker() *NamespaceChecker {
	return &NamespaceChecker{
		lock: semaphore.NewWeighted(1),
		ng:   &NameGenerator{NamespacesCount: 10},
	}
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *NamespaceChecker) Run(ctx *context.Context) pkg.Results {
	logger.Warnf("namespace check is deprecated. Please use the kubernetes resource check")
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.Namespace {
		if c.k8s == nil {
			c.k8s = ctx.Kubernetes()
			c.ctx = ctx
		}
		results = append(results, c.Check(c.ctx, conf)...)
	}
	return results
}

// Type: returns checker type
func (c *NamespaceChecker) Type() string {
	return "namespace"
}

func (c *NamespaceChecker) newPod(check canaryv1.NamespaceCheck, ns *v1.Namespace) (*v1.Pod, error) {
	if check.PodSpec == "" {
		return nil, fmt.Errorf("pod spec cannot be empty")
	}

	pod := &v1.Pod{}
	if err := yaml.Unmarshal([]byte(check.PodSpec), pod); err != nil {
		return nil, fmt.Errorf("failed to unmarshall pod spec: %v", err)
	}

	pod.Name = "canary-check-pod"
	pod.Namespace = ns.Name
	pod.Labels[nameLabel] = pod.Name
	pod.Labels[podCheckSelector] = c.podCheckSelectorValue(check, ns)
	pod.Labels[podGeneralSelector] = "true" //nolint: goconst
	if check.PriorityClass != "" {
		pod.Spec.PriorityClassName = check.PriorityClass
	}
	return pod, nil
}

func (c *NamespaceChecker) getConditionTimes(ns *v1.Namespace, pod *v1.Pod) (times map[v1.PodConditionType]metav1.Time, err error) {
	pods := c.k8s.CoreV1().Pods(ns.Name)
	times = make(map[v1.PodConditionType]metav1.Time)
	pod, err = pods.Get(c.ctx, pod.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Status == v1.ConditionTrue {
			times[condition.Type] = condition.LastTransitionTime
		}
	}
	return times, nil
}

func (c *NamespaceChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(canaryv1.NamespaceCheck)
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)
	if !c.lock.TryAcquire(1) {
		logger.Tracef("Check already in progress, skipping")
		return nil
	}
	defer func() { c.lock.Release(1) }()
	startTimer := NewTimer()

	logger.Debugf("Running namespace check %s", check.Name)
	five := int64(5)
	if _, err := c.k8s.CoreV1().Nodes().List(ctx, metav1.ListOptions{TimeoutSeconds: &five}); err != nil {
		return results.Failf("cannot connect to API server: %v", err)
	}

	namespaceName := c.ng.NamespaceName(check.NamespaceNamePrefix)
	namespaces := c.k8s.CoreV1().Namespaces()
	ns := &v1.Namespace{
		TypeMeta: metav1.TypeMeta{Kind: "Namespace", APIVersion: "v1"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        namespaceName,
			Labels:      check.NamespaceLabels,
			Annotations: check.NamespaceAnnotations,
		},
	}
	if _, err := namespaces.Create(ctx, ns, metav1.CreateOptions{}); err != nil {
		return results.Failf("unable to create namespace: %v", err)
	}
	defer func() {
		c.Cleanup(ns) // nolint: errcheck
	}()

	pod, err := c.newPod(check, ns)
	if err != nil {
		return results.Failf("invalid pod spec: %v", err)
	}

	pods := c.k8s.CoreV1().Pods(ns.Name)

	if _, err := pods.Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		return results.Failf("unable to create pod: %v", err)
	}
	pod, _ = c.WaitForPod(ns.Name, pod.Name, time.Millisecond*time.Duration(check.ScheduleTimeout), v1.PodRunning)
	created := pod.GetCreationTimestamp()

	conditions, err := c.getConditionTimes(ns, pod)
	if err != nil {
		return results.Failf("could not list conditions: %v", err)
	}

	scheduled := diff(conditions, v1.PodInitialized, v1.PodScheduled)
	started := diff(conditions, v1.PodScheduled, v1.ContainersReady)
	running := diff(conditions, v1.ContainersReady, v1.PodReady)

	logger.Debugf("%s created=%s, scheduled=%d, started=%d, running=%d wall=%s", pod.Name, created, scheduled, started, running, startTimer)
	logger.Tracef("%v", conditions)

	if err := c.createServiceAndIngress(check, ns, pod); err != nil {
		return results.Failf("failed to create ingress and service: %v", err)
	}

	deadline := time.Now().Add(time.Duration(check.Deadline) * time.Millisecond)

	ingressTime, requestTime, ingressResult := c.httpCheck(check, deadline)

	deleteOk := true
	deletion := NewTimer()
	if err := pods.Delete(c.ctx, pod.Name, metav1.DeleteOptions{}); err != nil {
		return results.Failf("failed to delete pod: %v", err)
	}
	result.Pass = ingressResult.Pass && deleteOk
	result.Message = ingressResult.Message
	result.Metrics = []pkg.Metric{
		{
			Name:   "schedule_time",
			Type:   metrics.HistogramType,
			Labels: map[string]string{"namespaceCheck": check.Name},
			Value:  float64(scheduled),
		},
		{
			Name:   "creation_time",
			Type:   metrics.HistogramType,
			Labels: map[string]string{"namespaceCheck": check.Name},
			Value:  float64(started),
		},
		{
			Name:   "delete_time",
			Type:   metrics.HistogramType,
			Labels: map[string]string{"namespaceCheck": check.Name},
			Value:  deletion.Elapsed(),
		},
		{
			Name:   "ingress_time",
			Type:   metrics.HistogramType,
			Labels: map[string]string{"namespaceCheck": check.Name},
			Value:  ingressTime,
		},
		{
			Name:   "request_time",
			Type:   metrics.HistogramType,
			Labels: map[string]string{"namespaceCheck": check.Name},
			Value:  requestTime,
		},
	}
	return results
}

func (c *NamespaceChecker) Cleanup(ns *v1.Namespace) error {
	if err := c.k8s.CoreV1().Namespaces().Delete(c.ctx, ns.Name, metav1.DeleteOptions{}); err != nil {
		return perrors.Wrapf(err, "Failed to delete namespace %s", ns.Name)
	}
	return nil
}

func (c *NamespaceChecker) httpCheck(check canaryv1.NamespaceCheck, deadline time.Time) (ingressTime float64, requestTime float64, result *pkg.CheckResult) {
	var hardDeadline time.Time
	ingressTimeout := time.Now().Add(time.Duration(check.IngressTimeout) * time.Millisecond)
	if ingressTimeout.After(deadline) {
		hardDeadline = deadline
	} else {
		hardDeadline = ingressTimeout
	}

	timer := NewTimer()
	retryInterval := time.Duration(check.HTTPRetryInterval) * time.Millisecond

	for {
		url := fmt.Sprintf("http://%s%s", check.IngressHost, check.Path)
		logger.Tracef("Checking url %s", url)
		httpTimer := NewTimer()
		response, responseCode, err := c.getHTTP(url, check.HTTPTimeout, hardDeadline)
		if err != nil && perrors.Is(err, gocontext.DeadlineExceeded) {
			if timer.Millis() > check.HTTPTimeout && time.Now().Before(hardDeadline) {
				logger.Tracef("[%s] request completed in %s, above threshold of %d", check, httpTimer, check.HTTPTimeout)
				time.Sleep(retryInterval)
				continue
			} else if timer.Millis() > check.HTTPTimeout && time.Now().After(hardDeadline) {
				return timer.Elapsed(), httpTimer.Elapsed(), Failf(check, "request timeout exceeded %s > %d", timer, check.HTTPTimeout)
			} else if time.Now().After(hardDeadline) {
				return timer.Elapsed(), 0, Failf(check, "ingress timeout exceeded %s > %d", timer, check.IngressTimeout)
			} else {
				logger.Debugf("now=%s deadline=%s", time.Now(), hardDeadline)
				continue
			}
		} else if err != nil {
			logger.Tracef("[%s] failed to get http URL %s: %v", check, url, err)
			time.Sleep(retryInterval)
			continue
		}

		found := false
		for _, c := range check.ExpectedHTTPStatuses {
			if c == int64(responseCode) {
				found = true
				break
			}
		}

		if !found && responseCode == http.StatusServiceUnavailable || responseCode == http.StatusNotFound {
			logger.Debugf("[%s] request completed with %d, expected %v, retrying", check, responseCode, check.ExpectedHTTPStatuses)
			time.Sleep(retryInterval)
			continue
		} else if !found {
			return timer.Elapsed(), httpTimer.Elapsed(), Failf(check, "status code %d not expected %v ", responseCode, check.ExpectedHTTPStatuses)
		}
		if !strings.Contains(response, check.ExpectedContent) {
			return timer.Elapsed(), httpTimer.Elapsed(), Failf(check, "content check failed")
		}
		if int64(httpTimer.Elapsed()) > check.HTTPTimeout {
			return timer.Elapsed(), httpTimer.Elapsed(), Failf(check, "request timeout exceeded %s > %d", httpTimer, check.HTTPTimeout)
		}
		return timer.Elapsed(), httpTimer.Elapsed(), Passf(check, "")
	}
}

func (c *NamespaceChecker) createServiceAndIngress(check canaryv1.NamespaceCheck, ns *v1.Namespace, pod *v1.Pod) error {
	if check.Port == 0 {
		return perrors.Errorf("Pod cannot be empty for pod %s in namespace %s", pod.Name, ns.Name)
	}

	svc := &v1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Labels: map[string]string{
				podCheckSelector: c.podCheckSelectorValue(check, ns),
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:       "check",
					Protocol:   v1.ProtocolTCP,
					Port:       int32(check.Port),
					TargetPort: intstr.FromInt(int(check.Port)),
				},
			},
			Selector: map[string]string{
				nameLabel: pod.Name,
			},
		},
	}

	if _, err := c.k8s.CoreV1().Services(svc.Namespace).Create(c.ctx, svc, metav1.CreateOptions{}); err != nil {
		return perrors.Wrapf(err, "Failed to create service for pod %s in namespace %s", pod.Name, pod.Namespace)
	}

	ingress, err := c.k8s.NetworkingV1().Ingresses(ns.Name).Get(c.ctx, check.IngressName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return perrors.Wrapf(err, "Failed to get ingress %s in namespace %s", check.IngressName, ns.Name)
	} else if err == nil {
		logger.Debugf("Updating ingress: %s", check.IngressName)
		pathType := networkingv1.PathTypePrefix
		ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Name = svc.Name
		ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Port.Number = int32(check.Port)
		ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].PathType = &pathType
		if _, err := c.k8s.NetworkingV1().Ingresses(ns.Name).Update(c.ctx, ingress, metav1.UpdateOptions{}); err != nil {
			return perrors.Wrapf(err, "failed to update ingress %s in namespace %s", check.IngressName, ns.Name)
		}
	} else {
		logger.Debugf("Creating ingress: %s", check.IngressName)
		ingress := c.newIngress(check, ns, svc.Name)
		if _, err := c.k8s.NetworkingV1().Ingresses(ns.Name).Create(c.ctx, ingress, metav1.CreateOptions{}); err != nil {
			return perrors.Wrapf(err, "failed to create ingress %s in namespace %s", check.IngressName, ns.Name)
		}
	}

	return nil
}

func (c *NamespaceChecker) newIngress(check canaryv1.NamespaceCheck, ns *v1.Namespace, svc string) *networkingv1.Ingress {
	pathType := networkingv1.PathTypePrefix
	ingress := &networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Ingress",
			APIVersion: "networking/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      check.IngressName,
			Namespace: ns.Name,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: check.IngressHost,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     check.Path,
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: svc,
											Port: networkingv1.ServiceBackendPort{
												Number: int32(check.Port),
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	return ingress
}

func (c *NamespaceChecker) getHTTP(url string, timeout int64, deadline time.Time) (string, int, error) {
	var hardDeadline time.Time
	softTimeoutDeadline := time.Now().Add(time.Duration(timeout) * time.Millisecond)
	if softTimeoutDeadline.After(deadline) {
		hardDeadline = deadline
	} else {
		hardDeadline = softTimeoutDeadline
	}

	ctx, cancelFunc := gocontext.WithDeadline(gocontext.Background(), hardDeadline)
	defer cancelFunc()

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", 0, perrors.Wrapf(err, "failed to create http request for url %s", url)
	}
	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return "", 0, perrors.Wrapf(err, "failed to get url %s", url)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, perrors.Wrapf(err, "failed to read body for url %s", url)
	}
	return string(respBytes), resp.StatusCode, nil
}

func (c *NamespaceChecker) podCheckSelectorValue(check canaryv1.NamespaceCheck, ns *v1.Namespace) string {
	return fmt.Sprintf("%s.%s", check.Name, ns.Name)
}

// WaitForPod waits for a pod to be in the specified phase, or returns an
// error if the timeout is exceeded
func (c *NamespaceChecker) WaitForPod(ns, name string, timeout time.Duration, phases ...v1.PodPhase) (*v1.Pod, error) {
	pods := c.k8s.CoreV1().Pods(ns)
	start := time.Now()
	for {
		pod, err := pods.Get(c.ctx, name, metav1.GetOptions{})
		if start.Add(timeout).Before(time.Now()) {
			return pod, fmt.Errorf("timeout exceeded waiting for %s is %s, error: %v", name, pod.Status.Phase, err)
		}

		if pod == nil || pod.Status.Phase == v1.PodPending {
			time.Sleep(1 * time.Second)
			continue
		}
		if pod.Status.Phase == v1.PodFailed {
			return pod, nil
		}

		for _, phase := range phases {
			if pod.Status.Phase == phase {
				return pod, nil
			}
		}
	}
}
