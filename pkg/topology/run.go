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
	dutyCtx "github.com/flanksource/duty/context"
	"github.com/flanksource/duty/job"
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

	canarySpec := v1.NewCanaryFromSpec(name, ctx.GetNamespace(), spec)
	canaryCtx := context.New(ctx.WithObject(canarySpec.ObjectMeta), canarySpec)
	canaryCtx.Context = ctx.Context
	canaryCtx.Namespace = ctx.GetNamespace()
	// canaryCtx.Environment = ctx.
	// canaryCtx.Logger = ctx.Logger

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
	_config, err := query.FindConfig(ctx.Context, *pkgConfig)
	if err != nil || _config == nil {
		return prop, err
	}

	configJSON, err := _config.ConfigJSONStringMap()
	if err != nil {
		return nil, fmt.Errorf("error converting config[%s] to json for lookup: %w", _config.ID, err)
	}

	templateEnv := _config.AsMap("type")
	templateEnv["config"] = configJSON
	templateEnv["config_type"] = _config.Type

	ctx.Tracef("%s property=%s => %s", ctx, property.Name, _config.String())

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
			ctx.Logger.V(3).Infof("%s property=%s => no results", ctx, property.Name)
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
			ctx.Tracef("%s property=%s => %s", ctx, property.Name, prop.Text)
			return json.Marshal(types.Properties{prop})
		}
		ctx.Tracef("%s property=%s => %s", ctx, property.Name, dataStr)
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

func populateParentRefMap(c *pkg.Component, parentRefMap map[string]*pkg.Component) {
	parentRefMap[genParentKey(c.Name, c.Type, c.Namespace)] = c
	for _, child := range c.Components {
		populateParentRefMap(child, parentRefMap)
	}
}

func changeComponentParents(c *pkg.Component, parentRefMap map[string]*pkg.Component) {
	var children pkg.Components
	for _, child := range c.Components {
		if child.ParentLookup == nil {
			children = append(children, child)
			continue
		}

		key := genParentKey(child.ParentLookup.Name, child.ParentLookup.Type, child.ParentLookup.Namespace)
		if parentComp, exists := parentRefMap[key]; exists {
			// Set nil to prevent processing again
			child.ParentLookup = nil
			parentComp.Components = append(parentComp.Components, child)
		} else {
			children = append(children, child)
		}
	}
	c.Components = children

	for _, child := range c.Components {
		changeComponentParents(child, parentRefMap)
	}
}

type TopologyRunOptions struct {
	job.JobRuntime
	Depth     int
	Namespace string
}

type TopologyJob struct {
	Topology  v1.Topology
	Namespace string
	Output    pkg.Components
}

func Run(ctx dutyCtx.Context, topology pkg.Topology) (pkg.Components, *models.JobHistory, error) {
	j := &job.Job{
		Name:         "topology",
		ResourceType: "topology",
		ResourceID:   fmt.Sprintf("%s/%s", topology.Namespace, topology.Name),
		JobHistory:   false,
	}

	v1, err := topology.ToV1()
	if err != nil {
		return nil, nil, err
	}
	tj := TopologyJob{
		Topology:  *v1,
		Namespace: topology.Namespace,
	}
	j.Context = ctx.WithObject(v1.ObjectMeta)
	j.Fn = tj.Run

	j.Run()

	return tj.Output, j.LastJob, nil
}

