package checks

import (
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

var (
	databaseScanObjectCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "canary_check_database_backup_scan_count",
			Help: "The total number of objects",
		},
		[]string{"project", "instance"},
	)
	databaseScanFailCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "canary_check_database_backup_fail_count",
			Help: "The number of failed backups detected",
		},
		[]string{"project", "instance"},
	)
)

func init() {
	prometheus.MustRegister(databaseScanObjectCount, databaseScanFailCount)
}

type DatabaseBackupChecker struct {
}

func (c *DatabaseBackupChecker) Type() string {
	return "databasebackup"
}

func (c *DatabaseBackupChecker) Run(ctx *context.Context) pkg.Results {
	var results pkg.Results
	for _, conf := range ctx.Canary.Spec.DatabaseBackup {
		results = append(results, c.Check(ctx, conf)...)
	}
	return results
}

func (c *DatabaseBackupChecker) Check(ctx *context.Context, extConfig external.Check) pkg.Results {
	check := extConfig.(v1.DatabaseBackupCheck)
	switch {
	case check.GCP != nil:
		return GCPDatabaseBackupCheck(ctx, check)
	default:
		return FailDatabaseBackupParse(ctx, check)
	}
}

func FailDatabaseBackupParse(ctx *context.Context, check v1.DatabaseBackupCheck) pkg.Results {
	result := pkg.Fail(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)
	return results.ErrorMessage(errors.New("Could not parse databaseBackup input"))
}
