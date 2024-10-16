package canary

import (
	"fmt"
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
	dutyEcho "github.com/flanksource/duty/echo"
	dutyjob "github.com/flanksource/duty/job"
	"github.com/flanksource/duty/models"
	"go.opentelemetry.io/otel/trace"

	"github.com/robfig/cron/v3"
	"go.opentelemetry.io/otel/attribute"
	"k8s.io/apimachinery/pkg/types"
)

var (
	CanaryScheduler              = cron.New()
	CanaryConfigFiles            []string
	DataFile                     string
	Executor                     bool
	LogPass, LogFail             bool
	MinimumTimeBetweenCanaryRuns = 10 * time.Second
	FuncScheduler                = cron.New()
)

var CanaryStatusChannel chan CanaryStatusPayload

var CanaryLastRuntimes = sync.Map{}

func init() {
	dutyEcho.RegisterCron(CanaryScheduler)
	dutyEcho.RegisterCron(FuncScheduler)
}

func StartScanCanaryConfigs(ctx context.Context, dataFile string, configFiles []string) {
	DataFile = dataFile
	CanaryConfigFiles = configFiles
	if _, err := FuncScheduler.AddFunc("@every 5m", func() {
		ScanCanaryConfigs(ctx)
	}); err != nil {
		logger.Errorf("Failed to schedule scan jobs: %v", err)
	}
	ScanCanaryConfigs(ctx)
}

type CanaryJob struct {
	Canary   v1.Canary
	DBCanary pkg.Canary
}

func (j CanaryJob) GetNamespacedName() types.NamespacedName {
	return types.NamespacedName{Name: j.Canary.Name, Namespace: j.Canary.Namespace}
}

func (j CanaryJob) Run(ctx dutyjob.JobRuntime) error {
	if runner.IsCanarySuspended(&j.Canary.ObjectMeta) || runner.IsCanaryIgnored(&j.Canary.ObjectMeta) {
		return nil
	}

	canaryID := j.DBCanary.ID.String()
	ctx.History.ResourceID = canaryID
	ctx.History.ResourceType = "canary"

	lastRunDelta := MinimumTimeBetweenCanaryRuns
	// Get last runtime from sync map
	var lastRuntime time.Time
	if lastRuntimeVal, exists := CanaryLastRuntimes.Load(canaryID); exists {
		lastRuntime = lastRuntimeVal.(time.Time)
		lastRunDelta = time.Since(lastRuntime)
	}

	// Skip run if job ran too recently
	if lastRunDelta < MinimumTimeBetweenCanaryRuns {
		ctx.Debugf("skipping since it last ran %.2f seconds ago", lastRunDelta.Seconds())
		return nil
	}

	canaryCtx := canarycontext.New(ctx.Context, j.Canary)
	var span trace.Span
	ctx.Context, span = ctx.StartSpan("RunCanaryChecks")
	defer span.End()
	span.SetAttributes(
		attribute.String("canary.id", canaryID),
		attribute.String("canary.name", j.Canary.Name),
		attribute.String("canary.namespace", j.Canary.Namespace),
	)

	results, err := checks.RunChecks(canaryCtx)
	if err != nil {
		return err
	}

	// Get transformed checks before and after, and then delete the olds ones that are not in new set.
	// NOTE: Webhook checks, although they are transformed, are handled entirely by the webhook caller
	// and should not be deleted manually in here.
	existingTransformedChecks, err := db.GetTransformedCheckIDs(ctx.Context, canaryID, checks.WebhookCheckType)
	if err != nil {
		ctx.Error(err, "error getting transformed checks")
	}

	// TODO: Use ctx with object here
	logPass := j.Canary.IsTrace() || j.Canary.IsDebug() || LogPass
	logFail := j.Canary.IsTrace() || j.Canary.IsDebug() || LogFail
	for _, result := range results {
		if logPass && result.Pass || logFail && !result.Pass {
			ctx.Logger.Named(result.GetName()).Infof(result.String())
		}
	}

	transformedChecksCreated, checkIDDeleteStrategyMap, err := SaveResults(ctx.Context, results)
	if err != nil {
		return fmt.Errorf("failed to save results: %w", err)
	}

	UpdateCanaryStatusAndEvent(ctx.Context, j.Canary, results)

	checkDeleteStrategyGroup := make(map[string][]string)
	checkIDsToRemove := utils.SetDifference(existingTransformedChecks, transformedChecksCreated)
	if len(checkIDsToRemove) > 0 && len(transformedChecksCreated) > 0 {
		for _, checkID := range checkIDsToRemove {
			switch checkIDDeleteStrategyMap[checkID] {
			case v1.OnTransformMarkHealthy:
				checkDeleteStrategyGroup[models.CheckStatusHealthy] = append(checkDeleteStrategyGroup[models.CheckStatusHealthy], checkID)
			case v1.OnTransformMarkUnhealthy:
				checkDeleteStrategyGroup[models.CheckStatusUnhealthy] = append(checkDeleteStrategyGroup[models.CheckStatusUnhealthy], checkID)
			}
		}

		for status, checkIDs := range checkDeleteStrategyGroup {
			if err := db.AddCheckStatuses(ctx.Context, checkIDs, models.CheckHealthStatus(status)); err != nil {
				ctx.Error(err, "error adding statuses for transformed checks")
			}
		}
		if err := db.RemoveTransformedChecks(ctx.Context, checkIDsToRemove); err != nil {
			ctx.Error(err, "error deleting transformed checks for canary")
		}
	}

	// Update last runtime map
	CanaryLastRuntimes.Store(canaryID, time.Now())
	ctx.History.SuccessCount = len(results)
	return nil
}

