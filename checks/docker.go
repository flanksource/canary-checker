package checks

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/flanksource/canary-checker/internal"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/jasonlvhit/gocron"
	"github.com/jinzhu/copier"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

type DockerPullChecker struct{}

// Schedule: Add every check as a cron job, calls MetricProcessor with the set of metrics
func (c *DockerPullChecker) Schedule(config pkg.Config, interval uint64, mp MetricProcessor) {
	for _, conf := range config.DockerPull {
		dockerPullCheck := pkg.DockerPullCheck{}
		if err := copier.Copy(&dockerPullCheck, &conf.DockerPullCheck); err != nil {
			log.Printf("error copying %v", err)
		}
		gocron.Every(interval).Seconds().Do(func() {
			metrics := c.Check(dockerPullCheck)
			mp([]*pkg.CheckResult{metrics})
		})
	}
}

func (c *DockerPullChecker) Run(config pkg.Config) []*pkg.CheckResult {
	var checks []*pkg.CheckResult
	for _, conf := range config.DockerPull {
		result := c.Check(conf.DockerPullCheck)
		checks = append(checks, result)
		fmt.Println(result)
	}
	return checks
}

// Type: returns checker type
func (c *DockerPullChecker) Type() string {
	return "docker-pull"
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *DockerPullChecker) Check(check pkg.DockerPullCheck) *pkg.CheckResult {
	start := time.Now()
	ctx := context.Background()
	digestVerified, sizeVerified := false, false
	authConfig := types.AuthConfig{
		Username: check.Username,
		Password: check.Password,
	}
	encodedJSON, _ := json.Marshal(authConfig)
	authStr := base64.URLEncoding.EncodeToString(encodedJSON)
	out, err := internal.DockerCLI.ImagePull(ctx, check.Image, types.ImagePullOptions{RegistryAuth: authStr})
	if err != nil {
		log.Printf("Failed to pull image: %s", err)
	} else {
		buf := new(bytes.Buffer)
		defer out.Close()
		_, _ = buf.ReadFrom(out)
		if strings.Contains(buf.String(), check.ExpectedDigest) {
			digestVerified = true
		}
	}

	args := filters.NewArgs()
	slice := strings.Split(check.Image, "/")
	args.Add("reference", fmt.Sprintf("*%s", slice[len(slice)-1]))
	images, _ := internal.DockerCLI.ImageList(ctx, types.ImageListOptions{
		Filters: args,
	})
	for _, imageSummary := range images {
		if imageSummary.Size == check.ExpectedSize {
			sizeVerified = true
		}
	}

	elapsed := time.Since(start)
	m := []pkg.Metric{
		{Name: "pull_time", Type: pkg.GaugeType, Value: float64(elapsed.Milliseconds())},
	}
	return &pkg.CheckResult{
		Pass:     digestVerified && sizeVerified,
		Invalid:  !(digestVerified && sizeVerified),
		Duration: elapsed.Milliseconds(),
		Metrics:  m,
	}
}
