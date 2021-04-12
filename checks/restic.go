package checks

import (
	"encoding/json"
	"fmt"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/exec"
	osExec "os/exec"
	"time"
)

func init() {
	//register metrics here
}

const (
	resticPasswordEnvKey       = "RESTIC_PASSWORD"
	resticAwsAccessKeyIdEnvKey = "AWS_ACCESS_KEY_ID"
	resticAwsSecretAccessKey   = "AWS_SECRET_ACCESS_KEY"
)

type ResticChecker struct{}

func (c *ResticChecker) Type() string {
	return "restic"
}

func (c *ResticChecker) Run(config v1.CanarySpec) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range config.Restic {
		results = append(results, c.Check(conf))
	}
	return results
}

func (c *ResticChecker) Check(extConfig external.Check) *pkg.CheckResult {
	start := time.Now()
	resticCheck := extConfig.(v1.ResticCheck)
	envVars := getEnvVars(resticCheck)
	if resticCheck.CheckIntegrity {
		if err := checkIntegrity(resticCheck.Repository, resticCheck.CaCert, envVars); err != nil {
			return Failf(resticCheck, "integrity check failed %v", err)
		}
	}
	if err := checkBackupFreshness(resticCheck.Repository, resticCheck.MaxAge, resticCheck.CaCert, envVars); err != nil {
		return Failf(resticCheck, "backup freshness check failed: %v", err)
	}
	return Success(resticCheck, start)
}

func checkIntegrity(repository, caCert string, envVars map[string]string) error {
	resticCmd := ""
	if caCert != "" {
		resticCmd = fmt.Sprintf("restic -r %s check --read-data --no-lock -q --cacert %s", repository, caCert)
	} else {
		resticCmd = fmt.Sprintf("restic -r %s check --read-data --no-lock -q", repository)
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

func getEnvVars(r v1.ResticCheck) map[string]string {
	return map[string]string{
		resticPasswordEnvKey:       r.Password,
		resticAwsSecretAccessKey:   r.SecretKey,
		resticAwsAccessKeyIdEnvKey: r.AccessKey,
	}
}
