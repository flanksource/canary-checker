package checks

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/api/context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"k8s.io/apimachinery/pkg/util/rand"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
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

type JunitChecker struct {
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

func newPod(ctx *context.Context, check v1.JunitCheck) *corev1.Pod {
	pod := &corev1.Pod{}
	pod.APIVersion = corev1.SchemeGroupVersion.Version
	pod.Kind = podKind
	pod.Labels = map[string]string{
		junitCheckSelector: getJunitCheckLabel(junitCheckLabelValue, ctx.Canary.Name, ctx.Namespace),
	}
	pod.Namespace = ctx.Namespace
	pod.Name = ctx.Canary.Name + "-" + strings.ToLower(rand.String(5))
	pod.Spec = check.Spec
	pod.Spec.InitContainers = pod.Spec.Containers
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
	pod.Spec.Volumes = []corev1.Volume{
		{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}
	pod.Spec.RestartPolicy = corev1.RestartPolicyNever
	pod.Spec.InitContainers[0].VolumeMounts = []corev1.VolumeMount{{Name: volumeName, MountPath: filepath.Dir(check.TestResults)}}
	pod.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{{Name: volumeName, MountPath: mountPath}}
	return pod
}

func deletePod(ctx *context.Context, pod *corev1.Pod) {
	if err := ctx.Kommons.DeleteByKind(podKind, pod.Namespace, pod.Name); err != nil {
		logger.Warnf("failed to delete pod %s/%s: %v", pod.Namespace, pod.Name, err)
	}
}

func podExecf(ctx *context.Context, pod corev1.Pod, result *pkg.CheckResult, cmd string, args ...interface{}) (string, bool) {
	_cmd := fmt.Sprintf(cmd, args...)
	stdout, stderr, err := ctx.Kommons.ExecutePodf(pod.Namespace, pod.Name, containerName, "bash", "-c", _cmd)
	if stderr != "" || err != nil {
		podFail(ctx, pod, result.Failf("error running %s: %v %v %v", _cmd, stdout, stderr, err))
		return "", false
	}
	return stdout, true
}

func podFail(ctx *context.Context, pod corev1.Pod, result *pkg.CheckResult) *pkg.CheckResult {
	message, _ := ctx.Kommons.GetPodLogs(pod.Namespace, pod.Name, pod.Spec.InitContainers[0].Name)
	if len(message) > 3000 {
		message = message[len(message)-3000:]
	}
	return result.ErrorMessage(fmt.Errorf("pod is not healthy: \n %v", message))
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
		nextRuntime, err := getNextRuntime(ctx.Canary, junitPod.CreationTimestamp.Time)
		if err != nil {
			return false, err
		}
		if time.Now().After(*nextRuntime) {
			defer deletePod(ctx, &junitPod)
			ctx.Warnf("stale pod found: %s, created=%s", junitPod.Name, time.Since(junitPod.GetCreationTimestamp().Time))
			skip = true
		}
	}
	if skip {
		logger.Debugf("%s has %d existing pods, skipping", ctx.Canary.Name, len(existingPods.Items))
	}
	return skip, err
}

func (c *JunitChecker) Check(ctx *context.Context, extConfig external.Check) *pkg.CheckResult {
	junitCheck := extConfig.(v1.JunitCheck)

	result := pkg.Success(junitCheck)
	k8s, err := ctx.Kommons.GetClientset()
	if err != nil {
		return result.ErrorMessage(err)
	}

	timeout := junitCheck.GetTimeout()
	pod := newPod(ctx, junitCheck)
	pods := k8s.CoreV1().Pods(ctx.Namespace)

	if skip, err := cleanupExistingPods(ctx, k8s, fmt.Sprintf("%s=%s", junitCheckSelector, pod.Labels[junitCheckSelector])); err != nil {
		return result.ErrorMessage(err)
	} else if skip {
		return nil
	}

	if err := ctx.Kommons.Apply(ctx.Namespace, pod); err != nil {
		return result.ErrorMessage(err)
	}

	defer deletePod(ctx, pod)

	logger.Tracef("[%s/%s] waiting for tests to complete", ctx.Namespace, ctx.Canary.Name)
	if ctx.IsTrace() {
		go func() {
			_ = ctx.Kommons.StreamLogs(ctx.Namespace, pod.Name)
		}()
	}

	if err := ctx.Kommons.WaitForPod(ctx.Namespace, pod.Name, time.Duration(timeout)*time.Minute, corev1.PodRunning, corev1.PodSucceeded, corev1.PodFailed); err != nil {
		result.ErrorMessage(err)
	}

	podObj, err := pods.Get(ctx, pod.Name, metav1.GetOptions{})
	if err != nil {
		return result.ErrorMessage(err)
	}

	if !kommons.IsPodHealthy(*podObj) {
		return podFail(ctx, *pod, result)
	}

	logger.Tracef("[%s/%s] pod is %s", ctx, &podObj.Status.Phase)

	var suites JunitTestSuites
	files, ok := podExecf(ctx, *pod, result, fmt.Sprintf("find %v -name \\*.xml -type f", mountPath))
	if !ok {
		return result
	}
	files = strings.TrimSpace(files)
	if files == "" {
		return result.Failf("No junit files found")
	}
	for _, file := range strings.Split(files, "\n") {
		output, ok := podExecf(ctx, *pod, result, "cat %v", file)
		if !ok {
			return result
		}
		if suites, err = suites.Ingest(output); err != nil {
			return result.ErrorMessage(err)
		}
	}
	// signal container to exit
	_, _ = podExecf(ctx, *pod, result, "touch %s/done", mountPath)
	result.AddDetails(suites)
	totals := suites.Aggregate()
	result.Duration = int64(totals.Duration * 1000)
	if totals.Failed > 0 {
		return result.Failf(totals.String())
	}
	return result
}

func getJunitCheckLabel(label, name, namespace string) string {
	return fmt.Sprintf("%v-%v-%v", label, name, namespace)
}
