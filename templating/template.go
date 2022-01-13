package templating

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	gotemplate "text/template"

	"github.com/antonmedv/expr"
	"github.com/pkg/errors"
	"github.com/robertkrimen/otto"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	_ "github.com/flanksource/canary-checker/templating/js"
	"github.com/flanksource/commons/files"
	"github.com/flanksource/commons/text"
	"github.com/robertkrimen/otto/registry"
	_ "github.com/robertkrimen/otto/underscore"
)

func LoadSharedLibrary(source string) error {
	if files.Exists(source) {
		data, err := ioutil.ReadFile(source)
		if err != nil {
			return fmt.Errorf("failed to read shared library %s: %s", source, err)
		}
		registry.Register(func() string { return string(data) })
	}
	return fmt.Errorf("shared library %s not found", source)
}

func Template(ctx *context.Context, template v1.Template) (string, error) {
	// javascript
	if template.Javascript != "" {
		// FIXME: whitelist allowed files
		vm := otto.New()
		for k, v := range ctx.Environment {
			if err := vm.Set(k, v); err != nil {
				return "", errors.Wrapf(err, "error setting %s", k)
			}
		}
		out, err := vm.Run(template.Javascript)
		if err != nil {
			return "", errors.Wrapf(err, "failed to run javascript")
		}

		if s, err := out.ToString(); err != nil {
			return "", errors.Wrapf(err, "failed to cast output to string")
		} else {
			return s, nil
		}
	}

	// gotemplate
	if template.Template != "" {
		tpl := gotemplate.New("")
		tpl, err := tpl.Funcs(text.GetTemplateFuncs()).Parse(template.Template)
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
		return strings.TrimSpace(buf.String()), nil
	}

	// exprv
	if template.Expression != "" {
		program, err := expr.Compile(template.Expression, text.MakeExpressionOptions(ctx.Environment)...)
		if err != nil {
			return "", err
		}
		output, err := expr.Run(program, text.MakeExpressionEnvs(ctx.Environment))
		if err != nil {
			return "", err
		}
		return fmt.Sprint(output), nil
	}
	return "", nil
}
