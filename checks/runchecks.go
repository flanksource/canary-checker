package checks

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/logger"
	dutyContext "github.com/flanksource/duty/context"
	"github.com/flanksource/duty/models"
	gocache "github.com/patrickmn/go-cache"
)

var checksCache = gocache.New(5*time.Minute, 5*time.Minute)

var DisabledChecks []string

func getDisabledChecks(ctx *context.Context) (map[string]struct{}, error) {
	if val, ok := checksCache.Get("disabledChecks"); ok {
		return val.(map[string]struct{}), nil
	}

	result := make(map[string]struct{})
	if ctx.DB() == nil {
		return result, nil
	}

	rows, err := ctx.DB().Raw("SELECT name FROM properties WHERE name LIKE 'check.disabled.%' AND value = 'true'").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}

		result[strings.TrimPrefix(name, "check.disabled.")] = struct{}{}
	}

	for _, check := range DisabledChecks {
		result[check] = struct{}{}
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	checksCache.SetDefault("disabledChecks", result)
	return result, nil
}

func RunChecks(ctx *context.Context) ([]*pkg.CheckResult, error) {
	var results []*pkg.CheckResult

	disabledChecks, err := getDisabledChecks(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting disabled checks: %v", err)
	}

	// Check if canary is not marked deleted in DB
	if db.Gorm != nil && ctx.Canary.GetPersistedID() != "" {
		var deletedAt sql.NullTime
		err := db.Gorm.Table("canaries").Select("deleted_at").Where("id = ?", ctx.Canary.GetPersistedID()).Scan(&deletedAt).Error
		if err != nil {
			return nil, fmt.Errorf("error getting canary: %v", err)
		}

		if deletedAt.Valid {
			return results, nil
		}
	}

	checks := ctx.Canary.Spec.GetAllChecks()
	ctx.Debugf("[%s] checking %d checks", ctx.Canary.Name, len(checks))
	for _, c := range All {
		// FIXME: this doesn't work correct with DNS,
		// t := GetDeadline(ctx.Canary)
		// ctx, cancel := ctx.WithDeadline(t)
		// defer cancel()

		if _, ok := disabledChecks[c.Type()]; ok {
			continue
		}
		if !Checks(checks).Includes(c) {
			continue
		}

		result := c.Run(ctx)
		transformedResults := TransformResults(ctx, result)
		results = append(results, transformedResults...)

		ExportCheckMetrics(ctx, transformedResults)
	}

	return ProcessResults(ctx, results), nil
}

func TransformResults(ctx *context.Context, in []*pkg.CheckResult) (out []*pkg.CheckResult) {
	for _, r := range in {
		checkCtx := ctx.WithCheckResult(r)
		transformed, err := transform(checkCtx, r)
		if err != nil {
			r.Failf("transformation failure: %v", err)
			out = append(out, r)
		} else {
			for _, t := range transformed {
				out = append(out, processTemplates(checkCtx, t))
			}
		}
	}
	return out
}

func ProcessResults(ctx *context.Context, results []*pkg.CheckResult) []*pkg.CheckResult {
	if ctx.Canary.Spec.ResultMode == "" {
		return results
	}
	switch ctx.Canary.Spec.ResultMode {
	case v1.JunitResultMode:
		suite := GetJunitReportFromResults(ctx.Canary.GetName(), results)
		var status = true
		if suite.Failed > 0 {
			status = false
		}
		return []*pkg.CheckResult{
			{
				Pass:   status,
				Canary: ctx.Canary,
				Detail: suite,
				Check: v1.JunitCheck{
					TestResults: "combined",
					Description: v1.Description{Description: "Result Mode: JUnit Report"},
				},
				Message: suite.String(),
				Start:   time.Now(),
			},
		}
	default:
		logger.Errorf("Unknown result mode: %s", ctx.Canary.Spec.ResultMode)
	}

	return results
}

func processTemplates(ctx *context.Context, r *pkg.CheckResult) *pkg.CheckResult {
	if r.Duration == 0 && r.GetDuration() > 0 {
		r.Duration = r.GetDuration()
	}
	switch v := r.Check.(type) {
	case v1.DisplayTemplate:
		if !v.GetDisplayTemplate().IsEmpty() {
			message, err := template(ctx, v.GetDisplayTemplate())
			if err != nil {
				r.ErrorMessage(err)
			} else {
				r.ResultMessage(message)
			}
		}
	}

	switch v := r.Check.(type) {
	case v1.TestFunction:
		tpl := v.GetTestFunction()
		if tpl.IsEmpty() {
			break
		}
		message, err := template(ctx, tpl)
		if err != nil {
			r.ErrorMessage(err)
		} else if message != "true" {
			if message != "false" {
				r.Failf("expecting either 'true' or 'false' but got '%v'", message)
			} else {
				r.Failf("")
			}
		}
	}

	return r
}

func PersistCheckResults(ctx dutyContext.Context, canaryID string, canary v1.Canary, results []*pkg.CheckResult) {
	// Get transformed checks before and after, and then delete the olds ones that are not in new set
	existingTransformedChecks, _ := db.GetTransformedCheckIDs(ctx, canaryID)
	var transformedChecksCreated []string
	// Transformed checks have a delete strategy
	// On deletion they can either be marked healthy, unhealthy or left as is
	checkIDDeleteStrategyMap := make(map[string]string)

	// TODO: Use ctx with object here
	logPass := canary.IsTrace() || canary.IsDebug()
	logFail := canary.IsTrace() || canary.IsDebug()

	for _, result := range results {
		if logPass && result.Pass || logFail && !result.Pass {
			logger.Infof(result.String())
		}

		transformedChecksAdded := cache.PostgresCache.Add(pkg.FromV1(result.Canary, result.Check), pkg.FromResult(*result))
		transformedChecksCreated = append(transformedChecksCreated, transformedChecksAdded...)
		for _, checkID := range transformedChecksAdded {
			checkIDDeleteStrategyMap[checkID] = result.Check.GetTransformDeleteStrategy()
		}
	}

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
			if err := db.AddCheckStatuses(ctx, checkIDs, models.CheckHealthStatus(status)); err != nil {
				logger.Errorf("error adding statuses for transformed checks: %v", err)
			}

			if err := db.RemoveTransformedChecks(ctx, checkIDs); err != nil {
				logger.Errorf("error deleting transformed checks for canary %s: %v", canaryID, err)
			}
		}
	}
}
