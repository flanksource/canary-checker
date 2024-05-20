package canary

import (
	"fmt"
	"path"
	"reflect"
	"sync"
	"time"

	canaryCtx "github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/runner"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/job"
	"github.com/flanksource/duty/models"
	"github.com/robfig/cron/v3"
)

var canaryJobs sync.Map

const DefaultCanarySchedule = "@every 5m"

func Unschedule(id string) {
	if j, exists := canaryJobs.Load(id); exists {
		j.(*job.Job).Unschedule()
		canaryJobs.Delete(id)
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
	j, exists := canaryJobs.Load(dbCanary.ID.String())
	if !exists {
		return nil
	}
	return j.(*job.Job)
}

func SyncCanaryJob(ctx context.Context, dbCanary pkg.Canary) error {
	id := dbCanary.ID.String()

	if disabled := ctx.Properties()["check.*.disabled"]; disabled == "true" {
		return nil
	}
	canary, err := dbCanary.ToV1()
	if err != nil {
		return err
	}

	if canary.Namespace == "" {
		canary.Namespace = "default"
	}

	if canary.Spec.Webhook != nil {
		// Webhook checks can be persisted immediately as they do not require scheduling & running.
		result := pkg.Success(canary.Spec.Webhook, *canary)
		_ = cache.PostgresCache.Add(pkg.FromV1(*canary, canary.Spec.Webhook), pkg.CheckStatusFromResult(*result))
	}

	var existingJob *job.Job
	if j, ok := canaryJobs.Load(id); ok {
		existingJob = j.(*job.Job)
	}

	if canary.Spec.GetSchedule() == "@never" || dbCanary.DeletedAt != nil {
		Unschedule(id)
		return nil
	}

	if runner.IsCanaryIgnored(&canary.ObjectMeta) {
		Unschedule(id)
		return nil
	}

	canaryJob := CanaryJob{
		Canary:   *canary,
		DBCanary: dbCanary,
	}

	if existingJob == nil {
		newCanaryJob(canaryJob)
		return nil
	}

	existingCanary := existingJob.Context.Value("canary")
	if existingCanary != nil && !reflect.DeepEqual(existingCanary.(v1.Canary).Spec, canary.Spec) {
		ctx.Debugf("Rescheduling %s canary with updated specs", canary)
		Unschedule(id)
		newCanaryJob(canaryJob)
	}

	return nil
}

func newCanaryJob(c CanaryJob) {
	schedule := c.Canary.Spec.Schedule
	if schedule == "" {
		schedule = DefaultCanarySchedule
	}

	j := &job.Job{
		Name:                 "Canary",
		Context:              canaryCtx.DefaultContext.WithObject(c.Canary.ObjectMeta).WithAnyValue("canary", c.Canary),
		Schedule:             schedule,
		RunNow:               true,
		Singleton:            true,
		JobHistory:           true,
		IgnoreSuccessHistory: true,
		Retention:            job.RetentionBalanced,
		ResourceID:           c.DBCanary.ID.String(),
		ResourceType:         "canary",
		ID:                   fmt.Sprintf("%s/%s", c.Canary.Namespace, c.Canary.Name),
		Fn:                   c.Run,
	}

	canaryJobs.Store(c.DBCanary.ID.String(), j)
	if err := j.AddToScheduler(CanaryScheduler); err != nil {
		logger.Errorf("[%s] failed to schedule %v", j.Name, err)
	}
}

var SyncCanaryJobs = &job.Job{
	Name:       "SyncCanaryJobs",
	JobHistory: true,
	Singleton:  true,
	RunNow:     true,
	Schedule:   "@every 5m",
	Retention:  job.RetentionFew,
	Fn: func(ctx job.JobRuntime) error {
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
				jobHistory := models.NewJobHistory(ctx.Logger, "SyncCanary", "canary", c.ID.String()).Start()
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
		ids = append(ids, string(entry.Job.(*job.Job).GetObjectMeta().UID))
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
