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

// The number of data points that should be strived for
// when aggregating check statuses.
const desiredNumOfCheckStatuses = 100

var (
	DefaultWindow = "1h"

	// allowed list of window durations that are used when aggregating check statuses.
	allowedWindows = []time.Duration{
		time.Minute,        // 1m
		time.Minute * 5,    // 5m
		time.Minute * 15,   // 15m
		time.Minute * 30,   // 30m
		time.Hour,          // 1h
		time.Hour * 3,      // 3h
		time.Hour * 6,      // 6h
		time.Hour * 12,     // 12h
		time.Hour * 24,     // 24h
		time.Hour * 24 * 7, // 1w
	}
)

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
	if totalChecks <= desiredNumOfCheckStatuses {
		q.WindowDuration = 0 // No need to perform window aggregation
	} else {
		rangeDuration := checkSummary.LatestRuntime.Sub(*checkSummary.EarliestRuntime)
		q.WindowDuration = getMostSuitableWindowDuration(rangeDuration)
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

// getMostSuitableWindowDuration returns best window duration to partition the
// given duration such that the partition is close to the given value
func getMostSuitableWindowDuration(rangeDuration time.Duration) time.Duration {
	bestDelta := 1000000 // sufficiently large delta to begin with

	for i, wp := range allowedWindows {
		numWindows := int(rangeDuration / wp)
		delta := abs(desiredNumOfCheckStatuses - numWindows)

		if delta < bestDelta {
			bestDelta = delta
			continue
		}

		// as soon as we notice that the delta gets worse, we return the previous window
		if i == 0 {
			return wp
		}

		return allowedWindows[i-1]
	}

	return allowedWindows[len(allowedWindows)-1]
}
