package checks

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/api/external"
	"k8s.io/api/extensions/v1beta1"

	"k8s.io/apimachinery/pkg/util/intstr"

	canaryv1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/commons/logger"
	perrors "github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"

	"golang.org/x/sync/semaphore"
	"k8s.io/client-go/kubernetes"
)

const (
	podLabelSelector   = "canary-checker.flanksource.com/podName"
	podCheckSelector   = "canary-checker.flanksource.com/podCheck"
	podGeneralSelector = "canary-checker.flanksource.com/generated"
)

type PodChecker struct {
	lock *semaphore.Weighted
	k8s  *kubernetes.Clientset
	ng   *NameGenerator

	latestNodeIndex int
}

type ingressHttpResult struct {
	IngressTime float64
	StatusOk    bool
	ContentOk   bool
	RequestTime float64
	StatusCode  int
	Content     string
}

func NewPodChecker() *PodChecker {
	pc := &PodChecker{
		lock: semaphore.NewWeighted(1),
		ng:   &NameGenerator{PodsCount: 20},
	}

	k8sClient, err := pkg.NewK8sClient()
	if err != nil {
		logger.Errorf("Failed to create kubernetes config %v", err)
		return pc
	}

	pc.k8s = k8sClient

	return pc
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *PodChecker) Run(config canaryv1.CanarySpec) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range config.Pod {
		result := c.Check(conf)
		if result != nil {
			results = append(results, result)
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
		return nil, fmt.Errorf("Pod spec cannot be empty")
	}

	pod := &v1.Pod{}
	if err := yaml.Unmarshal([]byte(podCheck.Spec), pod); err != nil {
		return nil, fmt.Errorf("Failed to unmarshall pod spec: %v", err)
	}

	pod.Name = c.ng.PodName(pod.Name + "-")
	pod.Labels[podLabelSelector] = pod.Name
	pod.Labels[podCheckSelector] = c.podCheckSelectorValue(podCheck)
	pod.Labels[podGeneralSelector] = "true"
	pod.Spec.NodeSelector = map[string]string{
		"kubernetes.io/hostname": nodeName,
	}
	if podCheck.PriorityClass != "" {
		pod.Spec.PriorityClassName = podCheck.PriorityClass
	}
	return pod, nil
}

func (c *PodChecker) getConditionTimes(podCheck canaryv1.PodCheck, pod *v1.Pod) (times map[v1.PodConditionType]metav1.Time, err error) {
	pods := c.k8s.CoreV1().Pods(podCheck.Namespace)
	times = make(map[v1.PodConditionType]metav1.Time)
	pod, err = pods.Get(pod.Name, metav1.GetOptions{})
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

func (c *PodChecker) Check(extConfig external.Check) *pkg.CheckResult {
	podCheck := extConfig.(canaryv1.PodCheck)
	if !c.lock.TryAcquire(1) {
		logger.Tracef("Check already in progress, skipping")
		return nil
	}
	defer func() { c.lock.Release(1) }()

	if err := c.Cleanup(podCheck); err != nil {
		return unexpectedErrorf(podCheck, err, "failed to cleanup old artifacts")
	}

	startTimer := NewTimer()

	logger.Debugf("Running pod check %s", podCheck.Name)
	five := int64(5)
	nodes, err := c.k8s.CoreV1().Nodes().List(metav1.ListOptions{TimeoutSeconds: &five})
	if err != nil {
		return unexpectedErrorf(podCheck, err, "cannot connect to API server")
	}
	nextNode, newIndex := c.nextNode(nodes, c.latestNodeIndex)
	c.latestNodeIndex = newIndex

	pod, err := c.newPod(podCheck, nextNode)
	if err != nil {
		return invalidErrorf(podCheck, err, "invalid pod spec")
	}

	pods := c.k8s.CoreV1().Pods(podCheck.Namespace)

	if _, err := pods.Create(pod); err != nil {
		return unexpectedErrorf(podCheck, err, "unable to create pod")
	}
	defer func() {
		c.Cleanup(podCheck)
	}()
	pod, err = c.WaitForPod(podCheck.Namespace, pod.Name, time.Millisecond*time.Duration(podCheck.ScheduleTimeout), v1.PodRunning)
	created := pod.GetCreationTimestamp()

	conditions, err := c.getConditionTimes(podCheck, pod)
	if err != nil {
		return unexpectedErrorf(podCheck, err, "could not list conditions")
	}

	scheduled := diff(conditions, v1.PodInitialized, v1.PodScheduled)
	started := diff(conditions, v1.PodScheduled, v1.ContainersReady)
	running := diff(conditions, v1.ContainersReady, v1.PodReady)

	logger.Debugf("%s created=%s, scheduled=%d, started=%d, running=%d wall=%s nodeName=%s", pod.Name, created, scheduled, started, running, startTimer, nextNode)
	logger.Tracef("%v", conditions)

	if err := c.createServiceAndIngress(podCheck, pod); err != nil {
		return unexpectedErrorf(podCheck, err, "failed to create ingress")
	}

	deadline := time.Now().Add(time.Duration(podCheck.Deadline) * time.Millisecond)

	ingressTime, requestTime, ingressResult := c.httpCheck(podCheck, deadline)

	message := ingressResult.Message

	if !ingressResult.Pass {
		podFailMessage, err := c.podFailMessage(podCheck, pod)
		if err != nil {
			logger.Errorf("failed to get pod fail message: %v", err)
		} else if podFailMessage != "" {
			message = message + " " + podFailMessage
		}
	}

	deletion := NewTimer()
	if err := pods.Delete(pod.Name, &metav1.DeleteOptions{}); err != nil {
		return unexpectedErrorf(podCheck, err, "failed to delete pod")
	}

	return &pkg.CheckResult{
		Check:    podCheck,
		Pass:     ingressResult.Pass,
		Duration: int64(startTimer.Elapsed()),
		Message:  message,
		Metrics: []pkg.Metric{
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
				Value:  float64(deletion.Elapsed()),
			},
			{
				Name:   "ingress_time",
				Type:   metrics.HistogramType,
				Labels: map[string]string{"podCheck": podCheck.Name},
				Value:  float64(ingressTime),
			},
			{
				Name:   "request_time",
				Type:   metrics.HistogramType,
				Labels: map[string]string{"podCheck": podCheck.Name},
				Value:  float64(requestTime),
			},
		},
	}
}

