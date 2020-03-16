package checks

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
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
	"k8s.io/client-go/util/homedir"
)

const (
	podLabelSelector   = "canary-checker.flanksource.com/podName"
	podGeneralSelector = "canary-checker.flanksource.com/generated"
)

type PodChecker struct{}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *PodChecker) Run(config pkg.Config) []*pkg.CheckResult {
	var checks []*pkg.CheckResult
	for _, conf := range config.Pod {
		for _, result := range c.Check(conf.PodCheck) {
			checks = append(checks, result)
		}
	}
	return checks
}

// Type: returns checker type
func (c *PodChecker) Type() string {
	return "pod"
}

func (c *PodChecker) Check(podCheck pkg.PodCheck) []*pkg.CheckResult {
	var result []*pkg.CheckResult

	var kubeConfig string

	if podCheck.Spec == "" {
		log.Errorf("Pod spec cannot be empty")
		result = append(result, &pkg.CheckResult{
			Pass:    false,
			Invalid: true,
			Message: "Pod spec cannot be empty",
		})
		return result
	}

	if home := homedir.HomeDir(); home != "" {
		kubeConfig = filepath.Join(home, ".kube", "config")
	}

	client, err := pkg.NewK8sClient(kubeConfig)
	if err != nil {
		log.Errorf("failed to create k8s client: %v", err)
		result = append(result, &pkg.CheckResult{
			Pass:     false,
			Invalid:  true,
			Endpoint: c.podEndpoint(podCheck),
			Message:  fmt.Sprintf("Failed to create k8s client: %v", err),
		})

		return result
	}

	podUid := utils.RandomString(6)
	pod := &v1.Pod{}
	if err := yaml.Unmarshal([]byte(podCheck.Spec), pod); err != nil {
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
	pod.Labels[podGeneralSelector] = "true"

	_, err = client.CoreV1().Pods(podCheck.Namespace).Create(pod)
	if err != nil {
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
		c.Cleanup(client, pod.Name, podCheck.Namespace)
	}()

	scheduledChan := make(chan bool)
	readyChan := make(chan bool)
	deletedChan := make(chan bool)
	errorChan := make(chan error)

	watchDuration := time.Duration(podCheck.Deadline) * time.Millisecond
	deadline := time.After(watchDuration)

	if err := c.createServiceAndIngress(client, podCheck, pod); err != nil {
		result = append(result, &pkg.CheckResult{
			Pass:     false,
			Invalid:  true,
			Endpoint: c.podEndpoint(podCheck),
			Message:  fmt.Sprintf("Failed to create service and ingress for pod %s: %v", pod.Name, err),
		})
		return result
	}

	go func() {
		c.WatchPod(client, labels, podCheck.Namespace, watchDuration, scheduledChan, readyChan, deletedChan, errorChan)
	}()

	timer := NewTimer()

loop:
	for {
		select {
		case <-scheduledChan:
			fmt.Printf("Pod %s was scheduled after: %fms\n", pod.Name, timer.Elapsed())
			timer = NewTimer()
		case <-readyChan:
			fmt.Printf("Pod %s is ready after %fms\n", pod.Name, timer.Elapsed())
			// Do the http checks here
			httpCheckResults := c.HttpCheck(client, podCheck, pod)
			result = append(result, httpCheckResults...)
			c.Cleanup(client, pod.Name, podCheck.Namespace)
			timer = NewTimer()
		case <-deletedChan:
			fmt.Printf("Pod %s was deleted after %fms\n", pod.Name, timer.Elapsed())
			break loop
		case err := <-errorChan:
			fmt.Printf("Pod %s has errors after %fms: %v\n", pod.Name, timer.Elapsed(), err)
			return result
		case <-deadline:
			fmt.Printf("Pod %s was not ready in %fms\n", podCheck.Deadline)
			return result
		}
	}

	result = append(result, &pkg.CheckResult{
		Pass:     true,
		Duration: int64(startTimer.Elapsed()),
		Endpoint: c.podEndpoint(podCheck),
		Message:  fmt.Sprintf("pod %s in namespace %s was successfully checked", pod.Name, podCheck.Namespace),
	})

	return result
}

func (c *PodChecker) WatchPod(client *kubernetes.Clientset, labels, namespace string, watchDuration time.Duration, scheduledChan, readyChan, deletedChan chan bool, errorChan chan error) {
	watcher, err := client.CoreV1().Pods(namespace).Watch(metav1.ListOptions{
		LabelSelector: labels,
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
			}).Info("event notified")

			log.Infof("waiting: %v", p.Status.ContainerStatuses)

			switch e.Type {
			case watch.Modified:
				switch p.Status.Phase {
				case v1.PodPending:
					for _, s := range p.Status.ContainerStatuses {
						if s.State.Waiting != nil && s.State.Waiting.Reason == "ContainerCreating" {
							if !scheduled {
								scheduled = true
								scheduledChan <- true
							}
							break
						} else if s.State.Waiting != nil && s.State.Waiting.Reason == "ImagePullBackOff" {
							errorChan <- perrors.Errorf("Failed to run pod %s error: ImagePullBackOff %s", p.Name, s.State.Waiting.Message)
							return
						}
					}
				case v1.PodRunning:
					if !created {
						created = true
						readyChan <- true
					}
				}
			case watch.Deleted:
				deletedChan <- true
				return
			}

		case <-time.After(watchDuration):
			log.Errorf("Watch pod expired after %fms", watchDuration.Milliseconds())
			return
		}
	}
}

