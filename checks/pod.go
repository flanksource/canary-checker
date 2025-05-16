package checks

import (
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	gocontext "context"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/commons/logger"

	"github.com/flanksource/canary-checker/api/external"
	networkingv1 "k8s.io/api/networking/v1"

	"k8s.io/apimachinery/pkg/util/intstr"

	canaryv1 "github.com/flanksource/canary-checker/api/v1"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/metrics"

	perrors "github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	dutyKubernetes "github.com/flanksource/duty/kubernetes"
	"golang.org/x/sync/semaphore"
)

const (
	nameLabel          = "kubernetes.io/metadata.name"
	podCheckSelector   = "canary-checker.flanksource.com/podCheck"
	podGeneralSelector = "canary-checker.flanksource.com/generated"
)

type PodChecker struct {
	lock *semaphore.Weighted
	k8s  *dutyKubernetes.Client
	ng   *NameGenerator

	latestNodeIndex int
}

func NewPodChecker() *PodChecker {
	pc := &PodChecker{
		lock: semaphore.NewWeighted(1),
		ng:   &NameGenerator{PodsCount: 20},
	}
	return pc
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *PodChecker) Run(ctx *context.Context) pkg.Results {
	logger.Warnf("pod check is deprecated. Please use the kubernetes resource check")

	var results pkg.Results
	if len(ctx.Canary.Spec.Pod) > 0 {
		if c.k8s == nil {
			var err error
			c.k8s, err = ctx.Kubernetes()
			if err != nil {
				return results.Failf("error creating kubernetes client: %v", err)
			}
		}
		for _, conf := range ctx.Canary.Spec.Pod {
			results = append(results, c.Check(ctx, conf)...)
		}
	}
	return results
}

// Type: returns checker type
func (c *PodChecker) Type() string {
	return "pod"
}

func (c *PodChecker) newPod(podCheck canaryv1.PodCheck, nodeName string) (*v1.Pod, error) {
	if podCheck.Spec == "" {
		return nil, fmt.Errorf("pod spec cannot be empty")
	}

	pod := &v1.Pod{}
	if err := yaml.Unmarshal([]byte(podCheck.Spec), pod); err != nil {
		return nil, fmt.Errorf("failed to unmarshall pod spec: %v", err)
	}

	pod.Name = c.ng.PodName(pod.Name + "-")
	pod.Namespace = podCheck.Namespace
	pod.Labels[nameLabel] = pod.Name
	pod.Labels[podCheckSelector] = c.podCheckSelectorValue(podCheck)
	pod.Labels[podGeneralSelector] = "true"

	if podCheck.RoundRobinNodes {
		pod.Spec.NodeSelector = map[string]string{
			"kubernetes.io/hostname": nodeName,
		}
	}

	if podCheck.PriorityClass != "" {
		pod.Spec.PriorityClassName = podCheck.PriorityClass
	}
	return pod, nil
}

