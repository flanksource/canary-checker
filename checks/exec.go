package checks

import (
	"bytes"
	"errors"
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
		if check.Connections.AWS == nil {
			return []*pkg.CheckResult{result.Failf("no AWS connection provided")}
		}

		configPath, err := createAWSCredentialsFile(ctx, check.Connections.AWS)
		defer os.RemoveAll(configPath)
		if err != nil {
			return []*pkg.CheckResult{result.Failf("failed to store AWS credentials: %v", err.Error())}
		}

		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, fmt.Sprintf("AWS_SHARED_CREDENTIALS_FILE=%s", configPath))

	case "az":

	case "gcloud":
		if check.Connections.GCP == nil {
			return []*pkg.CheckResult{result.Failf("no GCP connection provided")}
		}

		configPath, err := createGCloudCredentialsFile(ctx, check.Connections.GCP)
		defer os.RemoveAll(configPath)
		if err != nil {
			return []*pkg.CheckResult{result.Failf("failed to store gcloud credentials: %v", err.Error())}
		}

		// to configure gcloud CLI to use the service account specified in GOOGLE_APPLICATION_CREDENTIALS,
		// we need to explicitly activate it
		runCmd := osExec.Command("gcloud", "auth", "activate-service-account", "--key-file", configPath)
		if err := runCmd.Run(); err != nil {
			return []*pkg.CheckResult{result.Failf("failed to activate GCP service account: %v", err.Error())}
		}

		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, fmt.Sprintf("GOOGLE_APPLICATION_CREDENTIALS=%s", configPath))
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

func createGCloudCredentialsFile(ctx *context.Context, conn *v1.GCPConnection) (string, error) {
	if err := conn.HydrateConnection(ctx); err != nil {
		return "", err
	}

	if conn.Credentials.ValueStatic == "" {
		return "", errors.New("gcp credentials is empty")
	}

	return saveConfig("gcloud-*", gcloudConfigTemplate, conn)
}

func createAWSCredentialsFile(ctx *context.Context, conn *v1.AWSConnection) (string, error) {
	if err := conn.Populate(ctx, ctx.Kubernetes, ctx.Namespace); err != nil {
		return "", err
	}

	return saveConfig("aws-*", awsConfigTemplate, conn)
}

func saveConfig(dirPrefix string, configTemplate *textTemplate.Template, view any) (string, error) {
	dirPath, err := os.MkdirTemp("", dirPrefix)
	if err != nil {
		return "", err
	}

	configPath := fmt.Sprintf("%s/credentials", dirPath)
	logger.Tracef("Creating credentials file: %s", configPath)

	file, err := os.Create(configPath)
	if err != nil {
		return configPath, err
	}
	defer file.Close()

	if err := configTemplate.Execute(file, view); err != nil {
		return configPath, err
	}

	return configPath, nil
}

var (
	awsConfigTemplate    *textTemplate.Template
	gcloudConfigTemplate *textTemplate.Template
)

func init() {
	awsConfigTemplate = textTemplate.Must(textTemplate.New("").Parse(`[default]
aws_access_key_id = {{.AccessKey}}
aws_secret_access_key = {{.SecretKey}}
{{if .Region}}region = {{.Region}}{{end}}`))

	gcloudConfigTemplate = textTemplate.Must(textTemplate.New("").Parse(`{{.Credentials}}`))
}
