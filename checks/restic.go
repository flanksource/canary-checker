package checks

import (
	"encoding/json"
	"fmt"
	osExec "os/exec"
	"time"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/kommons"

	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/exec"
)

func init() {
	//register metrics here
}

const (
	resticPasswordEnvKey       = "RESTIC_PASSWORD"
	resticAwsAccessKeyIDEnvKey = "AWS_ACCESS_KEY_ID"
	resticAwsSecretAccessKey   = "AWS_SECRET_ACCESS_KEY"
)

type ResticChecker struct {
}

func (c *ResticChecker) Type() string {
	return "restic"
}

func (c *ResticChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.Restic {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

func (c *ResticChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.ResticCheck)
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)
	envVars, err := c.getEnvVars(check, ctx.Canary.Namespace, ctx.Kommons)
	if err != nil {
		return results.Failf("error getting envVars %v", err)
	}
	if check.CheckIntegrity {
		if err := checkIntegrity(check.Repository, check.CaCert, envVars); err != nil {
			return results.Failf("integrity check failed %v", err)
		}
	}
	if err := checkBackupFreshness(check.Repository, check.MaxAge, check.CaCert, envVars); err != nil {
		return results.Failf("backup freshness check failed: %v", err)
	}
	return results
}

func checkIntegrity(repository, caCert string, envVars map[string]string) error {
	resticCmd := ""
	if caCert != "" {
		resticCmd = fmt.Sprintf("restic -r %[1]s --cacert %[2]s dump --no-lock -q latest $(restic -r %[1]s --cacert %[2]s --no-lock ls -q latest | awk 'FNR==1') 1> /dev/null", repository, caCert)
	} else {
		resticCmd = fmt.Sprintf("restic -r %[1]s dump --no-lock -q latest $(restic -r %[1]s --no-lock ls -q latest | awk 'FNR==1') 1> /dev/null", repository)
	}
	return exec.ExecfWithEnv(resticCmd, envVars)
}

func checkBackupFreshness(repository, maxAge, caCert string, envVars map[string]string) error {
	envString := ""
	resticCmd := ""
	for k, v := range envVars {
		envString += fmt.Sprintf("%s=%s ", k, v)
	}
	if caCert != "" {
		resticCmd = fmt.Sprintf("%[1]v restic --cacert %[3]s -r %[2]s --no-lock cat snapshot -q $(%[1]v restic --cacert %[3]s -q --no-lock -r %[2]s snapshots latest  | awk 'FNR == 3 {print $1}')", envString, repository, caCert)
	} else {
		resticCmd = fmt.Sprintf("%[1]v restic -r %[2]s --no-lock cat snapshot -q $(%[1]v restic --no-lock -r %[2]s -q snapshots latest | awk 'FNR == 3 {print $1}')", envString, repository)
	}
	cmd := osExec.Command("bash", "-c", resticCmd)
	latestSnapshotInfo, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error checking backup freshness : %v", string(latestSnapshotInfo))
	}
	var parsedLatestSnapshotInfo = make(map[string]interface{})
	if err := json.Unmarshal(latestSnapshotInfo, &parsedLatestSnapshotInfo); err != nil {
		return err
	}
	timeCreated := parsedLatestSnapshotInfo["time"]
	backupTime, _ := time.Parse(time.RFC3339, timeCreated.(string))
	backupDuration := time.Since(backupTime)
	maxAllowedBackupDuration, err := time.ParseDuration(maxAge)
	if err != nil {
		return fmt.Errorf("error parsing the max age: %v", err)
	}
	if backupDuration > maxAllowedBackupDuration {
		return fmt.Errorf("backup is %s older than allowd maxAge for backup", (backupDuration - maxAllowedBackupDuration).String())
	}
	return nil
}

func (c *ResticChecker) getEnvVars(r v1.ResticCheck, namespace string, kommons *kommons.Client) (map[string]string, error) {
	var password, secretKey, accessKey string
	var err error
	_, password, err = kommons.GetEnvValue(*r.Password, namespace)
	if err != nil {
		return nil, err
	}
	if r.SecretKey != nil {
		_, secretKey, err = kommons.GetEnvValue(*r.SecretKey, namespace)
		if err != nil {
			return nil, err
		}
	}
	if r.AccessKey != nil {
		_, accessKey, err = kommons.GetEnvValue(*r.AccessKey, namespace)
		if err != nil {
			return nil, err
		}
	}
	return map[string]string{
		resticPasswordEnvKey:       password,
		resticAwsSecretAccessKey:   secretKey,
		resticAwsAccessKeyIDEnvKey: accessKey,
	}, nil
}
