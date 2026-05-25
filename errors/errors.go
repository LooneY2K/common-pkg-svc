package errors

import (
	"net/http"
)

type AppError struct {
	Message    string
	StatusCode int
}

func (err *AppError) Error() string {
	return err.Message
}

func BadRequest(message string) *AppError {
	return &AppError{message, http.StatusBadRequest}
}

func InternalServerError(message string) *AppError {
	return &AppError{message, http.StatusInternalServerError}
}

func NotFound(message string) *AppError {
	return &AppError{message, http.StatusNotFound}
}

func Unauthorized(message string) *AppError {
	return &AppError{message, http.StatusUnauthorized}
}

func Forbidden(message string) *AppError {
	return &AppError{message, http.StatusForbidden}
}

// Conflict returns an AppError with HTTP 409 — typically for state-machine
// transition violations or duplicate-resource detection where the caller
// must observe a different state before retrying.
func Conflict(message string) *AppError {
	return &AppError{message, http.StatusConflict}
}

// ServiceUnavailable returns an AppError with HTTP 503 — used for fail-closed
// behaviour when a hard-dependency upstream (e.g. OpenFGA, NATS, downstream
// service) cannot be reached. Callers SHOULD prefer this over 500 so clients
// retry with backoff instead of treating it as a permanent failure.
func ServiceUnavailable(message string) *AppError {
	return &AppError{message, http.StatusServiceUnavailable}
}
