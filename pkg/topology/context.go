package topology

import (
	"fmt"

	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"gorm.io/gorm"

	dutyContext "github.com/flanksource/duty/context"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/types"
	"github.com/flanksource/gomplate/v3"
	"github.com/pkg/errors"
)

type ComponentContext struct {
	*context.KubernetesContext
	Topology     v1.Topology
	ComponentAPI v1.Component
	// Components keep track of the components that properties can apply to,
	// properties can return a map of component names to properties to facilitate
	// queries that are more efficient to perform for all components rather than a component at a time
	Components *pkg.Components
	// Properties can either be looked up on an individual component, or act as a summary across all components
	CurrentComponent *pkg.Component
	templater        *gomplate.StructTemplater
	JobHistory       *models.JobHistory
	Duty             dutyContext.Context
	DB               *gorm.DB
}

func (c *ComponentContext) String() string {
	if c.CurrentComponent != nil {
		return fmt.Sprintf("[%s] %s", c.Topology.Name, c.CurrentComponent.Name)
	}
	return fmt.Sprintf("[%s]", c.Topology.Name)
}

func (c *ComponentContext) GetTemplater() gomplate.StructTemplater {
	if c.templater != nil {
		return *c.templater
	}
	var props = make(map[string]interface{})
	if c.CurrentComponent != nil && c.CurrentComponent.Properties != nil {
		props = c.CurrentComponent.Properties.AsMap()
	}
	c.templater = &gomplate.StructTemplater{
		// RequiredTag: "template",
		DelimSets: []gomplate.Delims{
			{
				Left:  "${",
				Right: "}",
			},
		},
		Values: map[string]interface{}{
			"component":  c.CurrentComponent,
			"properties": props,
		},
	}
	return *c.templater
}

func (c *ComponentContext) SetCurrentComponent(component *pkg.Component) {
	c.CurrentComponent = component
	if c.templater != nil {
		c.templater.Values = map[string]interface{}{
			"component":  c.CurrentComponent,
			"properties": c.CurrentComponent.Properties.AsMap(),
		}
	}
}

func (c *ComponentContext) TemplateProperty(property *v1.Property) error {
	templater := c.GetTemplater()
	if err := templater.Walk(property); err != nil {
		return errors.Wrapf(err, "failed to template property %s", property.Name)
	}
	return nil
}

func (c *ComponentContext) TemplateStruct(data interface{}) error {
	templater := c.GetTemplater()
	if err := templater.Walk(data); err != nil {
		return errors.Wrapf(err, "failed to template struct %s", data)
	}
	return nil
}

func (c *ComponentContext) TemplateConfig(config *types.ConfigQuery) error {
	templater := c.GetTemplater()
	if err := templater.Walk(config); err != nil {
		return errors.Wrapf(err, "failed to template config %s", *config)
	}
	//FIXME struct templater does not support maps
	var labels = make(map[string]string)
	for k, v := range config.Tags {
		labels[k], _ = templater.Template(v)
	}
	(*config).Tags = labels
	return nil
}

func (c *ComponentContext) TemplateComponent(component *v1.ComponentSpec) error {
	templater := c.GetTemplater()
	if err := templater.Walk(component); err != nil {
		return errors.Wrapf(err, "failed to template component %s", *component)
	}
	return nil
}

func (c *ComponentContext) Clone() *ComponentContext {
	return &ComponentContext{
		KubernetesContext: c.KubernetesContext.Clone(),
		Duty:              c.Duty,
		DB:                c.DB,
		Topology:          c.Topology,
		ComponentAPI:      c.ComponentAPI,
		Components:        c.Components,
		JobHistory:        c.JobHistory,
	}
}

func (c *ComponentContext) WithComponents(components *pkg.Components, current *pkg.Component) *ComponentContext {
	cloned := c.Clone()
	cloned.Components = components
	cloned.CurrentComponent = current
	return cloned
}

func NewComponentContext(ctx dutyContext.Context, system v1.Topology) *ComponentContext {
	return &ComponentContext{
		KubernetesContext: context.NewKubernetesContext(ctx.Kommons(), ctx.Kubernetes(), system.Namespace),
		Duty:              ctx,
		DB:                ctx.DB(),
		Topology:          system,
	}
}
