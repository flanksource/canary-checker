package api

import (
	"net/http"

	"github.com/flanksource/canary-checker/pkg/cache"
	"github.com/flanksource/commons/logger"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
)

func DetailsHandler(c echo.Context) error {
	queryParams := c.Request().URL.Query()
	key := queryParams.Get("key")
	time := queryParams.Get("time")
	if key == "" || time == "" {
		logger.Errorf("key and time are required parameters")
		return errorResponse(c, errors.New("key and time are required parameters"), http.StatusBadRequest)
	}
	detail := cache.PostgresCache.GetDetails(key, time)
	return c.JSON(http.StatusOK, detail)
}
