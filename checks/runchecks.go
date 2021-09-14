package checks

import (
	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
)

func RunChecks(ctx *context.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	ctx.Canary.Spec.SetSQLDrivers()
	for _, c := range All {
		// FIXME: this doesn't work correct with DNS,
		// t := GetDeadline(ctx.Canary)
		// ctx, cancel := ctx.WithDeadline(t)
		// defer cancel()

		result := c.Run(ctx)
		for _, r := range result {
			if r != nil {
				if r.Duration == 0 && r.GetDuration() > 0 {
					r.Duration = r.GetDuration()
				}
				switch v := r.Check.(type) {
				case v1.DisplayTemplate:
					message, err := template(ctx.New(r.Data), v.GetDisplayTemplate())
					if err != nil {
						r.ErrorMessage(err)
					} else {
						r.ResultMessage(message)
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
						r.Failf("")
					} else {
						ctx.Logger.Tracef("%s return %s", tpl, message)
					}
				}
				results = append(results, r)
			}
		}
	}
	return results
}
