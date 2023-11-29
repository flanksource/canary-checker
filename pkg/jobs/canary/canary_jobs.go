package canary

import (
	gocontext "context"
	"fmt"
	"path"
	"sync"
	"time"

	canarycontext "github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/canary-checker/pkg/push"
	"github.com/flanksource/canary-checker/pkg/runner"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/context"
	dutyjob "github.com/flanksource/duty/job"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/kommons"
	"go.opentelemetry.io/otel/trace"

	"github.com/robfig/cron/v3"
	"go.opentelemetry.io/otel/attribute"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

var CanaryScheduler = cron.New()
var CanaryConfigFiles []string
var DataFile string
var Executor bool
var LogPass, LogFail bool

var Kommons *kommons.Client
var Kubernetes kubernetes.Interface
var FuncScheduler = cron.New()

var CanaryStatusChannel chan CanaryStatusPayload

// concurrentJobLocks keeps track of the currently running jobs.
var concurrentJobLocks sync.Map

func StartScanCanaryConfigs(dataFile string, configFiles []string) {
	DataFile = dataFile
	CanaryConfigFiles = configFiles
	if _, err := ScheduleFunc("@every 5m", ScanCanaryConfigs); err != nil {
		logger.Errorf("Failed to schedule scan jobs: %v", err)
	}
	ScanCanaryConfigs()
}

type CanaryJob struct {
	Canary   v1.Canary
	DBCanary pkg.Canary
	LogPass  bool
	LogFail  bool
}

func (j CanaryJob) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: j.Canary.Name, Namespace: j.Canary.Namespace}
}

var MinimumTimeBetweenCanaryRuns = 10 * time.Second
var canaryLastRuntimes = sync.Map{}

func (j CanaryJob) Run(ctx dutyjob.JobRuntime) error {
	ctx.GetSpan().SetAttributes(attribute.String("canary-id", j.DBCanary.ID.String()))
	if runner.IsCanaryIgnored(&j.Canary.ObjectMeta) {
		return nil
	}

	canaryID := j.DBCanary.ID.String()
	val, _ := concurrentJobLocks.LoadOrStore(canaryID, &sync.Mutex{})
	lock, ok := val.(*sync.Mutex)
	if !ok {
		logger.Warnf("expected mutex but got %T for canary(id=%s)", lock, canaryID)
		return nil
	}

	if !lock.TryLock() {
		logger.Debugf("canary (id=%s) is already running. skipping this run ...", canaryID)
		return nil
	}
	defer lock.Unlock()

	lastRunDelta := MinimumTimeBetweenCanaryRuns
	// Get last runtime from sync map
	var lastRuntime time.Time
	if lastRuntimeVal, exists := canaryLastRuntimes.Load(canaryID); exists {
		lastRuntime = lastRuntimeVal.(time.Time)
		lastRunDelta = time.Since(lastRuntime)
	}

	// Skip run if job ran too recently
	if lastRunDelta < MinimumTimeBetweenCanaryRuns {
		logger.Infof("Skipping Canary[%s]:%s since it last ran %.2f seconds ago", canaryID, j.Canary.GetNamespacedName(), lastRunDelta.Seconds())
		return nil
	}

	canaryCtx := canarycontext.New(ctx.Kommons(), ctx.Kubernetes(), ctx.DB(), ctx.Pool(), j.Canary)
	var span trace.Span
	ctx.Context, span = ctx.StartSpan("RunCanaryChecks")
	results, err := checks.RunChecks(canaryCtx)
	if err != nil {
		logger.Errorf("error running checks for canary %s: %v", canaryID, err)
		return nil
	}
	span.End()

	// Get transformed checks before and after, and then delete the olds ones that are not in new set.
	// NOTE: Webhook checks, although they are transformed, are handled entirely by the webhook caller
	// and should not be deleted manually in here.
	existingTransformedChecks, err := db.GetTransformedCheckIDs(ctx.Context, canaryID, checks.WebhookCheckType)
	if err != nil {
		logger.Errorf("error getting transformed checks for canary %s: %v", canaryID, err)
	}

	var transformedChecksCreated []string
	// Transformed checks have a delete strategy
	// On deletion they can either be marked healthy, unhealthy or left as is
	checkIDDeleteStrategyMap := make(map[string]string)

	// TODO: Use ctx with object here
	logPass := j.Canary.IsTrace() || j.Canary.IsDebug() || LogPass
	logFail := j.Canary.IsTrace() || j.Canary.IsDebug() || LogFail
	for _, result := range results {
		if logPass && result.Pass || logFail && !result.Pass {
			logger.Infof(result.String())
		}
		transformedChecksAdded := cache.PostgresCache.Add(pkg.FromV1(result.Canary, result.Check), pkg.CheckStatusFromResult(*result))
		transformedChecksCreated = append(transformedChecksCreated, transformedChecksAdded...)
		for _, checkID := range transformedChecksAdded {
			checkIDDeleteStrategyMap[checkID] = result.Check.GetTransformDeleteStrategy()
		}
	}
	updateCanaryStatusAndEvent(ctx.Context, j.Canary, results)

	checkDeleteStrategyGroup := make(map[string][]string)
	checksToRemove := utils.SetDifference(existingTransformedChecks, transformedChecksCreated)
	if len(checksToRemove) > 0 && len(transformedChecksCreated) > 0 {
		for _, checkID := range checksToRemove {
			strategy := checkIDDeleteStrategyMap[checkID]
			// Empty status by default does not effect check status
			var status string
			if strategy == v1.OnTransformMarkHealthy {
				status = models.CheckStatusHealthy
			} else if strategy == v1.OnTransformMarkUnhealthy {
				status = models.CheckStatusUnhealthy
			}
			checkDeleteStrategyGroup[status] = append(checkDeleteStrategyGroup[status], checkID)
		}
		for status, checkIDs := range checkDeleteStrategyGroup {
			if err := db.AddCheckStatuses(ctx.Context, checkIDs, models.CheckHealthStatus(status)); err != nil {
				logger.Errorf("error adding statuses for transformed checks: %v", err)
			}
			if err := db.RemoveTransformedChecks(ctx.Context, checkIDs); err != nil {
				logger.Errorf("error deleting transformed checks for canary %s: %v", canaryID, err)
			}
		}
	}

	// Update last runtime map
	canaryLastRuntimes.Store(canaryID, time.Now())
	return nil
}

