package checks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/flanksource/artifacts"
	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/pkg/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/kommons"
	corev1 "k8s.io/api/core/v1"
)

func init() {
	//register metrics here
}

const (
	volumeName           = "junit-results"
	mountPath            = "/tmp/junit-results"
	containerName        = "junit-results"
	containerImage       = "ubuntu"
	podKind              = "Pod"
	junitCheckSelector   = "canary-checker.flanksource.com/check"
	junitCheckLabelValue = "junit-check"
)

// JunitChecker runs a junit test on a new kubernetes pod and then saves
// the test result.
type JunitChecker struct {
}

func (c *JunitChecker) Type() string {
	return "junit"
}

func (c *JunitChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.Junit {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

func newPod(ctx *context.Context, check v1.JunitCheck) (*corev1.Pod, error) {
	pod := &corev1.Pod{}
	pod.APIVersion = corev1.SchemeGroupVersion.Version
	pod.Kind = podKind
	pod.Labels = map[string]string{
		junitCheckSelector: getJunitCheckLabel(junitCheckLabelValue, ctx.Canary.Name, ctx.Namespace),
	}
	pod.Namespace = ctx.Namespace
	pod.Name = ctx.Canary.Name + "-" + strings.ToLower(rand.String(5))
	if err := json.Unmarshal(check.Spec, &pod.Spec); err != nil {
		return nil, err
	}
	// pod.Spec = check.Spec
	for _, container := range pod.Spec.Containers {
		if len(container.Command) > 0 {
			// attempt to wrap the command so that it always completes, allowing for access to junit results
			container.Args = []string{fmt.Sprintf(`
			set -e
			EXIT_CODE=0
			%s %s || EXIT_CODE=$?
			echo "Completed with exit code of $EXIT_CODE"
			echo $EXIT_CODE > %s/exit-code
			exit 0
			`, strings.Join(container.Command, " "), strings.Join(container.Args, " "), filepath.Dir(check.TestResults))}
			container.Command = []string{"bash", "-c"}
		}
		pod.Spec.InitContainers = append(pod.Spec.InitContainers, container)
	}
	pod.Spec.Containers = []corev1.Container{
		{
			Name:  containerName,
			Image: containerImage,
			Args: []string{
				"bash",
				"-c",
				fmt.Sprintf(`
				function wait() {
					until [ -f %s/done ]
					do
							echo "Waiting for done"
							find %s
							sleep 1
					done
				}
				export -f wait
				timeout 60s bash -c wait
				exit 0
				`, mountPath, mountPath),
			},
		}}
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
	pod.Spec.RestartPolicy = corev1.RestartPolicyNever
	//
	pod.Spec.InitContainers[0].VolumeMounts = append(pod.Spec.InitContainers[0].VolumeMounts, corev1.VolumeMount{Name: volumeName, MountPath: filepath.Dir(check.TestResults)})
	pod.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{{Name: volumeName, MountPath: mountPath}}
	return pod, nil
}

func deletePod(ctx *context.Context, pod *corev1.Pod) {
	if ctx.Canary.Annotations["skipDelete"] == "true" { // nolint: goconst
		return
	}
	if err := ctx.Kommons().DeleteByKind(podKind, pod.Namespace, pod.Name); err != nil {
		ctx.Warnf("failed to delete pod %s/%s: %v", pod.Namespace, pod.Name, err)
	}
}

func getLogs(ctx *context.Context, pod corev1.Pod) string {
	message, _ := ctx.Kommons().GetPodLogs(pod.Namespace, pod.Name, pod.Spec.InitContainers[0].Name)
	if !ctx.IsTrace() && !ctx.IsDebug() && len(message) > 3000 {
		message = message[len(message)-3000:]
	}
	return message
}

func podExecf(ctx *context.Context, pod corev1.Pod, results pkg.Results, cmd string, args ...interface{}) (string, bool) {
	if !ctx.IsTrace() {
		ctx.Kommons().Logger.SetLogLevel(0)
	}

	_cmd := fmt.Sprintf(cmd, args...)
	stdout, stderr, err := ctx.Kommons().ExecutePodf(pod.Namespace, pod.Name, containerName, "bash", "-c", _cmd)
	if stderr != "" || err != nil {
		podFail(ctx, pod, results.Failf("error running %s: %v %v %v", _cmd, stdout, stderr, err))
		return "", false
	}
	return strings.TrimSpace(stdout), true
}

func podFail(ctx *context.Context, pod corev1.Pod, results pkg.Results) pkg.Results {
	return results.ErrorMessage(fmt.Errorf("%s is %s\n %v", pod.Name, pod.Status.Phase, getLogs(ctx, pod)))
}

func cleanupExistingPods(ctx *context.Context, k8s kubernetes.Interface, selector string) (bool, error) {
	pods := k8s.CoreV1().Pods(ctx.Namespace)
	existingPods, err := pods.List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return false, err
	}

	ctx.Debugf("found %d pods for %s", len(existingPods.Items), selector)
	skip := len(existingPods.Items) > 1 // allow up to 1 duplicate running container
	for _, junitPod := range existingPods.Items {
		_junitPod := junitPod
		nextRuntime, err := getNextRuntime(ctx.Canary, junitPod.CreationTimestamp.Local())
		if err != nil {
			return false, err
		}

		if time.Now().After((*nextRuntime)) {
			defer deletePod(ctx, &_junitPod)
			ctx.Warnf("stale pod found: %s, created=%s", junitPod.Name, time.Since(junitPod.GetCreationTimestamp().Local()))
			skip = true
		}
	}
	if skip {
		ctx.Debugf("%d existing pods, skipping", len(existingPods.Items))
	}
	return skip, err
}

func (c *JunitChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.JunitCheck)
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	if ctx.Kommons() == nil {
		return results.Failf("Kubernetes is not initialized")
	}

	k8s := ctx.Kubernetes()
	timeout := time.Duration(check.GetTimeout()) * time.Minute
	pod, err := newPod(ctx, check)
	if err != nil {
		return results.ErrorMessage(err)
	}
	pods := k8s.CoreV1().Pods(ctx.Namespace)

	if skip, err := cleanupExistingPods(ctx, k8s, fmt.Sprintf("%s=%s", junitCheckSelector, pod.Labels[junitCheckSelector])); err != nil {
		return results.ErrorMessage(err)
	} else if skip {
		return nil
	}

	if _, err := k8s.CoreV1().Pods(ctx.Namespace).Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		return results.ErrorMessage(err)
	}

	defer deletePod(ctx, pod)

	ctx.Tracef("[%s/%s] waiting for tests to complete", ctx.Namespace, ctx.Canary.Name)
	if ctx.IsTrace() {
		go func() {
			if err := ctx.Kommons().StreamLogsV2(ctx.Namespace, pod.Name, timeout, pod.Spec.InitContainers[0].Name); err != nil {
				ctx.Error(err, "error streaming")
			}
		}()
	}

	if err := ctx.Kommons().WaitForPod(ctx.Namespace, pod.Name, timeout, corev1.PodRunning, corev1.PodSucceeded, corev1.PodFailed); err != nil {
		result.ErrorMessage(err)
	}

	podObj, err := pods.Get(ctx, pod.Name, metav1.GetOptions{})
	if err != nil {
		return results.ErrorMessage(err)
	}

	if !kommons.IsPodHealthy(*podObj) {
		return podFail(ctx, *pod, results)
	}

	var suites JunitTestSuites
	exitCode, _ := podExecf(ctx, *pod, results, "cat %v/exit-code", mountPath)

	if exitCode != "" && exitCode != "0" {
		// we don't exit early as junit files may have been generated in addition to a failing exit code
		result.Failf("process exited with: %s:\n%s", exitCode, getLogs(ctx, *pod))
	}
	files, ok := podExecf(ctx, *pod, results, fmt.Sprintf("find %v -name \\*.xml -type f", mountPath))
	if !ok {
		return results
	}
	files = strings.TrimSpace(files)
	if files == "" && exitCode != "" && exitCode != "0" {
		return results.Failf("No junit files found")
	}
	for _, file := range strings.Split(files, "\n") {
		output, ok := podExecf(ctx, *pod, results, "cat %v", file)
		if !ok {
			return results
		}
		if suites, err = suites.Ingest(output); err != nil {
			return results.ErrorMessage(err)
		}
	}

	// signal container to exit
	_, _ = podExecf(ctx, *pod, results, "touch %s/done", mountPath)
	result.AddDetails(suites)
	result.Duration = int64(suites.Duration * 1000)
	if check.Test.IsEmpty() && suites.Failed > 0 {
		if check.Display.IsEmpty() {
			return results.Failf(suites.Totals.String())
		}
		return results.Failf("")
	}

	for _, artifactConfig := range check.Artifacts {
		paths := utils.UnfoldGlobs(artifactConfig.Path)
		for _, path := range paths {
			file, err := os.Open(path)
			if err != nil {
				ctx.Error(err, "error opening file. path=%s", path)
				continue
			}

			result.Artifacts = append(result.Artifacts, artifacts.Artifact{
				Path:    path,
				Content: file,
			})
		}
	}

	return results
}

func getJunitCheckLabel(label, name, namespace string) string {
	return fmt.Sprintf("%v-%v-%v", label, name, namespace)
}
