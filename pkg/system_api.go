package pkg

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg/db/types"
	"github.com/flanksource/commons/console"
	"github.com/flanksource/commons/logger"
	"github.com/google/uuid"
	"gorm.io/gorm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const ComponentType = "component"

type System struct {
	Object           `yaml:",inline"`
	SystemTemplateID string
	ID               string            `json:"id"` //nolint
	Tooltip          string            `json:"tooltip,omitempty"`
	Icon             string            `json:"icon,omitempty"`
	Text             string            `json:"text,omitempty"`
	Label            string            `json:"label,omitempty"`
	Labels           map[string]string `json:"labels,omitempty"`
	Owner            string            `json:"owner,omitempty"`
	Components       Components        `json:"components,omitempty"` //ignor  this
	Properties       Properties        `json:"properties,omitempty"`
	Summary          v1.Summary        `json:"summary,omitempty"` // ignor ethis
	Status           string            `json:"status,omitempty"`
	Type             string            `json:"type,omitempty"`
	CreatedAt        string            `json:"created_at,omitempty"`
	UpdatedAt        string            `json:"updated_at,omitempty"`
	ExternalId       string            `json:"external_id,omitempty"` //nolint
	TopologyType     string            `json:"topologyType,omitempty"`
}

type SystemTemplate struct {
	ID        uuid.UUID `gorm:"default:generate_ulid()"`
	Name      string
	Namespace string
	Labels    types.JSONStringMap
	Spec      types.JSON
	Schedule  string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt
}

func SystemTemplateFromV1(systemTemplate *v1.SystemTemplate) *SystemTemplate {
	spec, _ := json.Marshal(systemTemplate.Spec)
	return &SystemTemplate{
		Name:      systemTemplate.GetName(),
		Namespace: systemTemplate.GetNamespace(),
		Labels:    types.JSONStringMap(systemTemplate.GetLabels()),
		Spec:      spec,
	}
}

func (s SystemTemplate) ToV1() v1.SystemTemplate {
	var systemTemplateSpec v1.SystemTemplateSpec
	id := s.ID.String()
	_ = json.Unmarshal(s.Spec, &systemTemplateSpec)
	return v1.SystemTemplate{
		TypeMeta: metav1.TypeMeta{
			Kind:       "SystemTemplate",
			APIVersion: "canaries.flanksource.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: s.Namespace,
			Labels:    s.Labels,
		},
		Spec: systemTemplateSpec,
		Status: v1.SystemTemplateStatus{
			PersistentID: &id,
		},
	}
}

func (s System) GetAsEnvironment() map[string]interface{} {
	return map[string]interface{}{
		"self":       s,
		"properties": s.Properties.AsMap(),
	}
}

type Object struct {
	Name      string            `json:"name,omitempty"`
	Namespace string            `json:"namespace,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

func (component *Component) UnmarshalJSON(b []byte) error {
	type UpstreamUnmarshal Component
	var c UpstreamUnmarshal
	if err := json.Unmarshal(b, &c); err != nil {
		return err
	}
	c.TopologyType = ComponentType
	*component = Component(c)
	return nil
}

func (components *Components) UnmarshalJSON(b []byte) error {
	var flat []Component
	if err := json.Unmarshal(b, &flat); err != nil {
		return err
	}
	for _, c := range flat {
		c.TopologyType = ComponentType
		if c.ParentId == "" {
			// first add parents
			parent := c
			*components = append(*components, &parent)
		}
	}

	for _, _c := range flat {
		c := _c
		c.TopologyType = ComponentType
		if c.ParentId != "" {
			parent := components.FindByID(c.ParentId)
			if parent == nil {
				*components = append(*components, &c)
			} else {
				parent.Components = append(parent.Components, &c)
			}
		}
	}

	for _, component := range *components {
		component.Summary = component.Summarize()
	}

	return nil
}

type Component struct {
	Name         string            `json:"name,omitempty"`
	Id           string            `json:"id,omitempty"` //nolint
	Text         string            `json:"text,omitempty"`
	TopologyType string            `json:"topologyType,omitempty"`
	Namespace    string            `json:"namespace,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	Tooltip      string            `json:"tooltip,omitempty"`
	Icon         string            `json:"icon,omitempty"`
	Owner        string            `json:"owner,omitempty"`
	Status       string            `json:"status,omitempty"`
	StatusReason string            `json:"statusReason,omitempty"`
	// The type of component, e.g. service, API, website, library, database, etc.
	Type    string     `json:"type,omitempty"`
	Summary v1.Summary `json:"summary,omitempty"`
	// The lifecycle state of the component e.g. production, staging, dev, etc.
	Lifecycle     string             `json:"lifecycle,omitempty"`
	Relationships []RelationshipSpec `json:"relationships,omitempty"`
	Properties    Properties         `json:"properties,omitempty"`
	Components    Components         `json:"components,omitempty"`
	ParentId      string             `json:"parent_id,omitempty"` //nolint
	CreatedAt     string             `json:"created_at,omitempty"`
	UpdatedAt     string             `json:"updated_at,omitempty"`
	ExternalId    string             `json:"external_id,omitempty"` //nolint
}

