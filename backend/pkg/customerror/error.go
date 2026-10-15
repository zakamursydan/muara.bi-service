package customerror

import (
	"fmt"
	"net/http"
)

type ErrorCode string

const (
	ErrNotFound            ErrorCode = "NOT_FOUND"
	ErrBadRequest          ErrorCode = "BAD_REQUEST"
	ErrUnauthorized        ErrorCode = "UNAUTHORIZED"
	ErrForbidden           ErrorCode = "FORBIDDEN"
	ErrConflict            ErrorCode = "CONFLICT"
	ErrInternalServerError ErrorCode = "INTERNAL_SERVER_ERROR"
)

type AppError struct {
	Code       ErrorCode `json:"code"`
	Message    string    `json:"message"`
	HTTPStatus int       `json:"-"`
	Err        error     `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func New(code ErrorCode, message string, httpStatus int, err error) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
		Err:        err,
	}
}

func NewNotFoundError(message string, err error) *AppError {
	return New(ErrNotFound, message, http.StatusNotFound, err)
}

func NewBadRequestError(message string, err error) *AppError {
	return New(ErrBadRequest, message, http.StatusBadRequest, err)
}

func NewUnauthorizedError(message string) *AppError {
	return New(ErrUnauthorized, message, http.StatusUnauthorized, nil)
}

func NewForbiddenError(message string) *AppError {
	return New(ErrForbidden, message, http.StatusForbidden, nil)
}

func NewConflictError(message string, err error) *AppError {
	return New(ErrConflict, message, http.StatusConflict, err)
}

func NewInternalError(message string, err error) *AppError {
	return New(ErrInternalServerError, message, http.StatusInternalServerError, err)
}
