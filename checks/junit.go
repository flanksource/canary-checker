package checks

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/api/context"

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
	failTestCount        = 10
)

type JunitChecker struct {
}

// Test represents the results of a single test run.
type JunitTest struct {
	// Name is a descriptor given to the test.
	Name string `json:"name" yaml:"name"`

	// Classname is an additional descriptor for the hierarchy of the test.
	Classname string `json:"classname" yaml:"classname"`

	// Duration is the total time taken to run the tests.
	Duration float64 `json:"duration" yaml:"duration"`

	// Status is the result of the test. Status values are passed, skipped,
	// failure, & error.
	Status junit.Status `json:"status" yaml:"status"`

	// Message is an textual description optionally included with a skipped,
	// failure, or error test case.
	Message string `json:"message,omitempty" yaml:"message,omitempty"`

	// Error is a record of the failure or error of a test, if applicable.
	//
	// The following relations should hold true.
	//   Error == nil && (Status == Passed || Status == Skipped)
	//   Error != nil && (Status == Failed || Status == Error)
	Error error `json:"error,omitempty" yaml:"error,omitempty"`

	// Additional properties from XML node attributes.
	// Some tools use them to store additional information about test location.
	Properties map[string]string `json:"properties,omitempty" yaml:"properties,omitempty"`

	// SystemOut is textual output for the test case. Usually output that is
	// written to stdout.
	SystemOut string `json:"stdout,omitempty" yaml:"stdout,omitempty"`

	// SystemErr is textual error output for the test case. Usually output that is
	// written to stderr.
	SystemErr string `json:"stderr,omitempty" yaml:"stderr,omitempty"`
}

type JunitStatus struct {
	passed  int
	failed  int
	skipped int
	error   int
}

type JunitResult struct {
	JunitAggreate `json:",inline"`
	Suites        []JunitTestSuite `json:"suites"`
}

type JunitTestSuite struct {
	JunitAggreate `json:",inline"`
	Name          string      `json:"name"`
	Tests         []JunitTest `json:"tests"`
}
type JunitAggreate struct {
	Passed   int     `json:"passed"`
	Failed   int     `json:"failed"`
	Skipped  int     `json:"skipped,omitempty"`
	Error    int     `json:"error,omitempty"`
	Duration float64 `json:"duration"`
}

func (c *JunitChecker) Type() string {
	return "junit"
}

func (c *JunitChecker) Run(ctx *context.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range ctx.Canary.Spec.Junit {
		result := c.Check(ctx, conf)
		if result != nil {
			results = append(results, result)
		}
	}
	return results
}

