package jobs

import (
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg/db"
	canaryJobs "github.com/flanksource/canary-checker/pkg/jobs/canary"
	systemJobs "github.com/flanksource/canary-checker/pkg/jobs/system"
	"github.com/flanksource/canary-checker/pkg/topology"
	"github.com/flanksource/canary-checker/pkg/topology/checks"
	"github.com/flanksource/canary-checker/pkg/topology/configs"
	"github.com/flanksource/commons/logger"
	"github.com/robfig/cron/v3"
)

var FuncScheduler = cron.New()

func Start() {
	logger.Infof("Starting jobs ...")

	systemJobs.SystemScheduler.Start()
	canaryJobs.CanaryScheduler.Start()
	FuncScheduler.Start()

	if _, err := ScheduleFunc(v1.SyncCanaryJobsSchedule, canaryJobs.SyncCanaryJobs); err != nil {
		logger.Errorf("Failed to schedule sync jobs for canary: %v", err)
	}
	if _, err := ScheduleFunc(v1.SyncSystemsJobsSchedule, systemJobs.SyncSystemsJobs); err != nil {
		logger.Errorf("Failed to schedule sync jobs for systems: %v", err)
	}
	if _, err := ScheduleFunc(v1.ComponentRunSchedule, topology.ComponentRun); err != nil {
		logger.Errorf("Failed to schedule component run: %v", err)
	}
	if _, err := ScheduleFunc(v1.ComponentStatusSummarySyncSchedule, topology.ComponentStatusSummarySync); err != nil {
		logger.Errorf("Failed to schedule component status summary sync: %v", err)
	}
	if _, err := ScheduleFunc(v1.ComponentCostSchedule, topology.ComponentCostRun); err != nil {
		logger.Errorf("Failed to schedule component cost sync: %v", err)
	}
	if _, err := ScheduleFunc(v1.ComponentCheckSchedule, checks.ComponentCheckRun); err != nil {
		logger.Errorf("Failed to schedule component check: %v", err)
	}
	if _, err := ScheduleFunc(v1.ComponentConfigSchedule, configs.ComponentConfigRun); err != nil {
		logger.Errorf("Failed to schedule component config: %v", err)
	}
	if _, err := ScheduleFunc(v1.CheckStatusSummarySchedule, db.RefreshCheckStatusSummary); err != nil {
		logger.Errorf("Failed to schedule check status summary refresh: %v", err)
	}
	if _, err := ScheduleFunc(v1.CheckStatusDeleteSchedule, db.DeleteAllOldCheckStatuses); err != nil {
		logger.Errorf("Failed to schedule check status deleter: %v", err)
	}
	if _, err := ScheduleFunc(v1.CheckStatusesAggregate1hSchedule, db.AggregateCheckStatuses1h); err != nil {
		logger.Errorf("Failed to schedule check statuses aggregator 1h: %v", err)
	}
	if _, err := ScheduleFunc(v1.CheckStatusesAggregate1dSchedule, db.AggregateCheckStatuses1d); err != nil {
		logger.Errorf("Failed to schedule check statuses aggregator 1d: %v", err)
	}
	canaryJobs.SyncCanaryJobs()
	systemJobs.SyncSystemsJobs()
}

func ScheduleFunc(schedule string, fn func()) (interface{}, error) {
	return FuncScheduler.AddFunc(schedule, fn)
}
