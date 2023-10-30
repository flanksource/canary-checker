package api

import (
	"github.com/labstack/echo/v4"
)

// Deprecated: use HTTPError
func errorResonse(c echo.Context, err error, code int) error {
	e := map[string]string{"error": err.Error()}
	return c.JSON(code, e)
}

// abs returns the absolute value of i.
// math.Abs only supports float64 and this avoids the needless type conversions
// and ugly expression.
func abs(n int) int {
	if n > 0 {
		return n
	}

	return -n
}
