package jobs

import (
	"time"

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

const (
	PullCanaryFromUpstreamSchedule     = "@every 30s"
	PushCanaryToUpstreamSchedule       = "@every 10s"
	ReconcileCanaryToUpstreamSchedule  = "@every 3h"
	SyncCanaryJobsSchedule             = "@every 2m"
	SyncSystemsJobsSchedule            = "@every 5m"
	ComponentRunSchedule               = "@every 2m"
	ComponentStatusSummarySyncSchedule = "@every 1m"
	ComponentCheckSchedule             = "@every 2m"
	ComponentConfigSchedule            = "@every 2m"
	ComponentCostSchedule              = "@every 1h"
	CheckStatusSummarySchedule         = "@every 1m"
	CheckStatusesAggregate1hSchedule   = "@every 1h"
	CheckStatusesAggregate1dSchedule   = "@every 24h"
	CheckStatusDeleteSchedule          = "@every 24h"
	CheckCleanupSchedule               = "@every 12h"
	CanaryCleanupSchedule              = "@every 12h"
	PrometheusGaugeCleanupSchedule     = "@every 1h"

	ReconcileDeletedTopologyComponentsSchedule = "@every 1h"
)

func Start() {
	logger.Infof("Starting jobs ...")

	systemJobs.TopologyScheduler.Start()
	canaryJobs.CanaryScheduler.Start()
	FuncScheduler.Start()

	if canaryJobs.UpstreamConf.Valid() {
		pushJob := &canaryJobs.UpstreamPushJob{MaxAge: time.Minute * 5}
		pushJob.Run()

		pullJob := &canaryJobs.UpstreamPullJob{}
		pullJob.Run()

		if _, err := FuncScheduler.AddJob(PullCanaryFromUpstreamSchedule, pullJob); err != nil {
			logger.Fatalf("Failed to schedule job [canaryJobs.Pull]: %v", err)
		}

		if _, err := FuncScheduler.AddJob(PushCanaryToUpstreamSchedule, pushJob); err != nil {
			logger.Fatalf("Failed to schedule job [canaryJobs.UpstreamPushJob]: %v", err)
		}

		if _, err := ScheduleFunc(ReconcileCanaryToUpstreamSchedule, canaryJobs.ReconcileCanaryResults); err != nil {
			logger.Fatalf("Failed to schedule job [canaryJobs.SyncWithUpstream]: %v", err)
		}
	}

	if _, err := ScheduleFunc(SyncCanaryJobsSchedule, canaryJobs.SyncCanaryJobs); err != nil {
		logger.Errorf("Failed to schedule sync jobs for canary: %v", err)
	}
	if _, err := ScheduleFunc(SyncSystemsJobsSchedule, systemJobs.SyncTopologyJobs); err != nil {
		logger.Errorf("Failed to schedule sync jobs for systems: %v", err)
	}
	if _, err := ScheduleFunc(ComponentRunSchedule, topology.ComponentRun); err != nil {
		logger.Errorf("Failed to schedule component run: %v", err)
	}
	if _, err := ScheduleFunc(ComponentStatusSummarySyncSchedule, topology.ComponentStatusSummarySync); err != nil {
		logger.Errorf("Failed to schedule component status summary sync: %v", err)
	}
	if _, err := ScheduleFunc(ComponentCostSchedule, topology.ComponentCostRun); err != nil {
		logger.Errorf("Failed to schedule component cost sync: %v", err)
	}
	if _, err := ScheduleFunc(ComponentCheckSchedule, checks.ComponentCheckRun); err != nil {
		logger.Errorf("Failed to schedule component check: %v", err)
	}
	if _, err := ScheduleFunc(ComponentConfigSchedule, configs.ComponentConfigRun); err != nil {
		logger.Errorf("Failed to schedule component config: %v", err)
	}
	if _, err := ScheduleFunc(CheckStatusSummarySchedule, db.RefreshCheckStatusSummary); err != nil {
		logger.Errorf("Failed to schedule check status summary refresh: %v", err)
	}
	if _, err := ScheduleFunc(CheckStatusDeleteSchedule, db.DeleteAllOldCheckStatuses); err != nil {
		logger.Errorf("Failed to schedule check status deleter: %v", err)
	}
	if _, err := ScheduleFunc(CheckStatusesAggregate1hSchedule, db.AggregateCheckStatuses1h); err != nil {
		logger.Errorf("Failed to schedule check statuses aggregator 1h: %v", err)
	}
	if _, err := ScheduleFunc(CheckStatusesAggregate1dSchedule, db.AggregateCheckStatuses1d); err != nil {
		logger.Errorf("Failed to schedule check statuses aggregator 1d: %v", err)
	}
	if _, err := ScheduleFunc(PrometheusGaugeCleanupSchedule, canaryJobs.CleanupMetricsGauges); err != nil {
		logger.Errorf("Failed to schedule prometheus gauge cleanup job: %v", err)
	}
	if _, err := ScheduleFunc(CheckCleanupSchedule, db.CleanupChecks); err != nil {
		logger.Errorf("Failed to schedule check cleanup job: %v", err)
	}
	if _, err := ScheduleFunc(CanaryCleanupSchedule, db.CleanupCanaries); err != nil {
		logger.Errorf("Failed to schedule canary cleanup job: %v", err)
	}
	if _, err := ScheduleFunc(ReconcileDeletedTopologyComponentsSchedule, systemJobs.ReconcileDeletedTopologyComponents); err != nil {
		logger.Errorf("Failed to schedule ReconcileDeletedTopologyComponents: %v", err)
	}
	if _, err := ScheduleFunc("@every 5m", canaryJobs.ReconcileCanaryChecks); err != nil {
		logger.Errorf("Failed to schedule ReconcileCanaryChecks: %v", err)
	}

	canaryJobs.ReconcileCanaryChecks()
	canaryJobs.CleanupMetricsGauges()
	canaryJobs.SyncCanaryJobs()
	systemJobs.SyncTopologyJobs()
}

func ScheduleFunc(schedule string, fn func()) (interface{}, error) {
	return FuncScheduler.AddFunc(schedule, fn)
}
