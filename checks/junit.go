package checks

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/flanksource/commons/text"
	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/kommons"
	"github.com/joshdk/go-junit"
	corev1 "k8s.io/api/core/v1"
)

func init() {
	//register metrics here
}

const (
	volumeName           = "junit-results"
	mounthPath           = "/tmp/junit-results"
	containerName        = "junit-results"
	containerImage       = "ubuntu"
	podKind              = "Pod"
	junitCheckSelector   = "canary-checker.flanksource.com/check"
	junitCheckLabelValue = "junit-check"
)

type JunitChecker struct {
	kommons *kommons.Client `yaml:"-" json:"-"`
}

type JunitStatus struct {
	passed  int
	failed  int
	skipped int
	error   int
}

func (c *JunitChecker) SetClient(client *kommons.Client) {
	c.kommons = client
}

func (c JunitChecker) GetClient() *kommons.Client {
	return c.kommons
}

func (c *JunitChecker) Type() string {
	return "junit"
}

func (c *JunitChecker) Run(config v1.CanarySpec) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range config.Junit {
		result := c.Check(conf)
		if result != nil {
			results = append(results, result)
		}
	}
	return results
}

func (c *JunitChecker) Check(extConfig external.Check) *pkg.CheckResult {
	start := time.Now()
	var textResults bool
	junitCheck := extConfig.(v1.JunitCheck)
	if junitCheck.GetDisplayTemplate() != "" {
		textResults = true
	}
	interval := junitCheck.GetInterval()
	timeout := junitCheck.GetTimeout()
	var junitStatus JunitStatus
	template := junitCheck.GetDisplayTemplate()
	pod := &corev1.Pod{}
	pod.APIVersion = corev1.SchemeGroupVersion.Version
	pod.Kind = podKind
	pod.Labels = map[string]string{
		junitCheckSelector: junitCheckLabelValue,
	}
	if junitCheck.GetNamespace() != "" {
		pod.Namespace = junitCheck.GetNamespace()
	} else {
		pod.Namespace = corev1.NamespaceDefault
	}
	if junitCheck.GetName() != "" {
		pod.Name = junitCheck.GetName() + "-" + strconv.Itoa(int(start.Unix()))
	} else {
		name := rand.String(5)
		pod.Name = strings.ToLower(name)
	}
	existingPods := getJunitPods(c.kommons, pod.Namespace)
	if len(existingPods) != 0 {
		for _, junitPod := range existingPods {
			obj, err := c.kommons.GetByKind(podKind, junitPod.Namespace, junitPod.Name)
			if err != nil {
				return junitFailF(junitCheck, textResults, junitStatus, template, "error fetching the pod: %v", err)
			}
			if obj != nil {
				logger.Tracef("pod already exist")
				createTime := obj.GetCreationTimestamp()
				duration := time.Since(createTime.Time)
				dur := uint64(duration.Seconds())
				if dur < 2*interval {
					logger.Tracef("Check already in progress, skipping")
					return nil
				}
				if err := c.kommons.DeleteByKind(podKind, junitPod.Namespace, junitPod.Name); err != nil {
					return junitFailF(junitCheck, textResults, junitStatus, template, "error deleting the pod: %v", err)
				}
			}
		}
	}
	pod.Spec = junitCheck.Spec
	pod.Spec.InitContainers = pod.Spec.Containers
	pod.Spec.Containers = getContainers()
	pod.Spec.Volumes = []corev1.Volume{
		{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}
	pod.Spec.RestartPolicy = corev1.RestartPolicyNever
	pod.Spec.InitContainers[0].VolumeMounts = getVolumeMount(volumeName, filepath.Dir(junitCheck.TestResults))
	pod.Spec.Containers[0].VolumeMounts = getVolumeMount(volumeName, mounthPath)
	err := c.kommons.Apply(pod.Namespace, pod)
	if err != nil {
		return junitFailF(junitCheck, textResults, junitStatus, template, "error creating pod: %v", err)
	}
	defer c.kommons.DeleteByKind(podKind, pod.Namespace, pod.Name) // nolint: errcheck
	logger.Tracef("waiting for pod to be ready")
	err = c.kommons.WaitForPod(pod.Namespace, pod.Name, time.Duration(timeout)*time.Minute, corev1.PodRunning)
	if err != nil {
		return junitFailF(junitCheck, textResults, junitStatus, template, "timeout waiting for pod: %v", err)
	}
	podObj, err := getJunitPod(c.kommons, pod.Namespace, pod.Name)
	if err != nil {
		return junitFailF(junitCheck, textResults, junitStatus, template, "error getting pod: %v", err)
	}
	if !kommons.IsPodHealthy(*podObj) {
		message, _ := c.kommons.GetPodLogs(pod.Namespace, pod.Name, pod.Spec.InitContainers[0].Name)
		return junitFailF(junitCheck, textResults, junitStatus, template, "pod is not healthy \n Logs : %v", message)
	}
	files, stderr, err := c.kommons.ExecutePodf(pod.Namespace, pod.Name, containerName, "bash", "-c", fmt.Sprintf("find %v -name \\*.xml -type f", mounthPath))
	if stderr != "" || err != nil {
		return junitFailF(junitCheck, textResults, junitStatus, template, "error fetching test files: %v %v", stderr, err)
	}
	files = strings.TrimSpace(files)
	var allTestSuite []junit.Suite
	for _, file := range strings.Split(files, "\n") {
		output, stderr, err := c.kommons.ExecutePodf(pod.Namespace, pod.Name, containerName, "cat", file)
		if stderr != "" || err != nil {
			return junitFailF(junitCheck, textResults, junitStatus, template, "error reading results: %v %v", stderr, err)
		}
		testSuite, err := junit.Ingest([]byte(output))
		if err != nil {
			return junitFailF(junitCheck, textResults, junitStatus, template, "error parsing the result file: %v", err)
		}
		allTestSuite = append(allTestSuite, testSuite...)
	}
	//initializing results map with 0 values
	var failedTests = make(map[string]string)
	for _, suite := range allTestSuite {
		for _, test := range suite.Tests {
			switch test.Status {
			case junit.StatusFailed:
				junitStatus.failed++
				failedTests[suite.Name+"/"+test.Name] = failedTests[test.Message]
			case junit.StatusPassed:
				junitStatus.passed++
			case junit.StatusSkipped:
				junitStatus.skipped++
			case junit.StatusError:
				junitStatus.error++
			}
			if test.Status == junit.StatusFailed {
				failedTests[suite.Name+"/"+test.Name] = failedTests[test.Message]
			}
		}
	}
	if junitStatus.failed != 0 {
		failMessage := ""
		for testName, testMessage := range failedTests {
			failMessage = failMessage + "\n" + testName + ":" + testMessage
		}
		return junitFailF(junitCheck, textResults, junitStatus, template, failMessage)
	}

	// Don't use junitTemplateResult since we also need to check if templating succeeds here if not we fail
	var results = map[junit.Status]int{junit.StatusFailed: junitStatus.failed, junit.StatusPassed: junitStatus.passed, junit.StatusSkipped: junitStatus.skipped, junit.StatusError: junitStatus.error}
	message, err := text.TemplateWithDelims(junitCheck.GetDisplayTemplate(), "[[", "]]", results)
	if err != nil {
		return junitFailF(junitCheck, textResults, junitStatus, template, "error templating the message: %v", err)
	}

	return Successf(junitCheck, start, textResults, message)
}

func getContainers() []corev1.Container {
	return []corev1.Container{
		{
			Name:  containerName,
			Image: containerImage,
			Args: []string{
				"sleep",
				"10000",
			},
		},
	}
}

func getVolumeMount(volumeName, mountPath string) []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      volumeName,
			MountPath: mountPath,
		},
	}
}

