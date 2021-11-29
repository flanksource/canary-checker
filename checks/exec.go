package checks

import (
	"bytes"
	osExec "os/exec"
	"runtime"
	"strings"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

type ExecChecker struct {
}

type ExecDetails struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
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
	return runCmd(cmd, result)
}

func execBash(check v1.ExecCheck, ctx *context.Context) *pkg.CheckResult {
	result := pkg.Success(check, ctx.Canary)
	cmd := osExec.Command("bash", "-c", *check.Script)
	return runCmd(cmd, result)
}

func runCmd(cmd *osExec.Cmd, result *pkg.CheckResult) *pkg.CheckResult {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	_ = cmd.Run()
	result.AddDetails(ExecDetails{
		Stdout:   strings.TrimSpace(stdout.String()),
		Stderr:   strings.TrimSpace(stderr.String()),
		ExitCode: cmd.ProcessState.ExitCode(),
	})
	return result
}
