package topology

import (
	"fmt"
	"reflect"
	"sync"

	canaryCtx "github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	pkgTopology "github.com/flanksource/canary-checker/pkg/topology"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/job"
	"github.com/flanksource/duty/models"
	"github.com/robfig/cron/v3"
)

var TopologyScheduler = cron.New()

var topologyJobs sync.Map

func newTopologyJob(ctx context.Context, topology pkg.Topology) {
	id := topology.ID.String()
	v1, err := topology.ToV1()
	if err != nil {
		logger.Errorf("[%s] failed to parse topology spec: %v", err)
		return
	}
	tj := pkgTopology.TopologyJob{
		Topology:  *v1,
		Namespace: topology.Namespace,
	}
	if v1.Spec.Schedule == "" {
		v1.Spec.Schedule = db.DefaultTopologySchedule
	}
	j := &job.Job{
		Name:         "Topology",
		Context:      canaryCtx.DefaultContext.WithObject(v1.ObjectMeta).WithTopology(*v1),
		Schedule:     v1.Spec.Schedule,
		Singleton:    true,
		JobHistory:   true,
		Retention:    job.RetentionFew,
		ResourceID:   id,
		ResourceType: "topology",
		RunNow:       ctx.Properties().On("topology.runNow"),
		ID:           fmt.Sprintf("%s/%s", topology.Namespace, topology.Name),
		Fn:           tj.Run,
	}

	topologyJobs.Store(topology.ID.String(), j)
	if err := j.AddToScheduler(TopologyScheduler); err != nil {
		logger.Errorf("[%s] failed to schedule %v", j.Name, err)
	}
}

var SyncTopology = &job.Job{
	Name:       "SyncTopology",
	Schedule:   "@every 5m",
	JobHistory: true,
	Singleton:  true,
	RunNow:     true,
	Fn: func(ctx job.JobRuntime) error {
		var topologies []pkg.Topology
		if err := ctx.DB().Table("topologies").Where(duty.LocalFilter).Where("source != ?", models.SourcePush).
			Find(&topologies).Error; err != nil {
			return err
		}

		for _, topology := range topologies {
			if err := SyncTopologyJob(ctx.Context, topology); err != nil {
				ctx.History.AddError(err.Error())
			} else {
				ctx.History.IncrSuccess()
			}
		}
		return nil
	},
}

func SyncTopologyJob(ctx context.Context, t pkg.Topology) error {
	id := t.ID.String()
	var existingJob *job.Job
	if j, ok := topologyJobs.Load(id); ok {
		existingJob = j.(*job.Job)
	}

	v1Topology, err := t.ToV1()
	if err != nil {
		return err
	}

	if t.DeletedAt != nil || v1Topology.Spec.Schedule == "@never" {
		existingJob.Unschedule()
		topologyJobs.Delete(id)
		return nil
	}

	if existingJob == nil {
		newTopologyJob(ctx, t)
		return nil
	}

	existingTopology := existingJob.Context.Value("topology")
	if existingTopology != nil && !reflect.DeepEqual(existingTopology.(v1.Topology).Spec, v1Topology.Spec) {
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

var CleanupDeletedTopologyComponents = &job.Job{
	Name:       "CleanupComponents",
	Schedule:   "@every 1h",
	Singleton:  true,
	JobHistory: true,
	Retention:  job.RetentionBalanced,
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
				ctx.History.IncrSuccess()
			}
			DeleteTopologyJob(r.ID)
		}
		return nil
	},
}
