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
