package details

import (
	"encoding/json"
	"net/http"

	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/commons/logger"
	"github.com/labstack/echo/v4"
)

func Handler(c echo.Context) error {
	queryParams := c.Request().URL.Query()
	key := queryParams.Get("key")
	time := queryParams.Get("time")
	if key == "" || time == "" {
		logger.Errorf("key and time are required parameters")
		return c.String(http.StatusBadRequest, "key and time are required parameters")
	}
	detail := cache.PostgresCache.GetDetails(key, time)
	jsonData, err := json.Marshal(detail)
	if err != nil {
		logger.Errorf("Failed to marshal data: %v", err)
		return c.String(http.StatusInternalServerError, "{\"error\": \"internal\"}")
	}
	if _, err = c.Response().Write(jsonData); err != nil {
		logger.Errorf("failed to write data in response: %v", err)
		return c.String(http.StatusInternalServerError, "{\"error\": \"internal\"}")
	}
	return nil
}
