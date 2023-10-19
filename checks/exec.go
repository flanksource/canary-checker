package checks

import (
	"bytes"
	"fmt"
	"os"
	osExec "os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/files"
	"github.com/flanksource/commons/hash"
	"github.com/flanksource/duty/models"
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

type execEnv struct {
	envs       []string
	mountPoint string
}

func (c *ExecChecker) prepareEnvironment(ctx *context.Context, check v1.ExecCheck) (*execEnv, error) {
	var result execEnv

	for _, env := range check.EnvVars {
		val, err := ctx.GetEnvValueFromCache(env)
		if err != nil {
			return nil, fmt.Errorf("error fetching env value (name=%s): %w", env.Name, err)
		}

		result.envs = append(result.envs, fmt.Sprintf("%s=%s", env.Name, val))
	}

	if check.Checkout != nil {
		sourceURL := check.Checkout.URL

		if connection, err := ctx.HydrateConnectionByURL(check.Checkout.Connection); err != nil {
			return nil, fmt.Errorf("error hydrating connection: %w", err)
		} else if connection != nil {
			goGetterURL, err := connection.AsGoGetterURL()
			if err != nil {
				return nil, fmt.Errorf("error getting go getter URL: %w", err)
			}
			sourceURL = goGetterURL
		}

		if sourceURL == "" {
			return nil, fmt.Errorf("error checking out. missing URL")
		}

		result.mountPoint = check.Checkout.Destination
		if result.mountPoint == "" {
			pwd, _ := os.Getwd()
			result.mountPoint = filepath.Join(pwd, ".downloads", hash.Sha256Hex(sourceURL))
		}

		if err := files.Getter(sourceURL, result.mountPoint); err != nil {
			return nil, fmt.Errorf("error checking out %s: %w", sourceURL, err)
		}
	}

	return &result, nil
}

func (c *ExecChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.ExecCheck)

	env, err := c.prepareEnvironment(ctx, check)
	if err != nil {
		return []*pkg.CheckResult{pkg.Fail(check, ctx.Canary).Failf("something went wrong while preparing exec env: %v", err)}
	}

	switch runtime.GOOS {
	case "windows":
		return execPowershell(ctx, check, env)
	default:
		return execBash(ctx, check, env)
	}
}

func execPowershell(ctx *context.Context, check v1.ExecCheck, envParams *execEnv) pkg.Results {
	result := pkg.Success(check, ctx.Canary)
	ps, err := osExec.LookPath("powershell.exe")
	if err != nil {
		result.Failf("powershell not found")
	}

	args := []string{check.Script}
	cmd := osExec.CommandContext(ctx, ps, args...)
	if len(envParams.envs) != 0 {
		cmd.Env = append(os.Environ(), envParams.envs...)
	}
	if envParams.mountPoint != "" {
		cmd.Dir = envParams.mountPoint
	}

	return runCmd(cmd, result)
}

func execBash(ctx *context.Context, check v1.ExecCheck, envParams *execEnv) pkg.Results {
	result := pkg.Success(check, ctx.Canary)
	fields := strings.Fields(check.Script)
	if len(fields) == 0 {
		return []*pkg.CheckResult{result.Failf("no script provided")}
	}

	cmd := osExec.CommandContext(ctx, "bash", "-c", check.Script)
	if len(envParams.envs) != 0 {
		cmd.Env = append(os.Environ(), envParams.envs...)
	}
	if envParams.mountPoint != "" {
		cmd.Dir = envParams.mountPoint
	}

	if err := setupConnection(ctx, check, cmd); err != nil {
		return []*pkg.CheckResult{result.Failf("failed to setup connection: %v", err)}
	}

	return runCmd(cmd, result)
}

func setupConnection(ctx *context.Context, check v1.ExecCheck, cmd *osExec.Cmd) error {
	var envPreps []models.EnvPrep

	if check.Connections.AWS != nil {
		if err := check.Connections.AWS.Populate(ctx, ctx.Kubernetes, ctx.Namespace); err != nil {
			return fmt.Errorf("failed to hydrate aws connection: %w", err)
		}

		c := models.Connection{
			Type:     models.ConnectionTypeAWS,
			Username: check.Connections.AWS.AccessKey.ValueStatic,
			Password: check.Connections.AWS.SecretKey.ValueStatic,
			Properties: map[string]string{
				"region": check.Connections.AWS.Region,
			},
		}
		envPreps = append(envPreps, c.AsEnv(ctx))
	}

	if check.Connections.Azure != nil {
		if err := check.Connections.Azure.HydrateConnection(ctx); err != nil {
			return fmt.Errorf("failed to hydrate connection %w", err)
		}

		c := models.Connection{
			Type:     models.ConnectionTypeAzure,
			Username: check.Connections.Azure.ClientID.ValueStatic,
			Password: check.Connections.Azure.ClientSecret.ValueStatic,
			Properties: map[string]string{
				"tenant": check.Connections.Azure.TenantID,
			},
		}
		envPreps = append(envPreps, c.AsEnv(ctx))
	}

	if check.Connections.GCP != nil {
		if err := check.Connections.GCP.HydrateConnection(ctx); err != nil {
			return fmt.Errorf("failed to hydrate connection %w", err)
		}

		c := models.Connection{
			Type:        models.ConnectionTypeGCP,
			Certificate: check.Connections.GCP.Credentials.ValueStatic,
			URL:         check.Connections.GCP.Endpoint,
		}
		envPreps = append(envPreps, c.AsEnv(ctx))
	}

	for _, envPrep := range envPreps {
		preRuns, err := envPrep.Inject(ctx, cmd)
		if err != nil {
			return err
		}

		for _, run := range preRuns {
			if err := run.Run(); err != nil {
				return err
			}
		}
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
