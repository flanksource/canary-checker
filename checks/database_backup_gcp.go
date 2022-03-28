package checks

import (
	"errors"
	"fmt"
	"strings"
	"time"

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
		if backup.Status != "SUCCESSFUL" {
			errorMessages = append(errorMessages, backup.Error.Message)
		}
	}
	if check.MaxAge > 0 {
		for _, backup := range backupList.Items {
			endTime, err := time.Parse(time.RFC3339, backup.EndTime)
			if err != nil {
				errorMessages = append(errorMessages, "Could not parse backup end time")
			} else {
				if endTime.Add(time.Duration(check.MaxAge) * time.Minute).After(time.Now()) {
					errorMessages = append(errorMessages, fmt.Sprintf("Most recent backup run too old - ended at %s", backup.EndTime))
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
