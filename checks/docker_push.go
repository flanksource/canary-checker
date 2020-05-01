package checks

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

var (
	imagePushFailed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "canary_check_docker_push_failed",
		Help: "The total number of docker image push failed",
	})
	imagePushTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "canary_check_image_push_time",
			Help:    "Image push time",
			Buckets: []float64{100, 500, 1000, 5000, 15000, 30000},
		},
		[]string{"image"},
	)
)

func init() {
	prometheus.MustRegister(imagePushFailed, imagePushTime)
}

type DockerPushChecker struct{}

func (c *DockerPushChecker) Run(config pkg.Config, results chan *pkg.CheckResult) {
	for _, conf := range config.DockerPush {
		results <- c.Check(conf.DockerPushCheck)
	}
}

// Type: returns checker type
func (c *DockerPushChecker) Type() string {
	return "docker-push"
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *DockerPushChecker) Check(check pkg.DockerPushCheck) *pkg.CheckResult {
	start := time.Now()
	pushSuccess := true
	message := ""
	ctx := context.Background()
	authConfig := types.AuthConfig{
		Username: check.Username,
		Password: check.Password,
	}
	encodedJSON, _ := json.Marshal(authConfig)
	authStr := base64.URLEncoding.EncodeToString(encodedJSON)
	out, err := dockerClient.ImagePush(ctx, check.Image, types.ImagePushOptions{RegistryAuth: authStr})
	elapsed := time.Since(start)
	if err != nil {
		log.Printf("Failed to push image: %s", err)
		imagePushFailed.Inc()
		pushSuccess = false
		message = fmt.Sprintf("Failed to push image: %s", err)
	}

	buf := new(bytes.Buffer)
	defer out.Close()
	_, _ = buf.ReadFrom(out)
	lines := strings.Split(buf.String(), "\n")
	for _, line := range lines {
		decodedResponse := struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		}{}
		err = json.Unmarshal([]byte(line), &decodedResponse)
		log.Debugf("docker push output: %s", line)
		if decodedResponse.Error != "" {
			imagePullFailed.Inc()
			pushSuccess = false
			message = decodedResponse.Error
			break
		}
	}

	if pushSuccess {
		message = fmt.Sprintf("Image %s successfully pushed", check.Image)
	}

	imagePushTime.WithLabelValues(check.Image).Observe(float64(elapsed.Milliseconds()))
	return &pkg.CheckResult{
		Check:    check,
		Pass:     pushSuccess,
		Duration: elapsed.Milliseconds(),
		Message:  message,
		Endpoint: check.Image,
		Metrics:  []pkg.Metric{},
	}
}
