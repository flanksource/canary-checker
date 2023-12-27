package topology

import (
	"fmt"
	"strings"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/collections"
	dutyContext "github.com/flanksource/duty/context"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/query"
	"github.com/flanksource/duty/types"
	"github.com/flanksource/gomplate/v3"
	"github.com/google/uuid"
	jsontime "github.com/liamylian/jsontime/v2/v2"
	"github.com/pkg/errors"
)

var json = jsontime.ConfigWithCustomTimeFormat

func mergeComponentLookup(ctx *ComponentContext, component *v1.ComponentSpec, spec *v1.CanarySpec) (pkg.Components, error) {
	name := component.Name
	components := pkg.Components{}
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
			var p pkg.Component
			data, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("error marshaling result to json: %w", err)
			}
			if err := json.Unmarshal(data, &p); err != nil {
				return nil, fmt.Errorf("error unmarshaling data from json: %w", err)
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

func forEachComponent(ctx *ComponentContext, spec *v1.ComponentSpec, component *pkg.Component) error {
	if spec.ForEach == nil {
		return nil
	}
	ctx.SetCurrentComponent(component)

	for _, property := range spec.ForEach.Properties {
		// Create a DeepCopy for templating
		prop := property.DeepCopy()
		if err := ctx.TemplateProperty(prop); err != nil {
			return err
		}

		props, err := lookupProperty(ctx, prop)
		if err != nil {
			ctx.JobHistory.AddError(fmt.Sprintf("Failed to lookup property %s: %v", property.Name, err))
			continue
		}

		// TODO: Ask Moshe Can for each handle component list
		if err := mergeComponentProperties(pkg.Components{component}, props); err != nil {
			continue
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
			ctx.JobHistory.AddError(fmt.Sprintf("Failed to lookup components %s: %v", child, err))
		} else {
			component.Components = append(component.Components, children...)
		}
	}

	for _, childConfig := range spec.ForEach.Configs {
		child := childConfig
		if err := ctx.TemplateConfig(&child); err != nil {
			ctx.JobHistory.AddError(fmt.Sprintf("Failed to lookup configs %s: %v", child, err))
		} else {
			component.Configs = append(component.Configs, &child)
		}
	}

	for _, _selector := range spec.ForEach.Selectors {
		selector := _selector
		if err := ctx.TemplateStruct(&selector); err != nil {
			ctx.JobHistory.AddError(fmt.Sprintf("Failed to lookup selectors %s: %v", selector, err))
		} else {
			component.Selectors = append(component.Selectors, selector)
		}
	}

	return nil
}

func lookupComponents(ctx *ComponentContext, component v1.ComponentSpec) (components pkg.Components, err error) {
	// A component can have either a lookup or child components
	// A lookup will translates flatly into a list of components

	if component.Lookup != nil {
		var lookedUpComponents pkg.Components
		if lookedUpComponents, err = mergeComponentLookup(ctx, &component, component.Lookup); err != nil {
			return nil, fmt.Errorf("error merging component lookup: %w", err)
		}

		components = append(components, lookedUpComponents...)
	} else {
		var childComponents pkg.Components
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

	for _, property := range component.Properties {
		props, err := lookupProperty(ctx, property)
		if err != nil {
			return nil, fmt.Errorf("error with property lookup: %w", err)
		}
		if err := mergeComponentProperties(components, props); err != nil {
			return nil, fmt.Errorf("error with merging component properties: %w", err)
		}
	}

	for _, comp := range components {
		if comp.Icon == "" {
			comp.Icon = component.Icon
		}
		if comp.Lifecycle == "" {
			comp.Lifecycle = component.Lifecycle
		}
		if comp.ExternalId == "" && component.Id != nil {
			id, err := gomplate.RunTemplate(comp.GetAsEnvironment(), component.Id.Gomplate())
			if err != nil {
				return nil, errors.Wrapf(err, "Failed to lookup id: %v", component.Id)
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
	var results []any

	canaryCtx := context.New(ctx.Duty, v1.NewCanaryFromSpec(name, ctx.Namespace, spec))
	canaryCtx.Context = ctx
	canaryCtx.Namespace = ctx.Namespace
	canaryCtx.Environment = ctx.Environment
	canaryCtx.Logger = ctx.Logger

	checkResults, err := checks.Exec(canaryCtx)
	if err != nil {
		return nil, err
	}

	for _, result := range checkResults {
		if result.Error != "" {
			ctx.JobHistory.AddError(fmt.Sprintf("failed to lookup property %s:  %s", name, result.Error))
			return nil, nil
		}
		if result.Message != "" {
			results = append(results, result.Message)
		} else if result.Detail != nil {
			switch result.Detail.(type) {
			case []any:
				results = append(results, result.Detail.([]interface{})...)
			case any:
				results = append(results, result.Detail)
			default:
				return nil, fmt.Errorf("unknown type %T", result.Detail)
			}
		} else {
			results = append(results, "")
		}
	}
	return results, nil
}

func lookupConfig(ctx *ComponentContext, property *v1.Property) (*types.Property, error) {
	prop := pkg.NewProperty(*property)
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
	pkgConfig := config
	pkgConfig.Name = configName
	_config, err := query.FindConfig(ctx.DB, *pkgConfig)
	if err != nil || _config == nil {
		return prop, err
	}

	templateEnv := _config.AsMap("type")
	templateEnv["spec"] = _config.Config
	templateEnv["config_type"] = _config.Type

	ctx.Duty.Tracef("%s property=%s => %s", ctx, property.Name, _config.String())

	prop.Text, err = gomplate.RunTemplate(templateEnv, property.ConfigLookup.Display.Template.Gomplate())
	return prop, err
}

func lookupProperty(ctx *ComponentContext, property *v1.Property) ([]byte, error) {
	if property.ConfigLookup != nil {
		prop, err := lookupConfig(ctx, property)
		if err != nil {
			return nil, errors.Wrapf(err, "property config lookup failed: %s", property)
		}
		return json.Marshal(types.Properties{prop})
	}

	if property.Lookup != nil {
		results, err := lookup(ctx, property.Name, *property.Lookup)
		if err != nil || len(results) == 0 {
			ctx.Duty.Tracef("%s property=%s => no results", ctx, property.Name)
			return nil, err
		}

		var dataStr string
		var ok bool
		if dataStr, ok = results[0].(string); !ok {
			return nil, fmt.Errorf("unknown property type %T", results)
		}
		data := []byte(dataStr)
		// When the lookup returns just a value
		// set the current property's text as that value
		if !isComponentList(data) && !isPropertyList(data) {
			prop := pkg.NewProperty(*property)
			prop.Text = dataStr
			ctx.Duty.Tracef("%s property=%s => %s", ctx, property.Name, prop.Text)
			return json.Marshal(types.Properties{prop})
		}
		ctx.Duty.Tracef("%s property=%s => %s", ctx, property.Name, dataStr)
		return data, nil
	}

	return json.Marshal(types.Properties{pkg.NewProperty(*property)})
}

func mergeComponentProperties(components pkg.Components, propertiesRaw []byte) error {
	if isComponentList(propertiesRaw) {
		// the result is map of components to properties, find the existing component
		// and then merge the property into it
		var componentsWithProperties pkg.Components
		err := json.Unmarshal(propertiesRaw, &componentsWithProperties)
		if err != nil {
			return err
		}
		for _, component := range componentsWithProperties {
			found := components.Find(component.Name)
			if found == nil {
				continue
			}
			for _, property := range component.Properties {
				foundProperty := found.Properties.Find(property.Name)
				if foundProperty == nil {
					return fmt.Errorf("property %s not found", property.Name)
				}
				foundProperty.Merge(property)
			}
		}
	} else if isPropertyList(propertiesRaw) {
		var properties types.Properties
		if err := json.Unmarshal(propertiesRaw, &properties); err != nil {
			return err
		}
		for _, comp := range components {
			comp.Properties = append(comp.Properties, properties...)
		}
	}
	return nil
}

type TopologyRunOptions struct {
	dutyContext.Context
	Depth     int
	Namespace string
}

func Run(opts TopologyRunOptions, t v1.Topology) ([]*pkg.Component, models.JobHistory) {
	jobHistory := models.NewJobHistory("TopologySync", "topology", t.GetPersistedID()).Start()
	defer func() {
		_ = jobHistory.End().Persist(opts.DB())
	}()

	_ = jobHistory.Persist(opts.DB())

	if t.Namespace == "" {
		t.Namespace = opts.Namespace
	}

	ctx := NewComponentContext(opts.Context, t)
	ctx.JobHistory = jobHistory
	ctx.Debugf("[%s] running topology depth=%d", t, opts.Depth)

	var results pkg.Components
	rootComponent := &pkg.Component{
		Name:      ctx.Topology.Spec.Text,
		Namespace: ctx.Topology.GetNamespace(),
		Labels:    ctx.Topology.Labels,
		Tooltip:   ctx.Topology.Spec.Tooltip,
		Icon:      ctx.Topology.Spec.Icon,
		Text:      ctx.Topology.Spec.Text,
		Type:      ctx.Topology.Spec.Type,
		Schedule:  ctx.Topology.Spec.Schedule,
	}

	if rootComponent.Name == "" {
		rootComponent.Name = ctx.Topology.Name
	}

	ignoreLabels := []string{"kustomize.toolkit.fluxcd.io/name", "kustomize.toolkit.fluxcd.io/namespace"}
	if opts.Depth > 0 {
		for _, comp := range ctx.Topology.Spec.Components {
			components, err := lookupComponents(ctx, comp)
			if err != nil {
				jobHistory.AddError(fmt.Sprintf("Error looking up component %s: %s", comp.Name, err))
				continue
			}
			// add topology labels to the components
			for _, component := range components {
				jobHistory.IncrSuccess()
				if component.Labels == nil {
					component.Labels = make(types.JSONStringMap)
				}
				for key, value := range ctx.Topology.Labels {
					// Workaround for avoiding a recursive loop
					// If resource is added via flux kustomize the label gets added to top level Pods and Nodes
					if strings.HasPrefix(component.Type, "Kubernetes") && collections.Contains(ignoreLabels, key) {
						continue
					}

					// don't overwrite the component labels
					if _, isPresent := component.Labels[key]; !isPresent {
						component.Labels[key] = value
					}
				}
			}
			if comp.Lookup == nil {
				rootComponent.Components = append(rootComponent.Components, components...)
				continue
			}

			rootComponent.Components = append(rootComponent.Components, components...)
		}
	}

	if len(rootComponent.Components) == 1 && rootComponent.Components[0].Type == "virtual" {
		// if there is only one component and it is virtual, then we don't need to show it
		ctx.Components = &rootComponent.Components[0].Components
		return *ctx.Components, *jobHistory
	}

	ctx.Components = &rootComponent.Components

	for _, property := range ctx.Topology.Spec.Properties {
		// TODO Yash: Usecase for this
		props, err := lookupProperty(ctx, &property)
		if err != nil {
			jobHistory.AddError(fmt.Sprintf("Failed to lookup property %s: %v", property.Name, err))
			continue
		}
		if err := mergeComponentProperties(pkg.Components{rootComponent}, props); err != nil {
			jobHistory.AddError(fmt.Sprintf("Failed to merge component property %s: %v", property.Name, err))
			continue
		}
	}

	if len(rootComponent.Components) > 0 {
		rootComponent.Summary = rootComponent.Components.Summarize()
	}
	if rootComponent.ID.String() == "" && ctx.Topology.Spec.Id != nil {
		id, err := gomplate.RunTemplate(rootComponent.GetAsEnvironment(), ctx.Topology.Spec.Id.Gomplate())
		if err != nil {
			jobHistory.AddError(fmt.Sprintf("Failed to lookup id: %v", err))
		} else {
			rootComponent.ID, _ = uuid.Parse(id)
		}
	}

	// TODO: Ask Moshe why we do this ?
	if rootComponent.ID.String() == "" {
		rootComponent.ID, _ = uuid.Parse(rootComponent.Name)
	}

	if rootComponent.ExternalId == "" {
		rootComponent.ExternalId = rootComponent.Name
	}

	rootComponent.Status = rootComponent.Summary.GetStatus()

	results = append(results, rootComponent)
	ctx.Infof("%s id=%s external_id=%s status=%s", rootComponent.Name, rootComponent.ID, rootComponent.ExternalId, rootComponent.Status)
	for _, c := range results.Walk() {
		if c.Namespace == "" {
			c.Namespace = ctx.Topology.GetNamespace()
		}
		c.Schedule = ctx.Topology.Spec.Schedule
	}

	return results, *jobHistory
}

func SyncComponents(opts TopologyRunOptions, topology v1.Topology) error {
	id := topology.GetPersistedID()
	opts.Context.Debugf("[%s] running topology sync", id)
	// Check if deleted
	var dbTopology models.Topology
	if err := opts.DB().Where("id = ?", id).First(&dbTopology).Error; err != nil {
		return fmt.Errorf("failed to query topology id: %s: %w", id, err)
	}

	if dbTopology.DeletedAt != nil {
		opts.Context.Debugf("Skipping topology[%s] as its deleted", id)
		// TODO: Should we run the db.DeleteTopology function always in this scenario
		return nil
	}

	components, _ := Run(opts, topology)
	topologyID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("failed to parse topology id: %w", err)
	}

	var compIDs []uuid.UUID
	for _, component := range components {
		component.Name = topology.Name
		component.Namespace = topology.Namespace
		component.Labels = topology.Labels
		component.TopologyID = &topologyID
		componentsIDs, err := db.PersistComponent(opts.Context, component)
		if err != nil {
			return fmt.Errorf("failed to persist component(id=%s, name=%s): %w", component.ID, component.Name, err)
		}

		compIDs = append(compIDs, componentsIDs...)
	}

	dbCompsIDs, err := db.GetActiveComponentsIDsOfTopology(opts.DB(), id)
	if err != nil {
		return fmt.Errorf("error getting components for topology (id=%s): %v", id, err)
	}

	deleteCompIDs := utils.SetDifference(dbCompsIDs, compIDs)
	if len(deleteCompIDs) != 0 {
		if err := db.DeleteComponentsWithIDs(opts.DB(), utils.UUIDsToStrings(deleteCompIDs)); err != nil {
			return fmt.Errorf("error deleting components: %v", err)
		}
	}

	return nil
}
