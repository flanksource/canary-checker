package checks

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
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
	"github.com/hashicorp/go-getter"
)

type ExecChecker struct {
}

type ExecDetails struct {
	Cmd      *exec.Cmd `json:"-"`
	Stdout   string    `json:"stdout"`
	Stderr   string    `json:"stderr"`
	ExitCode int       `json:"exitCode"`
	Error    error     `json:"-"`
}

func (e ExecDetails) String() string {
	return fmt.Sprintf("%s %s exit=%d %s %s", e.Cmd.Path, e.Cmd.Args, e.ExitCode, e.Stdout, e.Stderr)
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
		var err error
		var connection *models.Connection
		if connection, err = ctx.HydrateConnectionByURL(check.Checkout.Connection); err != nil {
			return nil, fmt.Errorf("error hydrating connection: %w", err)
		} else if connection == nil {
			connection = &models.Connection{Type: models.ConnectionTypeGit}
			if err != nil {
				return nil, fmt.Errorf("error getting go getter URL: %w", err)
			}
		}

		if connection, err = connection.Merge(ctx, check.Checkout); err != nil {
			return nil, err
		}
		var goGetterURL string
		if goGetterURL, err = connection.AsGoGetterURL(); err != nil {
			return nil, err
		}

		if goGetterURL == "" {
			return nil, fmt.Errorf("missing URL %v", *connection)
		}

		result.mountPoint = check.Checkout.Destination
		if result.mountPoint == "" {
			pwd, _ := os.Getwd()
			result.mountPoint = filepath.Join(pwd, ".downloads", hash.Sha256Hex(goGetterURL))
		}

		if err := checkout(ctx, goGetterURL, result.mountPoint); err != nil {
			return nil, fmt.Errorf("error checking out: %w", err)
		}
	}

	return &result, nil
}

func (c *ExecChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.ExecCheck)

	env, err := c.prepareEnvironment(ctx, check)
	if err != nil {
		return pkg.New(check, ctx.Canary).AddDetails(ExecDetails{}).Invalidf(err.Error())
	}

	switch runtime.GOOS {
	case "windows":
		return execPowershell(ctx, check, env)
	default:
		return execBash(ctx, check, env)
	}
}

func execPowershell(ctx *context.Context, check v1.ExecCheck, envParams *execEnv) pkg.Results {
	result := pkg.Success(check, ctx.Canary).AddDetails(ExecDetails{ExitCode: -1})
	ps, err := exec.LookPath("powershell.exe")
	if err != nil {
		result.Failf("powershell not found")
	}

	args := []string{check.Script}
	cmd := exec.CommandContext(ctx, ps, args...)
	if len(envParams.envs) != 0 {
		cmd.Env = append(os.Environ(), envParams.envs...)
	}
	if envParams.mountPoint != "" {
		cmd.Dir = envParams.mountPoint
	}

	return checkCmd(ctx, cmd, result)
}

func execBash(ctx *context.Context, check v1.ExecCheck, envParams *execEnv) pkg.Results {
	result := pkg.Success(check, ctx.Canary).AddDetails(ExecDetails{ExitCode: -1})
	fields := strings.Fields(check.Script)
	if len(fields) == 0 {
		return result.Invalidf("no script provided")
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", check.Script)
	if len(envParams.envs) != 0 {
		cmd.Env = append(os.Environ(), envParams.envs...)
	}
	if envParams.mountPoint != "" {
		cmd.Dir = envParams.mountPoint
	}

	if err := setupConnection(ctx, check, cmd); err != nil {
		return result.Invalidf("failed to setup connection: %v", err)
	}

	return checkCmd(ctx, cmd, result)
}

func setupConnection(ctx *context.Context, check v1.ExecCheck, cmd *exec.Cmd) error {
	var envPreps []models.EnvPrep

	if check.Connections.AWS != nil {
		if err := check.Connections.AWS.Populate(ctx); err != nil {
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

func checkCmd(ctx *context.Context, cmd *exec.Cmd, result *pkg.CheckResult) (results pkg.Results) {
	details := runCmd(ctx, cmd)
	result.AddDetails(details)
	if details.ExitCode != 0 {
		return result.Failf(details.String()).ToSlice()
	}

	results = append(results, result)
	return results
}

func run(ctx *context.Context, cwd string, name string, args ...string) ExecDetails {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = cwd
	return runCmd(ctx, cmd)
}

func runCmd(ctx *context.Context, cmd *exec.Cmd) ExecDetails {
	result := ExecDetails{}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	result.Cmd = cmd
	if ctx.IsTrace() {
		ctx.Infof("%s %s", cmd.Path, cmd.Args)
		cmd.Stderr = io.MultiWriter(&stderr, os.Stderr)
		cmd.Stdout = io.MultiWriter(&stdout, os.Stdout)
	}

	result.Error = cmd.Run()
	result.ExitCode = cmd.ProcessState.ExitCode()
	result.Stderr = strings.TrimSpace(stderr.String())
	result.Stdout = strings.TrimSpace(stdout.String())

	if ctx.IsTrace() {
		ctx.Infof(result.String())
	}

	return result
}

// Getter gets a directory or file using the Hashicorp go-getter library
// See https://github.com/hashicorp/go-getter
func checkout(ctx *context.Context, url, dst string) error {
	pwd, _ := os.Getwd()

	stashed := false
	if files.Exists(dst + "/.git") {
		if r := run(ctx, dst, "git", "status", "-s"); r.Stdout != "" {
			if r2 := run(ctx, dst, "git", "stash"); r2.Error != nil {
				return r2.Error
			}
			stashed = true
		}
	}
	client := &getter.Client{
		Ctx:     ctx,
		Src:     url,
		Dst:     dst,
		Pwd:     pwd,
		Mode:    getter.ClientModeDir,
		Options: []getter.ClientOption{},
	}
	if ctx.IsDebug() {
		ctx.Infof("Downloading %s -> %s", v1.SanitizeEndpoints(url), dst)
	}
	if err := client.Get(); err != nil {
		return err
	}
	if ctx.IsTraceEnabled() {
		ctx.Infof("Downloaded %s -> %s", v1.SanitizeEndpoints(url), dst)
	}
	if stashed {
		if r := run(ctx, dst, "git", "stash", "pop"); r.Error != nil {
			return fmt.Errorf("failed to pop: %v", r.Error)
		}
	}
	return nil
}
