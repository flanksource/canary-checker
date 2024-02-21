package api

import (
	"errors"
	"fmt"
)

// Application error codes.
//
// These are meant to be generic and they map well to HTTP error codes.
const (
	ECONFLICT       = "conflict"
	EFORBIDDEN      = "forbidden"
	EINTERNAL       = "internal"
	EINVALID        = "invalid"
	ENOTFOUND       = "not_found"
	ENOTIMPLEMENTED = "not_implemented"
	EUNAUTHORIZED   = "unauthorized"
)

// Error represents an application-specific error.
type Error struct {
	// Machine-readable error code.
	Code string

	// Human-readable error message.
	Message string

	// DebugInfo contains low-level internal error details that should only be logged.
	// End-users should never see this.
	DebugInfo string
}

// Error implements the error interface. Not used by the application otherwise.
func (e *Error) Error() string {
	return fmt.Sprintf("error: code=%s message=%s", e.Code, e.Message)
}

// WithDebugInfo wraps an application error with a debug message.
func (e *Error) WithDebugInfo(msg string, args ...any) *Error {
	e.DebugInfo = fmt.Sprintf(msg, args...)
	return e
}

// ErrorCode unwraps an application error and returns its code.
// Non-application errors always return EINTERNAL.
func ErrorCode(err error) string {
	var e *Error
	if err == nil {
		return ""
	} else if errors.As(err, &e) {
		return e.Code
	}
	return EINTERNAL
}

// ErrorMessage unwraps an application error and returns its message.
// Non-application errors always return "Internal error".
func ErrorMessage(err error) string {
	var e *Error
	if err == nil {
		return ""
	} else if errors.As(err, &e) {
		return e.Message
	}
	return "Internal error."
}

// ErrorDebugInfo unwraps an application error and returns its debug message.
func ErrorDebugInfo(err error) string {
	var e *Error
	if err == nil {
		return ""
	} else if errors.As(err, &e) {
		return e.DebugInfo
	}
	return ""
}

// Errorf is a helper function to return an Error with a given code and formatted message.
func Errorf(code string, format string, args ...any) *Error {
	return &Error{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}
