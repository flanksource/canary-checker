package controllers

import (
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg/topology"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/kommons"
	"github.com/robfig/cron/v3"
)

var Kommons *kommons.Client

var FuncScheduler = cron.New()

func Start() {
	SystemScheduler.Start()
	CanaryScheduler.Start()
	FuncScheduler.Start()
	if _, err := ScheduleFunc(v1.SyncCanaryJobsSchedule, SyncCanaryJobs); err != nil {
		logger.Errorf("Failed to schedule sync jobs for canary: %v", err)
	}
	if _, err := ScheduleFunc(v1.SyncSystemsJobsSchedule, SyncSystemsJobs); err != nil {
		logger.Errorf("Failed to schedule sync jobs for systems: %v", err)
	}
	if _, err := ScheduleFunc(v1.ComponentRunSchedule, topology.ComponentRun); err != nil {
		logger.Errorf("Failed to schedule component run: %v", err)
	}
}

func ScheduleFunc(schedule string, fn func()) (interface{}, error) {
	return FuncScheduler.AddFunc(schedule, fn)
}
