package v1

import (
	"fmt"

	"github.com/flanksource/duty/models"
	"github.com/flanksource/duty/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:object:root=true

type Component struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ComponentSpec   `json:"spec,omitempty"`
	Status            ComponentStatus `json:"status,omitempty"`
}

type ComponentSpec struct {
	Name    string    `json:"name,omitempty"`
	Tooltip string    `json:"tooltip,omitempty"`
	Icon    string    `json:"icon,omitempty"`
	Owner   string    `json:"owner,omitempty"`
	Id      *Template `json:"id,omitempty"` //nolint
	Order   int       `json:"order,omitempty"`
	// The type of component, e.g. service, API, website, library, database, etc.
	Type string `json:"type,omitempty"`
	// The lifecycle state of the component e.g. production, staging, dev, etc.
	Lifecycle     string             `json:"lifecycle,omitempty"`
	Relationships []RelationshipSpec `json:"relationships,omitempty"`
	// +kubebuilder:validation:XPreserveUnknownFields
	Properties []*Property `json:"properties,omitempty"`
	// +kubebuilder:validation:XPreserveUnknownFields
	// Lookup component definitions from an external source, use the
	// forEach property to iterate over the results to further enrich each component.
	Lookup *CanarySpec `json:"lookup,omitempty"`
	// +kubebuilder:validation:XPreserveUnknownFields
	// Create new child components
	Components []ComponentSpecObject `json:"components,omitempty"`
	// Lookup and associcate other components with this component
	Selectors       types.ResourceSelectors `json:"selectors,omitempty"`
	ComponentChecks ComponentChecks         `json:"checks,omitempty"`
	// Lookup and associate config items with this component
	Configs Configs `json:"configs,omitempty"`
	//
	Summary *types.Summary `json:"summary,omitempty"`
	// Only applies when using lookup, when specified the components and properties
	// specified under ForEach will be templated using the components returned by the lookup
	// ${.properties} can be used to reference the properties of the component
	// ${.component} can be used to reference the component itself
	ForEach *ForEach `json:"forEach,omitempty"`
	// Logs is a list of logs selector for apm-hub.
	LogSelectors types.LogSelectors `json:"logs,omitempty"`
}

// +kubebuilder:validation:Type=object
type ComponentSpecObject ComponentSpec

func (c ComponentSpec) String() string {
	if c.Name != "" {
		return c.Name
	}

	return fmt.Sprintf("unnamed component type=%s", c.Name)
}

type ForEach struct {
	Components []ComponentSpec `json:"components,omitempty"`
	// Properties are created once the full component tree is created, property lookup functions
	// can return a map of coomponent name => properties to allow for bulk property lookups
	// being applied to multiple components in the tree
	Properties      Properties              `json:"properties,omitempty"`
	Configs         []Config                `json:"configs,omitempty"`
	Selectors       types.ResourceSelectors `json:"selectors,omitempty"`
	Relationships   []RelationshipSpec      `json:"relationships,omitempty"`
	ComponentChecks ComponentChecks         `json:"checks,omitempty"`
}

func (f *ForEach) IsEmpty() bool {
	return len(f.Properties) == 0 && len(f.Components) == 0
}

func (f *ForEach) String() string {
	return fmt.Sprintf("ForEach(components=%d, properties=%d)", len(f.Components), len(f.Properties))
}

type ComponentStatus struct {
	Status ComponentPropertyStatus `json:"status,omitempty"`
}

type RelationshipSpec struct {
	// The type of relationship, e.g. dependsOn, subcomponentOf, providesApis, consumesApis
	Type string `json:"type,omitempty"`
	Ref  string `json:"ref,omitempty"`
}

type ComponentPropertyStatus string

var (
	ComponentPropertyStatusHealthy   ComponentPropertyStatus = "healthy"
	ComponentPropertyStatusUnhealthy ComponentPropertyStatus = "unhealthy"
	ComponentPropertyStatusWarning   ComponentPropertyStatus = "warning"
	ComponentPropertyStatusError     ComponentPropertyStatus = "error"
	ComponentPropertyStatusInfo      ComponentPropertyStatus = "info"
)

type Owner string

type Text struct {
	Tooltip string `json:"tooltip,omitempty"`
	Icon    string `json:"icon,omitempty"`
	Text    string `json:"text,omitempty"`
	Label   string `json:"label,omitempty"`
}

type Properties []Property

type Property struct {
	Label    string `json:"label,omitempty"`
	Name     string `json:"name,omitempty"`
	Tooltip  string `json:"tooltip,omitempty"`
	Icon     string `json:"icon,omitempty"`
	Text     string `json:"text,omitempty"`
	Order    int    `json:"order,omitempty"`
	Headline bool   `json:"headline,omitempty"`
	Type     string `json:"type,omitempty"`
	Color    string `json:"color,omitempty"`
	// e.g. milliseconds, bytes, millicores, epoch etc.
	Unit           string        `json:"unit,omitempty"`
	Value          int64         `json:"value,omitempty"`
	Max            *int64        `json:"max,omitempty"`
	Min            int64         `json:"min,omitempty"`
	Status         string        `json:"status,omitempty"`
	LastTransition string        `json:"lastTransition,omitempty"`
	Links          []models.Link `json:"links,omitempty"`
	// +kubebuilder:validation:XPreserveUnknownFields
	Lookup       *CanarySpec   `json:"lookup,omitempty"`
	ConfigLookup *ConfigLookup `json:"configLookup,omitempty"`
	Summary      *Template     `json:"summary,omitempty"`
}

func (p Property) ToModel() *models.Property {
	return &models.Property{
		Label:          p.Label,
		Name:           p.Name,
		Tooltip:        p.Tooltip,
		Icon:           p.Icon,
		Order:          p.Order,
		Text:           p.Text,
		Value:          p.Value,
		Unit:           p.Unit,
		Max:            p.Max,
		Min:            p.Min,
		Status:         p.Status,
		LastTransition: p.LastTransition,
		Links:          p.Links,
		Headline:       p.Headline,
		Type:           p.Type,
		Color:          p.Color,
	}
}

func (p *Property) String() string {
	if p.Label != "" {
		return p.Label
	}
	if p.Name != "" {
		return p.Name
	}
	if p.Icon != "" {
		return p.Icon
	}
	return fmt.Sprintf("unnamed property type=%s", p.Type)
}

type ConfigLookup struct {
	ID string `json:"id,omitempty"`
	// Lookup a config by it
	Config *Config `json:"config,omitempty"`
	// A JSONPath expression to lookup the value in the config
	Field string `json:"field,omitempty"`
	// Apply transformations to the value
	Display Display `json:"display,omitempty"`
}

// +kubebuilder:object:root=true

// ComponentList contains a list of Canary
type ComponentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Component `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Component{}, &ComponentList{})
}
