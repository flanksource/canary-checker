package topology

import (
	"fmt"
	"reflect"
	"sync"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	pkgTopology "github.com/flanksource/canary-checker/pkg/topology"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/job"
	"github.com/robfig/cron/v3"
)

var TopologyScheduler = cron.New()

var topologyJobs sync.Map

func newTopologyJob(ctx context.Context, topology v1.Topology) {
	j := &job.Job{
		Name:       "TopologyRun",
		Context:    ctx.WithObject(topology.ObjectMeta),
		Schedule:   topology.Spec.Schedule,
		JobHistory: true,
		Retention:  job.RetentionHour,
		ID:         topology.GetPersistedID(),
		Fn: func(ctx job.JobRuntime) error {
			ctx.History.ResourceID = topology.GetPersistedID()
			ctx.History.ResourceType = "topology"
			opts := pkgTopology.TopologyRunOptions{
				Context:   ctx.Context,
				Depth:     10,
				Namespace: topology.Namespace,
			}
			count, err := pkgTopology.SyncComponents(opts, topology)
			ctx.History.SuccessCount = count
			return err
		},
	}

	topologyJobs.Store(topology.GetPersistedID(), j)
	if err := j.AddToScheduler(TopologyScheduler); err != nil {
		logger.Errorf("[%s] failed to schedule %v", *j, err)
	}
}

var SyncTopology = &job.Job{
	Name:      "SyncTopology",
	Schedule:  "@every 5m",
	Singleton: true,
	RunNow:    true,
	Fn: func(ctx job.JobRuntime) error {
		var topologies []pkg.Topology

		if err := ctx.DB().Table("topologies").Where(duty.LocalFilter).
			Find(&topologies).Error; err != nil {
			return err
		}

		for _, topology := range topologies {
			if err := SyncTopologyJob(ctx.Context, topology.ToV1()); err != nil {
				ctx.History.AddError(err.Error())
			} else {
				ctx.History.IncrSuccess()
			}
		}
		return nil
	},
}

func SyncTopologyJob(ctx context.Context, t v1.Topology) error {
	id := t.GetPersistedID()
	var existingJob *job.Job
	if j, ok := topologyJobs.Load(id); ok {
		existingJob = j.(*job.Job)
	}
	if !t.DeletionTimestamp.IsZero() || t.Spec.GetSchedule() == "@never" {
		existingJob.Unschedule()
		topologyJobs.Delete(id)
		return nil
	}

	if existingJob == nil {
		newTopologyJob(ctx, t)
		return nil
	}

	existingTopology := existingJob.Context.Value("topology")
	if existingTopology != nil && !reflect.DeepEqual(existingTopology.(v1.Topology).Spec, t.Spec) {
		ctx.Debugf("Rescheduling %s topology with updated specs", t)
		existingJob.Unschedule()
		newTopologyJob(ctx, t)
	}
	return nil
}

func DeleteTopologyJob(id string) {
	if j, ok := topologyJobs.Load(id); ok {
		existingJob := j.(*job.Job)
		existingJob.Unschedule()
		topologyJobs.Delete(id)
	}
}

var CleanupComponents = &job.Job{
	Name:       "CleanupComponents",
	Schedule:   "@every 1h",
	Singleton:  true,
	JobHistory: true,
	Retention:  job.RetentionDay,
	Fn: func(ctx job.JobRuntime) error {
		var rows []struct {
			ID string
		}
		// Select all components whose topology ID is deleted but their deleted at is not marked
		if err := ctx.DB().Raw(`
        SELECT DISTINCT(topologies.id) FROM topologies
        INNER JOIN components ON topologies.id = components.topology_id
        WHERE
            components.deleted_at IS NULL AND
            topologies.deleted_at IS NOT NULL
        `).Scan(&rows).Error; err != nil {
			return err
		}

		for _, r := range rows {
			if err := db.DeleteComponentsOfTopology(ctx.DB(), r.ID); err != nil {
				ctx.History.AddError(fmt.Sprintf("Error deleting components for topology[%s]: %v", r.ID, err))
			} else {
				DeleteTopologyJob(r.ID)
				ctx.History.IncrSuccess()
			}
		}
		return nil
	},
}