func SaveResults(ctx context.Context, results []*pkg.CheckResult) ([]string, map[string]string, error) {
	var transformedChecksCreated []string
	// Transformed checks have a delete strategy
	// On deletion they can either be marked healthy, unhealthy or left as is
	checkIDDeleteStrategyMap := make(map[string]string)

	if len(results) == 0 {
		return transformedChecksCreated, checkIDDeleteStrategyMap, nil
	}

	tx := ctx.DB().Begin()
	if tx.Error != nil {
		return nil, nil, fmt.Errorf("error starting transaction: %w", tx.Error)
	}
	defer tx.Rollback()

	for _, result := range results {
		transformedChecksAdded, err := cache.PostgresCache.Add(ctx.WithDB(tx, ctx.Pool()), pkg.FromV1(result.Canary, result.Check), pkg.CheckStatusFromResult(*result))
		if err != nil {
			return nil, nil, fmt.Errorf("error adding check to cache: %w", err)
		}

		transformedChecksCreated = append(transformedChecksCreated, transformedChecksAdded)
		checkIDDeleteStrategyMap[transformedChecksAdded] = result.Check.GetTransformDeleteStrategy()

		// Establish relationship with components & configs
		if err := FormCheckRelationships(ctx.WithDB(tx, ctx.Pool()), result); err != nil {
			ctx.Logger.Named(result.Name).Errorf("error forming check relationships: %v", err)
		}
	}

	return transformedChecksCreated, checkIDDeleteStrategyMap, tx.Commit().Error
}

func logIfError(err error, description string) {
	if err != nil {
		logger.Errorf("%s: %v", description, err)
	}
}

var CleanupDeletedCanaryChecks = &dutyjob.Job{
	Name:       "CleanupDeletedCanaryChecks",
	Schedule:   "@every 1h",
	Singleton:  true,
	JobHistory: true,
	Retention:  dutyjob.RetentionBalanced,
	Fn: func(ctx dutyjob.JobRuntime) error {
		var rows []struct {
			ID string
		}
		// Select all checks whose canary ID is deleted but their deleted at is not marked
		if err := ctx.DB().Raw(`
        SELECT DISTINCT(canaries.id) FROM canaries
        INNER JOIN checks ON canaries.id = checks.canary_id
        WHERE
            checks.deleted_at IS NULL AND
            canaries.deleted_at IS NOT NULL
        `).Scan(&rows).Error; err != nil {
			return err
		}

		for _, r := range rows {
			if err := db.DeleteCanary(ctx.Context, r.ID); err != nil {
				ctx.History.AddError(fmt.Sprintf("Error deleting components for topology[%s]: %v", r.ID, err))
			} else {
				ctx.History.IncrSuccess()
			}
			Unschedule(r.ID)
		}
		return nil
	},
}
