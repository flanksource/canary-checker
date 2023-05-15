package topology

import (
	"fmt"
	"time"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/canary-checker/templating"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/models"
	dutyTypes "github.com/flanksource/duty/types"
	"github.com/flanksource/kommons"
	"github.com/google/uuid"
	jsontime "github.com/liamylian/jsontime/v2/v2"
	"github.com/pkg/errors"
)

var json = jsontime.ConfigWithCustomTimeFormat

func mergeComponentLookup(ctx *ComponentContext, component *v1.ComponentSpec, spec *v1.CanarySpec) (models.Components, error) {
	name := component.Name
	components := models.Components{}
	results, err := lookup(ctx, name, *spec)
	if err != nil {
		return nil, errors.Wrapf(err, "component lookup failed: %s", component)
	}
	if len(results) == 1 {
		if err := json.Unmarshal([]byte(results[0].(string)), &components); err != nil {
			return nil, errors.Wrapf(err, "component lookup returned invalid json: %s", component)
		}
	} else {
		// the property returned a list of new properties
		for _, result := range results {
			var p models.Component
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
	for _, _c := range components {
		if err = forEachComponent(ctx, component, _c); err != nil {
			return nil, err
		}
	}
	return components, nil
}

func forEachComponent(ctx *ComponentContext, spec *v1.ComponentSpec, component *models.Component) error {
	logger.Debugf("[%s] %s", component.Name, spec.ForEach)
	if spec.ForEach == nil {
		return nil
	}
	ctx.SetCurrentComponent(component)

	for _, property := range spec.ForEach.Properties {
		prop := property
		if err := ctx.TemplateProperty(&prop); err != nil {
			return err
		}
		props, err := lookupProperty(ctx, &prop)
		if err != nil {
			logger.Errorf("Failed to lookup property %s: %v", property.Name, err)
		} else {
			component.Properties = append(component.Properties, props...)
		}
	}
	ctx.SetCurrentComponent(component) // component properties may have changed

	for _, childComponent := range spec.ForEach.Components {
		child := childComponent
		if err := ctx.TemplateComponent(&child); err != nil {
			return err
		}
		children, err := lookupComponents(ctx, child)
		if err != nil {
			logger.Errorf("Failed to lookup components %s: %v", child, err)
		} else {
			component.Components = append(component.Components, children...)
		}
	}

	for _, childConfig := range spec.ForEach.Configs {
		child := childConfig
		if err := ctx.TemplateConfig(&child); err != nil {
			logger.Errorf("Failed to lookup configs %s: %v", child, err)
		} else {
			component.Configs = append(component.Configs, child.ToModel())
		}
	}

	for _, _selector := range spec.ForEach.Selectors {
		selector := _selector
		if err := ctx.TemplateStruct(&selector); err != nil {
			logger.Errorf("Failed to lookup selectors %s: %v", selector, err)
		} else {
			component.Selectors = append(component.Selectors, selector)
		}
	}

	return nil
}

func lookupComponents(ctx *ComponentContext, component v1.ComponentSpec) (components models.Components, err error) {
	// A component can have either a lookup or child components
	// A lookup will translates flatly into a list of components

	if component.Lookup != nil {
		var lookedUpComponents models.Components
		logger.Debugf("Looking up components for %s => %s", component, component.ForEach)
		if lookedUpComponents, err = mergeComponentLookup(ctx, &component, component.Lookup); err != nil {
			return nil, err
		}
		components = append(components, lookedUpComponents...)
	} else {
		var childComponents models.Components
		for _, child := range component.Components {
			children, err := lookupComponents(ctx, v1.ComponentSpec(child))
			if err != nil {
				return nil, err
			}
			childComponents = append(childComponents, children...)
		}

		pkgComp := pkg.NewComponent(component)
		pkgComp.Components = childComponents
		components = append(components, pkgComp)
	}

	for _, comp := range components {
		for _, property := range component.Properties {
			props, err := lookupProperty(ctx.WithComponents(&components, comp), property)
			if err != nil {
				return nil, errors.Wrapf(err, "property lookup failed: %s", property)
			}
			comp.Properties = append(comp.Properties, props...)
		}

		if comp.Icon == "" {
			comp.Icon = component.Icon
		}
		if comp.Lifecycle == "" {
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

func lookup(ctx *ComponentContext, name string, spec v1.CanarySpec) ([]interface{}, error) {
	results := []interface{}{}
	canaryCtx := &context.Context{
		Context:     ctx,
		Canary:      v1.NewCanaryFromSpec(name, spec),
		Namespace:   ctx.Namespace,
		Kommons:     ctx.Kommons,
		Environment: ctx.Environment,
		Logger:      ctx.Logger,
	}
	for _, result := range checks.RunChecks(canaryCtx) {
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

func lookupConfig(ctx *ComponentContext, property *v1.Property) (*models.Property, error) {
	prop := property.ToModel()

	logger.Debugf("Looking up config for %s => %s", property.Name, property.ConfigLookup.Config)
	if property.ConfigLookup.Config == nil {
		return nil, fmt.Errorf("empty config in configLookup")
	}
	if property.ConfigLookup.Display.Template.IsEmpty() {
		return prop, fmt.Errorf("configLookup cannot have empty display")
	}

	configName := property.ConfigLookup.Config.Name
	if property.ConfigLookup.ID != "" {
		if ctx.CurrentComponent != nil {
			// Lookup in the same properties
			for _, prop := range ctx.CurrentComponent.Properties {
				if prop.Name == property.ConfigLookup.ID {
					configName = fmt.Sprintf("%v", prop.GetValue())
					break
				}
			}
		}
	}

	config := property.ConfigLookup.Config
	if err := ctx.TemplateConfig(config); err != nil {
		return nil, err
	}

	configQuery := config.ToModel()
	configQuery.Name = configName
	_config, err := db.FindConfig(configQuery)
	if err != nil {
		return prop, err
	} else if _config == nil {
		return prop, nil
	}

	configMap, err := _config.ConfigJSONStringMap()
	if err != nil {
		return prop, fmt.Errorf("failed to marshal config: %w", err)
	}

	templateEnv := map[string]any{
		"config": configMap,
		"tags":   _config.Tags.ToMapStringAny(),
	}
	prop.Text, err = templating.Template(templateEnv, property.ConfigLookup.Display.Template)
	return prop, err
}

func lookupProperty(ctx *ComponentContext, property *v1.Property) (models.Properties, error) {
	if property == nil {
		return nil, nil
	}

	prop := property.ToModel()

	if property.ConfigLookup != nil {
		prop, err := lookupConfig(ctx, property)
		if err != nil {
			return nil, errors.Wrapf(err, "property config lookup failed: %s", property)
		}
		return models.Properties{prop}, nil
	}
	if property.Lookup == nil {
		return models.Properties{prop}, nil
	}

	results, err := lookup(ctx, property.Name, *property.Lookup)
	if err != nil {
		return nil, err
	}
	if len(results) != 1 {
		return nil, nil
	}

	var dataStr string
	var ok bool
	if dataStr, ok = results[0].(string); !ok {
		return nil, fmt.Errorf("unknown property type %T", results)
	}
	data := []byte(dataStr)
	if isComponentList(data) {
		// the result is map of components to properties, find the existing component
		// and then merge the property into it
		components := models.Components{}
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
		properties := models.Properties{}
		err = json.Unmarshal([]byte(results[0].(string)), &properties)
		return properties, err
	} else {
		prop.Text = string(data)
		return models.Properties{prop}, nil
	}
}

type TopologyRunOptions struct {
	*kommons.Client
	Depth     int
	Namespace string
}

func Run(opts TopologyRunOptions, s v1.Topology) []*models.Component {
	if s.Namespace == "" {
		s.Namespace = opts.Namespace
	}
	logger.Debugf("Running topology %s/%s depth=%d", s.Namespace, s.Name, opts.Depth)
	ctx := NewComponentContext(opts.Client, s)
	var results models.Components
	component := &models.Component{
		Name:      ctx.Topology.Spec.Text,
		Namespace: ctx.Topology.GetNamespace(),
		Labels:    ctx.Topology.Labels,
		Tooltip:   ctx.Topology.Spec.Tooltip,
		Icon:      ctx.Topology.Spec.Icon,
		Text:      ctx.Topology.Spec.Text,
		Type:      ctx.Topology.Spec.Type,
		Schedule:  ctx.Topology.Spec.Schedule,
	}

	if component.Name == "" {
		component.Name = ctx.Topology.Name
	}

	if opts.Depth > 0 {
		for _, comp := range ctx.Topology.Spec.Components {
			components, err := lookupComponents(ctx, comp)
			if err != nil {
				logger.Errorf("Error looking up component %s: %s", comp.Name, err)
				continue
			}
			// add topology labels to the components
			for _, component := range components {
				if component.Labels == nil {
					component.Labels = make(dutyTypes.JSONStringMap)
				}
				for key, value := range ctx.Topology.Labels {
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

			component.Components = append(component.Components, components...)
		}
	}

	if len(component.Components) == 1 && component.Components[0].Type == "virtual" {
		// if there is only one component and it is virtual, then we don't need to show it
		ctx.Components = &component.Components[0].Components
		return *ctx.Components
	}

	ctx.Components = &component.Components

	for _, property := range ctx.Topology.Spec.Properties {
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
	if component.ID.String() == "" && ctx.Topology.Spec.Id != nil {
		id, err := templating.Template(component.GetAsEnvironment(), *ctx.Topology.Spec.Id)
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

	component.Status = component.Summary.GetStatus()

	logger.Debugf(component.Components.Debug(""))
	results = append(results, component)
	logger.Infof("%s id=%s external_id=%s status=%s", component.Name, component.ID, component.ExternalId, component.Status)
	for _, c := range results.Walk() {
		c.Namespace = ctx.Topology.GetNamespace()
		c.Schedule = ctx.Topology.Spec.Schedule
	}
	return results
}

func SyncComponents(opts TopologyRunOptions, topology v1.Topology) error {
	logger.Tracef("Running sync for components with topology: %s", topology.GetPersistedID())
	// Check if deleted
	var dbTopology models.Topology
	if err := db.Gorm.Where("id = ?", topology.GetPersistedID()).First(&dbTopology).Error; err != nil {
		return fmt.Errorf("failed to query topology id: %s: %w", topology.GetPersistedID(), err)
	}

	if dbTopology.DeletedAt != nil {
		logger.Infof("Skipping topology[%s] as its deleted", topology.GetPersistedID())
		// TODO: Should we run the db.DeleteTopology function always in this scenario
		return nil
	}

	components := Run(opts, topology)
	topologyID, err := uuid.Parse(topology.GetPersistedID())
	if err != nil {
		return fmt.Errorf("failed to parse topology id: %w", err)
	}

	var compIDs []uuid.UUID
	for _, component := range components {
		component.Name = topology.Name
		component.Namespace = topology.Namespace
		component.Labels = topology.Labels
		component.TopologyID = &topologyID
		componentsIDs, err := db.PersistComponent(component)
		if err != nil {
			return fmt.Errorf("failed to persist component(id=%s, name=%s): %w", component.ID, component.Name, err)
		}

		compIDs = append(compIDs, componentsIDs...)
	}

	dbCompsIDs, err := db.GetActiveComponentsIDsOfTopology(topologyID.String())
	if err != nil {
		logger.Errorf("error getting components for system(id=%s): %v", topologyID.String(), err)
	}

	deleteCompIDs := utils.SetDifference(dbCompsIDs, compIDs)
	if len(deleteCompIDs) != 0 {
		if err := db.DeleteComponentsWithIDs(utils.UUIDsToStrings(deleteCompIDs), time.Now()); err != nil {
			logger.Errorf("error deleting components: %v", err)
		}
	}

	return nil
}
