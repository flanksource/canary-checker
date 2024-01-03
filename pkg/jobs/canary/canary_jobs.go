package canary

import (
	"sync"
	"time"

	canarycontext "github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/db"
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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

var CanaryScheduler = cron.New()
var CanaryConfigFiles []string
var DataFile string
var Executor bool
var LogPass, LogFail bool
var MinimumTimeBetweenCanaryRuns = 10 * time.Second
var Kommons *kommons.Client
var Kubernetes kubernetes.Interface
var FuncScheduler = cron.New()

var CanaryStatusChannel chan CanaryStatusPayload

// concurrentJobLocks keeps track of the currently running jobs.
var concurrentJobLocks sync.Map

var canaryLastRuntimes = sync.Map{}

func StartScanCanaryConfigs(ctx context.Context, dataFile string, configFiles []string) {
	DataFile = dataFile
	CanaryConfigFiles = configFiles
	if _, err := ScheduleFunc("@every 5m", func() {
		ScanCanaryConfigs(ctx.DB())
	}); err != nil {
		logger.Errorf("Failed to schedule scan jobs: %v", err)
	}
	ScanCanaryConfigs(ctx.DB())
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

func (j CanaryJob) Run(ctx dutyjob.JobRuntime) error {
	if runner.IsCanaryIgnored(&j.Canary.ObjectMeta) {
		return nil
	}

	canaryID := j.DBCanary.ID.String()
	ctx.History.ResourceID = canaryID
	ctx.History.ResourceType = "canary"
	ctx.GetSpan().SetAttributes(attribute.String("canary-id", canaryID))

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

	canaryCtx := canarycontext.New(ctx.Context, j.Canary)
	var span trace.Span
	ctx.Context, span = ctx.StartSpan("RunCanaryChecks")
	results, err := checks.RunChecks(canaryCtx)
	if err != nil {
		logger.Errorf("error running checks for canary %s: %v", canaryID, err)
		return nil
	}
	defer span.End()

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

		// Establish relationship with components & configs
		if err := formCheckRelationships(ctx.Context, result); err != nil {
			logger.Errorf("error forming check relationships: %v", err)
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
			if strategy != v1.OnTransformIgnore {
				checkDeleteStrategyGroup[status] = append(checkDeleteStrategyGroup[status], checkID)
			}
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

func logIfError(err error, description string) {
	if err != nil {
		logger.Errorf("%s: %v", description, err)
	}
}
