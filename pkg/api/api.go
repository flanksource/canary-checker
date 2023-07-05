package api

import (
	"net/http"
	"time"

	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/canary-checker/pkg/runner"
	"github.com/flanksource/duty/models"
	"github.com/labstack/echo/v4"

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
	if len(summary) == 0 {
		return c.JSON(http.StatusOK, DetailResponse{})
	}

	checkSummary := *summary[0]
	totalChecks := checkSummary.Uptime.Total()
	if totalChecks <= maxCheckStatuses {
		q.WindowDuration = 0 // No need to perform window aggregation
	} else {
		startTime := q.GetStartTime()
		endTime := q.GetEndTime()

		// NOTE: This doesn't work well when the range has huge gaps in datapoints.
		// Example: if the time range is 5 years and we only have data since the last week,
		// the generated window duration would be 5years/100 ~= 19 days. This would mean
		// all the data points since the last week would fall into the same window.
		//
		// Instead, we could take the duration between the earliest and the latest check statuses in the range
		// so that the window duration is small.
		rangeDuration := endTime.Sub(*startTime)
		q.WindowDuration = rangeDuration / time.Duration(maxCheckStatuses)
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