func (c *PodChecker) Cleanup(podCheck canaryv1.PodCheck) error {
	listOptions := metav1.ListOptions{LabelSelector: c.podCheckSelector(podCheck)}

	if c.k8s == nil {
		return fmt.Errorf("Connection to k8s not established")
	}
	err := c.k8s.CoreV1().Pods(podCheck.Namespace).DeleteCollection(nil, listOptions)
	if err != nil && !errors.IsNotFound(err) {
		return perrors.Wrapf(err, "Failed to delete pods for check %s in namespace %s : %v", podCheck.Name, podCheck.Namespace, err)
	}

	services, err := c.k8s.CoreV1().Services(podCheck.Namespace).List(listOptions)
	if err != nil {
		return perrors.Wrapf(err, "Failed to get services for check %s in namespace %s : %v", podCheck.Name, podCheck.Namespace, err)
	}
	for _, s := range services.Items {
		err = c.k8s.CoreV1().Services(podCheck.Namespace).Delete(s.Name, nil)
		if err != nil && !errors.IsNotFound(err) {
			return perrors.Wrapf(err, "Failed to delete service %s in namespace %s : %v", s.Name, podCheck.Namespace, err)
		}
	}
	return nil
}

func (c *PodChecker) httpCheck(podCheck canaryv1.PodCheck, deadline time.Time) (ingressTime float64, requestTime float64, result *pkg.CheckResult) {
	var hardDeadline time.Time
	ingressTimeout := time.Now().Add(time.Duration(podCheck.IngressTimeout) * time.Millisecond)
	if ingressTimeout.After(deadline) {
		hardDeadline = deadline
	} else {
		hardDeadline = ingressTimeout
	}

	timer := NewTimer()
	retryInterval := time.Duration(podCheck.HttpRetryInterval) * time.Millisecond

	for {
		url := fmt.Sprintf("http://%s%s", podCheck.IngressHost, podCheck.Path)
		if _, err := http.NewRequest("GET", url, nil); err != nil {
			return 0, 0, Failf(podCheck, "invalid url: %v", err)
		}
		httpTimer := NewTimer()
		response, responseCode, err := c.getHttp(url, podCheck.HttpTimeout, hardDeadline)
		if err != nil && perrors.Is(err, context.DeadlineExceeded) {
			if timer.Millis() > podCheck.HttpTimeout && time.Now().Before(hardDeadline) {
				logger.Debugf("[%s] request completed in %s, above threshold of %d", podCheck, httpTimer, podCheck.HttpTimeout)
				time.Sleep(retryInterval)
				continue
			} else if timer.Millis() > podCheck.HttpTimeout && time.Now().After(hardDeadline) {
				return timer.Elapsed(), httpTimer.Elapsed(), Failf(podCheck, "request timeout exceeded %s > %d", httpTimer, podCheck.HttpTimeout)
			} else if time.Now().After(hardDeadline) {
				return timer.Elapsed(), 0, Failf(podCheck, "ingress timeout exceeded %s > %d", timer, podCheck.IngressTimeout)
			} else {
				logger.Debugf("now=%s deadline=%s", time.Now(), hardDeadline)
				continue
			}
		} else if err != nil {
			logger.Debugf("[%s] failed to get http URL %s: %v", podCheck, url, err)
			time.Sleep(retryInterval)
			continue
		}

		found := false
		for _, c := range podCheck.ExpectedHttpStatuses {
			if c == responseCode {
				found = true
				break
			}
		}

		if !found && responseCode == http.StatusServiceUnavailable || responseCode == 404 {
			time.Sleep(retryInterval)
			continue
		} else if !found {
			return timer.Elapsed(), httpTimer.Elapsed(), Failf(podCheck, "status code %d not expected %v ", responseCode, podCheck.ExpectedHttpStatuses)
		}
		if !strings.Contains(response, podCheck.ExpectedContent) {
			return timer.Elapsed(), httpTimer.Elapsed(), Failf(podCheck, "content check failed")
		}
		if int64(httpTimer.Elapsed()) > podCheck.HttpTimeout {
			return timer.Elapsed(), httpTimer.Elapsed(), Failf(podCheck, "request timeout exceeded %s > %d", httpTimer, podCheck.HttpTimeout)
		}
		return timer.Elapsed(), httpTimer.Elapsed(), Passf(podCheck, "")
	}
}

