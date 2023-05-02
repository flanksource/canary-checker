package pkg

import (
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg/db/types"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/models"
	"github.com/google/uuid"
	jsontime "github.com/liamylian/jsontime/v2/v2"
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

func NewComponent(c v1.ComponentSpec) *models.Component {
	configs := NewConfigs(c.Configs)
	configJSON, _ := json.Marshal(configs)
	_c := models.Component{
		Name:         c.Name,
		Owner:        c.Owner,
		Type:         c.Type,
		Order:        c.Order,
		Lifecycle:    c.Lifecycle,
		Tooltip:      c.Tooltip,
		Icon:         c.Icon,
		Selectors:    c.Selectors,
		Configs:      configJSON,
		LogSelectors: c.LogSelectors,
	}

	if c.Summary != nil {
		_c.Summary = *c.Summary
	}

	return &_c
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
