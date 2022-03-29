package topology

import (
	"github.com/flanksource/canary-checker/api/context"
	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/kommons"
)

type SystemContext struct {
	*context.KubernetesContext
	SystemAPI    v1.System
	ComponentAPI v1.Component
	System       *pkg.System
	// Components keep track of the components that properties can apply to,
	// properties can return a map of component names to properties to facilitate
	// queries that are more efficient to perform for all components rather than a component at a time
	Components *pkg.Components
	// Properties can either be looked up on an individual component, or act as a summary across all components
	CurrentComponent *pkg.Component
}

func (c *SystemContext) Clone() *SystemContext {
	return &SystemContext{
		KubernetesContext: c.KubernetesContext.Clone(),
		SystemAPI:         c.SystemAPI,
		ComponentAPI:      c.ComponentAPI,
		System:            c.System,
		Components:        c.Components,
	}
}
func (c *SystemContext) WithComponents(components *pkg.Components, current *pkg.Component) *SystemContext {
	cloned := c.Clone()
	cloned.Components = components
	cloned.CurrentComponent = current
	return cloned
}

func NewSystemContext(client *kommons.Client, system v1.System) *SystemContext {
	return &SystemContext{
		KubernetesContext: context.NewKubernetesContext(client, system.Namespace),
		SystemAPI:         system,
	}
}
