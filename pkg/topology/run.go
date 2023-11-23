package topology

import (
	"fmt"
	"strings"
	"time"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/db/types"
	"github.com/flanksource/canary-checker/pkg/utils"
	"github.com/flanksource/commons/collections"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/gomplate/v3"
	"github.com/flanksource/kommons"
	"github.com/google/uuid"
	jsontime "github.com/liamylian/jsontime/v2/v2"
	"github.com/pkg/errors"
	"k8s.io/client-go/kubernetes"
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

func forEachComponent(ctx *ComponentContext, spec *v1.ComponentSpec, component *pkg.Component) error {
	logger.Debugf("[%s] %s", component.Name, spec.ForEach)
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
			errMsg := fmt.Sprintf("Failed to lookup property %s: %v", property.Name, err)
			logger.Errorf(errMsg)
			ctx.JobHistory.AddError(errMsg)
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
			errMsg := fmt.Sprintf("Failed to lookup components %s: %v", child, err)
			logger.Errorf(errMsg)
			ctx.JobHistory.AddError(errMsg)
		} else {
			component.Components = append(component.Components, children...)
		}
	}

	for _, childConfig := range spec.ForEach.Configs {
		child := childConfig
		if err := ctx.TemplateConfig(&child); err != nil {
			errMsg := fmt.Sprintf("Failed to lookup configs %s: %v", child, err)
			logger.Errorf(errMsg)
			ctx.JobHistory.AddError(errMsg)
		} else {
			component.Configs = append(component.Configs, pkg.NewConfig(child))
		}
	}

	for _, _selector := range spec.ForEach.Selectors {
		selector := _selector
		if err := ctx.TemplateStruct(&selector); err != nil {
			errMsg := fmt.Sprintf("Failed to lookup selectors %s: %v", selector, err)
			logger.Errorf(errMsg)
			ctx.JobHistory.AddError(errMsg)
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
		logger.Debugf("Looking up components for %s => %s", component, component.ForEach)
		if lookedUpComponents, err = mergeComponentLookup(ctx, &component, component.Lookup); err != nil {
			return nil, err
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

	canaryCtx := context.New(ctx.Kommons, ctx.Kubernetes, db.Gorm, db.Pool, v1.NewCanaryFromSpec(name, ctx.Namespace, spec))
	canaryCtx.Context = ctx
	canaryCtx.Namespace = ctx.Namespace
	canaryCtx.Environment = ctx.Environment
	canaryCtx.Logger = ctx.Logger

	checkResults, err := checks.RunChecks(canaryCtx)
	if err != nil {
		return nil, err
	}

	for _, result := range checkResults {
		if result.Error != "" {
			errMsg := fmt.Sprintf("Failed to lookup property: %s. Error in lookup: %s", name, result.Error)
			logger.Errorf(errMsg)
			ctx.JobHistory.AddError(errMsg)
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

func lookupConfig(ctx *ComponentContext, property *v1.Property) (*pkg.Property, error) {
	prop := pkg.NewProperty(*property)
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
	pkgConfig := pkg.NewConfig(*config)
	pkgConfig.Name = configName
	_config, err := db.FindConfig(*pkgConfig)
	if err != nil {
		return prop, err
	}
	if _config == nil {
		return prop, nil
	}

	templateEnv := map[string]any{
		"config": *_config.Spec,
		"tags":   toMapStringAny(_config.Tags),
	}
	prop.Text, err = gomplate.RunTemplate(templateEnv, property.ConfigLookup.Display.Template.Gomplate())
	return prop, err
}

func toMapStringAny(m map[string]string) map[string]any {
	r := make(map[string]any)
	for k, v := range m {
		r[k] = v
	}
	return r
}

func lookupProperty(ctx *ComponentContext, property *v1.Property) ([]byte, error) {
	if property.ConfigLookup != nil {
		prop, err := lookupConfig(ctx, property)
		if err != nil {
			return nil, errors.Wrapf(err, "property config lookup failed: %s", property)
		}
		return json.Marshal(pkg.Properties{prop})
	}

	if property.Lookup != nil {
		results, err := lookup(ctx, property.Name, *property.Lookup)
		lp, _ := json.Marshal(property.Lookup)
		logger.Infof("Results of %v are %v", string(lp), results)
		if err != nil {
			return nil, err
		}
		if len(results) == 0 {
			return nil, nil
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
			return json.Marshal(pkg.Properties{prop})
		}
		return data, nil
	}

	return json.Marshal(pkg.Properties{pkg.NewProperty(*property)})
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
		var properties pkg.Properties
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
	*kommons.Client
	Kubernetes kubernetes.Interface
	Depth      int
	Namespace  string
}

func Run(opts TopologyRunOptions, t v1.Topology) []*pkg.Component {
	jobHistory := models.NewJobHistory("TopologySync", "topology", t.GetPersistedID()).Start()
	_ = db.PersistJobHistory(jobHistory)

	if t.Namespace == "" {
		t.Namespace = opts.Namespace
	}
	logger.Debugf("Running topology %s/%s depth=%d", t.Namespace, t.Name, opts.Depth)

	ctx := NewComponentContext(opts.Client, opts.Kubernetes, t)
	ctx.JobHistory = jobHistory

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
				errMsg := fmt.Sprintf("Error looking up component %s: %s", comp.Name, err)
				logger.Errorf(errMsg)
				jobHistory.AddError(errMsg)
				continue
			}
			// add topology labels to the components
			for _, component := range components {
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
		return *ctx.Components
	}

	ctx.Components = &rootComponent.Components

	for _, property := range ctx.Topology.Spec.Properties {
		// TODO Yash: Usecase for this
		props, err := lookupProperty(ctx, &property)
		if err != nil {
			errMsg := fmt.Sprintf("Failed to lookup property %s: %v", property.Name, err)
			logger.Errorf(errMsg)
			jobHistory.AddError(errMsg)
			continue
		}
		if err := mergeComponentProperties(pkg.Components{rootComponent}, props); err != nil {
			errMsg := fmt.Sprintf("Failed to merge component property %s: %v", property.Name, err)
			logger.Errorf(errMsg)
			jobHistory.AddError(errMsg)
			continue
		}
	}

	if len(rootComponent.Components) > 0 {
		rootComponent.Summary = rootComponent.Components.Summarize()
	}
	if rootComponent.ID.String() == "" && ctx.Topology.Spec.Id != nil {
		id, err := gomplate.RunTemplate(rootComponent.GetAsEnvironment(), ctx.Topology.Spec.Id.Gomplate())
		if err != nil {
			errMsg := fmt.Sprintf("Failed to lookup id: %v", err)
			logger.Errorf(errMsg)
			jobHistory.AddError(errMsg)
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

	rootComponent.Status = pkg.ComponentStatus(rootComponent.Summary.GetStatus())

	logger.Debugf(rootComponent.Components.Debug(""))

	results = append(results, rootComponent)
	logger.Infof("%s id=%s external_id=%s status=%s", rootComponent.Name, rootComponent.ID, rootComponent.ExternalId, rootComponent.Status)
	for _, c := range results.Walk() {
		if c.Namespace == "" {
			c.Namespace = ctx.Topology.GetNamespace()
		}
		c.Schedule = ctx.Topology.Spec.Schedule
	}

	_ = db.PersistJobHistory(jobHistory.IncrSuccess().End())
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