func (component Component) Clone() Component {
	return Component{
		Name:         component.Name,
		TopologyType: component.TopologyType,
		Id:           component.Id,
		Text:         component.Text,
		Namespace:    component.Namespace,
		Labels:       component.Labels,
		Tooltip:      component.Tooltip,
		Icon:         component.Icon,
		Owner:        component.Owner,
		Status:       component.Status,
		StatusReason: component.StatusReason,
		Type:         component.Type,
		Lifecycle:    component.Lifecycle,
		Properties:   component.Properties,
		ExternalId:   component.ExternalId,
	}
}

func (component Component) String() string {
	s := ""
	if component.Type != "" {
		s += component.Type + "/"
	}
	if component.Namespace != "" {
		s += component.Namespace + "/"
	}
	if component.Text != "" {
		s += component.Text
	} else if component.Name != "" {
		s += component.Name
	} else {
		s += component.Id
	}
	return s
}

func (component Component) GetAsEnvironment() map[string]interface{} {
	return map[string]interface{}{
		"self":       component,
		"properties": component.Properties.AsMap(),
	}
}

func NewComponent(c v1.ComponentSpec) *Component {
	return &Component{
		Name:      c.Name,
		Owner:     c.Owner,
		Type:      c.Type,
		Lifecycle: c.Lifecycle,
		Tooltip:   c.Tooltip,
		Icon:      c.Icon,
	}
}

func (component Component) GetID() string {
	if component.Id != "" {
		return component.Id
	}
	if component.Text != "" {
		return component.Text
	}
	return component.Name
}

func (components Components) Debug(prefix string) string {
	s := ""
	for _, component := range components {
		status := component.Status

		if component.IsHealthy() {
			status = console.Greenf(status)
		} else {
			status = console.Redf(status)
		}

		s += fmt.Sprintf("%s%s (%s) => %s\n", prefix, component, component.GetID(), status)
		s += component.Components.Debug(prefix + "\t")
	}
	return s
}

type Components []*Component

func (components Components) Find(name string) *Component {
	for _, component := range components {
		if component.Name == name {
			return component
		}
	}
	return nil
}

func (components Components) GetIds() []string {
	ids := []string{}
	for _, component := range components {
		ids = append(ids, component.Id)
	}
	return ids
}

