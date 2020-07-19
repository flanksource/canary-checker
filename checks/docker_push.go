package checks

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

type DockerPushChecker struct{}

func (c *DockerPushChecker) Run(config v1.CanarySpec) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range config.DockerPush {
		results = append(results, c.Check(conf))
	}
	return results
}

// Type: returns checker type
func (c *DockerPushChecker) Type() string {
	return "docker-push"
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *DockerPushChecker) Check(check v1.DockerPushCheck) *pkg.CheckResult {
	start := time.Now()
	ctx := context.Background()
	authConfig := types.AuthConfig{
		Username: check.Username,
		Password: check.Password,
	}
	encodedJSON, _ := json.Marshal(authConfig)
	authStr := base64.URLEncoding.EncodeToString(encodedJSON)
	out, err := dockerClient.ImagePush(ctx, check.Image, types.ImagePushOptions{RegistryAuth: authStr})
	if err != nil {
		return Failf(check, "Failed to push image: %s", err)
	}

	buf := new(bytes.Buffer)
	defer out.Close()
	_, _ = buf.ReadFrom(out)
	lines := strings.Split(buf.String(), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		decodedResponse := struct {
			Status string `json:"status"`
			Error  string `json:"error"`
		}{}
		err = json.Unmarshal([]byte(line), &decodedResponse)
		if err != nil {
			return Failf(check, "Invalid response: %v: %s", err, line)
		}
		if decodedResponse.Error != "" {
			return Failf(check, "Failed to push %v", decodedResponse.Error)
		}
	}

	return &pkg.CheckResult{
		Check:    check,
		Pass:     true,
		Duration: time.Since(start).Milliseconds(),
		Metrics:  []pkg.Metric{},
	}
}
