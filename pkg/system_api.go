package pkg

import (
	"context"
	"database/sql/driver"
	"fmt"
	"strings"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg/db/types"
	"github.com/flanksource/commons/console"
	"github.com/flanksource/commons/logger"
	dutyTypes "github.com/flanksource/duty/types"
	"github.com/google/uuid"
	jsontime "github.com/liamylian/jsontime/v2/v2"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sTypes "k8s.io/apimachinery/pkg/types"
)

var json = jsontime.ConfigWithCustomTimeFormat

type ComponentStatus string

var (
	ComponentPropertyStatusHealthy   ComponentStatus = "healthy"
	ComponentPropertyStatusUnhealthy ComponentStatus = "unhealthy"
	ComponentPropertyStatusWarning   ComponentStatus = "warning"
	ComponentPropertyStatusError     ComponentStatus = "error"
	ComponentPropertyStatusInfo      ComponentStatus = "info"

	ComponentStatusOrder = map[ComponentStatus]int{
		ComponentPropertyStatusInfo:      0,
		ComponentPropertyStatusHealthy:   1,
		ComponentPropertyStatusUnhealthy: 2,
		ComponentPropertyStatusWarning:   3,
		ComponentPropertyStatusError:     4,
	}
)

func (status ComponentStatus) Compare(other ComponentStatus) int {
	if status == other {
		return 0
	}
	if ComponentStatusOrder[status] > ComponentStatusOrder[other] {
		return 1
	}
	return -1
}

const ComponentType = "component"

type Topology struct {
	ID        uuid.UUID `gorm:"default:generate_ulid()"`
	Name      string
	Namespace string
	Labels    types.JSONStringMap
	Spec      types.JSON
	Schedule  string
	CreatedAt time.Time  `json:"created_at,omitempty" time_format:"postgres_timestamp"`
	UpdatedAt time.Time  `json:"updated_at,omitempty" time_format:"postgres_timestamp"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" time_format:"postgres_timestamp"`
}

func TopologyFromV1(topology *v1.Topology) *Topology {
	spec, _ := json.Marshal(topology.Spec)
	return &Topology{
		Name:      topology.GetName(),
		Namespace: topology.GetNamespace(),
		Labels:    types.JSONStringMap(topology.GetLabels()),
		Spec:      spec,
	}
}

func (s *Topology) ToV1() v1.Topology {
	var topologySpec v1.TopologySpec
	id := s.ID.String()
	if err := json.Unmarshal(s.Spec, &topologySpec); err != nil {
		logger.Errorf("error unmarshalling topology spec %s", err)
	}
	return v1.Topology{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Topology",
			APIVersion: "canaries.flanksource.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Name,
			Namespace: s.Namespace,
			Labels:    s.Labels,
			UID:       k8sTypes.UID(id),
		},
		Spec: topologySpec,
	}
}

type Object struct {
	Name      string              `json:"name,omitempty"`
	Namespace string              `json:"namespace,omitempty"`
	Labels    types.JSONStringMap `json:"labels,omitempty"`
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
	for _, _c := range flat {
		c := _c
		c.TopologyType = ComponentType
		*components = append(*components, &c)
	}
	return nil
}

func (components Components) Walk() Components {
	var comps Components
	for _, _c := range components {
		c := _c
		comps = append(comps, c)
		if c.Components != nil {
			comps = append(comps, c.Components.Walk()...)
		}
	}
	return comps
}

