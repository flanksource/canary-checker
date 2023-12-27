package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	pkgTopology "github.com/flanksource/canary-checker/pkg/topology"
	dutyContext "github.com/flanksource/duty/context"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type CheckErrorMessage struct {
	Description string `json:"description"`
	Error       string `json:"error"`
}

// RunCanaryResponse represents the response body for a run now request
type RunCanaryResponse struct {
	Total   int                 `json:"total"`
	Failed  int                 `json:"failed"`
	Success int                 `json:"success"`
	Errors  []CheckErrorMessage `json:"errors,omitempty"`
}

func (t *RunCanaryResponse) FromCheckResults(result []*pkg.CheckResult) {
	t.Total = len(result)
	for _, r := range result {
		if r.Pass {
			t.Success++
			continue
		}

		t.Failed++
		if r.Error != "" {
			t.Errors = append(t.Errors, CheckErrorMessage{
				Description: r.GetDescription(),
				Error:       r.Error,
			})
		}
	}
}

func RunCanaryHandler(c echo.Context) error {
	id := c.Param("id")

	duty := c.Request().Context().(dutyContext.Context)

	canaryModel, err := db.FindCanaryByID(duty, id)
	if err != nil {
		return errorResponse(c, err, http.StatusInternalServerError)
	}

	if canaryModel == nil {
		return errorResponse(c, fmt.Errorf("canary with id=%s was not found", id), http.StatusNotFound)
	}

	canary, err := canaryModel.ToV1()
	ctx := context.New(duty, *canary)
	if err != nil {
		return errorResponse(c, err, http.StatusInternalServerError)
	}

	result, err := checks.RunChecks(ctx)
	if err != nil {
		return errorResponse(c, err, http.StatusInternalServerError)
	}

	var response RunCanaryResponse
	response.FromCheckResults(result)
	return c.JSON(http.StatusOK, response)
}

func RunTopologyHandler(c echo.Context) error {
	id := c.Param("id")

	topologyRunDepth := 10
	_depth := c.QueryParam("depth")
	if _depth != "" {
		num, err := strconv.Atoi(_depth)
		if err != nil {
			return errorResponse(c, err, http.StatusBadRequest)
		}

		if num < 0 {
			return errorResponse(c, fmt.Errorf("depth must be greater than 0"), http.StatusBadRequest)
		}

		topologyRunDepth = num
	}

	ctx := c.Request().Context().(dutyContext.Context)
	topology, err := db.GetTopology(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorResponse(c, fmt.Errorf("topology with id=%s was not found", id), http.StatusNotFound)
		}

		return errorResponse(c, err, http.StatusInternalServerError)
	}

	opts := pkgTopology.TopologyRunOptions{
		Context:   ctx,
		Depth:     topologyRunDepth,
		Namespace: topology.Namespace,
	}
	if err := pkgTopology.SyncComponents(opts, *topology); err != nil {
		return errorResponse(c, err, http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