func junitFailF(check external.Check, textResults bool, junitState JunitStatus, template, msg string, args ...interface{}) *pkg.CheckResult {
	message := junitTemplateResult(template, junitState.passed, junitState.failed, junitState.skipped, junitState.error)
	message = message + "\n" + fmt.Sprintf(msg, args...)
	return TextFailf(check, textResults, message)
}

func junitTemplateResult(template string, passed, failed, skipped, error int) (message string) {
	var results = map[junit.Status]int{junit.StatusFailed: failed, junit.StatusPassed: passed, junit.StatusSkipped: skipped, junit.StatusError: error}
	message, err := text.TemplateWithDelims(template, "[[", "]]", results)
	if err != nil {
		message = message + "\n" + err.Error()
	}
	return message
}

func getJunitPods(kommonsClient *kommons.Client, namespace string) []corev1.Pod {
	client, err := kommonsClient.GetClientset()
	if err != nil {
		return nil
	}
	//kommonsClient.GetFirstPodByLabelSelector()
	podList, err := client.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: junitCheckSelector,
	})
	if err != nil {
		return nil
	}
	return podList.Items
}

func getJunitPod(kommonsClient *kommons.Client, namespace, name string) (*corev1.Pod, error) {
	client, err := kommonsClient.GetClientset()
	if err != nil {
		return nil, err
	}
	pod, err := client.CoreV1().Pods(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	return pod, err
}
