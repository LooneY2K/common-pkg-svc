package respond

import (
	"encoding/json"
	"net/http"

	"github.com/LooneY2K/common-pkg-svc/errors"
)

type Response struct {
	Status  int         `json:"status"`
	Data    interface{} `json:"data,omitempty"`
	Error   bool        `json:"error"`
	Message string      `json:"message,omitempty"`
}

func toJSON(rw http.ResponseWriter, status int, data interface{}, message string, isError bool) *errors.AppError {
	rw.WriteHeader(status)
	rw.Header().Set("Content-Type", "application/json")
	response := Response{
		Status:  status,
		Data:    data,
		Error:   isError,
		Message: message,
	}
	if err := json.NewEncoder(rw).Encode(response); err != nil {
		return errors.New(errors.CodeInternal, err.Error(), "")
	}
	return nil
}

func OK(rw http.ResponseWriter, data interface{}) *errors.AppError {
	return toJSON(rw, http.StatusOK, data, "", false)
}

func Created(rw http.ResponseWriter, data interface{}) *errors.AppError {
	return toJSON(rw, http.StatusCreated, data, "", false)
}

func Fail(rw http.ResponseWriter, data interface{}) *errors.AppError {
	return toJSON(rw, http.StatusInternalServerError, data, "", false)
}

func Error(rw http.ResponseWriter, appErr *errors.AppError) *errors.AppError {
	return toJSON(rw, appErr.HTTPStatus(), nil, appErr.Error(), true)
}
