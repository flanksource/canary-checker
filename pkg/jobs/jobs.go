package jobs

import (
	"github.com/flanksource/canary-checker/api/context"
	"github.com/pkg/errors"

	"github.com/flanksource/canary-checker/pkg/db"
	canaryJobs "github.com/flanksource/canary-checker/pkg/jobs/canary"
	topologyJobs "github.com/flanksource/canary-checker/pkg/jobs/topology"
	"github.com/flanksource/canary-checker/pkg/topology"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/job"
	"github.com/robfig/cron/v3"
)

var FuncScheduler = cron.New()

func Start() error {
	logger.Infof("Starting jobs ...")

	if canaryJobs.UpstreamConf.Valid() {
		// Push checks to upstream in real-time
		if err := canaryJobs.StartUpstreamEventQueueConsumer(context.DefaultContext); err != nil {
			// Cannot continue on failing to start consumers as we may lose events
			return errors.Wrap(err, "Failed to start upstream event queue consumer")
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

	for _, j := range []*job.Job{topologyJobs.CleanupComponents, topologyJobs.SyncTopology, canaryJobs.SyncCanaryJobs} {
		var job = j
		job.Context = context.DefaultContext
		if err := job.AddToScheduler(FuncScheduler); err != nil {
			logger.Errorf(err.Error())
		}
	}
	return nil
}
