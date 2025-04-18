package main

import (
	"fmt"
	"net/http"
)

type ApiError struct {
	Code    int    `json:"-"`
	Message string `json:"message"`
	Err     error  `json:"-"`
}

func (e *ApiError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s", e.Message, e.Err.Error())
	}

	return e.Message
}

func (e *ApiError) Unwrap() error {
	return e.Err
}

func NewBadRequestError(message string) *ApiError {
	return &ApiError{
		Code:    http.StatusBadRequest,
		Message: message,
	}
}

func NewNotFoundError() *ApiError {
	return &ApiError{
		Code:    http.StatusNotFound,
		Message: "not found",
	}
}

func NewInternalServerError(err error) *ApiError {
	return &ApiError{
		Code:    http.StatusInternalServerError,
		Message: http.StatusText(http.StatusInternalServerError),
		Err:     err,
	}
}

func NewUnauthorizedError(err error) *ApiError {
	return &ApiError{
		Code:    http.StatusUnauthorized,
		Message: "unauthorized",
	}
}
