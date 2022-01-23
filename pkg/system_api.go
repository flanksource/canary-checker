package pkg

import (
	"encoding/json"
	"fmt"
	"strings"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/commons/logger"
)

type System struct {
	Object     `yaml:",inline"`
	Id         string     `json:"id"`
	Tooltip    string     `json:"tooltip,omitempty"`
	Icon       string     `json:"icon,omitempty"`
	Text       string     `json:"text,omitempty"`
	Label      string     `json:"label,omitempty"`
	Owner      string     `json:"owner,omitempty"`
	Components Components `json:"components,omitempty"`
	Properties Properties `json:"properties,omitempty"`
	Summary    Summary    `json:"summary,omitempty"`
	Status     string     `json:"status,omitempty"`
	Type       string     `json:"type,omitempty"`
	CreatedAt  string     `json:"created_at,omitempty"`
	UpdatedAt  string     `json:"updated_at,omitempty"`
	ExternalId string     `json:"external_id,omitempty"`
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

func (components *Components) UnmarshalJSON(b []byte) error {
	var flat []Component
	if err := json.Unmarshal(b, &flat); err != nil {
		return err
	}
	for _, c := range flat {
		if c.ParentId == "" {
			// first add parents
			parent := c
			*components = append(*components, &parent)
		}
	}

	for _, c := range flat {
		if c.ParentId != "" {
			parent := components.FindById(c.ParentId)
			if parent == nil {
				logger.Errorf("Invalid parent id %s for component %s in (%s)", c.ParentId, c.Id, strings.Join(components.GetIds(), ","))
				*components = append(*components, &c)
			} else {
				c.ParentId = ""
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
	Id           string            `json:"id,omitempty"`
	Namespace    string            `json:"namespace,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	Tooltip      string            `json:"tooltip,omitempty"`
	Icon         string            `json:"icon,omitempty"`
	Owner        string            `json:"owner,omitempty"`
	Status       string            `json:"status,omitempty"`
	StatusReason string            `json:"statusReason,omitempty"`
	// The type of component, e.g. service, API, website, library, database, etc.
	Type    string  `json:"type,omitempty"`
	Summary Summary `json:"summary,omitempty"`
	// The lifecycle state of the component e.g. production, staging, dev, etc.
	Lifecycle     string             `json:"lifecycle,omitempty"`
	Relationships []RelationshipSpec `json:"relationships,omitempty"`
	Properties    Properties         `json:"properties,omitempty"`
	Components    Components         `json:"components,omitempty"`
	ParentId      string             `json:"parent_id,omitempty"`
	CreatedAt     string             `json:"created_at,omitempty"`
	UpdatedAt     string             `json:"updated_at,omitempty"`
	ExternalId    string             `json:"external_id,omitempty"`
}

func (c Component) GetAsEnvironment() map[string]interface{} {
	return map[string]interface{}{
		"self":       c,
		"properties": c.Properties.AsMap(),
	}
}

func NewComponent(c v1.ComponentSpec) *Component {
	return &Component{
		Name:      c.Name,
		Owner:     c.Owner,
		Type:      c.Type,
		Lifecycle: c.Lifecycle,
	}
}

type Components []*Component

func (c Components) Find(name string) *Component {
	for _, component := range c {
		if component.Name == name {
			return component
		}
	}
	return nil
}

func (c Components) GetIds() []string {
	ids := []string{}
	for _, component := range c {
		ids = append(ids, component.Id)
	}
	return ids
}

func (c Components) FindById(id string) *Component {
	for _, component := range c {
		if component.Id == id {
			return component
		}
	}
	return nil
}

type ComponentStatus struct {
	Status ComponentPropertyStatus `json:"status,omitempty"`
}

type Summary struct {
	Healthy   int `json:"healthy,omitempty"`
	Unhealthy int `json:"unhealthy,omitempty"`
	Warning   int `json:"warning,omitempty"`
	Info      int `json:"info,omitempty"`
}

func (s Summary) GetStatus() string {
	if s.Unhealthy > 0 {
		return "unhealthy"
	} else if s.Warning > 0 {
		return "warning"
	} else if s.Healthy > 0 {
		return "healthy"
	}
	return "unknown"
}

func (s Summary) Add(b Summary) Summary {
	return Summary{
		Healthy:   s.Healthy + b.Healthy,
		Unhealthy: s.Unhealthy + b.Unhealthy,
		Warning:   s.Warning + b.Warning,
		Info:      s.Info + b.Info,
	}
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
	}
}

func (component Component) Summarize() Summary {
	s := Summary{}
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

func (components Components) Summarize() Summary {
	s := Summary{}
	for _, component := range components {
		s = s.Add(component.Summarize())
	}
	return s
}
