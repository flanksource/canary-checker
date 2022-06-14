package controllers

import (
	"fmt"
	"reflect"

	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/topology"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/kommons"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"k8s.io/apimachinery/pkg/types"
)

var SystemScheduler = cron.New()

type SystemJob struct {
	*kommons.Client
	v1.SystemTemplate
}

func (job SystemJob) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: job.SystemTemplate.Name, Namespace: job.SystemTemplate.Namespace}
}

func (job SystemJob) Run() {
	opts := topology.TopologyRunOptions{
		Client:    job.Client,
		Depth:     10,
		Namespace: job.Namespace,
	}
	systems := topology.Run(opts, job.SystemTemplate)
	for _, system := range systems {
		system.Name = job.SystemTemplate.Name
		system.Namespace = job.SystemTemplate.Namespace
		system.Labels = job.SystemTemplate.Labels
		systemTemplateID, _ := uuid.Parse(job.SystemTemplate.GetPersistedID())
		system.SystemTemplateID = &systemTemplateID
		systemID, comps, err := db.PersistSystem(system)
		if err != nil {
			logger.Errorf("error persisting the system: %v", err)
		}
		dbComps, err := db.GetComponentsWithSystemID(systemID)
		if err != nil {
			logger.Errorf("error getting components for system: %v", err)
			continue
		}
		compIDs := difference(dbComps, comps)
		db.DeleteComponentsWithID(compIDs, time.Now())
	}
	topology.ComponentRun()
}

func SyncSystemsJobs() {
	logger.Infof("Syncing systemTemplate jobs")
	if Kommons == nil {
		var err error
		Kommons, err = pkg.NewKommonsClient()
		if err != nil {
			logger.Warnf("Failed to get kommons client, features that read kubernetes config will fail: %v", err)
		}
	}

	systemTemplates, err := db.GetAllSystemTemplates()
	if err != nil {
		logger.Errorf("Failed to get systemTemplate: %v", err)
		return
	}

	for _, systemTemplate := range systemTemplates {
		if err := SyncSystemJob(systemTemplate); err != nil {
			logger.Errorf(err.Error())
		}
	}
	logger.Infof("Synced system template jobs %d", len(SystemScheduler.Entries()))
}

func SyncSystemJob(systemTemplate v1.SystemTemplate) error {
	if !systemTemplate.DeletionTimestamp.IsZero() || systemTemplate.Spec.GetSchedule() == "@never" {
		DeleteSystemJob(systemTemplate)
		return nil
	}
	if Kommons == nil {
		var err error
		Kommons, err = pkg.NewKommonsClient()
		if err != nil {
			logger.Warnf("Failed to get kommons client, features that read kubernetes config will fail: %v", err)
		}
	}
	entry := findSystemTemplateCronEntry(systemTemplate)
	if entry != nil {
		job := entry.Job.(SystemJob)
		if !reflect.DeepEqual(job.SystemTemplate.Spec, systemTemplate.Spec) {
			logger.Infof("Rescheduling %s system template with updated specs", systemTemplate)
			SystemScheduler.Remove(entry.ID)
		} else {
			return nil
		}
	}
	job := SystemJob{
		Client:         Kommons,
		SystemTemplate: systemTemplate,
	}

	_, err := SystemScheduler.AddJob(systemTemplate.Spec.GetSchedule(), job)
	if err != nil {
		return fmt.Errorf("failed to schedule system template %s/%s: %v", systemTemplate.Namespace, systemTemplate.Name, err)
	} else {
		logger.Infof("Scheduled %s/%s: %s", systemTemplate.Namespace, systemTemplate.Name, systemTemplate.Spec.GetSchedule())
	}

	entry = findSystemTemplateCronEntry(systemTemplate)
	if entry != nil && time.Until(entry.Next) < 1*time.Hour {
		// run all regular systemTemplate on startup
		job = entry.Job.(SystemJob)
		go job.Run()
	}
	return nil
}

func findSystemTemplateCronEntry(systemTemplate v1.SystemTemplate) *cron.Entry {
	for _, entry := range SystemScheduler.Entries() {
		if entry.Job.(SystemJob).GetPersistedID() == systemTemplate.GetPersistedID() {
			return &entry
		}
	}
	return nil
}

func DeleteSystemJob(systemTemplate v1.SystemTemplate) {
	entry := findSystemTemplateCronEntry(systemTemplate)
	if entry == nil {
		return
	}
	logger.Tracef("deleting cron entry for system template %s/%s with entry ID: %v", systemTemplate.Name, systemTemplate.Namespace, entry.ID)
	SystemScheduler.Remove(entry.ID)
}

// difference returns the elements in `a` that aren't in `b`.
func difference(a, b []pkg.Component) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x.ID.String()] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x.ID.String()]; !found {
			diff = append(diff, x.ID.String())
		}
	}
	return diff
}
