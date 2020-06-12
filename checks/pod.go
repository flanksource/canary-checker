package checks

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"strings"
	"time"

	"k8s.io/api/extensions/v1beta1"

	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/yaml"

	"github.com/flanksource/canary-checker/pkg"
	perrors "github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
		log.Errorf("Failed to create kubernetes config %v", err)
		return pc
	}

	pc.k8s = k8sClient

	return pc
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *PodChecker) Run(config pkg.Config, results chan *pkg.CheckResult) {
	for _, conf := range config.Pod {
		deadline := time.Now().Add(config.Interval)
		if deadline.Before(time.Now().Add(time.Duration(conf.Deadline) * time.Millisecond)) {
			deadline = time.Now().Add(time.Duration(conf.Deadline) * time.Millisecond)
		}
		for _, result := range c.Check(conf.PodCheck, deadline) {
			results <- result
		}
	}
}

// Type: returns checker type
func (c *PodChecker) Type() string {
	return "pod"
}

func (c *PodChecker) newPod(podCheck pkg.PodCheck, nodeName string) (*v1.Pod, error) {

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
	return pod, nil
}

func (c *PodChecker) getEventTime(podCheck pkg.PodCheck, pod *v1.Pod, event string) (*metav1.MicroTime, error) {
	events := c.k8s.CoreV1().Events(podCheck.Namespace)

	list, err := events.List(metav1.ListOptions{
		FieldSelector: "involvedObject.name=" + pod.Name,
	})

	if err != nil {
		return nil, err
	}

	for _, evt := range list.Items {
		if evt.Reason == event {
			created := evt.EventTime
			if created.IsZero() {
				created = metav1.MicroTime{evt.LastTimestamp.Time}
			}
			return &created, nil
		}

	}
	return nil, fmt.Errorf("Event not found: %s", event)
}

