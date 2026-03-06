package topology

import "github.com/flanksource/duty/job"

var CleanupJobs = []*job.Job{
	CleanupSoftDeletedComponents,
	CleanupCanaries,
	CleanupChecks,
	CleanupMetricsGauges,
}
