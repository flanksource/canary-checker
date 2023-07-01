package canary

import (
	"fmt"
	"path"
	"sync"
	"time"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/metrics"
	"github.com/flanksource/canary-checker/pkg/push"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/kommons"
	"github.com/robfig/cron/v3"
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
	*kommons.Client
	Kubernetes kubernetes.Interface
	v1.Canary
	// model   pkg.Canary
	LogPass bool
	LogFail bool
}

func (job CanaryJob) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: job.Canary.Name, Namespace: job.Canary.Namespace}
}

var minimumTimeBetweenCanaryRuns = 10 * time.Second
var canaryLastRuntimes = sync.Map{}

func (job CanaryJob) Run() {
	val, _ := concurrentJobLocks.LoadOrStore(job.Canary.GetPersistedID(), &sync.Mutex{})
	lock, ok := val.(*sync.Mutex)
	if !ok {
		logger.Warnf("expected mutex but got %T for canary(id=%s)", lock, job.Canary.GetPersistedID())
		return
	}

	if !lock.TryLock() {
		logger.Debugf("config (id=%s) is already running. skipping this run ...", job.Canary.GetPersistedID())
		return
	}
	defer lock.Unlock()

	lastRunDelta := minimumTimeBetweenCanaryRuns
	// Get last runtime from sync map
	var lastRuntime time.Time
	if lastRuntimeVal, exists := canaryLastRuntimes.Load(job.Canary.GetPersistedID()); exists {
		lastRuntime = lastRuntimeVal.(time.Time)
		lastRunDelta = time.Since(lastRuntime)
	}

	// Skip run if job ran too recently
	if lastRunDelta < minimumTimeBetweenCanaryRuns {
		logger.Infof("Skipping Canary[%s]:%s since it last ran %.2f seconds ago", job.Canary.GetPersistedID(), job.GetNamespacedName(), lastRunDelta.Seconds())
		return
	}

	// Get transformed checks before and after, and then delete the olds ones that are not in new set
	existingTransformedChecks, _ := db.GetTransformedCheckIDs(job.Canary.GetPersistedID())
	var newChecksCreated []string
	results := checks.RunChecks(job.NewContext())
	for _, result := range results {
		if job.LogPass && result.Pass || job.LogFail && !result.Pass {
			logger.Infof(result.String())
		}
		checkIDsAdded := cache.PostgresCache.Add(pkg.FromV1(result.Canary, result.Check), pkg.FromResult(*result))
		newChecksCreated = append(newChecksCreated, checkIDsAdded...)
	}
	job.updateStatusAndEvent(results)

	// Checks which are not present now should be marked as healthy
	checksToMarkHealthy := utils.SetDifference(existingTransformedChecks, newChecksCreated)
	if err := db.UpdateChecksStatus(checksToMarkHealthy, models.CheckStatusHealthy); err != nil {
		logger.Errorf("error deleting transformed checks for canary %s: %v", job.Canary.GetPersistedID(), err)
	}

	// Update last runtime map
	canaryLastRuntimes.Store(job.Canary.GetPersistedID(), time.Now())
}

func (job *CanaryJob) NewContext() *context.Context {
	return context.New(job.Client, job.Kubernetes, db.Gorm, job.Canary)
}

