package api

import (
	"encoding/json"
	"net/http"

	v1 "github.com/flanksource/canary-checker/api/v1"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/labstack/echo/v4"
)

// Pull returns all canaries for the requested agent
func Pull(c echo.Context) error {
	agentName := c.Param("agent_name")

	canaries, err := db.GetCanariesOfAgent(c.Request().Context(), agentName)
	if err != nil {
		return errorResonse(c, err, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, canaries)
}

// Push stores all the check statuses sent by the agent
func Push(c echo.Context) error {
	agentName := c.Param("agent_name")

	agent, err := db.FindAgent(c.Request().Context(), agentName)
	if err != nil {
		return errorResonse(c, err, http.StatusInternalServerError)
	} else if agent == nil {
		return errorResonse(c, err, http.StatusNotFound)
	}

	var req v1.PushData
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return errorResonse(c, err, http.StatusBadRequest)
	}
	req.SetAgentID(agent.ID)

	return db.InsertAgentCheckResults(c.Request().Context(), req)
}
