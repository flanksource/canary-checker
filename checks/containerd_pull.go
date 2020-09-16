package checks

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/flanksource/canary-checker/api/external"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

var (
	containerdSocket = "/run/containerd/containerd.sock"
)

func init() {
	socket := os.Getenv("CONTAINER_RUNTIME_ENDPOINT")
	if socket != "" {
		containerdSocket = socket
	}
}

type ContainerdPullChecker struct{}

func (c *ContainerdPullChecker) Run(config v1.CanarySpec) []*pkg.CheckResult {

	var results []*pkg.CheckResult
	for _, conf := range config.ContainerdPull {
		results = append(results, c.Check(conf))
	}
	return results
}

// Type: returns checker type
func (c *ContainerdPullChecker) Type() string {
	return "containerdPull"
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *ContainerdPullChecker) Check(extConfig external.Check) *pkg.CheckResult {
	check := extConfig.(v1.ContainerdPullCheck)
	start := time.Now()

	containerdClient, err := containerd.New(containerdSocket)
	if err != nil {
		return Failf(check, err.Error())
	}

	ctx := namespaces.WithNamespace(context.Background(), "default")

	image, err := containerdClient.Pull(ctx, check.Image, containerd.WithPullUnpack)
	elapsed := time.Since(start)
	if err != nil {
		return Failf(check, "Failed to pull image: %s", err)
	}

	digest := fmt.Sprintf("sha256:%s", image.Target().Digest.Hex())
	if digest != check.ExpectedDigest {
		return Failf(check, "digests do not match %s != %s", digest, check.ExpectedDigest)
	}

	size, err := image.Size(ctx)
	if err != nil {
		return Failf(check, "Failed to get image size: %s", err)
	}
	if check.ExpectedSize > 0 && size != check.ExpectedSize {
		return Failf(check, "size does not match: %d != %d", size, check.ExpectedSize)
	}

	return &pkg.CheckResult{
		Check:    check,
		Pass:     true,
		Duration: elapsed.Milliseconds(),
	}
}