type Component struct {
	Name         string               `json:"name,omitempty"`
	ID           uuid.UUID            `json:"id,omitempty" gorm:"default:generate_ulid()"` //nolint
	Text         string               `json:"text,omitempty"`
	Schedule     string               `json:"schedule,omitempty"`
	TopologyType string               `json:"topology_type,omitempty"`
	Namespace    string               `json:"namespace,omitempty"`
	Labels       types.JSONStringMap  `json:"labels,omitempty"`
	Tooltip      string               `json:"tooltip,omitempty"`
	Icon         string               `json:"icon,omitempty"`
	Owner        string               `json:"owner,omitempty"`
	Status       ComponentStatus      `json:"status,omitempty"`
	StatusReason dutyTypes.NullString `json:"status_reason,omitempty"`
	Path         string               `json:"path,omitempty"`
	Order        int                  `json:"order,omitempty"  gorm:"-"`
	// The type of component, e.g. service, API, website, library, database, etc.
	Type    string     `json:"type,omitempty"`
	Summary v1.Summary `json:"summary,omitempty" gorm:"type:summary"`
	// The lifecycle state of the component e.g. production, staging, dev, etc.
	Lifecycle       string                   `json:"lifecycle,omitempty"`
	Properties      Properties               `json:"properties,omitempty" gorm:"type:properties"`
	Components      Components               `json:"components,omitempty" gorm:"-"`
	ParentId        *uuid.UUID               `json:"parent_id,omitempty"` //nolint
	Selectors       v1.ResourceSelectors     `json:"selectors,omitempty" gorm:"resourceSelectors" swaggerignore:"true"`
	ComponentChecks v1.ComponentChecks       `json:"-" gorm:"componentChecks" swaggerignore:"true"`
	Checks          Checks                   `json:"checks,omitempty" gorm:"-"`
	Configs         Configs                  `json:"configs,omitempty" gorm:"type:configs"`
	TopologyID      *uuid.UUID               `json:"topology_id,omitempty"` //nolint
	CreatedAt       time.Time                `json:"created_at,omitempty" time_format:"postgres_timestamp"`
	UpdatedAt       time.Time                `json:"updated_at,omitempty" time_format:"postgres_timestamp"`
	DeletedAt       *time.Time               `json:"deleted_at,omitempty" time_format:"postgres_timestamp" swaggerignore:"true"`
	ExternalId      string                   `json:"external_id,omitempty"` //nolint
	IsLeaf          bool                     `json:"is_leaf"`
	SelectorID      string                   `json:"-" gorm:"-"`
	Incidents       []Incident               `json:"incidents,omitempty" gorm:"-"`
	ConfigInsights  []map[string]interface{} `json:"insights,omitempty" gorm:"-"`
	CostPerMinute   float64                  `json:"cost_per_minute,omitempty" gorm:"column:cost_per_minute"`
	CostTotal1d     float64                  `json:"cost_total_1d,omitempty" gorm:"column:cost_total_1d"`
	CostTotal7d     float64                  `json:"cost_total_7d,omitempty" gorm:"column:cost_total_7d"`
	CostTotal30d    float64                  `json:"cost_total_30d,omitempty" gorm:"column:cost_total_30d"`
	LogSelectors    dutyTypes.LogSelectors   `json:"logs,omitempty" gorm:"column:log_selectors"`
}

