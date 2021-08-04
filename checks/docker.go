package checks

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/flanksource/kommons"

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
	kommons *kommons.Client `yaml:"-" json:"-"`
}

func (c *DockerPullChecker) SetClient(client *kommons.Client) {
	c.kommons = client
}

func (c *DockerPullChecker) Run(config v1.CanarySpec) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range config.DockerPull {
		results = append(results, c.Check(conf))
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
func (c *DockerPullChecker) Check(extConfig external.Check) *pkg.CheckResult {
	check := extConfig.(v1.DockerPullCheck)
	start := time.Now()
	ctx := context.Background()
	var username, password string
	var err error
	namespace := check.GetNamespace()
	if check.Auth != nil {
		_, username, err = c.kommons.GetEnvValue(check.Auth.Username, namespace)
		if err != nil {
			return Failf(check, "failed to fetch username from envVar: %v", err)
		}
		_, password, err = c.kommons.GetEnvValue(check.Auth.Password, namespace)
		if err != nil {
			return Failf(check, "failed to fetch password from envVar: %v", err)
		}
	}
	authConfig := types.AuthConfig{
		Username: username,
		Password: password,
	}
	encodedJSON, _ := json.Marshal(authConfig)
	authStr := base64.URLEncoding.EncodeToString(encodedJSON)
	out, err := dockerClient.ImagePull(ctx, check.Image, types.ImagePullOptions{RegistryAuth: authStr})
	elapsed := time.Since(start)
	if err != nil {
		return Failf(check, "Failed to pull image: %s", err)
	}
	digest := getDigestFromOutput(out)
	if digest != check.ExpectedDigest {
		return Failf(check, "digests do not match %s != %s", digest, check.ExpectedDigest)
	}

	inspect, _, _ := dockerClient.ImageInspectWithRaw(ctx, check.Image)
	if check.ExpectedSize > 0 && inspect.Size != check.ExpectedSize {
		return Failf(check, "size does not match: %d != %d", inspect.Size, check.ExpectedSize)
	}

	return &pkg.CheckResult{
		Check:    check,
		Pass:     true,
		Duration: elapsed.Milliseconds(),
	}
}
