package api

import (
	"net/http"

	"github.com/flanksource/canary-checker/pkg/topology"
	"github.com/flanksource/commons/logger"
	"github.com/flanksource/duty/context"
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
	logger.Infof("YASH 1 %v", params)
	results, err := topology.Query(ctx, params)
	if err != nil {
		return errorResponse(c, err, http.StatusBadRequest)
	}

	return c.JSON(http.StatusOK, results)
}
