//+go:build linux darwin
//+go:build !windows

package checks

import (
	"github.com/flanksource/canary-checker/api/context"

	"fmt"
	"os"

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

func (c *ContainerdPullChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.ContainerdPull {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

// Type: returns checker type
func (c *ContainerdPullChecker) Type() string {
	return "containerdPull"
}

// Run: Check every entry from config according to Checker interface
// Returns check result and metrics
func (c *ContainerdPullChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.ContainerdPullCheck)
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)
	containerdClient, err := containerd.New(containerdSocket)
	if err != nil {
		return results.ErrorMessage(err)
	}

	containerdCtx := namespaces.WithNamespace(ctx, "default")

	image, err := containerdClient.Pull(containerdCtx, check.Image, containerd.WithPullUnpack)
	if err != nil {
		return results.Failf("Failed to pull image: %s", err)
	}

	digest := fmt.Sprintf("sha256:%s", image.Target().Digest.Hex())
	if digest != check.ExpectedDigest {
		return results.Failf("digests do not match %s != %s", digest, check.ExpectedDigest)
	}

	size, err := image.Size(containerdCtx)
	if err != nil {
		return results.Failf("Failed to get image size: %s", err)
	}
	if check.ExpectedSize > 0 && size != check.ExpectedSize {
		return results.Failf("size does not match: %d != %d", size, check.ExpectedSize)
	}

	return results
}