func (components Components) FindByID(id string) *Component {
	for _, component := range components {
		if component.Id == id {
			return component
		}
	}
	return nil
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

type Link struct {
	// e.g. documentation, support, playbook
	Type string `json:"type,omitempty"`
	URL  string `json:"url,omitempty"`
	Text `json:",inline"`
}

// Property is a realized v1.Property without the lookup definition
type Property struct {
	Label   string `json:"label,omitempty"`
	Name    string `json:"name,omitempty"`
	Tooltip string `json:"tooltip,omitempty"`
	Icon    string `json:"icon,omitempty"`
	Type    string `json:"type,omitempty"`
	Color   string `json:"color,omitempty"`

	Headline bool `json:"headline,omitempty"`

	// Either text or value is required, but not both.
	Text  string `json:"text,omitempty"`
	Value int64  `json:"value,omitempty"`

	// e.g. milliseconds, bytes, millicores, epoch etc.
	Unit string `json:"unit,omitempty"`
	Max  *int64 `json:"max,omitempty"`
	Min  int64  `json:"min,omitempty"`

	Status         string    `json:"status,omitempty"`
	LastTransition string    `json:"lastTransition,omitempty"`
	Links          []v1.Link `json:"links,omitempty"`
}

type Properties []*Property

func (p Properties) AsJSON() []byte {
	if len(p) == 0 {
		return []byte("[]")
	}
	data, err := json.Marshal(p)
	if err != nil {
		logger.Errorf("Error marshalling properties: %v", err)
	}
	return data
}

func (p Properties) AsMap() map[string]interface{} {
	result := make(map[string]interface{})
	for _, property := range p {
		result[property.Name] = property.GetValue()
	}
	return result
}

func (p Properties) Find(name string) *Property {
	for _, prop := range p {
		if prop.Name == name {
			return prop
		}
	}
	return nil
}

func (p Property) GetValue() interface{} {
	if p.Text != "" {
		return p.Text
	}
	if p.Value != 0 {
		return p.Value
	}
	return nil
}

func (p *Property) String() string {
	s := fmt.Sprintf("%s[", p.Name)
	if p.Text != "" {
		s += fmt.Sprintf("text=%s ", p.Text)
	}
	if p.Value != 0 {
		s += fmt.Sprintf("value=%d ", p.Value)
	}
	if p.Unit != "" {
		s += fmt.Sprintf("unit=%s ", p.Unit)
	}
	if p.Max != nil {
		s += fmt.Sprintf("max=%d ", *p.Max)
	}
	if p.Min != 0 {
		s += fmt.Sprintf("min=%d ", p.Min)
	}
	if p.Status != "" {
		s += fmt.Sprintf("status=%s ", p.Status)
	}
	if p.LastTransition != "" {
		s += fmt.Sprintf("lastTransition=%s ", p.LastTransition)
	}

	return strings.TrimRight(s, " ") + "]"
}

func (p *Property) Merge(other *Property) {
	if other.Text != "" {
		p.Text = other.Text
	}
	if other.Value != 0 {
		p.Value = other.Value
	}
	if other.Unit != "" {
		p.Unit = other.Unit
	}
	if other.Max != nil {
		p.Max = other.Max
	}
	if other.Min != 0 {
		p.Min = other.Min
	}
	if other.Status != "" {
		p.Status = other.Status
	}
	if other.LastTransition != "" {
		p.LastTransition = other.LastTransition
	}
	if other.Links != nil {
		p.Links = other.Links
	}
	if other.Type != "" {
		p.Type = other.Type
	}
	if other.Color != "" {
		p.Color = other.Color
	}
}

func NewProperty(property v1.Property) *Property {
	return &Property{
		Label:          property.Label,
		Name:           property.Name,
		Tooltip:        property.Tooltip,
		Icon:           property.Icon,
		Text:           property.Text,
		Value:          property.Value,
		Unit:           property.Unit,
		Max:            property.Max,
		Min:            property.Min,
		Status:         property.Status,
		LastTransition: property.LastTransition,
		Links:          property.Links,
		Headline:       property.Headline,
		Type:           property.Type,
		Color:          property.Color,
	}
}

func (component Component) IsHealthy() bool {
	s := component.Summarize()
	return s.Healthy > 0 && s.Unhealthy == 0 && s.Warning == 0
}

func (component Component) Summarize() v1.Summary {
	s := v1.Summary{}
	if len(component.Components) == 0 {
		switch component.Status {
		case "healthy":
			s.Healthy++
		case "unhealthy":
			s.Unhealthy++
		case "warning":
			s.Warning++
		}
		return s
	}
	for _, child := range component.Components {
		s = s.Add(child.Summarize())
	}
	return s
}

func (components Components) Summarize() v1.Summary {
	s := v1.Summary{}
	for _, component := range components {
		s = s.Add(component.Summarize())
	}
	return s
}
