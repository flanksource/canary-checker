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
	"github.com/flanksource/canary-checker/pkg/db"
)

var (
	allowedStatus = []string{
		sql.SqlBackupRunStatus_SUCCESSFUL.String(),
		sql.SqlBackupRunStatus_RUNNING.String(),
		sql.SqlBackupRunStatus_ENQUEUED.String(),
	}
)

func GCPDatabaseBackupCheck(ctx *context.Context, check v1.DatabaseBackupCheck) pkg.Results {
	databaseScanObjectCount.WithLabelValues(check.GCP.Project, check.GCP.Instance).Inc()
	result := pkg.Success(check, ctx.Canary)
	result.Start = time.Now()
	var results pkg.Results
	results = append(results, result)

	if err := check.GCP.PopulateFromConnection(ctx, db.Gorm); err != nil {
		return results.Failf("failed to populate GCP connection: %v", err)
	}

	svc, err := gcp.NewSQLAdmin(ctx, check.GCP.GCPConnection)
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
		statusPass := false
		for _, status := range allowedStatus {
			if backup.Status == status {
				statusPass = true
			}
		}
		if !statusPass {
			errorMessages = append(errorMessages, fmt.Sprintf("Backup %d has status %s with error %s", backup.Id, backup.Status, backup.Error.Message))
		}
	}
	if check.MaxAge != "" {
		backup := backupList.Items[0]
		var checkTime time.Time
		var checkString string
		parseFail := false
		// Ideally running for too long and being enqueued for too long would have stricter age restrictions, but that might make the checks too complicated
		// This handles the most recent valid timestamp that each state would present
		if backup.EndTime != "" {
			checkTime, err = time.Parse(time.RFC3339, backup.EndTime)
			if err != nil {
				errorMessages = append(errorMessages, "Could not parse backup end time")
				parseFail = true
			}
			checkString = "ended"
		} else if backup.StartTime != "" {
			checkTime, err = time.Parse(time.RFC3339, backup.StartTime)
			if err != nil {
				errorMessages = append(errorMessages, "Could not parse backup start time")
				parseFail = true
			}
			checkString = "started"
		} else if backup.EnqueuedTime != "" {
			checkTime, err = time.Parse(time.RFC3339, backup.EnqueuedTime)
			if err != nil {
				errorMessages = append(errorMessages, "Could not parse backup enqueued time")
				parseFail = true
			}
			checkString = "enqueued"
		} else {
			errorMessages = append(errorMessages, fmt.Sprintf("BackupRun %d did not contain a time to validate", backup.Id))
			parseFail = true
		}
		if !parseFail {
			maxTime, err := check.MaxAge.GetDuration()
			if err != nil || maxTime == nil {
				errorMessages = append(errorMessages, fmt.Sprintf("Could not parse age string: %s", err))
			} else {
				if checkTime.Add(*maxTime).Before(time.Now()) {
					errorMessages = append(errorMessages, fmt.Sprintf("BackupRun %d too old - %s at %s", backup.Id, checkString, checkTime.String()))
				}
			}
		}
	}
	if len(errorMessages) > 0 {
		databaseScanFailCount.WithLabelValues(check.GCP.Project, check.GCP.Instance).Inc()
		return results.ErrorMessage(errors.New(strings.Join(errorMessages, ", ")))
	}

	backupRaw, err := backupList.Items[0].MarshalJSON()
	if err != nil {
		result.ResultMessage("Could not output raw backup data")
	}
	result.ResultMessage(string(backupRaw))
	return results
}
