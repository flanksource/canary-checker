package jobs

import (
	"github.com/flanksource/canary-checker/api/context"

	"github.com/flanksource/canary-checker/pkg/db"
	canaryJobs "github.com/flanksource/canary-checker/pkg/jobs/canary"
	topologyJobs "github.com/flanksource/canary-checker/pkg/jobs/topology"
	"github.com/flanksource/canary-checker/pkg/topology"
	"github.com/flanksource/commons/logger"
	dutyEcho "github.com/flanksource/duty/echo"
	"github.com/flanksource/duty/job"
	dutyQuery "github.com/flanksource/duty/query"
	"github.com/robfig/cron/v3"
)

var FuncScheduler = cron.New()

func Start() {
	logger.Infof("Starting jobs ...")
	dutyEcho.RegisterCron(FuncScheduler)
	if canaryJobs.UpstreamConf.Valid() {
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

	for _, j := range []*job.Job{topologyJobs.CleanupDeletedTopologyComponents, topologyJobs.SyncTopology, canaryJobs.SyncCanaryJobs, canaryJobs.CleanupDeletedCanaryChecks, dutyQuery.SyncComponentCacheJob} {
		var job = j
		job.Context = context.DefaultContext
		if err := job.AddToScheduler(FuncScheduler); err != nil {
			logger.Errorf(err.Error())
		}
	}
}
