package api

import (
	"math"
	"net/http"
	"time"

	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/runner"
	"github.com/flanksource/duty/models"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"

	"github.com/flanksource/canary-checker/pkg"
)

// The maximum data points returned by the graph API
const maxCheckStatuses = 100

var DefaultWindow = "1h"

type Response struct {
	Duration      int           `json:"duration,omitempty"`
	RunnerName    string        `json:"runnerName"`
	Checks        pkg.Checks    `json:"checks"`
	ChecksSummary models.Checks `json:"checks_summary,omitempty"`
}

type DetailResponse struct {
	Duration   int              `json:"duration,omitempty"`
	RunnerName string           `json:"runnerName"`
	Status     []pkg.Timeseries `json:"status"`
	Latency    pkg.Latency      `json:"latency"`
	Uptime     pkg.Uptime       `json:"uptime"`
}

func About(c echo.Context) error {
	data := map[string]interface{}{
		"Timestamp": time.Now(),
		"Version":   runner.Version,
	}
	return c.JSON(http.StatusOK, data)
}

func CheckDetails(c echo.Context) error {
	q, err := cache.ParseQuery(c)
	if err != nil {
		return errorResonse(c, err, http.StatusBadRequest)
	}

	start := time.Now()

	summary, err := cache.PostgresCache.Query(*q)
	if err != nil {
		return errorResonse(c, err, http.StatusInternalServerError)
	}
	checkSummary := summary[0]

	totalChecks := checkSummary.Uptime.Total()
	if totalChecks <= maxCheckStatuses {
		q.WindowDuration = time.Second // TODO: Maybe do not window at all
	} else {
		startTime := q.GetStartTime()
		if startTime == nil {
			return errorResonse(c, errors.New("start time must be a duration or RFC3339 timestamp"), http.StatusBadRequest)
		}

		startDuration := time.Since(*startTime)

		windowsCount := int(math.Ceil(float64(totalChecks) / float64(maxCheckStatuses)))
		q.WindowDuration = startDuration / time.Duration(windowsCount)
	}

	results, err := cache.PostgresCache.QueryStatus(c.Request().Context(), *q)
	if err != nil {
		return errorResonse(c, err, http.StatusInternalServerError)
	}

	apiResponse := &DetailResponse{
		RunnerName: runner.RunnerName,
		Status:     results,
		Duration:   int(time.Since(start).Milliseconds()),
		Latency:    checkSummary.Latency,
		Uptime:     checkSummary.Uptime,
	}

	return c.JSON(http.StatusOK, apiResponse)
}

func CheckSummary(c echo.Context) error {
	q, err := cache.ParseQuery(c)
	if err != nil {
		return errorResonse(c, err, http.StatusBadRequest)
	}

	start := time.Now()
	results, err := cache.PostgresCache.Query(*q)
	if err != nil {
		return errorResonse(c, err, http.StatusInternalServerError)
	}

	apiResponse := &Response{
		RunnerName: runner.RunnerName,
		Checks:     results,
		Duration:   int(time.Since(start).Milliseconds()),
	}
	return c.JSON(http.StatusOK, apiResponse)
}

func HealthSummary(c echo.Context) error {
	start := time.Now()
	results, err := cache.PostgresCache.QuerySummary()
	if err != nil {
		return errorResonse(c, err, http.StatusInternalServerError)
	}

	apiResponse := &Response{
		RunnerName:    runner.RunnerName,
		ChecksSummary: results,
		Duration:      int(time.Since(start).Milliseconds()),
	}
	return c.JSON(http.StatusOK, apiResponse)
}
