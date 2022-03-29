package checks

import (
	"errors"
	"fmt"
	"strings"
	"time"

	sql "google.golang.org/genproto/googleapis/cloud/sql/v1beta4"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/clients/gcp"
)

func GCPDatabaseBackupCheck(ctx *context.Context, check v1.DatabaseBackupCheck) pkg.Results {
	databaseScanObjectCount.WithLabelValues(check.GCP.Project, check.GCP.Instance).Inc()
	result := pkg.Success(check, ctx.Canary)
	var results pkg.Results
	results = append(results, result)

	svc, err := gcp.NewSQLAdmin(ctx, *check.GCP.GCPConnection)
	if err != nil {
		databaseScanFailCount.WithLabelValues(check.GCP.Project, check.GCP.Instance).Inc()
		return results.ErrorMessage(err)
	}
	// Only checking one backup for now, but setting up the logic that this could maybe be configurable.
	// Would need some extra parsing on the age to select latest
	backupList, err := svc.BackupRuns.List(check.GCP.Project, check.GCP.Instance).MaxResults(1).Do()
	if err != nil {
		databaseScanFailCount.WithLabelValues(check.GCP.Project, check.GCP.Instance).Inc()
		return results.ErrorMessage(err)
	}
	var errorMessages []string
	for _, backup := range backupList.Items {
		if !(backup.Status == sql.SqlBackupRunStatus_SUCCESSFUL.String() || backup.Status == sql.SqlBackupRunStatus_RUNNING.String() || backup.Status == sql.SqlBackupRunStatus_ENQUEUED.String()) {
			errorMessages = append(errorMessages, fmt.Sprintf("Backup %d has status %s with error %s", backup.Id, backup.Status, backup.Error.Message))
		}
	}
	if check.MaxAge > 0 {
		for _, backup := range backupList.Items {
			var checkTime time.Time
			var checkString string
			parseFail := false
			// Ideally running for too long and being enqueued for too long would have stricter age restrictions, but that might make the checks too complicated
			// This handles the most recent valid timestamp that each state would present
			if backup.Status == sql.SqlBackupRunStatus_RUNNING.String() {
				checkTime, err = time.Parse(time.RFC3339, backup.StartTime)
				if err != nil {
					errorMessages = append(errorMessages, "Could not parse backup start time")
					parseFail = true
				}
				checkString = "started"
			} else if backup.Status == sql.SqlBackupRunStatus_ENQUEUED.String() {
				checkTime, err = time.Parse(time.RFC3339, backup.EnqueuedTime)
				if err != nil {
					errorMessages = append(errorMessages, "Could not parse backup enqueued time")
					parseFail = true
				}
				checkString = "enqueued"
			} else {
				checkTime, err = time.Parse(time.RFC3339, backup.EndTime)
				if err != nil {
					errorMessages = append(errorMessages, "Could not parse backup end time")
					parseFail = true
				}
				checkString = "ended"
			}
			if !parseFail {
				if checkTime.Add(time.Duration(check.MaxAge) * time.Minute).After(time.Now()) {
					errorMessages = append(errorMessages, fmt.Sprintf("Most recent backup run too old - %s at %s", checkString, checkTime.String()))
				}
			}
		}
	}
	if len(errorMessages) > 0 {
		databaseScanFailCount.WithLabelValues(check.GCP.Project, check.GCP.Instance).Inc()
		return results.ErrorMessage(errors.New(strings.Join(errorMessages, ", ")))
	}
	return results
}
