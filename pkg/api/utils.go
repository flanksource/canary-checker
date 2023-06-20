package api

import (
	"fmt"

	"github.com/labstack/echo/v4"
)

func errorResonse(c echo.Context, err error, code int) error {
	e := map[string]string{"error": err.Error()}
	return c.JSON(code, e)
}

func errorMsgResponse(c echo.Context, code int, msgFormat string, args ...any) error {
	e := map[string]string{"error": fmt.Sprintf(msgFormat, args...)}
	return c.JSON(code, e)
}
