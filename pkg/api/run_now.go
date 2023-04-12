package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/flanksource/canary-checker/api/context"
	"github.com/flanksource/canary-checker/checks"
	"github.com/flanksource/canary-checker/pkg"
	"github.com/flanksource/canary-checker/pkg/db"
	"github.com/flanksource/commons/logger"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// RunNowRequest represents the request body for a run now request
type RunNowRequest struct {
	CanaryID string `json:"id"`
}

// RunNowResponse represents the response body for a run now request
type RunNowResponse struct {
	Total   int      `json:"total"`
	Failed  int      `json:"failed"`
	Success int      `json:"success"`
	Errors  []string `json:"errors,omitempty"`
}

func (t *RunNowResponse) FromCheckResults(result []*pkg.CheckResult) {
	t.Total = len(result)
	for _, r := range result {
		if r.Pass {
			t.Success++
			continue
		}

		t.Failed++
		if r.Error != "" {
			t.Errors = append(t.Errors, r.Error)
		}
	}
}

func RunNowHandler(c echo.Context) error {
	var req RunNowRequest
	if err := c.Bind(&req); err != nil {
		return errorResonse(c, err, http.StatusBadRequest)
	}

	canaryModel, err := db.GetCanary(req.CanaryID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errorResonse(c, fmt.Errorf("canary with id=%s was not found", req.CanaryID), http.StatusNotFound)
		}

		return errorResonse(c, err, http.StatusBadRequest)
	}

	canary, err := canaryModel.ToV1()
	if err != nil {
		return errorResonse(c, err, http.StatusInternalServerError)
	}

	kommonsClient, err := pkg.NewKommonsClient()
	if err != nil {
		logger.Warnf("failed to get kommons client, features that read kubernetes configs will fail: %v", err)
	}

	ctx := context.New(kommonsClient, *canary)
	result := checks.RunChecks(ctx)

	var response RunNowResponse
	response.FromCheckResults(result)
	return c.JSON(http.StatusOK, response)
}