func updateCanaryStatusAndEvent(ctx context.Context, canary v1.Canary, results []*pkg.CheckResult) {
	if CanaryStatusChannel == nil {
		return
	}

	var checkStatus = make(map[string]*v1.CheckStatus)
	var duration int64
	var messages, errors []string
	var failEvents []string
	var msg, errorMsg string
	var pass = true
	var lastTransitionedTime *metav1.Time
	var highestLatency float64
	var uptimeAgg pkg.Uptime

	transitioned := false
	for _, result := range results {
		// Increment duration
		duration += result.Duration

		// Set uptime and latency
		uptime, latency := metrics.Record(canary, result)
		checkID := canary.Status.Checks[result.Check.GetName()]
		checkStatus[checkID] = &v1.CheckStatus{
			Uptime1H:  uptime.String(),
			Latency1H: latency.String(),
		}

		// Increment aggregate uptime
		uptimeAgg.Passed += uptime.Passed
		uptimeAgg.Failed += uptime.Failed

		// Use highest latency for canary status
		if latency.Rolling1H > highestLatency {
			highestLatency = latency.Rolling1H
		}

		// Transition
		q := cache.QueryParams{Check: checkID, StatusCount: 1}
		if canary.Status.LastTransitionedTime != nil {
			q.Start = canary.Status.LastTransitionedTime.Format(time.RFC3339)
		}

		latestCheckStatus, err := db.LatestCheckStatus(ctx, checkID)
		if err != nil || latestCheckStatus == nil {
			transitioned = true
		} else if latestCheckStatus.Status != result.Pass {
			transitioned = true
		}
		if transitioned {
			transitionTime := time.Now()
			if latestCheckStatus != nil {
				transitionTime = latestCheckStatus.CreatedAt
			}

			checkStatus[checkID].LastTransitionedTime = &metav1.Time{Time: transitionTime}
			lastTransitionedTime = &metav1.Time{Time: transitionTime}
		}

		// TODO Why is this here ?
		push.Queue(pkg.FromV1(canary, result.Check), pkg.CheckStatusFromResult(*result))

		// Update status message
		if len(messages) == 1 {
			msg = messages[0]
		} else if len(messages) > 1 {
			msg = fmt.Sprintf("%s, (%d more)", messages[0], len(messages)-1)
		}
		if len(errors) == 1 {
			errorMsg = errors[0]
		} else if len(errors) > 1 {
			errorMsg = fmt.Sprintf("%s, (%d more)", errors[0], len(errors)-1)
		}

		if !result.Pass {
			failEvents = append(failEvents, fmt.Sprintf("%s-%s: %s", result.Check.GetType(), result.Check.GetEndpoint(), result.Message))
			pass = false
		}
	}

	payload := CanaryStatusPayload{
		Pass:                 pass,
		CheckStatus:          checkStatus,
		FailEvents:           failEvents,
		LastTransitionedTime: lastTransitionedTime,
		Message:              msg,
		ErrorMessage:         errorMsg,
		Uptime:               uptimeAgg.String(),
		Latency:              utils.Age(time.Duration(highestLatency) * time.Millisecond),
		NamespacedName:       canary.GetNamespacedName(),
	}

	CanaryStatusChannel <- payload
}

