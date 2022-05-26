package topology

import (
	"fmt"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/templating"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/kommons"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

func mergeComponentLookup(ctx *SystemContext, name string, spec *v1.CanarySpec) (pkg.Components, error) {
	components := pkg.Components{}
	results, err := lookup(ctx.Kommons, name, *spec)
	if err != nil {
		return nil, errors.Wrapf(err, "component lookup failed: %s", name)
	}
	if len(results) == 1 {
		fmt.Println(results[0])
		if err := json.Unmarshal([]byte(results[0].(string)), &components); err != nil {
			return nil, errors.Wrapf(err, "component lookup returned invalid json: %s", name)
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
	return components, nil
}

func lookupComponents(ctx *SystemContext, component v1.ComponentSpec) ([]*pkg.Component, error) {
	components := pkg.Components{}
	for _, childRaw := range component.Components {
		child := v1.ComponentSpec{}
		if err := json.Unmarshal([]byte(childRaw), &child); err != nil {
			return nil, err
		}
		children, err := lookupComponents(ctx, child)
		if err != nil {
			return nil, err
		}
		components = append(components, children...)
	}

	if component.Lookup == nil {
		components = append(components, pkg.NewComponent(component))
	} else {
		logger.Debugf("Looking up components for %s", component.Name)
		if children, err := mergeComponentLookup(ctx, component.Name, component.Lookup); err != nil {
			return nil, err
		} else {
			components = append(components, children...)
		}
	}

	for _, comp := range components {
		for _, property := range component.Properties {
			props, err := lookupProperty(ctx.WithComponents(&components, comp), property)
			if err != nil {
				return nil, errors.Wrapf(err, "property lookup failed: %s", property.Name)
			}
			comp.Properties = append(comp.Properties, props...)
		}

		if comp.Icon == "" && component.Icon != "" {
			comp.Icon = component.Icon
		}
		if comp.Lifecycle == "" && component.Lifecycle != "" {
			comp.Lifecycle = component.Lifecycle
		}
		if comp.ExternalId == "" && component.Id != nil {
			id, err := templating.Template(comp.GetAsEnvironment(), *component.Id)
			if err != nil {
				logger.Errorf("Failed to lookup id: %v", err)
			} else {
				comp.ExternalId = id
			}
		}
		if comp.ExternalId == "" {
			comp.ExternalId = comp.Name
		}
	}
	return components, nil
}

func lookup(client *kommons.Client, name string, spec v1.CanarySpec) ([]interface{}, error) {
	results := []interface{}{}
	for _, result := range checks.RunChecks(context.New(client, v1.NewCanaryFromSpec(name, spec))) {
		if result.Error != "" {
			logger.Errorf("error in running checks; check: %s wouldn't be persisted: %s", name, result.Error)
			return nil, nil
		}
		if result.Message != "" {
			results = append(results, result.Message)
		} else if result.Detail == nil {
			return nil, fmt.Errorf("no details returned by lookup, did you specify a display template?")
		} else {
			switch result.Detail.(type) {
			case []interface{}:
				results = append(results, result.Detail.([]interface{})...)
			case interface{}:
				results = append(results, result.Detail)
			default:
				return nil, fmt.Errorf("unknown type %T", result.Detail)
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
	if len(results) == 0 {
		return nil, nil
	}

	if len(results) == 1 {
		data := []byte(results[0].(string))
		if isComponentList(data) {
			components := pkg.Components{}
			err = json.Unmarshal([]byte(results[0].(string)), &components)
			if err != nil {
				return nil, err
			}
			for _, component := range components {
				found := ctx.Components.Find(component.Name)
				if found == nil {
					return nil, fmt.Errorf("component %s not found", component.Name)
				}
				for _, property := range component.Properties {
					foundProperty := found.Properties.Find(property.Name)
					if foundProperty == nil {
						return nil, fmt.Errorf("property %s not found", property.Name)
					}
					foundProperty.Merge(property)
				}
			}
			return nil, nil
		} else if isPropertyList(data) {
			properties := pkg.Properties{}
			err = json.Unmarshal([]byte(results[0].(string)), &properties)
			return properties, err
		} else {
			logger.Errorf("unknown type %T: %v", data, string(data))
			return nil, nil
		}
	}
	logger.Errorf("unknown type %T", results)

	return nil, nil
}

type TopologyRunOptions struct {
	*kommons.Client
	Depth     int
	Namespace string
}

func Run(opts TopologyRunOptions, s v1.SystemTemplate) []*pkg.System {
	logger.Debugf("Running topology %s depth=%d", s.Name, opts.Depth)
	if s.Namespace == "" {
		s.Namespace = opts.Namespace
	}
	ctx := NewSystemContext(opts.Client, s)
	var results []*pkg.System
	sys := &pkg.System{
		Object: pkg.Object{
			Name:      ctx.SystemAPI.Name,
			Namespace: ctx.SystemAPI.Namespace,
		},
		Labels:  ctx.SystemAPI.Labels,
		Tooltip: ctx.SystemAPI.Spec.Tooltip,
		Icon:    ctx.SystemAPI.Spec.Icon,
		Text:    ctx.SystemAPI.Spec.Text,
		Type:    ctx.SystemAPI.Spec.Type,
	}

	if opts.Depth > 0 {
		for _, comp := range ctx.SystemAPI.Spec.Components {
			components, err := lookupComponents(ctx, comp)
			// add systemTemplates lables to the components
			for _, component := range components {
				for key, value := range ctx.SystemAPI.Labels {
					// don't overwrite the component labels
					if _, isPresent := component.Labels[key]; !isPresent {
						component.Labels[key] = value
					}
				}
			}
			if err != nil {
				logger.Errorf("Error looking up component %s: %s", comp.Name, err)
				continue
			}
			if comp.Lookup == nil {
				sys.Components = append(sys.Components, components...)
				continue
			}
			group := pkg.NewComponent(comp)
			group.Components = append(group.Components, components...)
			if comp.Summary == nil {
				group.Summary = group.Components.Summarize()
			} else {
				group.Summary = *comp.Summary
			}
			group.Status = group.Summary.GetStatus()
			sys.Components = append(sys.Components, group)
		}
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
	sys.Summary = sys.Components.Summarize()
	if sys.ID.String() == "" && ctx.SystemAPI.Spec.Id != nil {
		id, err := templating.Template(sys.GetAsEnvironment(), *ctx.SystemAPI.Spec.Id)
		if err != nil {
			logger.Errorf("Failed to lookup id: %v", err)
		} else {
			sys.ID, _ = uuid.Parse(id)
		}
	}

	if sys.ID.String() == "" {
		sys.ID, _ = uuid.Parse(sys.Name)
	}
	sys.Status = sys.Summary.GetStatus()
	// if logger.IsTraceEnabled() {
	logger.Debugf(sys.Components.Debug(""))
	// }
	results = append(results, sys)
	logger.Infof("%s id=%s status=%s", sys.Name, sys.ID, sys.Status)
	return results
}

// Fetches and updates the selected component for components
func ComponentRun() {
	logger.Debugf("Syncing Component Relationships")
	components, err := db.GetAllComponentWithSelectors()
	if err != nil {
		logger.Errorf("error getting components: %v", err)
		return
	}
	for _, component := range components {
		comps, err := db.GetComponensWithSelectors(component.Selectors)
		if err != nil {
			logger.Errorf("error getting components with selectors: %s. err: %v", component.Selectors, err)
			continue
		}
		relationships, err := db.GetComponentRelationships(component.ID, comps)
		if err != nil {
			logger.Errorf("error getting relationships: %v", err)
			continue
		}
		err = db.PersisComponentRelationships(relationships)
		if err != nil {
			logger.Errorf("error persisting relationships: %v", err)
			continue
		}
	}
}
