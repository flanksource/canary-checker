package system

import (
	"fmt"
	"reflect"

	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	pkgTopology "github.com/flanksource/canary-checker/pkg/topology"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/kommons"
	"github.com/robfig/cron/v3"
	"k8s.io/apimachinery/pkg/types"
)

var TopologyScheduler = cron.New()

var Kommons *kommons.Client

type TopologyJob struct {
	*kommons.Client
	v1.Topology
}

func (job TopologyJob) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: job.Topology.Name, Namespace: job.Topology.Namespace}
}

func (job TopologyJob) Run() {
	opts := pkgTopology.TopologyRunOptions{
		Client:    job.Client,
		Depth:     10,
		Namespace: job.Namespace,
	}
	if err := pkgTopology.SyncComponents(opts, job.Topology); err != nil {
		logger.Errorf("failed to run topology %s: %v", job.GetNamespacedName(), err)
	}
}

func SyncTopologyJobs() {
	logger.Infof("Syncing topology jobs")
	if Kommons == nil {
		var err error
		Kommons, err = pkg.NewKommonsClient()
		if err != nil {
			logger.Warnf("Failed to get kommons client, features that read kubernetes config will fail: %v", err)
		}
	}

	topologies, err := db.GetAllTopologies()
	if err != nil {
		logger.Errorf("Failed to get topology: %v", err)
		return
	}
	logger.Debugf("Found %d topologies", len(topologies))

	for _, topology := range topologies {
		jobHistory := models.NewJobHistory("TopologySync", "topology", topology.GetPersistedID()).Start()
		_ = db.PersistJobHistory(jobHistory)
		if err := SyncTopologyJob(topology); err != nil {
			logger.Errorf("Error syncing topology job: %v", err)
			_ = db.PersistJobHistory(jobHistory.AddError(err.Error()).End())
			continue
		}
		_ = db.PersistJobHistory(jobHistory.IncrSuccess().End())
	}
	logger.Infof("Synced topology jobs %d", len(TopologyScheduler.Entries()))
}

func SyncTopologyJob(t v1.Topology) error {
	if !t.DeletionTimestamp.IsZero() || t.Spec.GetSchedule() == "@never" {
		DeleteTopologyJob(t)
		return nil
	}
	if Kommons == nil {
		var err error
		Kommons, err = pkg.NewKommonsClient()
		if err != nil {
			logger.Warnf("Failed to get kommons client, features that read kubernetes config will fail: %v", err)
		}
	}
	entry := findTopologyCronEntry(t)
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
		Client:   Kommons,
		Topology: t,
	}

	_, err := TopologyScheduler.AddJob(t.Spec.GetSchedule(), job)
	if err != nil {
		return fmt.Errorf("failed to schedule topology %s/%s: %v", t.Namespace, t.Name, err)
	} else {
		logger.Infof("Scheduled %s/%s: %s", t.Namespace, t.Name, t.Spec.GetSchedule())
	}

	entry = findTopologyCronEntry(t)
	if entry != nil && time.Until(entry.Next) < 1*time.Hour {
		// run all regular topologies on startup
		job = entry.Job.(TopologyJob)
		go job.Run()
	}
	return nil
}

func findTopologyCronEntry(t v1.Topology) *cron.Entry {
	for _, entry := range TopologyScheduler.Entries() {
		if entry.Job.(TopologyJob).GetPersistedID() == t.GetPersistedID() {
			return &entry
		}
	}
	return nil
}

func DeleteTopologyJob(t v1.Topology) {
	entry := findTopologyCronEntry(t)
	if entry == nil {
		return
	}
	logger.Tracef("deleting cron entry for topology %s/%s with entry ID: %v", t.Name, t.Namespace, entry.ID)
	TopologyScheduler.Remove(entry.ID)
}
