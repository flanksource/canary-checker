package checks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"

	gotemplate "text/template"

	"github.com/hairyhenderson/gomplate/v3"
)

func RunChecks(ctx *context.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	ctx.Canary.Spec.SetSQLDrivers()
	for _, c := range All {
		time := GetDeadline(ctx.Canary)
		ctx, cancel := ctx.WithDeadline(time)
		defer cancel()
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
					if tpl.Template == "" {
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

func template(ctx *context.Context, template v1.Template) (string, error) {
	if template.Template != "" {
		tpl := gotemplate.New("")

		funcs := gomplate.Funcs(nil)
		funcs["humanizeBytes"] = mb
		funcs["humanizeTime"] = humanize.Time
		funcs["ftoa"] = humanize.Ftoa
		tpl, err := tpl.Funcs(funcs).Parse(template.Template)
		if err != nil {
			return "", err
		}

		// marshal data from interface{} to map[string]interface{}
		data, _ := json.Marshal(ctx.Environment)
		unstructured := make(map[string]interface{})
		if err := json.Unmarshal(data, &unstructured); err != nil {
			return "", err
		}

		var buf bytes.Buffer
		if err := tpl.Execute(&buf, unstructured); err != nil {
			return "", fmt.Errorf("error executing template %s: %v", strings.Split(template.Template, "\n")[0], err)
		}
		return buf.String(), nil
	}
	return "", nil
}
