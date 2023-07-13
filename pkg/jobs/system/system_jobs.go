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
	"k8s.io/client-go/kubernetes"
)

var TopologyScheduler = cron.New()

var Kommons *kommons.Client
var Kubernetes kubernetes.Interface

type TopologyJob struct {
	*kommons.Client
	Kubernetes kubernetes.Interface
	v1.Topology
}

func (job TopologyJob) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: job.Topology.Name, Namespace: job.Topology.Namespace}
}

func (job TopologyJob) Run() {
	opts := pkgTopology.TopologyRunOptions{
		Client:     job.Client,
		Kubernetes: job.Kubernetes,
		Depth:      10,
		Namespace:  job.Namespace,
	}
	if err := pkgTopology.SyncComponents(opts, job.Topology); err != nil {
		logger.Errorf("failed to run topology %s: %v", job.GetNamespacedName(), err)
	}
}

func SyncTopologyJobs() {
	logger.Infof("Syncing topology jobs")
	if Kommons == nil {
		var err error
		Kommons, Kubernetes, err = pkg.NewKommonsClient()
		if err != nil {
			logger.Warnf("Failed to get kommons client, features that read kubernetes config will fail: %v", err)
		}
	}

	topologies, err := db.GetAllTopologiesForSync()
	if err != nil {
		logger.Errorf("Failed to get topology: %v", err)
		return
	}

	for _, topology := range topologies {
		if err := SyncTopologyJob(topology); err != nil {
			logger.Errorf("Error syncing topology job: %v", err)
			continue
		}
	}
	logger.Infof("Synced topology jobs %d", len(TopologyScheduler.Entries()))
}

func SyncTopologyJob(t v1.Topology) error {
	if !t.DeletionTimestamp.IsZero() || t.Spec.GetSchedule() == "@never" {
		DeleteTopologyJob(t.GetPersistedID())
		return nil
	}
	if Kommons == nil {
		var err error
		Kommons, Kubernetes, err = pkg.NewKommonsClient()
		if err != nil {
			logger.Warnf("Failed to get kommons client, features that read kubernetes config will fail: %v", err)
		}
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
		Client:     Kommons,
		Kubernetes: Kubernetes,
		Topology:   t,
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

func ReconcileDeletedTopologyComponents() {
	jobHistory := models.NewJobHistory("ReconcileDeletedTopologyComponents", "", "").Start()
	_ = db.PersistJobHistory(jobHistory)
	defer func() { _ = db.PersistJobHistory(jobHistory.End()) }()

	var rows []struct {
		ID        string
		DeletedAt time.Time
	}
	// Select all components whose topology ID is deleted but their deleted at is not marked
	err := db.Gorm.Raw(`
        SELECT DISTINCT(topologies.id), topologies.deleted_at as deleted_at
        FROM topologies
        INNER JOIN components ON topologies.id = components.topology_id
        WHERE
            components.deleted_at IS NULL AND
            topologies.deleted_at IS NOT NULL
        `).Scan(&rows).Error

	if err != nil {
		logger.Errorf("Error fetching deleted topology components: %v", err)
		jobHistory.AddError(err.Error())
		return
	}

	for _, r := range rows {
		if err := db.DeleteComponentsOfTopology(r.ID, r.DeletedAt); err != nil {
			logger.Errorf("Error deleting components for topology[%s]: %v", r.ID, err)
			jobHistory.AddError(err.Error())
		}
		DeleteTopologyJob(r.ID)
	}
	jobHistory.IncrSuccess()
}