type CanaryStatusPayload struct {
	Pass                 bool
	CheckStatus          map[string]*v1.CheckStatus
	FailEvents           []string
	LastTransitionedTime *metav1.Time
	Message              string
	ErrorMessage         string
	Uptime               string
	Latency              string
	NamespacedName       types.NamespacedName
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

func ScanCanaryConfigs() {
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
			_, err := db.PersistCanary(canary, path.Base(configfile))
			if err != nil {
				logger.Errorf("could not persist %s: %v", canary.Name, err)
			} else {
				logger.Infof("[%s] persisted %s", path.Base(configfile), canary.Name)
			}
		}
	}
}

var canaryUpdateTimeCache = sync.Map{}

// TODO: Refactor to use database object instead of kubernetes
func SyncCanaryJob(ctx context.Context, dbCanary pkg.Canary) error {
	canary, err := dbCanary.ToV1()
	if err != nil {
		return err
	}

	if canary.Spec.Webhook != nil {
		// Webhook checks can be persisted immediately as they do not require scheduling & running.
		result := pkg.Success(canary.Spec.Webhook, *canary)
		_ = cache.PostgresCache.Add(pkg.FromV1(*canary, canary.Spec.Webhook), pkg.CheckStatusFromResult(*result))
	}

	if canary.Spec.GetSchedule() == "@never" {
		DeleteCanaryJob(canary.GetPersistedID())
		return nil
	}

	if runner.IsCanaryIgnored(&canary.ObjectMeta) {
		return nil
	}

	if Kommons == nil {
		var err error
		Kommons, Kubernetes, err = pkg.NewKommonsClient()
		ctx = ctx.WithKommons(Kommons).WithKubernetes(Kubernetes)
		if err != nil {
			logger.Warnf("Failed to get kommons client, features that read kubernetes config will fail: %v", err)
		}
	}

	updateTime, exists := canaryUpdateTimeCache.Load(dbCanary.ID.String())
	cj := CanaryJob{
		Canary:   *canary,
		DBCanary: dbCanary,
	}

	// Create new job context from empty context to create root spans for jobs
	jobCtx := ctx.Wrap(gocontext.Background()).WithObject(canary.ObjectMeta)
	newJob := dutyjob.NewJob(jobCtx, "SyncCanaryJob", canary.Spec.GetSchedule(), cj.Run).SetID(dbCanary.ID.String())
	entry := findCronEntry(dbCanary.ID.String())
	if !exists || dbCanary.UpdatedAt.After(updateTime.(time.Time)) || entry == nil {
		// Remove entry if it exists
		if entry != nil {
			CanaryScheduler.Remove(entry.ID)
		}

		// Schedule canary for the first time
		if err := newJob.AddToScheduler(CanaryScheduler); err != nil {
			return fmt.Errorf("failed to schedule canary %s/%s: %v", canary.Namespace, canary.Name, err)
		}

		entry = newJob.GetEntry(CanaryScheduler)
		logger.Infof("Scheduled %s: %s", canary, canary.Spec.GetSchedule())

		canaryUpdateTimeCache.Store(dbCanary.ID.String(), dbCanary.UpdatedAt)
	}

	// Run all regularly scheduled canaries on startup (<1h) and not daily/weekly schedules
	if entry != nil && time.Until(entry.Next) < 1*time.Hour && !exists {
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
		logIfError(db.PersistJobHistory(jobHistory.AddError(err.Error()).End()), "failed to persist job history [SyncCanaries]")

		return err
	}

	existingIDsInCron := getAllCanaryIDsInCron()
	idsInNewFetch := make([]string, 0, len(canaries))
	for _, c := range canaries {
		jobHistory := models.NewJobHistory("CanarySync", "canary", c.ID.String()).Start()

		idsInNewFetch = append(idsInNewFetch, c.ID.String())
		if err := SyncCanaryJob(ctx.Context, c); err != nil {
			logger.Errorf("Error syncing canary[%s]: %v", c.ID, err.Error())
			logIfError(db.PersistJobHistory(jobHistory.AddError(err.Error()).End()), "failed to persist job history [CanarySync]")
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

func logIfError(err error, description string) {
	if err != nil {
		logger.Errorf("%s: %v", description, err)
	}
}
