package checks

import (
	"fmt"
	"time"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/commons/logger"
)

func RunChecks(ctx *context.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	ctx.Canary.Spec.SetSQLDrivers()
	// for _, check := range ctx.Canary.Spec.GetAllChecks() {

	// }
	checks := ctx.Canary.Spec.GetAllChecks()
	for _, c := range All {
		// FIXME: this doesn't work correct with DNS,
		// t := GetDeadline(ctx.Canary)
		// ctx, cancel := ctx.WithDeadline(t)
		// defer cancel()
		if Checks(checks).Includes(c) {
			result := c.Run(ctx)
			for _, r := range result {
				if r != nil {
					fmt.Println("result:")
					fmt.Printf("%+v\n\n\n", r)
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
							if message != "false" {
								r.Failf("expecting either 'true' or 'false' but got '%v'", message)
							} else {
								r.Failf("")
							}
						} else {
							ctx.Logger.Tracef("%s return %s", tpl, message)
						}
					}
					results = append(results, r)
				}
			}
		}
	}
	if ctx.Canary.Spec.ResultMode != "" {
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
			logger.Debugf("Unknown result mode: %s", ctx.Canary.Spec.ResultMode)
			return results
		}
	}
	return results
}
