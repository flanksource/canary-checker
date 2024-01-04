package pkg

import (
	"fmt"
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/commons/console"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/models"
	dutyTypes "github.com/flanksource/duty/types"
	"github.com/google/uuid"
	jsontime "github.com/liamylian/jsontime/v2/v2"
	"gorm.io/gorm"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sTypes "k8s.io/apimachinery/pkg/types"
)

var json = jsontime.ConfigWithCustomTimeFormat

const ComponentType = "component"

// Topology mirrors the models.Topology struct except that instead of raw JSON serialized to the DB, it has the full CRD based spec.
type Topology struct {
	ID        uuid.UUID `gorm:"default:generate_ulid()"`
	Name      string
	Namespace string
	Labels    dutyTypes.JSONStringMap
	Spec      dutyTypes.JSON
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
		Labels:    dutyTypes.JSONStringMap(topology.GetLabels()),
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
	Name      string                  `json:"name,omitempty"`
	Namespace string                  `json:"namespace,omitempty"`
	Labels    dutyTypes.JSONStringMap `json:"labels,omitempty"`
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

// Component mirrors the models.Component struct except that instead of raw JSON serialized to the DB, it has the full CRD based spec.
type Component struct {
	Name         string                    `json:"name,omitempty"`
	ID           uuid.UUID                 `json:"id,omitempty" gorm:"default:generate_ulid()"` //nolint
	Text         string                    `json:"text,omitempty"`
	Schedule     string                    `json:"schedule,omitempty"`
	TopologyType string                    `json:"topology_type,omitempty"`
	Namespace    string                    `json:"namespace,omitempty"`
	Labels       dutyTypes.JSONStringMap   `json:"labels,omitempty"`
	Tooltip      string                    `json:"tooltip,omitempty"`
	Icon         string                    `json:"icon,omitempty"`
	Owner        string                    `json:"owner,omitempty"`
	Status       dutyTypes.ComponentStatus `json:"status,omitempty"`
	StatusReason dutyTypes.NullString      `json:"status_reason,omitempty"`
	Path         string                    `json:"path,omitempty"`
	Order        int                       `json:"order,omitempty"  gorm:"-"`
	// The type of component, e.g. service, API, website, library, database, etc.
	Type    string            `json:"type,omitempty"`
	Summary dutyTypes.Summary `json:"summary,omitempty" gorm:"type:summary"`
	// The lifecycle state of the component e.g. production, staging, dev, etc.
	Lifecycle       string                      `json:"lifecycle,omitempty"`
	Properties      dutyTypes.Properties        `json:"properties,omitempty" gorm:"type:properties"`
	Components      Components                  `json:"components,omitempty" gorm:"-"`
	ParentId        *uuid.UUID                  `json:"parent_id,omitempty"` //nolint
	Selectors       dutyTypes.ResourceSelectors `json:"selectors,omitempty" gorm:"resourceSelectors" swaggerignore:"true"`
	ComponentChecks v1.ComponentChecks          `json:"-" gorm:"componentChecks" swaggerignore:"true"`
	Checks          Checks                      `json:"checks,omitempty" gorm:"-"`
	Configs         dutyTypes.ConfigQueries     `json:"configs,omitempty" gorm:"type:configs"`
	TopologyID      *uuid.UUID                  `json:"topology_id,omitempty"` //nolint
	CreatedAt       time.Time                   `json:"created_at,omitempty" time_format:"postgres_timestamp"`
	UpdatedAt       time.Time                   `json:"updated_at,omitempty" time_format:"postgres_timestamp"`
	DeletedAt       *time.Time                  `json:"deleted_at,omitempty" time_format:"postgres_timestamp" swaggerignore:"true"`
	ExternalId      string                      `json:"external_id,omitempty"` //nolint
	IsLeaf          bool                        `json:"is_leaf"`
	SelectorID      string                      `json:"-" gorm:"-"`
	Incidents       []dutyTypes.Incident        `json:"incidents,omitempty" gorm:"-"`
	ConfigInsights  []map[string]interface{}    `json:"insights,omitempty" gorm:"-"`
	CostPerMinute   float64                     `json:"cost_per_minute,omitempty" gorm:"column:cost_per_minute"`
	CostTotal1d     float64                     `json:"cost_total_1d,omitempty" gorm:"column:cost_total_1d"`
	CostTotal7d     float64                     `json:"cost_total_7d,omitempty" gorm:"column:cost_total_7d"`
	CostTotal30d    float64                     `json:"cost_total_30d,omitempty" gorm:"column:cost_total_30d"`
	LogSelectors    dutyTypes.LogSelectors      `json:"logs,omitempty" gorm:"column:log_selectors"`
}

func (component *Component) FindExisting(db *gorm.DB) (*models.Component, error) {
	var existing models.Component
	tx := db.Model(component).Select("id", "deleted_at")
	if component.ID == uuid.Nil {
		if component.ParentId == nil {
			tx = tx.Find(&existing, "name = ? AND type = ? and parent_id is NULL", component.Name, component.Type)
		} else {
			tx = tx.Find(&existing, "name = ? AND type = ? and parent_id = ?", component.Name, component.Type, component.ParentId).Pluck("id", &existing)
		}
	} else {
		if component.ParentId == nil {
			tx = tx.Find(&existing, "topology_id = ? AND name = ? AND type = ? and parent_id is NULL", component.TopologyID, component.Name, component.Type).Pluck("id", &existing)
		} else {
			tx = tx.Find(&existing, "topology_id = ? AND name = ? AND type = ? and parent_id = ?", component.TopologyID, component.Name, component.Type, component.ParentId).Pluck("id", &existing)
		}
	}
	return &existing, tx.Error
}

func (component *Component) GetConfigs(db *gorm.DB) (relationships []models.ConfigComponentRelationship, err error) {
	err = db.Where("component_id = ? AND deleted_at IS NULL", component.ID).Find(&relationships).Error
	return relationships, err
}

func (component *Component) GetChecks(db *gorm.DB) (relationships []models.CheckComponentRelationship, err error) {
	err = db.Where("component_id = ? AND deleted_at IS NULL", component.ID).Find(&relationships).Error
	return relationships, err
}

func (component *Component) GetChildren(db *gorm.DB) (relationships []models.ComponentRelationship, err error) {
	err = db.Where("relationship_id = ? AND deleted_at IS NULL", component.ID).Find(&relationships).Error
	return relationships, err
}

func (component *Component) GetParents(db *gorm.DB) (relationships []models.CheckComponentRelationship, err error) {
	err = db.Where("component_id = ? AND deleted_at IS NULL", component.ID).Find(&relationships).Error
	return relationships, err
}

func (component *Component) Clone() Component {
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
		Configs:         c.Configs,
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
			status = dutyTypes.ComponentStatus(console.Greenf(string(status)))
		} else {
			status = dutyTypes.ComponentStatus(console.Redf(string(status)))
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

func NewProperty(property v1.Property) *dutyTypes.Property {
	return &dutyTypes.Property{
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

func (component Component) Summarize() dutyTypes.Summary {
	s := dutyTypes.Summary{
		Incidents: component.Summary.Incidents,
		Insights:  component.Summary.Insights,
	}
	if component.Checks != nil && component.Components == nil {
		for _, check := range component.Checks {
			if dutyTypes.ComponentStatus(check.Status) == dutyTypes.ComponentStatusHealthy {
				s.Healthy++
			} else {
				s.Unhealthy++
			}
		}
		return s
	}
	if len(component.Components) == 0 {
		switch component.Status {
		case dutyTypes.ComponentStatusHealthy:
			s.Healthy++
		case dutyTypes.ComponentStatusUnhealthy:
			s.Unhealthy++
		case dutyTypes.ComponentStatusWarning:
			s.Warning++
		case dutyTypes.ComponentStatusInfo:
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

func (components Components) Summarize() dutyTypes.Summary {
	s := dutyTypes.Summary{}
	for _, component := range components {
		s = s.Add(component.Summarize())
	}
	return s
}

func (component Component) GetStatus() dutyTypes.ComponentStatus {
	if component.Summary.Healthy > 0 && component.Summary.Unhealthy > 0 {
		return dutyTypes.ComponentStatusWarning
	} else if component.Summary.Unhealthy > 0 {
		return dutyTypes.ComponentStatusUnhealthy
	} else if component.Summary.Warning > 0 {
		return dutyTypes.ComponentStatusWarning
	} else if component.Summary.Healthy > 0 {
		return dutyTypes.ComponentStatusHealthy
	} else {
		return dutyTypes.ComponentStatusInfo
	}
}
