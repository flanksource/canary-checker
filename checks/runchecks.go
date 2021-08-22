package checks

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/api/external"
	v1 "github.com/flanksource/canary-checker/api/v1"

	"github.com/flanksource/canary-checker/pkg"

	gotemplate "text/template"

	"github.com/hairyhenderson/gomplate/v3"
)

func RunChecks(ctx *context.Context, canary v1.Canary) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	ctx.Canary.Spec.SetSQLDrivers()
	for _, c := range All {
		result := c.Run(ctx)
		for _, r := range result {
			if r != nil {
				switch r.Check.(type) {
				case external.DisplayTemplate:
					message, err := template(ctx.New(r.Data), r.Check, r.Check.(external.DisplayTemplate).GetDisplayTemplate())
					if err != nil {
						r.ErrorMessage(err)
					} else {
						r.ResultMessage(message)
					}
				case external.TestFunction:
					message, err := template(ctx.New(r.Data), r.Check, r.Check.(external.TestFunction).GetTestFunction())
					if err != nil {
						r.ErrorMessage(err)
					} else if message != "true" {
						r.Failf("")
					}
				}
				results = append(results, r)
			}
		}
	}
	return results
}

func template(ctx *context.Context, check external.Check, template external.Template) (string, error) {

	if template.Template != "" {
		tpl := gotemplate.New("")

		tpl, err := tpl.Funcs(gomplate.Funcs(nil)).Parse(template.Template)
		if err != nil {
			return "", err
		}

		var buf bytes.Buffer
		if err := tpl.Execute(&buf, ctx.Environment); err != nil {
			return "", fmt.Errorf("error executing template %s: %v", strings.Split(template.Template, "\n")[0], err)
		}
		return buf.String(), nil
	}
	return "", nil
}