func (c *JunitChecker) Check(ctx *context.Context, extConfig external.Check) *pkg.CheckResult {
	start := time.Now()
	var textResults bool
	junitCheck := extConfig.(v1.JunitCheck)
	if junitCheck.GetDisplayTemplate() != "" {
		textResults = true
	}
	interval := ctx.Canary.Spec.Interval
	name := ctx.Canary.Name
	namespace := ctx.Canary.Namespace
	schedule := ctx.Canary.Spec.Schedule
	timeout := junitCheck.GetTimeout()
	var junitStatus JunitStatus
	template := junitCheck.GetDisplayTemplate()
	pod := &corev1.Pod{}
	pod.APIVersion = corev1.SchemeGroupVersion.Version
	pod.Kind = podKind
	pod.Labels = map[string]string{
		junitCheckSelector: getJunitCheckLabel(junitCheckLabelValue, name, namespace),
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
	existingPods := getJunitPods(ctx, name, namespace)
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
		if err := ctx.Kommons.DeleteByKind(podKind, junitPod.Namespace, junitPod.Name); err != nil {
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
	err := ctx.Kommons.Apply(pod.Namespace, pod)
	if err != nil {
		return pkg.Fail(junitCheck).TextResults(textResults).ResultMessage(junitTemplateResult(template, junitStatus)).ErrorMessage(err).StartTime(start)
	}
	defer ctx.Kommons.DeleteByKind(podKind, pod.Namespace, pod.Name) // nolint: errcheck
	logger.Tracef("waiting for pod to be ready")
	err = ctx.Kommons.WaitForPod(pod.Namespace, pod.Name, time.Duration(timeout)*time.Minute, corev1.PodRunning)
	if err != nil {
		return pkg.Fail(junitCheck).TextResults(textResults).ResultMessage(junitTemplateResult(template, junitStatus)).ErrorMessage(fmt.Errorf("timeout waiting for pod: %v", err)).StartTime(start)
	}
	var podObj = corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind: podKind,
		},
	}
	err = ctx.Kommons.Get(pod.Namespace, pod.Name, &podObj)
	if err != nil {
		return pkg.Fail(junitCheck).TextResults(textResults).ResultMessage(junitTemplateResult(template, junitStatus)).ErrorMessage(err).StartTime(start)
	}
	if !kommons.IsPodHealthy(podObj) {
		message, _ := ctx.Kommons.GetPodLogs(pod.Namespace, pod.Name, pod.Spec.InitContainers[0].Name)
		return pkg.Fail(junitCheck).TextResults(textResults).ResultMessage(junitTemplateResult(template, junitStatus)).ErrorMessage(fmt.Errorf("pod is not healthy \n Logs : %v", message)).StartTime(start)
	}
	files, stderr, err := ctx.Kommons.ExecutePodf(pod.Namespace, pod.Name, containerName, "bash", "-c", fmt.Sprintf("find %v -name \\*.xml -type f", mounthPath))
	if stderr != "" || err != nil {
		return pkg.Fail(junitCheck).TextResults(textResults).ResultMessage(junitTemplateResult(template, junitStatus)).ErrorMessage(fmt.Errorf("error fetching test files: %v %v", stderr, err)).StartTime(start)
	}
	files = strings.TrimSpace(files)
	var allTestSuite []junit.Suite
	for _, file := range strings.Split(files, "\n") {
		output, stderr, err := ctx.Kommons.ExecutePodf(pod.Namespace, pod.Name, containerName, "cat", file)
		if stderr != "" || err != nil {
			return pkg.Fail(junitCheck).TextResults(textResults).ResultMessage(junitTemplateResult(template, junitStatus)).ErrorMessage(fmt.Errorf("error reading results: %v %v", stderr, err)).StartTime(start)
		}
		testSuite, err := junit.Ingest([]byte(output))
		if err != nil {
			return pkg.Fail(junitCheck).TextResults(textResults).ResultMessage(junitTemplateResult(template, junitStatus)).ErrorMessage(err).StartTime(start)
		}
		allTestSuite = append(allTestSuite, testSuite...)
	}
	var junitResults JunitResult
	junitResults.Suites = make([]JunitTestSuite, len(allTestSuite))
	for i, suite := range allTestSuite {
		// Aggregate results
		suite.Aggregate()

		junitResults.Passed += suite.Totals.Passed
		junitResults.Failed += suite.Totals.Failed
		junitResults.Skipped += suite.Totals.Skipped
		junitResults.Error += suite.Totals.Error
		junitResults.Duration += suite.Totals.Duration.Seconds()

		junitResults.Suites[i].Passed = suite.Totals.Passed
		junitResults.Suites[i].Failed = suite.Totals.Failed
		junitResults.Suites[i].Skipped = suite.Totals.Skipped
		junitResults.Suites[i].Error = suite.Totals.Error
		junitResults.Suites[i].Duration = suite.Totals.Duration.Seconds()
		junitResults.Suites[i].Name = suite.Name

		// remove duplicate properties form test cases
		suite.Tests = removeDuplicateProperties(suite.Tests)
		junitResults.Suites[i].Tests = getJunitTestsFromJunit(suite.Tests)
	}
	// update status for template results
	junitStatus.passed = junitResults.Passed
	junitStatus.failed = junitResults.Failed
	junitStatus.skipped = junitResults.Skipped
	junitStatus.error = junitResults.Error

	if junitStatus.failed != 0 {
		failMessage := getFailMessageFromTests(junitResults.Suites)
		return pkg.Fail(junitCheck).TextResults(textResults).ResultMessage(junitTemplateResult(template, junitStatus)).ErrorMessage(fmt.Errorf(failMessage)).StartTime(start).AddDetails(junitResults)
	}

	// Don't use junitTemplateResult since we also need to check if templating succeeds here if not we fail
	var results = map[junit.Status]int{junit.StatusFailed: junitStatus.failed, junit.StatusPassed: junitStatus.passed, junit.StatusSkipped: junitStatus.skipped, junit.StatusError: junitStatus.error}
	message, err := text.TemplateWithDelims(junitCheck.GetDisplayTemplate(), "[[", "]]", results)
	if err != nil {
		return pkg.Fail(junitCheck).TextResults(textResults).ResultMessage(junitTemplateResult(template, junitStatus)).ErrorMessage(err).StartTime(start).AddDetails(junitResults)
	}

	return pkg.Success(junitCheck).TextResults(textResults).ResultMessage(message).StartTime(start).AddDetails(junitResults)
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

func getJunitPods(ctx *context.Context, name, namespace string) []corev1.Pod {
	k8s, err := ctx.Kommons.GetClientset()
	if err != nil {
		return nil
	}
	podList, err := k8s.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: junitCheckSelector,
		FieldSelector: getJunitCheckLabel(junitCheckLabelValue, name, namespace),
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

func getJunitCheckLabel(label, name, namespace string) string {
	return fmt.Sprintf("%v-%v-%v", label, name, namespace)
}

// remove duplicate properties from the tests
func removeDuplicateProperties(tests []junit.Test) []junit.Test {
	for _, test := range tests {
		if test.Classname != "" {
			delete(test.Properties, "classname")
		}
		if test.Name != "" {
			delete(test.Properties, "name")
		}
		if test.Duration.String() != "" {
			delete(test.Properties, "time")
		}
	}
	return tests
}

func getFailMessageFromTests(suites []JunitTestSuite) string {
	var message string
	count := 0
	for _, suite := range suites {
		for _, test := range suite.Tests {
			if test.Status == junit.StatusFailed {
				message = message + "\n" + test.Name
				count++
			}
			if count >= failTestCount {
				return message
			}
		}
	}
	return message
}

func getJunitTestsFromJunit(tests []junit.Test) []JunitTest {
	junitTests := make([]JunitTest, len(tests))
	for i, test := range tests {
		junitTests[i] = JunitTest{
			Name:       test.Name,
			Classname:  test.Classname,
			Status:     test.Status,
			Duration:   test.Duration.Seconds(),
			Properties: test.Properties,
			Message:    test.Message,
			Error:      test.Error,
			SystemOut:  test.SystemOut,
			SystemErr:  test.SystemErr,
		}
	}
	return junitTests
}
