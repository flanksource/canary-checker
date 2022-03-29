package controllers

import (
	"path"
	"time"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/canary-checker/pkg/push"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/kommons"
	"github.com/robfig/cron/v3"
	"k8s.io/apimachinery/pkg/types"
)

var Scheduler = cron.New()
var SystemScheduler = cron.New()
var Kommons *kommons.Client
var CanaryConfigFiles []string
var DataFile string

func Start() {
	SystemScheduler.Start()
	Scheduler.Start()
	if _, err := ScheduleSystemFunc("@every 5m", SyncCanaryJobs); err != nil {
		logger.Errorf("Failed to schedule sync jobs: %v", err)
	}
	SyncCanaryJobs()
}

func StartScanCanaryConfigs(dataFile string, configFiles []string) {
	DataFile = dataFile
	CanaryConfigFiles = configFiles
	if _, err := ScheduleSystemFunc("@every 5m", ScanCanaryConfigs); err != nil {
		logger.Errorf("Failed to schedule scan jobs: %v", err)
	}
	ScanCanaryConfigs()
}

func ScheduleSystemFunc(schedule string, fn func()) (interface{}, error) {
	return SystemScheduler.AddFunc(schedule, fn)
}

type CanaryJob struct {
	*kommons.Client
	v1.Canary
	// model   pkg.Canary
	LogPass bool
	LogFail bool
}

func (job CanaryJob) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: job.Canary.Name, Namespace: job.Canary.Namespace}
}

func (job CanaryJob) Run() {
	results := checks.RunChecks(job.NewContext())

	for _, result := range results {
		if job.LogPass && result.Pass || job.LogFail && !result.Pass {
			logger.Infof(result.String())
		}
		cache.PostgresCache.Add(pkg.FromV1(result.Canary, result.Check), pkg.FromResult(*result))
		metrics.Record(result.Canary, result)
		push.Queue(pkg.FromV1(result.Canary, result.Check), pkg.FromResult(*result))
	}
}

func (job *CanaryJob) NewContext() *context.Context {
	return context.New(job.Client, job.Canary)
}

func findCronEntry(canary v1.Canary) *cron.Entry {
	for _, entry := range Scheduler.Entries() {
		if entry.Job.(CanaryJob).Status.PersistedID == canary.Status.PersistedID {
			return &entry
		}
	}
	return nil
}

func ScanCanaryConfigs() {
	logger.Infof("Syncing canary specs: %v", CanaryConfigFiles)
	for _, configfile := range CanaryConfigFiles {
		configs, err := pkg.ParseConfig(configfile, DataFile)
		if err != nil {
			logger.Errorf("could not parse %s: %v", configfile, err)
		}

		for _, canary := range configs {
			_, err := db.PersistCanary(canary, path.Base(configfile))
			if err != nil {
				logger.Errorf("could not persist %s: %v", canary.Name, err)
			} else {
				logger.Infof("[%s] persisted %s", path.Base(configfile), canary.Name)
			}
		}
	}
}

func SyncCanaryJobs() {
	logger.Infof("Syncing canary jobs")
	seenEntryIds := map[cron.EntryID]bool{}

	if Kommons == nil {
		var err error
		Kommons, err = pkg.NewKommonsClient()
		if err != nil {
			logger.Warnf("Failed to get kommons client, features that read kubernetes config will fail: %v", err)
		}
	}

	canaries, err := db.GetAllCanaries()
	if err != nil {
		logger.Errorf("Failed to get canaries: %v", err)
		return
	}
	for _, canary := range canaries {
		schedule := canary.Spec.GetSchedule()
		entry := findCronEntry(canary)
		if entry != nil {
			job := entry.Job.(CanaryJob)
			if schedule != job.Canary.Spec.GetSchedule() {
				logger.Infof("Rescheduling %s from %s to %s", canary, job.Canary.Spec.GetSchedule(), canary.Spec.GetSchedule())
				Scheduler.Remove(entry.ID)
			} else {
				seenEntryIds[entry.ID] = true
				job.Canary = canary
				(*entry).Job = job
				continue
			}
		}

		job := CanaryJob{
			Client:  Kommons,
			Canary:  canary,
			LogPass: true,
			LogFail: true,
		}
		if canary.Spec.GetSchedule() == "@never" {
			continue
		}
		entryID, err := Scheduler.AddJob(canary.Spec.GetSchedule(), job)
		if err != nil {
			logger.Errorf("Failed to schedule canary %s/%s: %v", canary.Namespace, canary.Name, err)
			continue
		} else {
			logger.Infof("Scheduling %s to %s", canary, canary.Spec.GetSchedule())
			seenEntryIds[entryID] = true
		}

		entry = findCronEntry(canary)
		if entry != nil && time.Until(entry.Next) < 1*time.Hour {
			// run all regular canaries on startup
			go job.Run()
		}
	}

	for _, entry := range Scheduler.Entries() {
		if !seenEntryIds[entry.ID] {
			logger.Infof("Removing  %s", entry.Job.(CanaryJob).Canary)
			Scheduler.Remove(entry.ID)
		}
	}
}
