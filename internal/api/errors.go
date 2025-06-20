package api

import (
	"fmt"
	"net/http"
	"strings"
)

type ApiError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Err        error  `json:"-"`
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

func lower(s string) string {
	return strings.ToLower(s)
}

func NewBadRequestError() *ApiError {
	return &ApiError{
		StatusCode: http.StatusBadRequest,
		Message:    lower(http.StatusText(http.StatusBadRequest)),
	}
}

func NewNotFoundError() *ApiError {
	return &ApiError{
		StatusCode: http.StatusNotFound,
		Message:    lower(http.StatusText(http.StatusNotFound)),
	}
}

func NewInternalServerError(err error) *ApiError {
	return &ApiError{
		StatusCode: http.StatusInternalServerError,
		Message:    lower(http.StatusText(http.StatusInternalServerError)),
		Err:        err,
	}
}

func NewUnauthorizedError() *ApiError {
	return &ApiError{
		StatusCode: http.StatusUnauthorized,
		Message:    lower(http.StatusText(http.StatusUnauthorized)),
	}
}

func NewForbiddenError() *ApiError {
	return &ApiError{
		StatusCode: http.StatusForbidden,
		Message:    lower(http.StatusText(http.StatusForbidden)),
	}
}

func NewMethodNotAllowedError() *ApiError {
	return &ApiError{
		StatusCode: http.StatusMethodNotAllowed,
		Message:    lower(http.StatusText(http.StatusMethodNotAllowed)),
	}
}