func (c *PodChecker) getConditionTimes(podCheck canaryv1.PodCheck, pod *v1.Pod) (times map[v1.PodConditionType]metav1.Time, err error) {
	pods := c.k8s.CoreV1().Pods(podCheck.Namespace)
	times = make(map[v1.PodConditionType]metav1.Time)
	pod, err = pods.Get(gocontext.TODO(), pod.Name, metav1.GetOptions{})
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

func diff(times map[v1.PodConditionType]metav1.Time, c1 v1.PodConditionType, c2 v1.PodConditionType) int64 {
	t1, ok1 := times[c1]
	t2, ok2 := times[c2]
	if ok1 && ok2 {
		return t2.Sub(t1.Time).Milliseconds()
	}
	return -1
}

func (c *PodChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	podCheck := extConfig.(canaryv1.PodCheck)

	if !c.lock.TryAcquire(1) {
		ctx.Tracef("Check already in progress, skipping")
		return nil
	}
	defer func() { c.lock.Release(1) }()

	if podCheck.Namespace == "" {
		podCheck.Namespace = ctx.Namespace
	}

	result := pkg.Success(podCheck, ctx.Canary)
	var results pkg.Results
	results = append(results, result)
	startTimer := NewTimer()
	pods := c.k8s.CoreV1().Pods(podCheck.Namespace)

	if skip, err := cleanupExistingPods(ctx, c.k8s, c.podCheckSelector(podCheck)); err != nil {
		return results.ErrorMessage(err)
	} else if skip {
		return nil
	}

	c.Cleanup(ctx, podCheck)       // cleanup existing resources
	defer c.Cleanup(ctx, podCheck) // cleanup resources created during test

	five := int64(5)
	nodes, err := c.k8s.CoreV1().Nodes().List(ctx, metav1.ListOptions{TimeoutSeconds: &five})
	if err != nil {
		return results.Failf("cannot connect to API server: %v", err)
	}
	nextNode, newIndex := c.nextNode(nodes, c.latestNodeIndex)
	c.latestNodeIndex = newIndex

	pod, err := c.newPod(podCheck, nextNode)
	if err != nil {
		return results.Failf("invalid pod spec: %v", err)
	}

	if _, err := c.k8s.CoreV1().Pods(podCheck.Namespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		return results.ErrorMessage(err)
	}

	pod, err = c.WaitForPod(podCheck.Namespace, pod.Name, time.Millisecond*time.Duration(podCheck.ScheduleTimeout), v1.PodRunning)
	if err != nil {
		return results.Failf("unable to fetch pod details: %v", err)
	}
	created := pod.GetCreationTimestamp()

	conditions, err := c.getConditionTimes(podCheck, pod)
	if err != nil {
		return results.Failf("could not list conditions: %v", err)
	}

	scheduled := diff(conditions, v1.PodInitialized, v1.PodScheduled)
	started := diff(conditions, v1.PodScheduled, v1.ContainersReady)
	running := diff(conditions, v1.ContainersReady, v1.PodReady)

	ctx.Debugf("%s created=%s, scheduled=%d, started=%d, running=%d wall=%s nodeName=%s", pod.Name, created, scheduled, started, running, startTimer, nextNode)

	if err := c.createServiceAndIngress(ctx, podCheck, pod); err != nil {
		return results.Failf("failed to create service or ingress: %v", err)
	}

	deadline := time.Now().Add(time.Duration(podCheck.Deadline) * time.Millisecond)

	host := podCheck.IngressHost
	if host == "" {
		host = fmt.Sprintf("%s.%s:%d", pod.Name, pod.Namespace, podCheck.Port)
	}
	ingressTime, requestTime, ingressResult := c.httpCheck(ctx, podCheck, deadline, host)

	message := ingressResult.Message

	if !ingressResult.Pass {
		podFailMessage, err := c.podFailMessage(pod)
		if err != nil {
			ctx.Error(err, "failed to get pod fail message")
		}
		if podFailMessage != "" {
			return results.Failf("%s", message+" "+podFailMessage)
		}
		return results.Failf("%s", message)
	}

	deletion := NewTimer()
	if err := pods.Delete(ctx, pod.Name, metav1.DeleteOptions{}); err != nil {
		return results.Failf("failed to delete pod: %v", err)
	}

	result.ResultMessage("%s", message)

	result.Metrics = []pkg.Metric{
		{
			Name:   "schedule_time",
			Type:   metrics.HistogramType,
			Labels: map[string]string{"podCheck": podCheck.Name},
			Value:  float64(scheduled),
		},
		{
			Name:   "creation_time",
			Type:   metrics.HistogramType,
			Labels: map[string]string{"podCheck": podCheck.Name},
			Value:  float64(started),
		},
		{
			Name:   "delete_time",
			Type:   metrics.HistogramType,
			Labels: map[string]string{"podCheck": podCheck.Name},
			Value:  deletion.Elapsed(),
		},
		{
			Name:   "ingress_time",
			Type:   metrics.HistogramType,
			Labels: map[string]string{"podCheck": podCheck.Name},
			Value:  ingressTime,
		},
		{
			Name:   "request_time",
			Type:   metrics.HistogramType,
			Labels: map[string]string{"podCheck": podCheck.Name},
			Value:  requestTime,
		},
	}
	return results
}

func (c *PodChecker) Cleanup(ctx *context.Context, podCheck canaryv1.PodCheck) {
	listOptions := metav1.ListOptions{LabelSelector: c.podCheckSelector(podCheck)}

	if c.k8s == nil {
		ctx.Warnf("connection to k8s not established")
	}
	err := c.k8s.CoreV1().Pods(podCheck.Namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, listOptions)
	if err != nil && !errors.IsNotFound(err) {
		ctx.Warnf("Failed to delete pods for check %s in namespace %s : %v", podCheck.Name, podCheck.Namespace, err)
	}

	services, err := c.k8s.CoreV1().Services(podCheck.Namespace).List(ctx, listOptions)
	if err != nil {
		ctx.Warnf("Failed to get services to cleanup %s in namespace %s : %v", podCheck.Name, podCheck.Namespace, err)
	}

	for _, s := range services.Items {
		if err := c.k8s.CoreV1().Services(podCheck.Namespace).Delete(ctx, s.Name, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			ctx.Warnf("Failed delete services %s in namespace %s : %v", s.Name, podCheck.Namespace, err)
		}
	}
}

func (c *PodChecker) httpCheck(ctx *context.Context, podCheck canaryv1.PodCheck, deadline time.Time, host string) (ingressTime float64, requestTime float64, result *pkg.CheckResult) {
	httpTimeout := podCheck.HTTPTimeout
	if httpTimeout == 0 {
		httpTimeout = 5000
	}
	ingressTimeout := podCheck.IngressTimeout
	if ingressTimeout == 0 {
		ingressTimeout = 5000
	}
	retryInterval := podCheck.HTTPRetryInterval
	if retryInterval == 0 {
		retryInterval = 750
	}
	retry := time.Duration(retryInterval) * time.Millisecond
	hardTimeout := int64(math.Max(float64(httpTimeout), float64(ingressTimeout)))

	if deadline2 := time.Now().Add(time.Duration(hardTimeout) * time.Second); deadline2.Before(deadline) {
		deadline = deadline2
	}

	timer := NewTimer()
	url := fmt.Sprintf("http://%s%s", host, podCheck.Path)

	for {
		if _, err := http.NewRequest("GET", url, nil); err != nil {
			return 0, 0, Failf(podCheck, "invalid url: %v", err)
		}
		httpTimer := NewTimer()
		response, responseCode, err := c.getHTTP(url, podCheck.HTTPTimeout, deadline)
		if err != nil && perrors.Is(err, gocontext.DeadlineExceeded) {
			if timer.Millis() > podCheck.HTTPTimeout && time.Now().Before(deadline) {
				ctx.Tracef("[%s] request completed in %s, above threshold of %d", podCheck, httpTimer, httpTimeout)
				time.Sleep(retry)
				continue
			} else if httpTimer.Millis() > httpTimeout && time.Now().After(deadline) {
				return timer.Elapsed(), httpTimer.Elapsed(), Failf(podCheck, "request timeout exceeded %d > %d", httpTimer.Millis(), httpTimeout)
			} else if time.Now().After(deadline) {
				return timer.Elapsed(), 0, Failf(podCheck, "hard deadline timeout %s", time.Since(deadline))
			} else {
				continue
			}
		} else if err != nil {
			ctx.Tracef("[%s] failed to get http URL %s: %v", podCheck, url, err)
			time.Sleep(retry)
			continue
		}

		found := false
		for _, c := range podCheck.ExpectedHTTPStatuses {
			if c == responseCode {
				found = true
				break
			}
		}

		if !found && responseCode == http.StatusServiceUnavailable || responseCode == 404 {
			time.Sleep(retry)
			continue
		} else if !found {
			return timer.Elapsed(), httpTimer.Elapsed(), Failf(podCheck, "status code %d not expected %v ", responseCode, podCheck.ExpectedHTTPStatuses)
		}
		if !strings.Contains(response, podCheck.ExpectedContent) {
			return timer.Elapsed(), httpTimer.Elapsed(), Failf(podCheck, "content check failed")
		}
		if int64(httpTimer.Elapsed()) > httpTimeout {
			return timer.Elapsed(), httpTimer.Elapsed(), Failf(podCheck, "request timeout exceeded %s > %d", httpTimer, httpTimeout)
		}
		return timer.Elapsed(), httpTimer.Elapsed(), Passf(podCheck, "")
	}
}

func (c *PodChecker) createServiceAndIngress(ctx *context.Context, podCheck canaryv1.PodCheck, pod *v1.Pod) error {
	if podCheck.Port == 0 {
		return perrors.Errorf("Pod cannot be empty for pod %s in namespace %s", pod.Name, pod.Namespace)
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
				podCheckSelector: c.podCheckSelectorValue(podCheck),
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Name:       "check",
					Protocol:   v1.ProtocolTCP,
					Port:       int32(podCheck.Port),
					TargetPort: intstr.FromInt(int(podCheck.Port)),
				},
			},
			Selector: map[string]string{
				nameLabel: pod.Name,
			},
		},
	}

	if _, err := c.k8s.CoreV1().Services(svc.Namespace).Create(gocontext.TODO(), svc, metav1.CreateOptions{}); err != nil {
		return perrors.Wrapf(err, "Failed to create service for pod %s in namespace %s", pod.Name, pod.Namespace)
	}

	if podCheck.IngressHost == "" || podCheck.IngressName == "" {
		return nil
	}
	ingress, err := c.k8s.NetworkingV1().Ingresses(podCheck.Namespace).Get(gocontext.TODO(), podCheck.IngressName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return perrors.Wrapf(err, "Failed to get ingress %s in namespace %s", podCheck.IngressName, podCheck.Namespace)
	} else if err == nil {
		ctx.Debugf("Updating ingress: %s", podCheck.IngressName)
		pathType := networkingv1.PathTypePrefix
		ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Name = svc.Name
		ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.Service.Port.Number = int32(podCheck.Port)
		ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].PathType = &pathType
		if podCheck.IngressClass != "" {
			ingress.Spec.IngressClassName = &podCheck.IngressClass
		}
		if _, err := c.k8s.NetworkingV1().Ingresses(podCheck.Namespace).Update(gocontext.TODO(), ingress, metav1.UpdateOptions{}); err != nil {
			return perrors.Wrapf(err, "failed to update ingress %s in namespace %s", podCheck.IngressName, podCheck.Namespace)
		}
	} else {
		ctx.Debugf("Creating ingress: %s", podCheck.IngressName)
		ingress := c.newIngress(podCheck, svc.Name)
		if _, err := c.k8s.NetworkingV1().Ingresses(podCheck.Namespace).Create(gocontext.TODO(), ingress, metav1.CreateOptions{}); err != nil {
			return perrors.Wrapf(err, "failed to create ingress %s in namespace %s", podCheck.IngressName, podCheck.Namespace)
		}
	}

	return nil
}

