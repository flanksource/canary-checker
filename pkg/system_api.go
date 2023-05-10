package pkg

import (
	"time"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg/db/types"
	"github.com/flanksource/duty/models"
	"github.com/google/uuid"
)

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
	_c := models.Component{
		Name:         c.Name,
		Owner:        c.Owner,
		Type:         c.Type,
		Order:        c.Order,
		Lifecycle:    c.Lifecycle,
		Tooltip:      c.Tooltip,
		Icon:         c.Icon,
		Selectors:    c.Selectors,
		Configs:      c.Configs.ToModel(),
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
