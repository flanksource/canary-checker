package controllers

import (
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/kommons"
)

var Kommons *kommons.Client

func Start() {
	SystemScheduler.Start()
	CanaryScheduler.Start()
	if _, err := ScheduleCanaryFunc("@every 120s", SyncCanaryJobs); err != nil {
		logger.Errorf("Failed to schedule sync jobs for canary: %v", err)
	}
	if _, err := ScheduleCanaryFunc("@every 120s", SyncSystemsJobs); err != nil {
		logger.Errorf("Failed to schedule sync jobs for systems: %v", err)
	}
	SyncCanaryJobs()
	SyncSystemsJobs()
}
