package topology

import (
	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/duty/models"
	"github.com/flanksource/kommons"
	"github.com/flanksource/kommons/ktemplate"
	"github.com/pkg/errors"
)

type ComponentContext struct {
	*context.KubernetesContext
	Topology     v1.Topology
	ComponentAPI v1.Component
	// Components keep track of the components that properties can apply to,
	// properties can return a map of component names to properties to facilitate
	// queries that are more efficient to perform for all components rather than a component at a time
	Components *models.Components
	// Properties can either be looked up on an individual component, or act as a summary across all components
	CurrentComponent *models.Component
	templater        *ktemplate.StructTemplater
}

func (c *ComponentContext) GetTemplater() ktemplate.StructTemplater {
	if c.templater != nil {
		return *c.templater
	}
	var props = make(map[string]interface{})
	if c.CurrentComponent != nil && c.CurrentComponent.Properties != nil {
		props = c.CurrentComponent.Properties.AsMap()
	}
	c.templater = &ktemplate.StructTemplater{
		// RequiredTag: "template",
		DelimSets: []ktemplate.Delims{
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

func (c *ComponentContext) SetCurrentComponent(component *models.Component) {
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

func (c *ComponentContext) TemplateConfig(config *v1.Config) error {
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
		Topology:          c.Topology,
		ComponentAPI:      c.ComponentAPI,
		Components:        c.Components,
	}
}
func (c *ComponentContext) WithComponents(components *models.Components, current *models.Component) *ComponentContext {
	cloned := c.Clone()
	cloned.Components = components
	cloned.CurrentComponent = current
	return cloned
}

func NewComponentContext(client *kommons.Client, system v1.Topology) *ComponentContext {
	return &ComponentContext{
		KubernetesContext: context.NewKubernetesContext(client, system.Namespace),
		Topology:          system,
	}
}
