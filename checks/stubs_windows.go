//+go:build windows
//+go:build !linux !darwin

package checks

import (
	"errors"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	"github.com/flanksource/canary-checker/pkg"
)

// FIXME: disabling due to the following error
// Error: ../../../go/pkg/mod/github.com/containerd/containerd@v1.4.0/archive/tar_windows.go:234:3: cannot use syscall.NsecToFiletime(hdr.AccessTime.UnixNano()) (type syscall.Filetime) as type "golang.org/x/sys/windows".Filetime in field value
type ContainerdPullChecker struct{}

func (c *ContainerdPullChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	result := pkg.Fail(extConfig, ctx.Canary).ErrorMessage(errors.New("containerd not supported on windows"))
	var results pkg.Results
	results = append(results, result)
	return results
}

func (c *ContainerdPullChecker) Type() string {
	return "containerdPull"
}

func (c *ContainerdPullChecker) Run(ctx *context.Context) pkg.Results {
	return pkg.SetupError(ctx.Canary, errors.New("containerd not supported on windows"))
}
