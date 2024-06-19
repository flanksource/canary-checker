package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/canary-checker/pkg/topology"
	"github.com/flanksource/duty/context"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// TopologyQuery godoc
// @Id TopologyQuery
// @Summary      Topology query
// @Description Query the topology graph
// @Tags         topology
// @Produce      json
// @Param        id  query   string false "Topology ID"
// @Param        topologyId query  string false "Topology ID"
// @Param        componentId query   string false "Component ID"
// @Param        owner  query  string false "Owner"
// @Param        status  query  string false "Comma separated list of status"
// @Param        types    query string false "Comma separated list of types"
// @Param        flatten  query  string false "Flatten the topology"
// @Success      200  {object}  pkg.Components
// @Router /api/topology [get]
func Topology(c echo.Context) error {
	ctx := c.Request().Context().(context.Context)
	params := topology.NewTopologyParams(c.QueryParams())
	results, err := topology.Query(ctx, params)
	if err != nil {
		return errorResponse(c, err, http.StatusBadRequest)
	}

	return c.JSON(http.StatusOK, results)
}

func PushTopology(c echo.Context) error {
	if c.Request().Body == nil {
		return errorResponse(c, errors.New("missing request body"), http.StatusBadRequest)
	}
	defer c.Request().Body.Close()

	ctx := c.Request().Context().(context.Context)

	var data pkg.Component
	reqBody, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return errorResponse(c, fmt.Errorf("error reading request body: %w", err), http.StatusInternalServerError)
	}
	if err := json.Unmarshal(reqBody, &data); err != nil {
		return errorResponse(c, fmt.Errorf("error unmarshaling json: %w", err), http.StatusBadRequest)
	}

	agentID := uuid.Nil
	agentName := c.QueryParam("agentName")
	if agentName != "" {
		agent, err := db.GetAgent(ctx, agentName)
		if err != nil {
			return errorResponse(c, fmt.Errorf("agent [%s] not found: %w", agentName, err), http.StatusBadRequest)
		}
		agentID = agent.ID
	}

	topologyObj := pkg.Topology{
		ID:        data.TopologyID,
		AgentID:   agentID,
		Name:      data.Name,
		Namespace: data.Namespace,
		Labels:    data.Labels,
	}

	if _, err = db.PersistTopology(ctx, &topologyObj); err != nil {
		return errorResponse(c, fmt.Errorf("error persisting topology: %w", err), http.StatusInternalServerError)
	}

	data.AgentID = agentID
	for _, c := range data.Components.Walk() {
		c.AgentID = agentID
	}
	if _, err = db.PersistComponent(ctx, &data); err != nil {
		return errorResponse(c, fmt.Errorf("error persisting components: %w", err), http.StatusInternalServerError)
	}
	return c.JSON(http.StatusOK, data)
}