func (c *PodChecker) createServiceAndIngress(podCheck canaryv1.PodCheck, pod *v1.Pod) error {
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
				podLabelSelector: pod.Name,
			},
		},
	}

	if _, err := c.k8s.CoreV1().Services(svc.Namespace).Create(svc); err != nil {
		return perrors.Wrapf(err, "Failed to create service for pod %s in namespace %s", pod.Name, pod.Namespace)
	}

	ingress, err := c.k8s.ExtensionsV1beta1().Ingresses(podCheck.Namespace).Get(podCheck.IngressName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return perrors.Wrapf(err, "Failed to get ingress %s in namespace %s", podCheck.IngressName, podCheck.Namespace)
	} else if err == nil {
		ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServiceName = svc.Name
		ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort = intstr.FromInt(int(podCheck.Port))
		if _, err := c.k8s.ExtensionsV1beta1().Ingresses(podCheck.Namespace).Update(ingress); err != nil {
			return perrors.Wrapf(err, "failed to update ingress %s in namespace %s", podCheck.IngressName, podCheck.Namespace)
		}
	} else {
		ingress := c.newIngress(podCheck, svc.Name)
		if _, err := c.k8s.ExtensionsV1beta1().Ingresses(podCheck.Namespace).Create(ingress); err != nil {
			return perrors.Wrapf(err, "failed to create ingress %s in namespace %s", podCheck.IngressName, podCheck.Namespace)
		}
	}

	return nil
}

func (c *PodChecker) newIngress(podCheck canaryv1.PodCheck, svc string) *v1beta1.Ingress {
	ingress := &v1beta1.Ingress{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Ingress",
			APIVersion: "extensions/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podCheck.IngressName,
			Namespace: podCheck.Namespace,
		},
		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					Host: podCheck.IngressHost,
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								{
									Path: podCheck.Path,
									Backend: v1beta1.IngressBackend{
										ServiceName: svc,
										ServicePort: intstr.FromInt(int(podCheck.Port)),
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

func (c *PodChecker) getHttp(url string, timeout int64, deadline time.Time) (string, int, error) {
	var hardDeadline time.Time
	softTimeoutDeadline := time.Now().Add(time.Duration(timeout) * time.Millisecond)
	if softTimeoutDeadline.After(deadline) {
		hardDeadline = deadline
	} else {
		hardDeadline = softTimeoutDeadline
	}

	ctx, _ := context.WithDeadline(context.Background(), hardDeadline)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", 0, perrors.Wrapf(err, "failed to create http request for url %s", url)
	}

	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return "", 0, perrors.Wrapf(err, "failed to get url %s", url)
	}
	defer resp.Body.Close()

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", 0, perrors.Wrapf(err, "failed to read body for url %s", url)
	}
	return string(respBytes), resp.StatusCode, nil
}

func (c *PodChecker) findPort(pod *v1.Pod) (int32, error) {
	for _, container := range pod.Spec.Containers {
		for _, port := range container.Ports {
			return port.ContainerPort, nil
		}
	}
	return 0, perrors.Errorf("Failed to find any port for pod %s", pod.Name)
}

func (c *PodChecker) podCheckSelectorValue(podCheck canaryv1.PodCheck) string {
	return fmt.Sprintf("%s.%s", podCheck.Name, podCheck.Namespace)
}

func (c *PodChecker) podCheckSelector(podCheck canaryv1.PodCheck) string {
	return fmt.Sprintf("%s=%s", podCheckSelector, c.podCheckSelectorValue(podCheck))
}

func (c *PodChecker) podFailMessage(podCheck canaryv1.PodCheck, pod *v1.Pod) (string, error) {
	pods := c.k8s.CoreV1().Pods(pod.Namespace)
	p, err := pods.Get(pod.Name, metav1.GetOptions{})
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
		pod, err := pods.Get(name, metav1.GetOptions{})
		if start.Add(timeout).Before(time.Now()) {
			return pod, fmt.Errorf("Timeout exceeded waiting for %s is %s, error: %v", name, pod.Status.Phase, err)
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
