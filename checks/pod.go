package checks

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"k8s.io/api/extensions/v1beta1"

	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/yaml"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/utils"
	perrors "github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

const (
	podLabelSelector   = "canary-checker.flanksource.com/podName"
	podCheckSelector   = "canary-checker.flanksource.com/podCheck"
	podGeneralSelector = "canary-checker.flanksource.com/generated"
)

type PodChecker struct {
	mutex chan bool
	k8s   *kubernetes.Clientset
}

type watchPod struct {
	Labels        string
	Namespace     string
	Deadline      time.Time
	ScheduledChan chan bool
	ReadyChan     chan bool
	DeletedChan   chan bool
	ErrorChan     chan error
	k8s           *kubernetes.Clientset
	PodName       string
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
		mutex: make(chan bool, 1),
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
func (c *PodChecker) Run(config pkg.Config) []*pkg.CheckResult {
	var checks []*pkg.CheckResult

	deadline := time.Now().Add(config.Interval)
	// Allowed time is required to allow all check goroutines to return results even if deadline was exceeded
	allowedTime := 100 * time.Millisecond
	deadlineChan := time.After(config.Interval + allowedTime)

	// Check if running in serve mode.
	// No two concurrent checks can run at the same time
	// If we do not aquire the lock during the serve interval we just return
	if config.Interval != time.Duration(0) {
		select {
		case c.mutex <- true: // try locking mutex
			defer func() {
				<-c.mutex // unlock mutex
			}()
			break
		case <-deadlineChan:
			return checks
		}
	}

	var checksChan = make(chan []*pkg.CheckResult)
	var checksCount = 0

	for _, conf := range config.Pod {
		go func(podCheck pkg.PodCheck) {
			results := c.Check(podCheck, deadline)
			checksChan <- results
		}(conf.PodCheck)

		checksCount += 1
	}

	for i := 0; i < checksCount; i++ {
		select {
		case results := <-checksChan:
			checks = append(checks, results...)
		case <-deadlineChan:
			return checks
		}
	}

	return checks
}

// Type: returns checker type
func (c *PodChecker) Type() string {
	return "pod"
}

func (c *PodChecker) Check(podCheck pkg.PodCheck, checkDeadline time.Time) []*pkg.CheckResult {
	var result []*pkg.CheckResult

	deadline := time.Now().Add(time.Duration(podCheck.Deadline) * time.Millisecond)
	if deadline.After(checkDeadline) {
		log.Errorf("Pod check %s configured deadline is after check deadline", podCheck.Name)
		deadline = checkDeadline
	}

	log.Infof("Running pod check %s", podCheck.Name)

	if err := c.Cleanup(podCheck); err != nil {
		log.Errorf("Failed to cleanup old artifacts for check %s: %v", podCheck.Name, err)
	}

	if podCheck.Spec == "" {
		log.Errorf("Pod spec cannot be empty")
		result = append(result, &pkg.CheckResult{
			Pass:    false,
			Invalid: true,
			Message: "Pod spec cannot be empty",
		})
		return result
	}

	podUid := utils.RandomString(6)
	pod := &v1.Pod{}
	if err := yaml.Unmarshal([]byte(podCheck.Spec), pod); err != nil {
		log.Errorf("Failed to unmarshall pod spec %s: %v", podCheck.Name, err)
		result = append(result, &pkg.CheckResult{
			Pass:     false,
			Invalid:  true,
			Endpoint: c.podEndpoint(podCheck),
			Message:  fmt.Sprintf("Failed to unmarshall pod spec: %v", err),
		})
		return result
	}

	startTimer := NewTimer()
	pod.Name = fmt.Sprintf("%s-%s", pod.Name, podUid)
	pod.Labels[podLabelSelector] = pod.Name
	pod.Labels[podCheckSelector] = c.podCheckSelectorValue(podCheck)
	pod.Labels[podGeneralSelector] = "true"

	_, err := c.k8s.CoreV1().Pods(podCheck.Namespace).Create(pod)
	if err != nil {
		log.Errorf("Failed to create pod %s: %v", pod.Name, err)
		result = append(result, &pkg.CheckResult{
			Pass:     false,
			Invalid:  true,
			Endpoint: c.podEndpoint(podCheck),
			Message:  fmt.Sprintf("Failed to create pod %s: %v", pod.Name, err),
		})
		return result
	}

	labels := fmt.Sprintf("%s=%s", podLabelSelector, pod.Name)

	defer func() {
		c.Cleanup(podCheck)
	}()

	if err := c.createServiceAndIngress(podCheck, pod); err != nil {
		log.Errorf("Failed to create service %s and ingress %s: %v", pod.Name, podCheck.IngressName, err)
		result = append(result, &pkg.CheckResult{
			Pass:     false,
			Invalid:  true,
			Endpoint: c.podEndpoint(podCheck),
			Message:  fmt.Sprintf("Failed to create service and ingress for pod %s: %v", pod.Name, err),
		})
		return result
	}
	watchPod := newWatchPod(c.k8s, pod.Name, labels, podCheck.Namespace, deadline)

	go func() {
		watchPod.WatchPod()
	}()

	scheduleTime, err := c.WatchEvent("schedule", watchPod.ScheduledChan, watchPod.ErrorChan, podCheck.ScheduleTimeout, deadline)
	if err != nil {
		log.Errorf("Pod %s failed to schedule: %v", pod.Name, err)
		return result
	}

	creationTime, err := c.WatchEvent("create", watchPod.ReadyChan, watchPod.ErrorChan, podCheck.ReadyTimeout, deadline)
	if err != nil {
		log.Errorf("Pod %s failed to create: %v", pod.Name, err)
		return result
	}

	// Do the http checks here
	ingressResult, err := c.httpCheck(podCheck, deadline)
	if err != nil {
		fmt.Printf("Error checking ingress %s: %v", podCheck.IngressName, err)
		return result
	}

	cleanupOk := true
	deleteOk := true

	cleanupErr := c.Cleanup(podCheck)
	if cleanupErr != nil {
		log.Errorf("Error cleaning up for check %s in namespace %s: %v", podCheck.Name, podCheck.Namespace, err)
		cleanupOk = false
	}

	deletionTime, deleteErr := c.WatchEvent("delete", watchPod.DeletedChan, watchPod.ErrorChan, podCheck.DeleteTimeout, deadline)
	if err != nil {
		log.Errorf("Pod %s failed to delete: %v", pod.Name, err)
		deleteOk = false
	}

	message := fmt.Sprintf("pod %s in namespace %s was successfully checked", pod.Name, podCheck.Namespace)

	if !ingressResult.StatusOk {
		message = fmt.Sprintf("Ingress check %s for ingress %s returned wrong status code %d", podCheck.Name, podCheck.IngressName, ingressResult.StatusCode)
	} else if !ingressResult.ContentOk {
		message = fmt.Sprintf("Ingress check %s for ingress %s returned wrong content. Expected %s to contain %s", podCheck.Name, podCheck.IngressName, ingressResult.Content, podCheck.ExpectedContent)
	} else if !cleanupOk {
		message = fmt.Sprintf("Failed to cleanup after pod check %s: %v", podCheck.Name, cleanupErr)
	} else if !deleteOk {
		message = fmt.Sprintf("Failed to delete pod %s for pod check %s: %v", pod.Name, podCheck.Name, deleteErr)
	}

	result = append(result, &pkg.CheckResult{
		Pass:     ingressResult.StatusOk && ingressResult.ContentOk && cleanupOk && deleteOk,
		Duration: int64(startTimer.Elapsed()),
		Endpoint: c.podEndpoint(podCheck),
		Message:  message,
		Metrics: []pkg.Metric{
			{
				Name:   "schedule_time",
				Type:   pkg.HistogramType,
				Labels: map[string]string{"podCheck": podCheck.Name},
				Value:  float64(scheduleTime),
			},
			{
				Name:   "creation_time",
				Type:   pkg.HistogramType,
				Labels: map[string]string{"podCheck": podCheck.Name},
				Value:  float64(creationTime),
			},
			{
				Name:   "delete_time",
				Type:   pkg.HistogramType,
				Labels: map[string]string{"podCheck": podCheck.Name},
				Value:  float64(deletionTime),
			},
			{
				Name:   "ingress_time",
				Type:   pkg.HistogramType,
				Labels: map[string]string{"podCheck": podCheck.Name},
				Value:  float64(ingressResult.IngressTime),
			},
			{
				Name:   "request_time",
				Type:   pkg.HistogramType,
				Labels: map[string]string{"podCheck": podCheck.Name},
				Value:  float64(ingressResult.RequestTime),
			},
		},
	})

	return result
}

func (c *PodChecker) WatchEvent(eventType string, doneChan chan bool, errChan chan error, timeout int64, deadline time.Time) (float64, error) {
	softTimeout := time.After(time.Duration(timeout) * time.Millisecond)
	timer := NewTimer()

	for {
		select {
		case <-doneChan:
			return timer.Elapsed(), nil
		case err := <-errChan:
			return 0, perrors.Wrapf(err, "Received error while trying to %s pod", eventType)
		case <-softTimeout:
			return 0, perrors.Errorf("Timeout %dms exceeded while trying to %s pod", timeout, eventType)
		case <-time.After(time.Until(deadline)):
			return 0, perrors.Errorf("Deadline exceeded while trying to %s pod", timeout, eventType)
		}
	}
}

func (c *PodChecker) Cleanup(podCheck pkg.PodCheck) error {
	listOptions := metav1.ListOptions{LabelSelector: c.podCheckSelector(podCheck)}

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

func (c *PodChecker) httpCheck(podCheck pkg.PodCheck, deadline time.Time) (*ingressHttpResult, error) {
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
		if err != nil {
			log.Debugf("Failed to get http URL %s: %v", url, err)
			time.Sleep(retryInterval)
			continue
		}
		responseTime := httpTimer.Elapsed()

		found := false
		for _, c := range podCheck.ExpectedHttpStatuses {
			if c == responseCode {
				found = true
				break
			}
		}

		if !found && responseCode == http.StatusServiceUnavailable {
			log.Debugf("Expected http check for ingress %s to return %v statuses codes, returned %d", podCheck.IngressName, podCheck.ExpectedHttpStatuses, responseCode)
			time.Sleep(retryInterval)
			continue
		} else if !found {
			result := &ingressHttpResult{
				IngressTime: timer.Elapsed(),
				StatusOk:    false,
				ContentOk:   false,
				RequestTime: responseTime,
				StatusCode:  responseCode,
				Content:     response,
			}
			return result, nil
		}

		result := &ingressHttpResult{
			IngressTime: timer.Elapsed(),
			StatusOk:    true,
			ContentOk:   strings.Contains(response, podCheck.ExpectedContent),
			RequestTime: responseTime,
			StatusCode:  responseCode,
			Content:     response,
		}
		return result, nil
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
	return fmt.Sprintf("canary-checker.flanksource.com/pod/%s/%s", podCheck.Namespace, podCheck.Name)
}

func (c *PodChecker) podCheckSelectorValue(podCheck pkg.PodCheck) string {
	return fmt.Sprintf("%s.%s", podCheck.Name, podCheck.Namespace)
}

func (c *PodChecker) podCheckSelector(podCheck pkg.PodCheck) string {
	return fmt.Sprintf("%s=%s", podCheckSelector, c.podCheckSelectorValue(podCheck))
}

func newWatchPod(k8s *kubernetes.Clientset, podName, labels, namespace string, deadline time.Time) *watchPod {
	w := &watchPod{
		Labels:        labels,
		Namespace:     namespace,
		Deadline:      deadline,
		ScheduledChan: make(chan bool),
		ReadyChan:     make(chan bool),
		DeletedChan:   make(chan bool),
		ErrorChan:     make(chan error),
		k8s:           k8s,
		PodName:       podName,
	}
	return w
}

func (w *watchPod) WatchPod() {
	watcher, err := w.k8s.CoreV1().Pods(w.Namespace).Watch(metav1.ListOptions{
		LabelSelector: w.Labels,
	})
	if err != nil {
		log.Errorf("Cannot create pod event watcher: %v", err)
		return
	}

	var scheduled, created bool

	for {
		select {
		case e := <-watcher.ResultChan():
			if e.Object == nil {
				log.Errorf("Object returned by watcher is nil")
				return
			}

			p, ok := e.Object.(*v1.Pod)
			if !ok {
				continue
			}

			log.WithFields(log.Fields{
				"action":     e.Type,
				"namespace":  p.Namespace,
				"name":       p.Name,
				"phase":      p.Status.Phase,
				"reason":     p.Status.Reason,
				"container#": len(p.Status.ContainerStatuses),
			}).Debugf("event notified")

			switch e.Type {
			case watch.Modified:
				switch p.Status.Phase {
				case v1.PodPending:
					for _, s := range p.Status.ContainerStatuses {
						if s.State.Waiting != nil && s.State.Waiting.Reason == "ContainerCreating" {
							if !scheduled {
								scheduled = true
								w.ScheduledChan <- true
							}
							break
						} else if s.State.Waiting != nil && s.State.Waiting.Reason == "ImagePullBackOff" {
							w.ErrorChan <- perrors.Errorf("Failed to run pod %s error: ImagePullBackOff %s", p.Name, s.State.Waiting.Message)
							return
						}
					}
				case v1.PodRunning:
					if !created {
						created = true
						w.ReadyChan <- true
					}
				}
			case watch.Deleted:
				w.DeletedChan <- true
				return
			}

		case <-time.After(time.Until(w.Deadline)):
			log.Errorf("Watch pod %s exceeded deadline", w.PodName)
			return
		}
	}
}
