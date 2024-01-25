package canary

import (
	gocontext "context"
	"fmt"
	"path"
	"time"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/runner"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/job"
	dutyjob "github.com/flanksource/duty/job"
	"github.com/flanksource/duty/models"
	"github.com/robfig/cron/v3"
)

var cronJobs = make(map[string]*job.Job)

func Unschedule(id string) {
	if job := cronJobs[id]; job != nil {
		job.Unschedule()
		delete(cronJobs, id)
	}
}

func TriggerAt(ctx context.Context, dbCanary pkg.Canary, runAt time.Time) error {
	var job *job.Job
	if job = findJob(dbCanary); job != nil {
		ctx.Warnf("job not found for: %v", dbCanary.ID)
		return nil
	}
	if !runAt.After(time.Now()) {
		go job.Run()
		return nil
	}
	onceOff := fmt.Sprintf("%d %d %d %d *", runAt.Minute(), runAt.Hour(), runAt.Day(), runAt.Month())
	var entry cron.EntryID
	var err error
	entry, err = CanaryScheduler.AddFunc(onceOff, func() {
		job.Run()
		CanaryScheduler.Remove(entry)
	})
	return err
}

func findJob(dbCanary pkg.Canary) *job.Job {
	return cronJobs[dbCanary.ID.String()]
}

// TODO: Refactor to use database object instead of kubernetes
func SyncCanaryJob(ctx context.Context, dbCanary pkg.Canary) error {
	if disabled := ctx.Properties()["check.*.disabled"]; disabled == "true" {
		return nil
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
		schedule = canary.Spec.GetSchedule()
	)

	job := cronJobs[canary.GetPersistedID()]

	if schedule == "@never" {
		if job != nil {
			Unschedule(canary.GetPersistedID())
		}
		return nil
	}

	if runner.IsCanaryIgnored(&canary.ObjectMeta) {
		if job != nil {
			Unschedule(canary.GetPersistedID())
		}
		return nil
	}

	canaryJob := CanaryJob{
		Canary:   *canary,
		DBCanary: dbCanary,
	}

	if job == nil {
		// Create new job context from empty context to create root spans for cronJobs
		jobCtx := ctx.Wrap(gocontext.Background()).WithObject(canary.ObjectMeta)
		jobCtx.WithAnyValue("canaryJob", canaryJob)
		job = dutyjob.NewJob(jobCtx, "Canary", schedule, canaryJob.Run).
			SetID(fmt.Sprintf("%s/%s", canary.Namespace, canary.Name))
		job.Singleton = true
		job.Retention.Success = 0
		job.Retention.Failed = 3
		job.Retention.Age = time.Hour * 48
		job.Retention.Interval = time.Minute * 15
		cronJobs[canary.GetPersistedID()] = job
		if err := job.AddToScheduler(CanaryScheduler); err != nil {
			return err
		}
	} else {
		job.Context = job.Context.WithAnyValue("canaryJob", canaryJob)
	}

	if job.Schedule != schedule {
		if err := job.Reschedule(schedule, CanaryScheduler); err != nil {
			return err
		}
	}
	return nil
}

var SyncCanaryJobs = &dutyjob.Job{
	Name:       "SyncCanaryJobs",
	JobHistory: true,
	Singleton:  true,
	RunNow:     true,
	Schedule:   "@every 5m",
	Retention:  dutyjob.RetentionHour,
	Fn: func(ctx dutyjob.JobRuntime) error {
		canaries, err := db.GetAllCanariesForSync(ctx.Context, runner.WatchNamespace)
		if err != nil {
			return err
		}

		existingIDsInCron := getAllCanaryIDsInCron()
		idsInNewFetch := make([]string, 0, len(canaries))
		for _, c := range canaries {
			idsInNewFetch = append(idsInNewFetch, c.ID.String())
			if err := SyncCanaryJob(ctx.Context, c); err != nil {
				// log the error against the canary itself
				jobHistory := models.NewJobHistory("SyncCanary", "canary", c.ID.String()).Start()
				logger.Errorf("Error syncing canary[%s]: %v", c.ID, err.Error())
				logIfError(jobHistory.AddError(err.Error()).End().Persist(ctx.DB()), "failed to persist job history [CanarySync]")
				// log the error for the sync job itself
				ctx.History.AddError(err.Error())
				continue
			} else {
				ctx.History.IncrSuccess()
			}
		}

		idsToRemoveFromCron := utils.SetDifference(existingIDsInCron, idsInNewFetch)
		for _, id := range idsToRemoveFromCron {
			Unschedule(id)
		}
		return nil
	},
}

func getAllCanaryIDsInCron() []string {
	var ids []string
	for _, entry := range CanaryScheduler.Entries() {
		ids = append(ids, string(entry.Job.(*dutyjob.Job).GetObjectMeta().UID))
	}
	return ids
}

func ScanCanaryConfigs(ctx context.Context) {
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
			_, err := db.PersistCanary(ctx, canary, path.Base(configfile))
			if err != nil {
				logger.Errorf("could not persist %s: %v", canary.Name, err)
			} else {
				logger.Infof("[%s] persisted %s", path.Base(configfile), canary.Name)
			}
		}
	}
}
