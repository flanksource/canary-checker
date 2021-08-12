package checks

import (
	"encoding/json"
	"fmt"
	osExec "os/exec"
	"time"

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
	kommons *kommons.Client `yaml:"-" json:"-"`
}

func (c *ResticChecker) SetClient(client *kommons.Client) {
	c.kommons = client
}
func (c *ResticChecker) Type() string {
	return "restic"
}

func (c *ResticChecker) Run(canary v1.Canary) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	for _, conf := range canary.Spec.Restic {
		results = append(results, c.Check(canary, conf))
	}
	return results
}

func (c *ResticChecker) Check(canary v1.Canary, extConfig external.Check) *pkg.CheckResult {
	start := time.Now()
	resticCheck := extConfig.(v1.ResticCheck)
	envVars, err := c.getEnvVars(resticCheck, canary.Namespace)
	if err != nil {
		return Failf(resticCheck, "error getting envVars %v", err)
	}
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

func (c *ResticChecker) getEnvVars(r v1.ResticCheck, namespace string) (map[string]string, error) {
	var password, secretKey, accessKey string
	var err error
	_, password, err = c.kommons.GetEnvValue(*r.Password, namespace)
	if err != nil {
		return nil, err
	}
	if r.SecretKey != nil {
		_, secretKey, err = c.kommons.GetEnvValue(*r.SecretKey, namespace)
		if err != nil {
			return nil, err
		}
	}
	if r.AccessKey != nil {
		_, accessKey, err = c.kommons.GetEnvValue(*r.AccessKey, namespace)
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
