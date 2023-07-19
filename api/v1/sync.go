package v1

import (
	"github.com/flanksource/duty/models"
	"github.com/google/uuid"
)

type PushData struct {
	Checks        []models.Check       `json:"checks,omitempty"`
	CheckStatuses []models.CheckStatus `json:"check_statuses,omitempty"`
}

func (t *PushData) SetAgentID(id uuid.UUID) {
	for i := range t.Checks {
		t.Checks[i].AgentID = id
	}
}