func (job CanaryJob) updateStatusAndEvent(results []*pkg.CheckResult) {
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
		uptime, latency := metrics.Record(job.Canary, result)
		checkKey := job.Canary.GetKey(result.Check)
		checkStatus[checkKey] = &v1.CheckStatus{}
		checkStatus[checkKey].Uptime1H = uptime.String()
		checkStatus[checkKey].Latency1H = latency.String()

		// Increment aggregate uptime
		uptimeAgg.Passed += uptime.Passed
		uptimeAgg.Failed += uptime.Failed

		// Use highest latency for canary status
		if latency.Rolling1H > highestLatency {
			highestLatency = latency.Rolling1H
		}

		// Transition
		q := cache.QueryParams{Check: checkKey, StatusCount: 1}
		if job.Canary.Status.LastTransitionedTime != nil {
			q.Start = job.Canary.Status.LastTransitionedTime.Format(time.RFC3339)
		}
		lastStatus, err := cache.PostgresCache.Query(q)
		if err != nil || len(lastStatus) == 0 || len(lastStatus[0].Statuses) == 0 {
			transitioned = true
		} else if len(lastStatus) > 0 && (lastStatus[0].Statuses[0].Status != result.Pass) {
			transitioned = true
		}
		if transitioned {
			checkStatus[checkKey].LastTransitionedTime = &metav1.Time{Time: time.Now()}
			lastTransitionedTime = &metav1.Time{Time: time.Now()}
		}

		push.Queue(pkg.FromV1(job.Canary, result.Check), pkg.FromResult(*result))

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
		NamespacedName:       job.GetNamespacedName(),
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

var canaryUpdateTimeCache = make(map[string]time.Time)

// TODO: Refactor to use database object instead of kubernetes
func SyncCanaryJob(canary v1.Canary) error {
	if !canary.DeletionTimestamp.IsZero() || canary.Spec.GetSchedule() == "@never" {
		DeleteCanaryJob(canary)
		return nil
	}

	if Kommons == nil {
		var err error
		Kommons, Kubernetes, err = pkg.NewKommonsClient()
		if err != nil {
			logger.Warnf("Failed to get kommons client, features that read kubernetes config will fail: %v", err)
		}
	}

	dbCanary, err := db.GetCanary(canary.GetPersistedID())
	if err != nil {
		return err
	}

	job := CanaryJob{
		Client:     Kommons,
		Kubernetes: Kubernetes,
		Canary:     canary,
		LogPass:    canary.IsTrace() || canary.IsDebug() || LogPass,
		LogFail:    canary.IsTrace() || canary.IsDebug() || LogFail,
	}

	updateTime, exists := canaryUpdateTimeCache[dbCanary.ID.String()]
	entry := findCronEntry(canary)
	if !exists || dbCanary.UpdatedAt.After(updateTime) || entry == nil {
		// Remove entry if it exists
		if entry != nil {
			CanaryScheduler.Remove(entry.ID)
		}

		// Schedule canary for the first time
		entryID, err := CanaryScheduler.AddJob(canary.Spec.GetSchedule(), job)
		if err != nil {
			return fmt.Errorf("failed to schedule canary %s/%s: %v", canary.Namespace, canary.Name, err)
		}
		entry = utils.Ptr(CanaryScheduler.Entry(entryID))
		logger.Infof("Scheduled %s: %s", canary, canary.Spec.GetSchedule())

		canaryUpdateTimeCache[dbCanary.ID.String()] = dbCanary.UpdatedAt
	}

	// Run all regularly scheduled canaries on startup (<1h) and not daily/weekly schedules
	if entry != nil && time.Until(entry.Next) < 1*time.Hour && !exists {
		go entry.Job.Run()
	}

	return nil
}

func SyncCanaryJobs() {
	logger.Debugf("Syncing canary jobs")

	jobHistory := models.NewJobHistory("CanarySync", "canary", "").Start()
	_ = db.PersistJobHistory(jobHistory)
	defer func() { _ = db.PersistJobHistory(jobHistory.End()) }()

	canaries, err := db.GetAllCanaries()
	if err != nil {
		logger.Errorf("Failed to get canaries: %v", err)
		jobHistory.AddError(err.Error())
		return
	}

	for _, c := range canaries {
		canary, err := c.ToV1()
		if err != nil {
			logger.Errorf("Error parsing canary[%s]: %v", c.ID, err)
			jobHistory.AddError(err.Error())
			continue
		}

		if len(canary.Status.Checks) == 0 && len(canary.Spec.GetAllChecks()) > 0 {
			logger.Infof("Persisting %s as it has no checks", canary.Name)
			pkgCanary, _, _, err := db.PersistCanary(*canary, canary.Annotations["source"])
			if err != nil {
				logger.Errorf("Failed to persist canary %s: %v", canary.Name, err)
				jobHistory.AddError(err.Error())
				continue
			}

			v1canary, err := pkgCanary.ToV1()
			if err != nil {
				logger.Errorf("Failed to convert canary to V1 %s: %v", canary.Name, err)
				jobHistory.AddError(err.Error())
				continue
			}

			if err := SyncCanaryJob(*v1canary); err != nil {
				logger.Errorf(err.Error())
				jobHistory.AddError(err.Error())
			}
		} else if err := SyncCanaryJob(*canary); err != nil {
			logger.Errorf(err.Error())
			jobHistory.AddError(err.Error())
		}
	}

	jobHistory.IncrSuccess()
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

func init() {
	// We are adding a small buffer to prevent blocking
	CanaryStatusChannel = make(chan CanaryStatusPayload, 64)
}