func (tj *TopologyJob) Run(job job.JobRuntime) error {
	t := tj.Topology

	id := t.GetPersistedID()
	topologyID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("failed to parse topology id: %v", err)
	}

	// Check if deleted
	var dbTopology models.Topology
	if err := job.DB().Where("id = ?", id).First(&dbTopology).Error; err != nil {
		return fmt.Errorf("failed to get topology %v", err)
	}

	if dbTopology.DeletedAt != nil {
		job.Debugf("Skipping topology as its deleted")
		// TODO: Should we run the db.DeleteTopology function always in this scenario
		return nil
	}

	if t.Namespace == "" {
		t.Namespace = tj.Namespace
	}

	ctx := NewComponentContext(job.Context, t)
	ctx.JobHistory = job.History

	ctx.Debugf("running topology")

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
	for _, comp := range ctx.Topology.Spec.Components {
		components, err := lookupComponents(ctx, comp)
		if err != nil {
			job.History.AddError(fmt.Sprintf("Error looking up component %s: %s", comp.Name, err))
			continue
		}
		// add topology labels to the components
		for _, component := range components {
			job.History.IncrSuccess()
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

	// Update component parents based on ParentLookup
	parentRefMap := make(map[string]*pkg.Component)
	populateParentRefMap(rootComponent, parentRefMap)
	changeComponentParents(rootComponent, parentRefMap)

	if len(rootComponent.Components) == 1 && rootComponent.Components[0].Type == "virtual" {
		// if there is only one component and it is virtual, then we don't need to show it
		ctx.Components = &rootComponent.Components[0].Components
		tj.Output = *ctx.Components
		return nil
	}

	ctx.Components = &rootComponent.Components

	for _, property := range ctx.Topology.Spec.Properties {
		// TODO Yash: Usecase for this
		props, err := lookupProperty(ctx, &property)
		if err != nil {
			job.History.AddError(fmt.Sprintf("Failed to lookup property %s: %v", property.Name, err))
			continue
		}
		if err := mergeComponentProperties(pkg.Components{rootComponent}, props); err != nil {
			job.History.AddError(fmt.Sprintf("Failed to merge component property %s: %v", property.Name, err))
			continue
		}
	}

	if len(rootComponent.Components) > 0 {
		rootComponent.Summary = rootComponent.Components.Summarize()
	}
	if rootComponent.ID.String() == "" && ctx.Topology.Spec.Id != nil {
		id, err := gomplate.RunTemplate(rootComponent.GetAsEnvironment(), ctx.Topology.Spec.Id.Gomplate())
		if err != nil {
			job.History.AddError(fmt.Sprintf("Failed to lookup id: %v", err))
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

	if ctx.IsTrace() {
		ctx.Tracef(results.Debug(ctx.Logger.IsLevelEnabled(5), ""))
	} else if ctx.Logger.IsLevelEnabled(5) {
		ctx.Infof(results.Debug(ctx.Logger.IsLevelEnabled(5), ""))
	}
	for _, c := range results.Walk() {
		if c.Namespace == "" {
			c.Namespace = ctx.Topology.GetNamespace()
		}
		c.Schedule = ctx.Topology.Spec.Schedule
	}

	var compIDs []uuid.UUID
	for _, component := range results {
		// Is this step required ever ?
		component.Name = dbTopology.Name
		component.Namespace = dbTopology.Namespace
		component.Labels = dbTopology.Labels
		component.TopologyID = topologyID

		componentsIDs, err := db.PersistComponent(job.Context, component)
		if err != nil {
			return fmt.Errorf("failed to persist component(id=%s, name=%s): %v", component.ID, component.Name, err)
		}

		compIDs = append(compIDs, componentsIDs...)
	}

	ctx.Infof("%s id=%s external_id=%s status=%s", rootComponent.Name, rootComponent.ID, rootComponent.ExternalId, rootComponent.Status)

	dbCompsIDs, err := db.GetActiveComponentsIDsOfTopology(ctx.DB(), id)
	if err != nil {
		return fmt.Errorf("error getting components %v", err)
	}

	deleteCompIDs := utils.SetDifference(dbCompsIDs, compIDs)
	if len(deleteCompIDs) != 0 {
		if err := db.DeleteComponentsWithIDs(job.DB(), utils.UUIDsToStrings(deleteCompIDs)); err != nil {
			return fmt.Errorf("error deleting components %v", err)
		}
	}
	job.History.SuccessCount = len(rootComponent.Components)
	tj.Output = results
	return nil
}
