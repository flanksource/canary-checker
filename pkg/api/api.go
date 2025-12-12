package api

import (
	"net/http"
	"time"

	"github.com/flanksource/canary-checker/pkg/runner"
	"github.com/flanksource/duty/context"
	"github.com/flanksource/duty/query"
	"github.com/flanksource/duty/types"
	"github.com/labstack/echo/v4"
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

type DetailResponse struct {
	Duration   int                `json:"duration,omitempty"`
	RunnerName string             `json:"runnerName"`
	Status     []query.Timeseries `json:"status"`
	Latency    types.Latency      `json:"latency"`
	Uptime     types.Uptime       `json:"uptime"`
}

func About(c echo.Context) error {
	data := map[string]interface{}{
		"Timestamp": time.Now(),
		"Version":   runner.Version,
	}
	return c.JSON(http.StatusOK, data)
}

func CheckDetails(c echo.Context) error {
	ctx := c.Request().Context().(context.Context)

	var q query.CheckQueryParams
	if err := q.Init(c.QueryParams()); err != nil {
		return errorResponse(c, err, http.StatusBadRequest)
	}

	start := time.Now()
	results, uptime, latency, err := q.ExecuteDetails(ctx)
	if err != nil {
		return errorResponse(c, err, http.StatusInternalServerError)
	}

	apiResponse := &DetailResponse{
		RunnerName: runner.RunnerName,
		Status:     results,
		Duration:   int(time.Since(start).Milliseconds()),
		Latency:    latency,
		Uptime:     uptime,
	}

	return c.JSON(http.StatusOK, apiResponse)
}

func GetBestPartitioner(totalChecks int, rangeDuration time.Duration) time.Duration {
	if totalChecks <= desiredNumOfCheckStatuses {
		return 0 // No need to perform window aggregation
	}

	bestDelta := 100000000 // sufficiently large delta to begin with
	bestWindow := allowedWindows[0]

	for _, wp := range allowedWindows {
		numWindows := int(rangeDuration / wp)
		delta := abs(desiredNumOfCheckStatuses - numWindows)

		if delta < bestDelta {
			bestDelta = delta
			bestWindow = wp
		} else {
			// as soon as we notice that the delta gets worse, we break the loop
			break
		}
	}

	numWindows := int(rangeDuration / bestWindow)
	if abs(desiredNumOfCheckStatuses-totalChecks) <= abs(desiredNumOfCheckStatuses-numWindows) {
		// If this best partition creates windows such that the resulting number of data points deviate more
		// from the desired data points than the actual data points, then we do not aggregate.
		// Example: if there are 144 checks for the duration of 6 days,
		// then the best partition, 1 hour, would generate 144 data points.
		// But the original data points (120) are closer to 100, so we do not aggregate.
		return 0
	}

	return bestWindow
}
