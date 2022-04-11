package api

import (
	"net/http"

	"github.com/flanksource/canary-checker/pkg/topology"
	"github.com/labstack/echo/v4"
)

func Topology(c echo.Context) error {
	results, err := topology.Query(topology.NewTopologyParams(c.Request().URL.Query()))
	if err != nil {
		return errorResonse(c, err, http.StatusBadRequest)
	}
	return c.JSON(http.StatusOK, results)
}
