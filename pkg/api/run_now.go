package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// RunNowRequest represents the request body for a run now request
type RunNowRequest struct {
	CanaryID string `json:"id"`
}

// RunNowResponse represents the response body for a run now request
type RunNowResponse struct {
	Results []*pkg.CheckResult `json:"results"`
}

func RunNowHandler(c echo.Context) error {
	var req RunNowRequest
	if err := c.Bind(&req); err != nil {
		return errorResonse(c, err, http.StatusBadRequest)
	}

	canaryModel, err := db.GetCanary(req.CanaryID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorResonse(c, fmt.Errorf("canary with id=%s was not found.", req.CanaryID), http.StatusNotFound)
		}

		return errorResonse(c, err, http.StatusBadRequest)
	}

	canary, err := canaryModel.ToV1()
	if err != nil {
		return errorResonse(c, err, http.StatusInternalServerError)
	}

	ctx := context.New(nil, *canary)
	result := checks.RunChecks(ctx)
	return c.JSON(http.StatusOK, RunNowResponse{Results: result})
}