func (c *PodChecker) newIngress(podCheck canaryv1.PodCheck, svc string) *networkingv1.Ingress {
	pathType := networkingv1.PathTypePrefix
	ingress := &networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Ingress",
			APIVersion: "networking/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podCheck.IngressName,
			Namespace: podCheck.Namespace,
			Labels: map[string]string{
				podCheckSelector: c.podCheckSelectorValue(podCheck),
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: podCheck.IngressHost,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     podCheck.Path,
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: svc,
											Port: networkingv1.ServiceBackendPort{
												Number: int32(podCheck.Port),
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
	if podCheck.IngressClass != "" {
		ingress.Spec.IngressClassName = &podCheck.IngressClass
	}
	return ingress
}

func (c *PodChecker) getHTTP(url string, timeout int64, deadline time.Time) (string, int, error) {
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

func (c *PodChecker) podCheckSelectorValue(podCheck canaryv1.PodCheck) string {
	return fmt.Sprintf("%s.%s", podCheck.Name, podCheck.Namespace)
}

func (c *PodChecker) podCheckSelector(podCheck canaryv1.PodCheck) string {
	return fmt.Sprintf("%s=%s", podCheckSelector, c.podCheckSelectorValue(podCheck))
}

func (c *PodChecker) podFailMessage(pod *v1.Pod) (string, error) {
	pods := c.k8s.CoreV1().Pods(pod.Namespace)
	p, err := pods.Get(gocontext.Background(), pod.Name, metav1.GetOptions{})
	if err != nil {
		return "", perrors.Wrapf(err, "failed to get pod %s in namespace %s", pod.Name, pod.Namespace)
	}
	if p.Status.Phase != v1.PodRunning {
		msg := []string{}
		for _, cs := range p.Status.ContainerStatuses {
			if !cs.Ready && cs.State.Waiting != nil {
				msg = append(msg, fmt.Sprintf("[container=%s message=%s reason=%s]", cs.Name, cs.State.Waiting.Message, cs.State.Waiting.Reason))
			}
		}
		return fmt.Sprintf("podPhase=%s %s", p.Status.Phase, strings.Join(msg, " ")), nil
	}
	return "", nil
}

// WaitForPod waits for a pod to be in the specified phase, or returns an
// error if the timeout is exceeded
func (c *PodChecker) WaitForPod(ns, name string, timeout time.Duration, phases ...v1.PodPhase) (*v1.Pod, error) {
	pods := c.k8s.CoreV1().Pods(ns)
	start := time.Now()
	for {
		pod, err := pods.Get(gocontext.TODO(), name, metav1.GetOptions{})
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

func (c *PodChecker) nextNode(nodes *v1.NodeList, lastIndex int) (string, int) {
	nodeCount := len(nodes.Items)
	nodeNames := make([]string, nodeCount)
	for i, n := range nodes.Items {
		nodeNames[i] = n.Name
	}
	sort.Strings(nodeNames)
	nextIndex := (lastIndex + 1) % nodeCount
	return nodeNames[nextIndex], nextIndex
}
