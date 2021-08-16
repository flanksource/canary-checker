package checks

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

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

func (c *JunitChecker) Run(canary v1.Canary) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range canary.Spec.Junit {
		result := c.Check(canary, conf)
		if result != nil {
			results = append(results, result)
		}
	}
	return results
}

func (c *JunitChecker) Check(canary v1.Canary, extConfig external.Check) *pkg.CheckResult {
	start := time.Now()
	var textResults bool
	junitCheck := extConfig.(v1.JunitCheck)
	if junitCheck.GetDisplayTemplate() != "" {
		textResults = true
	}
	interval := canary.Spec.Interval
	name := canary.Name
	namespace := canary.Namespace
	schedule := canary.Spec.Schedule
	timeout := junitCheck.GetTimeout()
	var junitStatus JunitStatus
	template := junitCheck.GetDisplayTemplate()
	pod := &corev1.Pod{}
	pod.APIVersion = corev1.SchemeGroupVersion.Version
	pod.Kind = podKind
	pod.Labels = map[string]string{
		junitCheckSelector: junitCheckLabelValue,
	}
	if namespace != "" {
		pod.Namespace = namespace
	} else {
		pod.Namespace = corev1.NamespaceDefault
	}
	if name != "" {
		pod.Name = name + "-" + strings.ToLower(rand.String(5))
	} else {
		pod.Name = strings.ToLower(rand.String(5))
	}
	existingPods := getJunitPods(c.kommons, pod.Namespace)
	for _, junitPod := range existingPods {
		createTime := junitPod.CreationTimestamp.Time
		wait, err := waitForExistingJunitCheck(interval, schedule, createTime)
		if err != nil {
			return pkg.Fail(junitCheck).TextResults(textResults).ResultMessage(junitTemplateResult(template, junitStatus)).ErrorMessage(err).StartTime(start)
		}
		if wait {
			logger.Tracef("junit check already in progress, skipping")
			return nil
		}
		if err := c.kommons.DeleteByKind(podKind, junitPod.Namespace, junitPod.Name); err != nil {
			return pkg.Fail(junitCheck).TextResults(textResults).ResultMessage(junitTemplateResult(template, junitStatus)).ErrorMessage(err).StartTime(start)
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
		return pkg.Fail(junitCheck).TextResults(textResults).ResultMessage(junitTemplateResult(template, junitStatus)).ErrorMessage(err).StartTime(start)
	}
	defer c.kommons.DeleteByKind(podKind, pod.Namespace, pod.Name) // nolint: errcheck
	logger.Tracef("waiting for pod to be ready")
	err = c.kommons.WaitForPod(pod.Namespace, pod.Name, time.Duration(timeout)*time.Minute, corev1.PodRunning)
	if err != nil {
		return pkg.Fail(junitCheck).TextResults(textResults).ResultMessage(junitTemplateResult(template, junitStatus)).ErrorMessage(fmt.Errorf("timeout waiting for pod: %v", err)).StartTime(start)
	}
	var podObj = corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind: podKind,
		},
	}
	err = c.kommons.Get(pod.Namespace, pod.Name, &podObj)
	if err != nil {
		return pkg.Fail(junitCheck).TextResults(textResults).ResultMessage(junitTemplateResult(template, junitStatus)).ErrorMessage(err).StartTime(start)
	}
	if !kommons.IsPodHealthy(podObj) {
		message, _ := c.kommons.GetPodLogs(pod.Namespace, pod.Name, pod.Spec.InitContainers[0].Name)
		return pkg.Fail(junitCheck).TextResults(textResults).ResultMessage(junitTemplateResult(template, junitStatus)).ErrorMessage(fmt.Errorf("pod is not healthy \n Logs : %v", message)).StartTime(start)
	}
	files, stderr, err := c.kommons.ExecutePodf(pod.Namespace, pod.Name, containerName, "bash", "-c", fmt.Sprintf("find %v -name \\*.xml -type f", mounthPath))
	if stderr != "" || err != nil {
		return pkg.Fail(junitCheck).TextResults(textResults).ResultMessage(junitTemplateResult(template, junitStatus)).ErrorMessage(fmt.Errorf("error fetching test files: %v %v", stderr, err)).StartTime(start)
	}
	files = strings.TrimSpace(files)
	var allTestSuite []junit.Suite
	for _, file := range strings.Split(files, "\n") {
		output, stderr, err := c.kommons.ExecutePodf(pod.Namespace, pod.Name, containerName, "cat", file)
		if stderr != "" || err != nil {
			return pkg.Fail(junitCheck).TextResults(textResults).ResultMessage(junitTemplateResult(template, junitStatus)).ErrorMessage(fmt.Errorf("error reading results: %v %v", stderr, err)).StartTime(start)
		}
		testSuite, err := junit.Ingest([]byte(output))
		if err != nil {
			return pkg.Fail(junitCheck).TextResults(textResults).ResultMessage(junitTemplateResult(template, junitStatus)).ErrorMessage(err).StartTime(start)
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
		return pkg.Fail(junitCheck).TextResults(textResults).ResultMessage(junitTemplateResult(template, junitStatus)).ErrorMessage(fmt.Errorf(failMessage)).StartTime(start)
	}

	// Don't use junitTemplateResult since we also need to check if templating succeeds here if not we fail
	var results = map[junit.Status]int{junit.StatusFailed: junitStatus.failed, junit.StatusPassed: junitStatus.passed, junit.StatusSkipped: junitStatus.skipped, junit.StatusError: junitStatus.error}
	message, err := text.TemplateWithDelims(junitCheck.GetDisplayTemplate(), "[[", "]]", results)
	if err != nil {
		return pkg.Fail(junitCheck).TextResults(textResults).ResultMessage(junitTemplateResult(template, junitStatus)).ErrorMessage(err).StartTime(start)
	}

	return pkg.Success(junitCheck).TextResults(textResults).ResultMessage(message).StartTime(start)
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

func junitTemplateResult(template string, junitStatus JunitStatus) (message string) {
	var results = map[junit.Status]int{junit.StatusFailed: junitStatus.failed, junit.StatusPassed: junitStatus.passed, junit.StatusSkipped: junitStatus.skipped, junit.StatusError: junitStatus.error}
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
	podList, err := client.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: junitCheckSelector,
	})
	if err != nil {
		return nil
	}
	return podList.Items
}

func waitForExistingJunitCheck(interval uint64, spec string, createTime time.Time) (wait bool, err error) {
	if spec != "" {
		schedule, err := cron.ParseStandard(spec)
		if err != nil {
			return false, err
		}
		checkTime := schedule.Next(time.Now())
		if time.Since(createTime) < 2*time.Until(checkTime) {
			return true, nil
		}
		return false, nil
	}
	if uint64(time.Since(createTime).Seconds()) < 2*interval {
		return true, nil
	}
	return false, nil
}
