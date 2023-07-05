package api

import (
	"github.com/labstack/echo/v4"
)

func errorResonse(c echo.Context, err error, code int) error {
	e := map[string]string{"error": err.Error()}
	return c.JSON(code, e)
}

// returns the absolute value of i.
// math.Abs only supports float64 and this avoid hairy type conversions.
func abs(i int) int {
	if i > 0 {
		return i
	}

	return -i
}
