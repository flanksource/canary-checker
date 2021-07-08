package checks

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

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
	volumeName     = "junit-results"
	mounthPath     = "/tmp/junit-results"
	containerName  = "junit-results"
	containerImage = "ubuntu"
	// time in minutes to wait for the initial pod is completed
	maxTime = 5
	podKind = "Pod"
)

type JunitChecker struct {
	kommons *kommons.Client `yaml:"-" json:"-"`
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
		results = append(results, c.Check(conf))
	}
	return results
}

func (c *JunitChecker) Check(extConfig external.Check) *pkg.CheckResult {
	start := time.Now()
	var textResults bool
	junitCheck := extConfig.(v1.JunitCheck)
	if junitCheck.GetTemplate() != "" {
		textResults = true
	}
	pod := &corev1.Pod{}
	pod.APIVersion = corev1.SchemeGroupVersion.Version
	pod.Kind = podKind
	if junitCheck.GetNamespace() != "" {
		pod.Namespace = junitCheck.GetNamespace()
	} else {
		pod.Namespace = corev1.NamespaceDefault
	}
	if junitCheck.GetName() != "" {
		pod.Name = junitCheck.GetName()
	} else {
		name := rand.String(5)
		pod.Name = strings.ToLower(name)
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
	pod.Spec.InitContainers[0].VolumeMounts = getVolumeMount(volumeName, filepath.Dir(junitCheck.TestResults))
	pod.Spec.Containers[0].VolumeMounts = getVolumeMount(volumeName, mounthPath)
	err := c.kommons.Apply(pod.Namespace, pod)
	if err != nil {
		return TextFailf(junitCheck, textResults, "error creating pod: %v", err)
	}
	defer c.kommons.DeleteByKind(pod.Kind, pod.Namespace, pod.Name) // nolint: errcheck
	logger.Tracef("waiting for pod to be ready")
	err = c.kommons.WaitForPod(pod.Namespace, pod.Name, maxTime*time.Minute, corev1.PodRunning)
	if err != nil {
		return TextFailf(junitCheck, textResults, "error waiting for pod: %v", err)
	}
	files, stderr, err := c.kommons.ExecutePodf(pod.Namespace, pod.Name, containerName, "bash", "-c", fmt.Sprintf("find %v -name \\*.xml -type f", mounthPath))
	if stderr != "" || err != nil {
		return TextFailf(junitCheck, textResults, "error fetching test files: %v %v", stderr, err)
	}
	files = strings.TrimSpace(files)
	var allTestSuite []junit.Suite
	for _, file := range strings.Split(files, "\n") {
		output, stderr, err := c.kommons.ExecutePodf(pod.Namespace, pod.Name, containerName, "cat", file)
		if stderr != "" || err != nil {
			return TextFailf(junitCheck, textResults, "error reading results: %v %v", stderr, err)
		}
		testSuite, err := junit.Ingest([]byte(output))
		if err != nil {
			return TextFailf(junitCheck, textResults, "error parsing the result file")
		}
		allTestSuite = append(allTestSuite, testSuite...)
	}
	//initializing results map with 0 values
	var failedTests = make(map[string]string)
	var results = map[junit.Status]int{junit.StatusFailed: 0, junit.StatusPassed: 0, junit.StatusSkipped: 0, junit.StatusError: 0}
	for _, suite := range allTestSuite {
		for _, test := range suite.Tests {
			results[test.Status] += 1
			if test.Status == junit.StatusFailed {
				failedTests[suite.Name+"/"+test.Name] = failedTests[test.Message]
			}
		}
	}
	message, err := text.Template(junitCheck.GetTemplate(), results)
	if err != nil {
		return TextFailf(junitCheck, textResults, "error templating the message: %v", err)
	}
	if results[junit.StatusFailed] != 0 {
		failMessage := ""
		for testName, testMessage := range failedTests {
			failMessage = failMessage + "\n" + testName + ":" + testMessage
		}
		message = message + failMessage
		return TextFailf(junitCheck, textResults, message)
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