func (c *PodChecker) Cleanup(client *kubernetes.Clientset, name, namespace string) error {
	err := client.CoreV1().Pods(namespace).Delete(name, nil)
	if err != nil && !errors.IsNotFound(err) {
		return perrors.Wrapf(err, "Failed to delete pod %s in namespace %s : %v", name, namespace, err)
	}
	err = client.CoreV1().Services(namespace).Delete(name, nil)
	if err != nil && !errors.IsNotFound(err) {
		return perrors.Wrapf(err, "Failed to delete service %s in namespace %s : %v", name, namespace, err)
	}
	return nil
}

func (c *PodChecker) HttpCheck(client *kubernetes.Clientset, podCheck pkg.PodCheck, pod *v1.Pod) []*pkg.CheckResult {
	var result []*pkg.CheckResult

	timer := NewTimer()

	url := fmt.Sprintf("http://%s%s", podCheck.IngressHost, podCheck.Path)
	response, err := c.getHttp(url, podCheck.HttpTimeout)
	if err != nil {
		result = append(result, &pkg.CheckResult{
			Pass:     false,
			Endpoint: c.podEndpoint(podCheck),
			Message:  fmt.Sprintf("Failed to get url %s: %v", url, err),
		})
		return result
	}

	if strings.Contains(response, podCheck.ExpectedContent) {
		result = append(result, &pkg.CheckResult{
			Pass:     true,
			Duration: int64(timer.Elapsed()),
			Endpoint: c.podEndpoint(podCheck),
			Message:  fmt.Sprintf("url %s successfully called and returned expected content", url),
		})
	} else {
		log.Infof("Url %s returned content %s", url, response)
		result = append(result, &pkg.CheckResult{
			Pass:     false,
			Duration: int64(timer.Elapsed()),
			Endpoint: c.podEndpoint(podCheck),
			Message:  fmt.Sprintf("url %s did not return expected content", url),
		})
	}

	return result
}

func (c *PodChecker) createServiceAndIngress(client *kubernetes.Clientset, podCheck pkg.PodCheck, pod *v1.Pod) error {
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

	if _, err := client.CoreV1().Services(svc.Namespace).Create(svc); err != nil {
		return perrors.Wrapf(err, "Failed to create service for pod %s in namespace %s", pod.Name, pod.Namespace)
	}

	ingress, err := client.ExtensionsV1beta1().Ingresses(podCheck.Namespace).Get(podCheck.IngressName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return perrors.Wrapf(err, "Failed to get ingress %s in namespace %s", podCheck.IngressName, podCheck.Namespace)
	} else if err == nil {
		ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServiceName = svc.Name
		ingress.Spec.Rules[0].IngressRuleValue.HTTP.Paths[0].Backend.ServicePort = intstr.FromInt(int(podCheck.Port))
		if _, err := client.ExtensionsV1beta1().Ingresses(podCheck.Namespace).Update(ingress); err != nil {
			return perrors.Wrapf(err, "failed to update ingress %s in namespace %s", podCheck.IngressName, podCheck.Namespace)
		}
	} else {
		ingress := c.newIngress(podCheck, svc.Name)
		if _, err := client.ExtensionsV1beta1().Ingresses(podCheck.Namespace).Create(ingress); err != nil {
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

func (c *PodChecker) getHttp(url string, timeout int64) (string, error) {
	ctx, _ := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Millisecond)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", perrors.Wrapf(err, "failed to create http request for url %s", url)
	}

	resp, err := http.DefaultClient.Do(req.WithContext(ctx))
	if err != nil {
		return "", perrors.Wrapf(err, "failed to get url %s", url)
	}
	defer resp.Body.Close()

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", perrors.Wrapf(err, "failed to read body for url %s", url)
	}
	return string(respBytes), nil
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
