package controllers

import (
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
	if _, err := ScheduleFunc("@every 120s", SyncCanaryJobs); err != nil {
		logger.Errorf("Failed to schedule sync jobs for canary: %v", err)
	}
	if _, err := ScheduleFunc("@every 120s", SyncSystemsJobs); err != nil {
		logger.Errorf("Failed to schedule sync jobs for systems: %v", err)
	}
}

func ScheduleFunc(schedule string, fn func()) (interface{}, error) {
	return FuncScheduler.AddFunc(schedule, fn)
}
