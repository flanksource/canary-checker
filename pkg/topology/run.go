package topology

import (
	"encoding/json"
	"fmt"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/templating"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/kommons"
)

func lookupComponents(ctx *SystemContext, component v1.ComponentSpec) ([]*pkg.Component, error) {
	components := pkg.Components{}
	if component.Lookup == nil {
		components = append(components, pkg.NewComponent(component))
	} else {
		results, err := lookup(ctx.Kommons, component.Name, *component.Lookup)
		if err != nil {
			return nil, err
		}
		if len(results) == 1 {
			if err := json.Unmarshal([]byte(results[0].(string)), &components); err != nil {
				return nil, err
			}
		} else {
			// the property returned a list of new properties
			for _, result := range results {
				var p pkg.Component
				data, err := json.Marshal(result)
				if err != nil {
					return nil, err
				}
				if err := json.Unmarshal(data, &p); err != nil {
					return nil, err
				}
				components = append(components, &p)
			}
		}
	}

	for _, comp := range components {
		for _, property := range component.Properties {
			props, err := lookupProperty(ctx.WithComponents(&components, comp), property)
			if err != nil {
				return nil, err
			}
			comp.Properties = append(comp.Properties, props...)
		}
		if comp.Type == "" && component.Type != "" {
			comp.Type = component.Type
		}
		if comp.Icon == "" && component.Icon != "" {
			comp.Icon = component.Icon
		}
		if comp.Lifecycle == "" && component.Lifecycle != "" {
			comp.Lifecycle = component.Lifecycle
		}
		if comp.Id == "" && component.Id != nil {
			id, err := templating.Template(comp.GetAsEnvironment(), *component.Id)
			if err != nil {
				logger.Errorf("Failed to lookup id: %v", err)
			} else {
				comp.Id = id
			}
		}
		if comp.Id == "" {
			comp.Id = comp.Name
		}
	}

	return components, nil
}

func template(ctx *SystemContext, tpl v1.Template) (string, error) {
	return templating.Template(ctx.Environment, tpl)
}

func lookup(client *kommons.Client, name string, spec v1.CanarySpec) ([]interface{}, error) {
	results := []interface{}{}
	for _, result := range checks.RunChecks(context.New(client, v1.NewCanaryFromSpec(name, spec))) {
		if result.Error != "" {
			return nil, fmt.Errorf("%s", result.Error)
		}
		if result.Message != "" {
			results = append(results, result.Message)
		} else {
			switch result.Detail.(type) {
			case []interface{}:
				results = append(results, result.Detail.([]interface{})...)
			case interface{}:
				results = append(results, result.Detail.(interface{}))
			default:
				return nil, fmt.Errorf("Unknown type %T", result.Detail)
			}
		}
	}
	return results, nil
}

func lookupProperty(ctx *SystemContext, property *v1.Property) (pkg.Properties, error) {
	prop := pkg.NewProperty(*property)
	if property.Lookup == nil {
		return pkg.Properties{prop}, nil
	}

	results, err := lookup(ctx.Kommons, property.Name, *property.Lookup)
	if err != nil {
		return nil, err
	}
	if len(results) == 1 {
		data := []byte(results[0].(string))
		if isComponentList(data) {
			components := pkg.Components{}
			err = json.Unmarshal([]byte(results[0].(string)), &components)
			for _, component := range components {
				found := ctx.Components.Find(component.Name)
				if found == nil {
					return nil, fmt.Errorf("Component %s not found", component.Name)
				}
				for _, property := range component.Properties {
					foundProperty := found.Properties.Find(property.Name)
					if foundProperty == nil {
						return nil, fmt.Errorf("Property %s not found", property.Name)
					}
					foundProperty.Merge(property)
				}
			}
			return nil, nil
		} else if isPropertyList(data) {
			properties := pkg.Properties{}
			err = json.Unmarshal([]byte(results[0].(string)), &properties)
			return properties, err
		}
	}

	return nil, fmt.Errorf("Unknown type %T: %v", results[0])
}

func Run(client *kommons.Client, s v1.System) []*pkg.System {
	ctx := NewSystemContext(client, s)
	var results []*pkg.System
	sys := &pkg.System{
		Object: pkg.Object{
			Name:      ctx.SystemAPI.Name,
			Namespace: ctx.SystemAPI.Namespace,
		},
		Tooltip: ctx.SystemAPI.Spec.Tooltip,
		Icon:    ctx.SystemAPI.Spec.Icon,
		Text:    ctx.SystemAPI.Spec.Text,
	}

	for _, comp := range ctx.SystemAPI.Spec.Components {
		components, err := lookupComponents(ctx, comp)

		if err != nil {
			logger.Errorf("Error looking up component %s: %s", comp.Name, err)
			continue
		}
		group := &pkg.Component{
			Name: comp.Name,
			Icon: comp.Icon,
		}
		for _, component := range components {
			group.Components = append(group.Components, component)
		}
		group.Summary = count(group.Components)
		group.Status = group.Summary.GetStatus()
		sys.Components = append(sys.Components, group)
	}
	ctx.System = sys
	ctx.Components = &sys.Components

	for _, property := range ctx.SystemAPI.Spec.Properties {
		props, err := lookupProperty(ctx, &property)
		if err != nil {
			logger.Errorf("Failed to lookup property %s: %v", property.Name, err)
		} else {
			sys.Properties = append(sys.Properties, props...)
		}
	}
	sys.Summary = count(sys.Components)
	if sys.Id == "" && ctx.SystemAPI.Spec.Id != nil {
		id, err := templating.Template(sys.GetAsEnvironment(), *ctx.SystemAPI.Spec.Id)
		if err != nil {
			logger.Errorf("Failed to lookup id: %v", err)
		} else {
			sys.Id = id
		}
	}

	if sys.Id == "" {
		sys.Id = sys.Name
	}
	sys.Status = sys.Summary.GetStatus()
	results = append(results, sys)
	return results
}