type ComponentRelationship struct {
	ComponentID      uuid.UUID  `json:"component_id,omitempty"`
	RelationshipID   uuid.UUID  `json:"relationship_id,omitempty"`
	SelectorID       string     `json:"selector_id,omitempty"`
	RelationshipPath string     `json:"relationship_path,omitempty"`
	CreatedAt        time.Time  `json:"created_at,omitempty"`
	UpdatedAt        time.Time  `json:"updated_at,omitempty"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
}

type CheckComponentRelationship struct {
	ComponentID uuid.UUID  `json:"component_id,omitempty"`
	CheckID     uuid.UUID  `json:"check_id,omitempty"`
	CanaryID    uuid.UUID  `json:"canary_id,omitempty"`
	SelectorID  string     `json:"selector_id,omitempty"`
	CreatedAt   time.Time  `json:"created_at,omitempty"`
	UpdatedAt   time.Time  `json:"updated_at,omitempty"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

func (component Component) Clone() Component {
	clone := Component{
		Name:            component.Name,
		TopologyType:    component.TopologyType,
		Order:           component.Order,
		ID:              component.ID,
		Text:            component.Text,
		Namespace:       component.Namespace,
		Labels:          component.Labels,
		Tooltip:         component.Tooltip,
		Icon:            component.Icon,
		Owner:           component.Owner,
		Status:          component.Status,
		StatusReason:    component.StatusReason,
		Type:            component.Type,
		Lifecycle:       component.Lifecycle,
		Checks:          component.Checks,
		Configs:         component.Configs,
		ComponentChecks: component.ComponentChecks,
		Properties:      component.Properties,
		ExternalId:      component.ExternalId,
		Schedule:        component.Schedule,
	}

	copy(clone.LogSelectors, component.LogSelectors)
	return clone
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
		s += component.ExternalId
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
	configs := NewConfigs(c.Configs)
	_c := Component{
		Name:            c.Name,
		Owner:           c.Owner,
		Type:            c.Type,
		Order:           c.Order,
		Lifecycle:       c.Lifecycle,
		Tooltip:         c.Tooltip,
		Icon:            c.Icon,
		Selectors:       c.Selectors,
		ComponentChecks: c.ComponentChecks,
		Labels:          c.Labels,
		Configs:         configs,
		LogSelectors:    c.LogSelectors,
	}
	if c.Summary != nil {
		_c.Summary = *c.Summary
	}
	return &_c
}

func (component Component) GetID() string {
	if component.ID != uuid.Nil {
		return component.ID.String()
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
			status = ComponentStatus(console.Greenf(string(status)))
		} else {
			status = ComponentStatus(console.Redf(string(status)))
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

type Incident struct {
	ID          uuid.UUID `json:"id"`
	Type        string    `json:"type"`
	Title       string    `json:"title"`
	Severity    int       `json:"severity"`
	Description string    `json:"description"`
}

// Property is a realized v1.Property without the lookup definition
type Property struct {
	Label    string `json:"label,omitempty"`
	Name     string `json:"name,omitempty"`
	Tooltip  string `json:"tooltip,omitempty"`
	Icon     string `json:"icon,omitempty"`
	Type     string `json:"type,omitempty"`
	Color    string `json:"color,omitempty"`
	Order    int    `json:"order,omitempty"`
	Headline bool   `json:"headline,omitempty"`

	// Either text or value is required, but not both.
	Text  string `json:"text,omitempty"`
	Value int64  `json:"value,omitempty"`

	// e.g. milliseconds, bytes, millicores, epoch etc.
	Unit string `json:"unit,omitempty"`
	Max  *int64 `json:"max,omitempty"`
	Min  *int64 `json:"min,omitempty"`

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
	if p.Min != nil {
		s += fmt.Sprintf("min=%d ", *p.Min)
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
	if other.Min != nil {
		p.Min = other.Min
	}
	if other.Order > 0 {
		p.Order = other.Order
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
		Order:          property.Order,
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
	s := v1.Summary{
		Incidents: component.Summary.Incidents,
		Insights:  component.Summary.Insights,
	}
	if component.Checks != nil && component.Components == nil {
		for _, check := range component.Checks {
			if ComponentStatus(check.Status) == ComponentPropertyStatusHealthy {
				s.Healthy++
			} else {
				s.Unhealthy++
			}
		}
		return s
	}
	if len(component.Components) == 0 {
		switch component.Status {
		case ComponentPropertyStatusHealthy:
			s.Healthy++
		case ComponentPropertyStatusUnhealthy:
			s.Unhealthy++
		case ComponentPropertyStatusWarning:
			s.Warning++
		case ComponentPropertyStatusInfo:
			s.Info++
		}
		return s
	}
	for _, child := range component.Components {
		s = s.Add(child.Summarize())
		child.Summary = child.Summarize()
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

func (component Component) GetStatus() ComponentStatus {
	if component.Summary.Healthy > 0 && component.Summary.Unhealthy > 0 {
		return ComponentPropertyStatusWarning
	} else if component.Summary.Unhealthy > 0 {
		return ComponentPropertyStatusUnhealthy
	} else if component.Summary.Warning > 0 {
		return ComponentPropertyStatusWarning
	} else if component.Summary.Healthy > 0 {
		return ComponentPropertyStatusHealthy
	} else {
		return ComponentPropertyStatusInfo
	}
}

// Scan scan value into Jsonb, implements sql.Scanner interface
func (p Properties) Value() (driver.Value, error) {
	if len(p) == 0 {
		return nil, nil
	}
	return p.AsJSON(), nil
}

// Scan scan value into Jsonb, implements sql.Scanner interface
func (p *Properties) Scan(val interface{}) error {
	if val == nil {
		*p = make(Properties, 0)
		return nil
	}
	var ba []byte
	switch v := val.(type) {
	case []byte:
		ba = v
	default:
		return errors.New(fmt.Sprint("Failed to unmarshal properties value:", val))
	}
	err := json.Unmarshal(ba, p)
	return err
}

// GormDataType gorm common data type
func (Properties) GormDataType() string {
	return "properties"
}

func (Properties) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	switch db.Dialector.Name() {
	case "sqlite":
		return "TEXT"
	case "postgres":
		return "JSONB"
	case "sqlserver":
		return "NVARCHAR(MAX)"
	}
	return ""
}

func (p Properties) GormValue(ctx context.Context, db *gorm.DB) clause.Expr {
	data, _ := json.Marshal(p)
	return gorm.Expr("?", data)
}

func (c Configs) GormValue(ctx context.Context, db *gorm.DB) clause.Expr {
	data, _ := json.Marshal(c)
	return gorm.Expr("?", data)
}

// Scan scan value into Jsonb, implements sql.Scanner interface
func (c Configs) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Scan scan value into Jsonb, implements sql.Scanner interface
func (c *Configs) Scan(val interface{}) error {
	if val == nil {
		*c = Configs{}
		return nil
	}
	var ba []byte
	switch v := val.(type) {
	case []byte:
		ba = v
	default:
		return errors.New(fmt.Sprint("Failed to unmarshal properties value:", val))
	}
	err := json.Unmarshal(ba, c)
	return err
}

// GormDataType gorm common data type
func (Configs) GormDataType() string {
	return "configs"
}

func (Configs) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	switch db.Dialector.Name() {
	case "sqlite":
		return "TEXT"
	case "postgres":
		return "JSONB"
	case "sqlserver":
		return "NVARCHAR(MAX)"
	}
	return ""
}
