package checks

import (
	"database/sql"
	"time"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/commons/logger"
)

func RunChecks(ctx *context.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult

	// Check if canary is not marked deleted in DB
	if db.Gorm != nil && ctx.Canary.GetPersistedID() != "" {
		var deletedAt sql.NullTime
		err := db.Gorm.Table("canaries").Select("deleted_at").Where("id = ?", ctx.Canary.GetPersistedID()).Scan(&deletedAt).Error
		if err == nil && deletedAt.Valid {
			return results
		}
	}

	checks := ctx.Canary.Spec.GetAllChecks()
	ctx.Debugf("[%s] checking %d checks", ctx.Canary.Name, len(checks))
	for _, c := range All {
		// FIXME: this doesn't work correct with DNS,
		// t := GetDeadline(ctx.Canary)
		// ctx, cancel := ctx.WithDeadline(t)
		// defer cancel()
		if Checks(checks).Includes(c) {
			result := c.Run(ctx)
			results = append(results, transformResults(ctx, result)...)
		}
	}

	return processResults(ctx, results)
}

func transformResults(ctx *context.Context, in []*pkg.CheckResult) (out []*pkg.CheckResult) {
	for _, r := range in {
		transformed, err := transform(ctx, r)
		if err != nil {
			r.Failf("transformation failure: %v", err)
			out = append(out, r)
		} else {
			for _, t := range transformed {
				out = append(out, processTemplates(ctx, t))
			}
		}
	}
	return out
}

func processResults(ctx *context.Context, results []*pkg.CheckResult) []*pkg.CheckResult {
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
			message, err := template(ctx.New(r.Data), v.GetDisplayTemplate())
			if err != nil {
				r.ErrorMessage(err)
			} else {
				r.ResultMessage(message)
			}
		}
	}

	switch v := r.Check.(type) {
	case v1.TestFunction:
		data := map[string]any{"duration": r.Duration}
		r.Severity = measureTestSeverity(ctx.New(data), v.GetTestThreshold())

		tpl := v.GetTestTemplate()
		if tpl.IsEmpty() {
			break
		}

		message, err := template(ctx.New(r.Data), tpl)
		if err != nil {
			r.ErrorMessage(err)
		} else if message != "true" {
			if message != "false" {
				r.Failf("expecting either 'true' or 'false' but got '%v'", message)
			} else {
				r.Failf("Test expression failed. Expecting true from: %v", tpl.Expression)
			}
		}
	}

	return r
}

func measureTestSeverity(ctx *context.Context, threshold *v1.TestThreshold) pkg.Severity {
	if threshold == nil {
		return pkg.SeverityInfo
	}

	thresholds := []struct {
		severity pkg.Severity
		expr     string
	}{
		{pkg.SeverityCritical, threshold.Critical},
		{pkg.SeverityHigh, threshold.High},
		{pkg.SeverityMedium, threshold.Medium},
		{pkg.SeverityLow, threshold.Low},
		{pkg.SeverityInfo, threshold.Info},
	}

	for _, t := range thresholds {
		if res, err := template(ctx, v1.Template{Expression: t.expr}); err != nil {
			return pkg.SeverityInfo
		} else if res == "true" {
			return t.severity
		}
	}

	return pkg.SeverityInfo
}
