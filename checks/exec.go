package checks

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	osExec "os/exec"
	"path/filepath"
	"runtime"
	"strings"
	textTemplate "text/template"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/types"
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
	for i, env := range check.EnvVars {
		val, err := ctx.GetEnvValueFromCache(env)
		if err != nil {
			return []*pkg.CheckResult{pkg.Fail(check, ctx.Canary).Failf("error fetching env value (name=%s): %v", env.Name, err)}
		}

		check.EnvVars[i].ValueStatic = val
	}

	switch runtime.GOOS {
	case "windows":
		return execPowershell(ctx, check)
	default:
		return execBash(ctx, check)
	}
}

func execPowershell(ctx *context.Context, check v1.ExecCheck) pkg.Results {
	result := pkg.Success(check, ctx.Canary)
	ps, err := osExec.LookPath("powershell.exe")
	if err != nil {
		result.Failf("powershell not found")
	}

	args := []string{check.Script}
	cmd := osExec.CommandContext(ctx, ps, args...)
	cmd.Env = append(os.Environ(), envVarSlice(check.EnvVars)...)
	return runCmd(cmd, result)
}

func execBash(ctx *context.Context, check v1.ExecCheck) pkg.Results {
	result := pkg.Success(check, ctx.Canary)
	fields := strings.Fields(check.Script)
	if len(fields) == 0 {
		return []*pkg.CheckResult{result.Failf("no script provided")}
	}

	cmd := osExec.CommandContext(ctx, "bash", "-c", check.Script)
	cmd.Env = append(os.Environ(), envVarSlice(check.EnvVars)...)
	if err := setupConnection(ctx, check, cmd); err != nil {
		return []*pkg.CheckResult{result.Failf("failed to setup connection: %v", err)}
	}

	return runCmd(cmd, result)
}

func setupConnection(ctx *context.Context, check v1.ExecCheck, cmd *osExec.Cmd) error {
	if check.Connections.AWS != nil {
		if err := check.Connections.AWS.Populate(ctx, ctx.Kubernetes, ctx.Namespace); err != nil {
			return fmt.Errorf("failed to hydrate aws connection: %w", err)
		}

		configPath, err := saveConfig(awsConfigTemplate, check.Connections.AWS)
		defer os.RemoveAll(filepath.Dir(configPath))
		if err != nil {
			return fmt.Errorf("failed to store AWS credentials: %w", err)
		}

		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, "AWS_EC2_METADATA_DISABLED=true") // https://github.com/aws/aws-cli/issues/5262#issuecomment-705832151
		cmd.Env = append(cmd.Env, fmt.Sprintf("AWS_SHARED_CREDENTIALS_FILE=%s", configPath))
		if check.Connections.AWS.Region != "" {
			cmd.Env = append(cmd.Env, fmt.Sprintf("AWS_DEFAULT_REGION=%s", check.Connections.AWS.Region))
		}
	}

	if check.Connections.Azure != nil {
		if err := check.Connections.Azure.HydrateConnection(ctx); err != nil {
			return fmt.Errorf("failed to hydrate connection %w", err)
		}

		// login with service principal
		runCmd := osExec.Command("az", "login", "--service-principal", "--username", check.Connections.Azure.ClientID.ValueStatic, "--password", check.Connections.Azure.ClientSecret.ValueStatic, "--tenant", check.Connections.Azure.TenantID)
		if err := runCmd.Run(); err != nil {
			return fmt.Errorf("failed to login: %w", err)
		}
	}

	if check.Connections.GCP != nil {
		if err := check.Connections.GCP.HydrateConnection(ctx); err != nil {
			return fmt.Errorf("failed to hydrate connection %w", err)
		}

		configPath, err := saveConfig(gcloudConfigTemplate, check.Connections.GCP)
		defer os.RemoveAll(filepath.Dir(configPath))
		if err != nil {
			return fmt.Errorf("failed to store gcloud credentials: %w", err)
		}

		// to configure gcloud CLI to use the service account specified in GOOGLE_APPLICATION_CREDENTIALS,
		// we need to explicitly activate it
		runCmd := osExec.Command("gcloud", "auth", "activate-service-account", "--key-file", configPath)
		if err := runCmd.Run(); err != nil {
			return fmt.Errorf("failed to activate GCP service account: %w", err)
		}

		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, fmt.Sprintf("GOOGLE_APPLICATION_CREDENTIALS=%s", configPath))
	}

	return nil
}

func runCmd(cmd *osExec.Cmd, result *pkg.CheckResult) (results pkg.Results) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	details := ExecDetails{
		Stdout:   strings.TrimSpace(stdout.String()),
		Stderr:   strings.TrimSpace(stderr.String()),
		ExitCode: cmd.ProcessState.ExitCode(),
	}
	result.AddDetails(details)
	if details.ExitCode != 0 {
		return result.Failf("non-zero exit-code: %d stdout=%s, stderr=%s, error=%v", details.ExitCode, details.Stdout, details.Stderr, err).ToSlice()
	}

	results = append(results, result)
	return results
}

func saveConfig(configTemplate *textTemplate.Template, view any) (string, error) {
	dirPath := filepath.Join(".creds", fmt.Sprintf("cred-%d", rand.Intn(10000000)))
	if err := os.MkdirAll(dirPath, 0700); err != nil {
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

func envVarSlice(envs []types.EnvVar) []string {
	result := make([]string, len(envs))
	for i, env := range envs {
		result[i] = fmt.Sprintf("%s=%s", env.Name, env.ValueStatic)
	}

	return result
}

var (
	awsConfigTemplate    *textTemplate.Template
	gcloudConfigTemplate *textTemplate.Template
)

func init() {
	awsConfigTemplate = textTemplate.Must(textTemplate.New("").Parse(`[default]
aws_access_key_id = {{.AccessKey.ValueStatic}}
aws_secret_access_key = {{.SecretKey.ValueStatic}}
{{if .SessionToken.ValueStatic}}aws_session_token={{.SessionToken.ValueStatic}}{{end}}
`))

	gcloudConfigTemplate = textTemplate.Must(textTemplate.New("").Parse(`{{.Credentials}}`))
}
