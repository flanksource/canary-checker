package checks

import (
	osExec "os/exec"
	"runtime"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

func init() {
	//register metrics here
}

type ExecChecker struct {
}

func (c *ExecChecker) Type() string {
	return "exec"
}

func (c *ExecChecker) Run(ctx *context.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range ctx.Canary.Spec.Exec {
		results = append(results, c.Check(ctx, conf))
	}
	return results
}

func (c *ExecChecker) Check(ctx *context.Context, extConfig external.Check) *pkg.CheckResult {
	check := extConfig.(v1.ExecCheck)
	switch runtime.GOOS {
	case "windows":
		return execPowershell(check, ctx)
	default:
		return execBash(check, ctx)
	}
}

func execPowershell(check v1.ExecCheck, ctx *context.Context) *pkg.CheckResult {
	result := pkg.Success(check, ctx.Canary)
	ps, err := osExec.LookPath("powershell.exe")
	if err != nil {
		result.Failf("powershell not found")
	}
	args := []string{*check.Script}
	cmd := osExec.Command(ps, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return result.Failf("error executing the command: %v", err)
	}
	result.AddDetails(string(output))
	return result
}

func execBash(check v1.ExecCheck, ctx *context.Context) *pkg.CheckResult {
	result := pkg.Success(check, ctx.Canary)
	cmd := osExec.Command("bash", "-c", *check.Script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return result.Failf("error executing the command: %v", err)
	}
	result.AddDetails(string(output))
	return result
}
