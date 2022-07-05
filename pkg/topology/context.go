package topology

import (
	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/kommons"
)

type ComponentContext struct {
	*context.KubernetesContext
	SystemTemplate v1.SystemTemplate
	ComponentAPI   v1.Component
	// Components keep track of the components that properties can apply to,
	// properties can return a map of component names to properties to facilitate
	// queries that are more efficient to perform for all components rather than a component at a time
	Components *pkg.Components
	// Properties can either be looked up on an individual component, or act as a summary across all components
	CurrentComponent *pkg.Component
}

func (c *ComponentContext) Clone() *ComponentContext {
	return &ComponentContext{
		KubernetesContext: c.KubernetesContext.Clone(),
		SystemTemplate:    c.SystemTemplate,
		ComponentAPI:      c.ComponentAPI,
		Components:        c.Components,
	}
}
func (c *ComponentContext) WithComponents(components *pkg.Components, current *pkg.Component) *ComponentContext {
	cloned := c.Clone()
	cloned.Components = components
	cloned.CurrentComponent = current
	return cloned
}

func NewComponentContext(client *kommons.Client, system v1.SystemTemplate) *ComponentContext {
	return &ComponentContext{
		KubernetesContext: context.NewKubernetesContext(client, system.Namespace),
		SystemTemplate:    system,
	}
}
