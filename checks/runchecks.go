package checks

import (
	"time"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
)

func RunChecks(ctx *context.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult
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
		tpl := v.GetTestFunction()
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
