package canary

import (
	"fmt"
	"path"
	"reflect"
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

var CanaryScheduler = cron.New()
var CanaryConfigFiles []string
var DataFile string
var Executor bool
var LogPass, LogFail bool

var Kommons *kommons.Client
var FuncScheduler = cron.New()

func StartScanCanaryConfigs(dataFile string, configFiles []string) {
	DataFile = dataFile
	CanaryConfigFiles = configFiles
	if _, err := ScheduleFunc("@every 5m", ScanCanaryConfigs); err != nil {
		logger.Errorf("Failed to schedule scan jobs: %v", err)
	}
	ScanCanaryConfigs()
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
	for _, entry := range CanaryScheduler.Entries() {
		if entry.Job.(CanaryJob).GetPersistedID() == canary.GetPersistedID() {
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
			_, _, _, err := db.PersistCanary(canary, path.Base(configfile))
			if err != nil {
				logger.Errorf("could not persist %s: %v", canary.Name, err)
			} else {
				logger.Infof("[%s] persisted %s", path.Base(configfile), canary.Name)
			}
		}
	}
}

func SyncCanaryJob(canary v1.Canary) error {
	if !canary.DeletionTimestamp.IsZero() || canary.Spec.GetSchedule() == "@never" { //nolint:goconst
		DeleteCanaryJob(canary)
		return nil
	}

	if Kommons == nil {
		var err error
		Kommons, err = pkg.NewKommonsClient()
		if err != nil {
			logger.Warnf("Failed to get kommons client, features that read kubernetes config will fail: %v", err)
		}
	}

	entry := findCronEntry(canary)
	if entry != nil {
		job := entry.Job.(CanaryJob)
		if !reflect.DeepEqual(job.Canary.Spec, canary.Spec) {
			logger.Infof("Rescheduling %s canary with updated specs", canary)
			CanaryScheduler.Remove(entry.ID)
		} else {
			return nil
		}
	}

	job := CanaryJob{
		Client:  Kommons,
		Canary:  canary,
		LogPass: canary.IsTrace() || canary.IsDebug() || LogPass,
		LogFail: canary.IsTrace() || canary.IsDebug() || LogFail,
	}

	_, err := CanaryScheduler.AddJob(canary.Spec.GetSchedule(), job)
	if err != nil {
		return fmt.Errorf("failed to schedule canary %s/%s: %v", canary.Namespace, canary.Name, err)
	} else {
		logger.Infof("Scheduled %s: %s", canary, canary.Spec.GetSchedule())
	}

	entry = findCronEntry(canary)
	if entry != nil && time.Until(entry.Next) < 1*time.Hour {
		// run all regular canaries on startup
		job = entry.Job.(CanaryJob)
		go job.Run()
	}

	return nil
}
func SyncCanaryJobs() {
	logger.Debugf("Syncing canary jobs")

	canaries, err := db.GetAllCanaries()
	if err != nil {
		logger.Errorf("Failed to get canaries: %v", err)
		return
	}

	for _, canary := range canaries {
		if err := SyncCanaryJob(canary); err != nil {
			logger.Errorf(err.Error())
		}
	}
	logger.Infof("Synced canary jobs %d", len(CanaryScheduler.Entries()))
}

func DeleteCanaryJob(canary v1.Canary) {
	entry := findCronEntry(canary)
	if entry == nil {
		return
	}
	logger.Tracef("deleting cron entry for canary %s/%s with entry ID: %v", canary.Name, canary.Namespace, entry.ID)
	CanaryScheduler.Remove(entry.ID)
}

func ScheduleFunc(schedule string, fn func()) (interface{}, error) {
	return FuncScheduler.AddFunc(schedule, fn)
}
