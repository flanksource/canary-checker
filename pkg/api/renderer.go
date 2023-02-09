package api

import (
	"bytes"
	"fmt"
	"net/http"
	"text/template"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/topology"
	babel "github.com/jvatic/goja-babel"
	"github.com/labstack/echo/v4"
)

var jsComponentTpl *template.Template

type component struct {
	Name string
	JS   string
}

// GetCustomRenderer returns an application/javascript HTTP response
// with custom components and a registry.
// This registry needs to be used to select custom components
// for rendering of properties and cards.
func GetCustomRenderer(ctx echo.Context) error {
	// 1. Read the template of the topology
	params := topology.NewTopologyParams(ctx.QueryParams())
	results, err := topology.QueryRenderComponents(ctx.Request().Context(), params.ID)
	if err != nil {
		return errorResonse(ctx, err, http.StatusBadRequest)
	}

	// 2. Create a registry of all the components
	var components = make(map[string]component)
	for _, r := range results {
		if err := compileComponents(components, r.Components, false); err != nil {
			return errorResonse(ctx, err, http.StatusInternalServerError)
		}

		if err := compileComponents(components, r.Properties, true); err != nil {
			return errorResonse(ctx, err, http.StatusInternalServerError)
		}
	}

	registryResp, err := renderComponent(components)
	if err != nil {
		return errorResonse(ctx, err, http.StatusInternalServerError)
	}

	ctx.Response().WriteHeader(http.StatusOK)
	ctx.Response().Header().Add("Content-Type", "application/javascript")
	ctx.Response().Write([]byte(registryResp))

	return nil
}

func compileComponents(output map[string]component, components []pkg.RenderComponent, isProp bool) error {
	babel.Init(len(components))
	for _, c := range components {
		res, err := babel.TransformString(c.JSX, map[string]interface{}{
			"plugins": []string{
				"transform-react-jsx",
				"transform-block-scoping",
			},
		})
		if err != nil {
			return err
		}

		output[componentKey(isProp, c)] = component{
			Name: c.Name,
			JS:   res,
		}
	}

	return nil
}

func componentKey(isProp bool, c pkg.RenderComponent) string {
	if isProp {
		return fmt.Sprintf("property-%s-name", c.Name)
	}

	return fmt.Sprintf("component-%s-name", c.Name)
}

func renderComponent(components map[string]component) (string, error) {
	var buf bytes.Buffer
	if err := jsComponentTpl.Execute(&buf, components); err != nil {
		return "", err
	}

	return buf.String(), nil
}

const jsComponentRegistryTpl = `
{{range $k, $v := .}}
{{$v.JS}}
{{end}}

const componentRegistry = {
	{{range $k, $v := .}}"{{$k}}": {{$v.Name}},
	{{end}}
}`

func init() {
	tpl, err := template.New("registry").Parse(jsComponentRegistryTpl)
	if err != nil {
		panic(err)
	}

	jsComponentTpl = tpl
}
