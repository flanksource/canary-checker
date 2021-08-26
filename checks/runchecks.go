package checks

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"gopkg.in/flanksource/yaml.v3"

	gotemplate "text/template"

	"github.com/hairyhenderson/gomplate/v3"
)

func RunChecks(ctx *context.Context) []*pkg.CheckResult {
	var results []*pkg.CheckResult
	ctx.Canary.Spec.SetSQLDrivers()
	for _, c := range All {
		result := c.Run(ctx)
		for _, r := range result {
			if r != nil {
				switch r.Check.(type) {
				case v1.DisplayTemplate:
					message, err := template(ctx.New(r.Data), r.Check.(v1.DisplayTemplate).GetDisplayTemplate())
					if err != nil {
						r.ErrorMessage(err)
					} else {
						r.ResultMessage(message)
					}
				}
				switch r.Check.(type) {
				case v1.TestFunction:
					tpl := r.Check.(v1.TestFunction).GetTestFunction()
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
				fmt.Printf("%s \t%s\t\n", time.Now().Format(time.RFC3339), r.String())
				results = append(results, r)
			}
		}
	}
	return results
}

func template(ctx *context.Context, template v1.Template) (string, error) {
	if template.Template != "" {
		tpl := gotemplate.New("")

		tpl, err := tpl.Funcs(gomplate.Funcs(nil)).Parse(template.Template)
		if err != nil {
			return "", err
		}

		var buf bytes.Buffer
		data, _ := yaml.Marshal(ctx.Environment)
		unstructured := make(map[string]interface{})
		if err := yaml.Unmarshal(data, &unstructured); err != nil {
			return "", err
		}
		if err := tpl.Execute(&buf, unstructured); err != nil {
			return "", fmt.Errorf("error executing template %s: %v", strings.Split(template.Template, "\n")[0], err)
		}
		return buf.String(), nil
	}
	return "", nil
}
