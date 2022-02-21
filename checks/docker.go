package checks

import (
	"bytes"

	"github.com/flanksource/canary-checker/api/context"

	"encoding/base64"
	"encoding/json"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/flanksource/canary-checker/api/external"
	"github.com/prometheus/client_golang/prometheus"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

var (
	dockerClient *client.Client

	size = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "canary_check_image_size",
			Help: "Size of docker image",
		},
		[]string{"image"},
	)

	imagePullTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "canary_check_image_pull_time",
			Help:    "Image pull time",
			Buckets: []float64{100, 500, 1000, 5000, 15000, 30000},
		},
		[]string{"image"},
	)
)

func init() {
	var err error
	dockerClient, err = client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}
	prometheus.MustRegister(size, imagePullTime)
}

type DockerPullChecker struct {
}

func (c *DockerPullChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.DockerPull {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

// Type: returns checker type
func (c *DockerPullChecker) Type() string {
	return "dockerPull"
}

func getDigestFromOutput(out io.ReadCloser) string {
	buf := new(bytes.Buffer)
	defer out.Close()
	_, _ = buf.ReadFrom(out)
	for _, line := range strings.Split(buf.String(), "\n") {
		var status = make(map[string]string)
		_ = json.Unmarshal([]byte(line), &status)

		if strings.HasPrefix(status["status"], "Digest:") {
			return status["status"][len("Digest: "):]
		}
	}
	return ""
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *DockerPullChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.DockerPullCheck)
	namespace := ctx.Canary.Namespace
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)
	var authStr string
	auth, err := GetAuthValues(check.Auth, ctx.Kommons, namespace)
	if err != nil {
		return results.Failf("failed to fetch auth details: %v", err)
	}
	if auth != nil {
		authConfig := types.AuthConfig{
			Username: auth.GetUsername(),
			Password: auth.GetPassword(),
		}
		encodedJSON, _ := json.Marshal(authConfig)
		authStr = base64.URLEncoding.EncodeToString(encodedJSON)
	}
	out, err := dockerClient.ImagePull(ctx, check.Image, types.ImagePullOptions{RegistryAuth: authStr})
	if err != nil {
		return results.Failf("Failed to pull image: %s", err)
	}
	digest := getDigestFromOutput(out)
	if digest != check.ExpectedDigest {
		return results.Failf("digests do not match %s != %s", digest, check.ExpectedDigest)
	}

	inspect, _, _ := dockerClient.ImageInspectWithRaw(ctx, check.Image)
	if check.ExpectedSize > 0 && inspect.Size != check.ExpectedSize {
		return results.Failf("size does not match: %d != %d", inspect.Size, check.ExpectedSize)
	}

	return results
}
