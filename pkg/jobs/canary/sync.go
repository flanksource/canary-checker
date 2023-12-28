package canary

import (
	gocontext "context"
	"fmt"
	"path"
	"sync"
	"time"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/runner"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/context"
	dutyjob "github.com/flanksource/duty/job"
	"github.com/flanksource/duty/models"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
)

var canaryUpdateTimeCache = sync.Map{}

type SyncCanaryJobConfig struct {
	RunNow bool

	// Schedule to override the schedule from the spec
	Schedule string
}

func WithRunNow(value bool) SyncCanaryJobOption {
	return func(config *SyncCanaryJobConfig) {
		config.RunNow = value
	}
}

func WithSchedule(schedule string) SyncCanaryJobOption {
	return func(config *SyncCanaryJobConfig) {
		config.Schedule = schedule
	}
}

type SyncCanaryJobOption func(*SyncCanaryJobConfig)

// TODO: Refactor to use database object instead of kubernetes
func SyncCanaryJob(ctx context.Context, dbCanary pkg.Canary, options ...SyncCanaryJobOption) error {
	if disabled, _ := ctx.Properties()["check.*.disabled"]; disabled == "true" {
		return nil
	}
	// Apply options to the configuration
	syncOption := &SyncCanaryJobConfig{}
	for _, option := range options {
		option(syncOption)
	}

	canary, err := dbCanary.ToV1()
	if err != nil {
		return err
	}

	if canary.Spec.Webhook != nil {
		// Webhook checks can be persisted immediately as they do not require scheduling & running.
		result := pkg.Success(canary.Spec.Webhook, *canary)
		_ = cache.PostgresCache.Add(pkg.FromV1(*canary, canary.Spec.Webhook), pkg.CheckStatusFromResult(*result))
	}

	var (
		schedule   = syncOption.Schedule
		scheduleID = dbCanary.ID.String() + "-scheduled"
	)

	if schedule == "" {
		schedule = canary.Spec.GetSchedule()
		scheduleID = dbCanary.ID.String()
	}

	if schedule == "@never" {
		DeleteCanaryJob(canary.GetPersistedID())
		return nil
	}

	if runner.IsCanaryIgnored(&canary.ObjectMeta) {
		return nil
	}

	updateTime, exists := canaryUpdateTimeCache.Load(dbCanary.ID.String())
	cj := CanaryJob{
		Canary:   *canary,
		DBCanary: dbCanary,
	}

	// Create new job context from empty context to create root spans for jobs
	jobCtx := ctx.Wrap(gocontext.Background()).WithObject(canary.ObjectMeta)
	newJob := dutyjob.NewJob(jobCtx, "SyncCanaryJob", schedule, cj.Run).SetID(scheduleID)
	entry := findCronEntry(scheduleID)

	shouldSchedule := !exists || // updated time cache was not found. So we reschedule anyway.
		dbCanary.UpdatedAt.After(updateTime.(time.Time)) || // the spec has been updated since it was last scheduled
		entry == nil || // the canary is not scheduled yet
		syncOption.Schedule != "" // custom schedule so we always need to reschedule

	if shouldSchedule {
		// Remove entry if it exists
		if entry != nil {
			CanaryScheduler.Remove(entry.ID)
		}

		// Schedule canary for the first time
		if err := newJob.AddToScheduler(CanaryScheduler); err != nil {
			return fmt.Errorf("failed to schedule canary %s/%s: %v", canary.Namespace, canary.Name, err)
		}

		entry = newJob.GetEntry(CanaryScheduler)
		logger.Infof("Scheduled %s (%s). Next run: %v", canary, schedule, entry.Next)

		canaryUpdateTimeCache.Store(dbCanary.ID.String(), dbCanary.UpdatedAt)
	}

	// Run all regularly scheduled canaries on startup (<1h) and not daily/weekly schedules
	if (entry != nil && time.Until(entry.Next) < 1*time.Hour && !exists) || syncOption.RunNow {
		go entry.Job.Run()
	}

	return nil
}

func SyncCanaryJobs(ctx dutyjob.JobRuntime) error {
	ctx.Debugf("Syncing canary jobs")

	canaries, err := db.GetAllCanariesForSync(ctx.Context, runner.WatchNamespace)
	if err != nil {
		logger.Errorf("Failed to get canaries: %v", err)

		jobHistory := models.NewJobHistory("SyncCanaries", "canary", "").Start()
		logIfError(jobHistory.AddError(err.Error()).End().Persist(ctx.DB()), "failed to persist job history [SyncCanaries]")

		return err
	}

	existingIDsInCron := getAllCanaryIDsInCron()
	idsInNewFetch := make([]string, 0, len(canaries))
	for _, c := range canaries {
		jobHistory := models.NewJobHistory("CanarySync", "canary", c.ID.String()).Start()

		idsInNewFetch = append(idsInNewFetch, c.ID.String())
		if err := SyncCanaryJob(ctx.Context, c); err != nil {
			logger.Errorf("Error syncing canary[%s]: %v", c.ID, err.Error())
			logIfError(jobHistory.AddError(err.Error()).End().Persist(ctx.DB()), "failed to persist job history [CanarySync]")
			continue
		}
	}

	idsToRemoveFromCron := utils.SetDifference(existingIDsInCron, idsInNewFetch)
	for _, id := range idsToRemoveFromCron {
		DeleteCanaryJob(id)
	}

	logger.Infof("Synced canary jobs %d", len(CanaryScheduler.Entries()))
	return nil
}

func DeleteCanaryJob(id string) {
	entry := findCronEntry(id)
	if entry == nil {
		return
	}
	logger.Tracef("deleting cron entry for canary:%s with entry ID: %v", id, entry.ID)
	CanaryScheduler.Remove(entry.ID)
}

func ScheduleFunc(schedule string, fn func()) (interface{}, error) {
	return FuncScheduler.AddFunc(schedule, fn)
}

func findCronEntry(id string) *cron.Entry {
	for _, entry := range CanaryScheduler.Entries() {
		if entry.Job.(*dutyjob.Job).ID == id {
			return &entry
		}
	}
	return nil
}

func getAllCanaryIDsInCron() []string {
	var ids []string
	for _, entry := range CanaryScheduler.Entries() {
		ids = append(ids, entry.Job.(*dutyjob.Job).ID)
	}
	return ids
}

func ScanCanaryConfigs(_db *gorm.DB) {
	logger.Infof("Syncing canary specs: %v", CanaryConfigFiles)
	for _, configfile := range CanaryConfigFiles {
		configs, err := pkg.ParseConfig(configfile, DataFile)
		if err != nil {
			logger.Errorf("could not parse %s: %v", configfile, err)
		}

		for _, canary := range configs {
			if runner.IsCanaryIgnored(&canary.ObjectMeta) {
				continue
			}
			_, err := db.PersistCanary(_db, canary, path.Base(configfile))
			if err != nil {
				logger.Errorf("could not persist %s: %v", canary.Name, err)
			} else {
				logger.Infof("[%s] persisted %s", path.Base(configfile), canary.Name)
			}
		}
	}
}
