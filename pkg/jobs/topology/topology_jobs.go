package topology

import (
	"fmt"
	"reflect"

	gocontext "context"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg/db"
	pkgTopology "github.com/flanksource/canary-checker/pkg/topology"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/job"
	"github.com/robfig/cron/v3"
	"k8s.io/apimachinery/pkg/types"
)

var TopologyScheduler = cron.New()

type TopologyJob struct {
	context.Context
	v1.Topology
}

func (job TopologyJob) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: job.Topology.Name, Namespace: job.Topology.Namespace}
}

func (job TopologyJob) Run() {
	opts := pkgTopology.TopologyRunOptions{
		Context:   job.Context.Wrap(gocontext.Background()),
		Depth:     10,
		Namespace: job.Namespace,
	}
	if err := pkgTopology.SyncComponents(opts, job.Topology); err != nil {
		logger.Errorf("failed to run topology %s: %v", job.GetNamespacedName(), err)
	}
}

var SyncTopology = &job.Job{
	Name:      "SyncTopology",
	Schedule:  "@every 5m",
	Singleton: true,
	RunNow:    true,
	Fn: func(ctx job.JobRuntime) error {
		var topologies []v1.Topology

		if err := ctx.DB().Table("topologies").Where(duty.LocalFilter).
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

func SyncTopologyJob(ctx context.Context, t v1.Topology) error {
	if !t.DeletionTimestamp.IsZero() || t.Spec.GetSchedule() == "@never" {
		DeleteTopologyJob(t.GetPersistedID())
		return nil
	}

	entry := findTopologyCronEntry(t.GetPersistedID())
	if entry != nil {
		job := entry.Job.(TopologyJob)
		if !reflect.DeepEqual(job.Topology.Spec, t.Spec) {
			logger.Infof("Rescheduling %s topology with updated specs", t)
			TopologyScheduler.Remove(entry.ID)
		} else {
			return nil
		}
	}
	job := TopologyJob{
		Context:  ctx.Wrap(gocontext.Background()).WithObject(t.ObjectMeta),
		Topology: t,
	}

	_, err := TopologyScheduler.AddJob(t.Spec.GetSchedule(), job)
	if err != nil {
		return fmt.Errorf("failed to schedule topology %s/%s: %v", t.Namespace, t.Name, err)
	} else {
		logger.Infof("Scheduled %s/%s: %s", t.Namespace, t.Name, t.Spec.GetSchedule())
	}

	entry = findTopologyCronEntry(t.GetPersistedID())
	if entry != nil && time.Until(entry.Next) < 1*time.Hour {
		// run all regular topologies on startup
		job = entry.Job.(TopologyJob)
		go job.Run()
	}
	return nil
}

func findTopologyCronEntry(id string) *cron.Entry {
	for _, entry := range TopologyScheduler.Entries() {
		if entry.Job.(TopologyJob).GetPersistedID() == id {
			return &entry
		}
	}
	return nil
}

func DeleteTopologyJob(id string) {
	entry := findTopologyCronEntry(id)
	if entry == nil {
		return
	}
	logger.Tracef("deleting cron entry for topology:%s with entry ID: %v", id, entry.ID)
	TopologyScheduler.Remove(entry.ID)
}

var CleanupComponents = &job.Job{
	Name:     "CleanupComponents",
	Schedule: "@every 1h",
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
				logger.Errorf("Error deleting components for topology[%s]: %v", r.ID, err)
				ctx.History.AddError(err.Error())
			} else {
				DeleteTopologyJob(r.ID)
				ctx.History.IncrSuccess()
			}
		}
		return nil
	},
}
