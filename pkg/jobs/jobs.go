package jobs

import (
	"github.com/flanksource/canary-checker/api/context"

	"github.com/flanksource/canary-checker/pkg/db"
	canaryJobs "github.com/flanksource/canary-checker/pkg/jobs/canary"
	topologyJobs "github.com/flanksource/canary-checker/pkg/jobs/topology"
	"github.com/flanksource/canary-checker/pkg/topology"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/job"
	"github.com/robfig/cron/v3"
)

var FuncScheduler = cron.New()

const (
	SyncCanaryJobsSchedule = "@every 2m"
)

func Start() {
	logger.Infof("Starting jobs ...")

	topologyJobs.TopologyScheduler.Start()
	canaryJobs.CanaryScheduler.Start()
	FuncScheduler.Start()

	if canaryJobs.UpstreamConf.Valid() {
		// Push checks to upstream in real-time
		if err := canaryJobs.StartUpstreamEventQueueConsumer(&context.DefaultContext); err != nil {
			logger.Fatalf("Failed to start upstream event queue consumer: %v", err)
		}

		for _, j := range canaryJobs.UpstreamJobs {
			var job = j
			job.Context = context.DefaultContext
			if err := job.AddToScheduler(FuncScheduler); err != nil {
				logger.Errorf(err.Error())
			}
		}
	}

	for _, j := range db.CheckStatusJobs {
		var job = j
		job.Context = context.DefaultContext
		if err := job.AddToScheduler(FuncScheduler); err != nil {
			logger.Errorf(err.Error())
		}
	}

	for _, j := range topology.Jobs {
		var job = j
		job.Context = context.DefaultContext
		if err := job.AddToScheduler(FuncScheduler); err != nil {
			logger.Errorf(err.Error())
		}
	}

	for _, j := range []*job.Job{topologyJobs.CleanupComponents} {
		var job = j
		job.Context = context.DefaultContext
		if err := job.AddToScheduler(FuncScheduler); err != nil {
			logger.Errorf(err.Error())
		}
	}

	if err := job.NewJob(context.DefaultContext, "SyncCanaryJobs", "@every 2m", canaryJobs.SyncCanaryJobs).
		RunOnStart().AddToScheduler(FuncScheduler); err != nil {
		logger.Fatalf("Failed to schedule job [canaryJobs.SyncCanaryJobs]: %v", err)
	}
}

func ScheduleFunc(schedule string, fn func()) (interface{}, error) {
	return FuncScheduler.AddFunc(schedule, fn)
}
