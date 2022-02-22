package checks

import (
	"bytes"

	"github.com/flanksource/canary-checker/api/context"

	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

type DockerPushChecker struct {
}

func (c *DockerPushChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.DockerPush {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

// Type: returns checker type
func (c *DockerPushChecker) Type() string {
	return "dockerPush"
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *DockerPushChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.DockerPushCheck)
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)
	namespace := ctx.Canary.Namespace
	var err error
	auth, err := GetAuthValues(check.Auth, ctx.Kommons, namespace)
	if err != nil {
		return results.Failf("failed to fetch auth details: %v", err)
	}
	authConfig := types.AuthConfig{
		Username: auth.Username.Value,
		Password: auth.Password.Value,
	}
	encodedJSON, _ := json.Marshal(authConfig)
	authStr := base64.URLEncoding.EncodeToString(encodedJSON)
	out, err := dockerClient.ImagePush(ctx, check.Image, types.ImagePushOptions{RegistryAuth: authStr})
	if err != nil {
		return results.Failf("Failed to push image: %v", err)
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
			return results.Failf("Invalid response: %v: %s", err, line)
		}
		if decodedResponse.Error != "" {
			return results.Failf("Failed to push %v", decodedResponse.Error)
		}
	}

	return results
}