func (c *PodChecker) getConditionTimes(podCheck pkg.PodCheck, pod *v1.Pod) (times map[v1.PodConditionType]metav1.Time, err error) {
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

func (c *PodChecker) Check(podCheck pkg.PodCheck, checkDeadline time.Time) []*pkg.CheckResult {
	if !c.lock.TryAcquire(1) {
		log.Trace("Check already in progress, skipping")
		return nil
	}
	defer func() { c.lock.Release(1) }()
	var result []*pkg.CheckResult

	if err := c.Cleanup(podCheck); err != nil {
		return unexpectedErrorf(podCheck, err, "failed to cleanup old artifacts")
	}

	startTimer := NewTimer()

	log.Debugf("Running pod check %s", podCheck.Name)
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

	log.Debugf("%s created=%s, scheduled=%d, started=%d, running=%d wall=%s nodeName=%s", pod.Name, created, scheduled, started, running, startTimer, nextNode)
	log.Tracef("%v", conditions)

	if err := c.createServiceAndIngress(podCheck, pod); err != nil {
		return unexpectedErrorf(podCheck, err, "failed to create ingress")
	}

	deadline := time.Now().Add(time.Duration(podCheck.Deadline) * time.Millisecond)

	ingressTime, requestTime, ingressResult := c.httpCheck(podCheck, deadline)

	deleteOk := true
	deletion := NewTimer()
	if err := pods.Delete(pod.Name, &metav1.DeleteOptions{}); err != nil {
		deleteOk = false
		return unexpectedErrorf(podCheck, err, "failed to delete pod")
	}

	result = append(result, &pkg.CheckResult{
		Check:    podCheck,
		Pass:     ingressResult.Pass && deleteOk,
		Duration: int64(startTimer.Elapsed()),
		Endpoint: c.podEndpoint(podCheck),
		Message:  ingressResult.Message,
		Metrics: []pkg.Metric{
			{
				Name:   "schedule_time",
				Type:   pkg.HistogramType,
				Labels: map[string]string{"podCheck": podCheck.Name},
				Value:  float64(scheduled),
			},
			{
				Name:   "creation_time",
				Type:   pkg.HistogramType,
				Labels: map[string]string{"podCheck": podCheck.Name},
				Value:  float64(started),
			},
			{
				Name:   "delete_time",
				Type:   pkg.HistogramType,
				Labels: map[string]string{"podCheck": podCheck.Name},
				Value:  float64(deletion.Elapsed()),
			},
			{
				Name:   "ingress_time",
				Type:   pkg.HistogramType,
				Labels: map[string]string{"podCheck": podCheck.Name},
				Value:  float64(ingressTime),
			},
			{
				Name:   "request_time",
				Type:   pkg.HistogramType,
				Labels: map[string]string{"podCheck": podCheck.Name},
				Value:  float64(requestTime),
			},
		},
	})

	return result
}

func (c *PodChecker) Cleanup(podCheck pkg.PodCheck) error {
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

func (c *PodChecker) httpCheck(podCheck pkg.PodCheck, deadline time.Time) (ingressTime float64, requestTime float64, result *pkg.CheckResult) {
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
		httpTimer := NewTimer()
		response, responseCode, err := c.getHttp(url, podCheck.HttpTimeout, hardDeadline)
		if err != nil && perrors.Is(err, context.DeadlineExceeded) {
			if timer.Millis() > podCheck.HttpTimeout && time.Now().Before(hardDeadline) {
				log.Debugf("[%s] request completed in %s, above threshold of %d", podCheck, httpTimer, podCheck.HttpTimeout)
				time.Sleep(retryInterval)
				continue
			} else if timer.Millis() > podCheck.HttpTimeout && time.Now().After(hardDeadline) {
				return timer.Elapsed(), httpTimer.Elapsed(), Failf(podCheck, "request timeout exceeded %s > %d", httpTimer, podCheck.HttpTimeout)[0]
			} else if time.Now().After(hardDeadline) {
				return timer.Elapsed(), 0, Failf(podCheck, "ingress timeout exceeded %s > %d", timer, podCheck.IngressTimeout)[0]
			} else {
				log.Debugf("now=%s deadline=%s", time.Now(), hardDeadline)
				continue
			}
		} else if err != nil {
			log.Debugf("[%s] failed to get http URL %s: %v", podCheck, url, err)
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
			log.Tracef("[%s] request completed with %d, expected %v, retrying", podCheck, responseCode, podCheck.ExpectedHttpStatuses)
			time.Sleep(retryInterval)
			continue
		} else if !found {
			return timer.Elapsed(), httpTimer.Elapsed(), Failf(podCheck, "status code %d not expected %v ", responseCode, podCheck.ExpectedHttpStatuses)[0]
		}
		if !strings.Contains(response, podCheck.ExpectedContent) {
			return timer.Elapsed(), httpTimer.Elapsed(), Failf(podCheck, "content check failed")[0]
		}
		if int64(httpTimer.Elapsed()) > podCheck.HttpTimeout {
			return timer.Elapsed(), httpTimer.Elapsed(), Failf(podCheck, "request timeout exceeded %s > %d", httpTimer, podCheck.HttpTimeout)[0]
		}
		return timer.Elapsed(), httpTimer.Elapsed(), Passf(podCheck, "")[0]
	}

}

func (c *PodChecker) createServiceAndIngress(podCheck pkg.PodCheck, pod *v1.Pod) error {
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
					Port:       podCheck.Port,
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

func (c *PodChecker) newIngress(podCheck pkg.PodCheck, svc string) *v1beta1.Ingress {
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

func (c *PodChecker) podEndpoint(podCheck pkg.PodCheck) string {
	return fmt.Sprintf("pod/%s", podCheck.Name)
}

func (c *PodChecker) podCheckSelectorValue(podCheck pkg.PodCheck) string {
	return fmt.Sprintf("%s.%s", podCheck.Name, podCheck.Namespace)
}

func (c *PodChecker) podCheckSelector(podCheck pkg.PodCheck) string {
	return fmt.Sprintf("%s=%s", podCheckSelector, c.podCheckSelectorValue(podCheck))
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
