package api

import (
	"net/http"

	"github.com/flanksource/canary-checker/pkg/topology"
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
	params := topology.NewTopologyParams(c.QueryParams())
	results, err := topology.Query(params)
	if err != nil {
		return errorResonse(c, err, http.StatusBadRequest)
	}

	return c.JSON(http.StatusOK, results)
}
