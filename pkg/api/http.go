package api

import (
	"net/http"

	"github.com/flanksource/commons/logger"
	"github.com/labstack/echo/v4"
)

type HTTPError struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

type HTTPSuccess struct {
	Message string `json:"message"`
	Payload any    `json:"payload,omitempty"`
}

func WriteError(c echo.Context, err error) error {
	code, message := ErrorCode(err), ErrorMessage(err)

	if debugInfo := ErrorDebugInfo(err); debugInfo != "" {
		logger.WithValues("code", code, "error", message).Errorf(debugInfo)
	}

	return c.JSON(ErrorStatusCode(code), &HTTPError{Error: message})
}

// ErrorStatusCode returns the associated HTTP status code for an application error code.
func ErrorStatusCode(code string) int {
	// lookup of application error codes to HTTP status codes.
	var codes = map[string]int{
		ECONFLICT:       http.StatusConflict,
		EINVALID:        http.StatusBadRequest,
		ENOTFOUND:       http.StatusNotFound,
		EFORBIDDEN:      http.StatusForbidden,
		ENOTIMPLEMENTED: http.StatusNotImplemented,
		EUNAUTHORIZED:   http.StatusUnauthorized,
		EINTERNAL:       http.StatusInternalServerError,
	}

	if v, ok := codes[code]; ok {
		return v
	}

	return http.StatusInternalServerError
}
