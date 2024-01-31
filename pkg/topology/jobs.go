package topology

import "github.com/flanksource/duty/job"

var Jobs = []*job.Job{
	ComponentConfigRun,
	ComponentCheckRun,
	CleanupSoftDeletedComponents,
	CleanupCanaries,
	CleanupChecks,
	CleanupMetricsGauges,
	ComponentCostRun,
	ComponentRelationshipSync,
	ComponentStatusSummarySync,
}
