package topology

import (
	"fmt"

	"github.com/PaesslerAG/jsonpath"
	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/db/types"
	"github.com/flanksource/canary-checker/templating"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/kommons"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

func mergeComponentLookup(ctx *ComponentContext, name string, spec *v1.CanarySpec) (pkg.Components, error) {
	components := pkg.Components{}
	results, err := lookup(ctx.Kommons, name, *spec)
	if err != nil {
		return nil, errors.Wrapf(err, "component lookup failed: %s", name)
	}
	if len(results) == 1 {
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

func lookupComponents(ctx *ComponentContext, component v1.ComponentSpec) ([]*pkg.Component, error) {
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

		// Lookup config and populate properties
		for _, property := range component.Properties {
			if property.ConfigLookup == nil {
				continue
			}
			prop, err := lookupConfig(property, comp.Properties)
			if err != nil {
				return nil, errors.Wrapf(err, "property config lookup failed: %s", property.Name)
			}
			comp.Properties = append(comp.Properties, prop)
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

func lookupConfig(property *v1.Property, sisterProperties pkg.Properties) (*pkg.Property, error) {
	prop := pkg.NewProperty(*property)

	configName := property.ConfigLookup.Config.Name
	if property.ConfigLookup.ID != "" {
		// Lookup in the same properties
		for _, prop := range sisterProperties {
			if prop.Name == property.ConfigLookup.ID {
				configName = fmt.Sprintf("%v", prop.GetValue())
				break
			}
		}
	}

	pkgConfig := pkg.NewConfig(property.ConfigLookup.Config)
	pkgConfig.Name = configName
	config, err := db.FindConfig(*pkgConfig)
	if err != nil {
		return prop, err
	}

	var v interface{}
	rawJSON, err := config.Spec.MarshalJSON()
	if err != nil {
		return prop, err
	}
	err = json.Unmarshal(rawJSON, &v)
	if err != nil {
		return prop, err
	}
	result, err := jsonpath.Get(property.ConfigLookup.Field, v)
	if err != nil {
		return prop, err
	}

	prop.Text = fmt.Sprintf("%s", result)
	return prop, nil
}

func lookupProperty(ctx *ComponentContext, property *v1.Property) (pkg.Properties, error) {
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

func Run(opts TopologyRunOptions, s v1.SystemTemplate) []*pkg.Component {
	logger.Debugf("Running topology %s depth=%d", s.Name, opts.Depth)
	if s.Namespace == "" {
		s.Namespace = opts.Namespace
	}
	ctx := NewComponentContext(opts.Client, s)
	var results pkg.Components
	systemTemplateConfigs := pkg.NewConfigs(ctx.SystemTemplate.Spec.Configs)
	component := &pkg.Component{
		Name:            ctx.SystemTemplate.Spec.Text,
		Namespace:       ctx.SystemTemplate.GetNamespace(),
		Labels:          ctx.SystemTemplate.Labels,
		Tooltip:         ctx.SystemTemplate.Spec.Tooltip,
		Icon:            ctx.SystemTemplate.Spec.Icon,
		Text:            ctx.SystemTemplate.Spec.Text,
		Type:            ctx.SystemTemplate.Spec.Type,
		Schedule:        ctx.SystemTemplate.Spec.Schedule,
		Configs:         systemTemplateConfigs,
		ComponentChecks: ctx.SystemTemplate.Spec.ComponentChecks,
	}

	if component.Name == "" {
		component.Name = ctx.SystemTemplate.Name
	}

	if opts.Depth > 0 {
		for _, comp := range ctx.SystemTemplate.Spec.Components {
			components, err := lookupComponents(ctx, comp)
			if err != nil {
				logger.Errorf("Error looking up component %s: %s", comp.Name, err)
				continue
			}
			// add systemTemplates labels to the components
			for _, component := range components {
				if component.Labels == nil {
					component.Labels = make(types.JSONStringMap)
				}
				for key, value := range ctx.SystemTemplate.Labels {
					// don't overwrite the component labels
					if _, isPresent := component.Labels[key]; !isPresent {
						component.Labels[key] = value
					}
				}
			}
			if comp.Lookup == nil {
				component.Components = append(component.Components, components...)
				continue
			}
			group := pkg.NewComponent(comp)
			group.Components = append(group.Components, components...)
			if len(group.Components) > 0 {
				group.Summary = group.Components.Summarize()
			}

			group.Status = pkg.ComponentStatus(group.Summary.GetStatus())
			component.Components = append(component.Components, group)
		}
	}
	ctx.Components = &component.Components

	for _, property := range ctx.SystemTemplate.Spec.Properties {
		props, err := lookupProperty(ctx, &property)
		if err != nil {
			logger.Errorf("Failed to lookup property %s: %v", property.Name, err)
		} else {
			component.Properties = append(component.Properties, props...)
		}
	}
	if len(component.Components) > 0 {
		component.Summary = component.Components.Summarize()
	}
	if component.ID.String() == "" && ctx.SystemTemplate.Spec.Id != nil {
		id, err := templating.Template(component.GetAsEnvironment(), *ctx.SystemTemplate.Spec.Id)
		if err != nil {
			logger.Errorf("Failed to lookup id: %v", err)
		} else {
			component.ID, _ = uuid.Parse(id)
		}
	}

	if component.ID.String() == "" {
		component.ID, _ = uuid.Parse(component.Name)
	}

	if component.ExternalId == "" {
		component.ExternalId = component.Name
	}

	component.Status = pkg.ComponentStatus(component.Summary.GetStatus())
	// if logger.IsTraceEnabled() {
	logger.Debugf(component.Components.Debug(""))
	// }
	results = append(results, component)
	logger.Infof("%s id=%s external_id=%s status=%s", component.Name, component.ID, component.ExternalId, component.Status)
	for _, c := range results.Walk() {
		c.Namespace = ctx.SystemTemplate.GetNamespace()
		c.Schedule = ctx.SystemTemplate.Spec.Schedule
	}
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
		comps, err := db.GetComponentsWithSelectors(component.Selectors)
		if err != nil {
			logger.Errorf("error getting components with selectors: %s. err: %v", component.Selectors, err)
			continue
		}
		relationships, err := db.GetComponentRelationships(component.ID, component.Path, comps)
		if err != nil {
			logger.Errorf("error getting relationships: %v", err)
			continue
		}
		err = db.PersistComponentRelationships(relationships)
		if err != nil {
			logger.Errorf("error persisting relationships: %v", err)
			continue
		}
	}
}

func ComponentStatusSummarySync() {
	logger.Debugf("Syncing Status and Summary for components")
	components, err := Query(TopologyParams{
		Depth: 0,
	})
	if err != nil {
		logger.Errorf("error getting components: %v", err)
		return
	}
	for _, component := range components.Walk() {
		_, err = db.UpdateStatusAndSummaryForComponent(component.ID, component.Status, component.Summary)
		if err != nil {
			logger.Errorf("error persisting component: %v", err)
			continue
		}
	}
}
