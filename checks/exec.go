package checks

import (
	"bytes"
	"fmt"
	"os"
	osExec "os/exec"
	"runtime"
	"strings"
	textTemplate "text/template"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
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

func (c *ExecChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.Exec {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

func (c *ExecChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.ExecCheck)
	switch runtime.GOOS {
	case "windows":
		return execPowershell(check, ctx)
	default:
		return execBash(check, ctx)
	}
}

func execPowershell(check v1.ExecCheck, ctx *context.Context) pkg.Results {
	result := pkg.Success(check, ctx.Canary)
	ps, err := osExec.LookPath("powershell.exe")
	if err != nil {
		result.Failf("powershell not found")
	}
	args := []string{check.Script}
	cmd := osExec.Command(ps, args...)
	return runCmd(cmd, result)
}

func execBash(check v1.ExecCheck, ctx *context.Context) pkg.Results {
	result := pkg.Success(check, ctx.Canary)
	fields := strings.Fields(check.Script)
	if len(fields) == 0 {
		return []*pkg.CheckResult{result.Failf("no script provided")}
	}

	cmd := osExec.Command("bash", "-c", check.Script)

	// Setup connection details
	switch strings.ToLower(fields[0]) {
	case "aws":
		configPath, err := createAWSCredentialsFile(ctx, check.Connections.AWS)
		defer os.RemoveAll(configPath)
		if err != nil {
			return []*pkg.CheckResult{result.Failf(err.Error())}
		}

		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, fmt.Sprintf("AWS_SHARED_CREDENTIALS_FILE=%s", configPath))

	case "az":

	case "gcloud":
	}

	return runCmd(cmd, result)
}

func runCmd(cmd *osExec.Cmd, result *pkg.CheckResult) (results pkg.Results) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	_ = cmd.Run()
	details := ExecDetails{
		Stdout:   strings.TrimSpace(stdout.String()),
		Stderr:   strings.TrimSpace(stderr.String()),
		ExitCode: cmd.ProcessState.ExitCode(),
	}
	result.AddDetails(details)
	if details.ExitCode != 0 {
		return result.Failf("non-zero exit-code: %d: %s %s", details.ExitCode, details.Stdout, details.Stderr).ToSlice()
	}
	results = append(results, result)
	return results
}

func createAWSCredentialsFile(ctx *context.Context, conn *v1.AWSConnection) (string, error) {
	if err := conn.Populate(ctx, ctx.Kubernetes, ctx.Namespace); err != nil {
		return "", err
	}

	dirPath, err := os.MkdirTemp("", "aws-*")
	if err != nil {
		return "", err
	}

	configPath := fmt.Sprintf("%s/credentials", dirPath)
	logger.Tracef("Creating AWS credentials file: %s", configPath)

	file, err := os.Create(configPath)
	if err != nil {
		return configPath, err
	}
	defer file.Close()

	if err := awsConfigTemplate.Execute(file, conn); err != nil {
		return configPath, err
	}

	return configPath, nil
}

var (
	awsConfigTemplate *textTemplate.Template
)

func init() {
	const (
		awsConfigTpl = `[default]
aws_access_key_id = {{.AccessKey}}
aws_secret_access_key = {{.SecretKey}}
{{if .Region}}region = {{.Region}}{{end}}`
	)

	awsConfigTemplate = textTemplate.Must(textTemplate.New("").Parse(awsConfigTpl))
}
