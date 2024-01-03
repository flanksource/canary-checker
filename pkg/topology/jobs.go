package topology

import "github.com/flanksource/duty/job"

var Jobs = []*job.Job{
	ComponentConfigRun,
	ComponentCheckRun,
	CleanupComponents,
	CleanupCanaries,
	CleanupChecks,
	CleanupMetricsGauges,
	ComponentCostRun,
	ComponentRelationshipSync,
	ComponentStatusSummarySync,
}
