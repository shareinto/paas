package shared

import (
	"errors"
	"fmt"
	"net/http"
)

type ErrorCode string

const (
	CodeInvalidArgument    ErrorCode = "invalid_argument"
	CodeUnauthenticated    ErrorCode = "unauthenticated"
	CodePermissionDenied   ErrorCode = "permission_denied"
	CodeNotFound           ErrorCode = "not_found"
	CodeConflict           ErrorCode = "conflict"
	CodeFailedPrecondition ErrorCode = "failed_precondition"
	CodeInternal           ErrorCode = "internal"
	CodeUnavailable        ErrorCode = "unavailable"
)

type AppError struct {
	Code    ErrorCode      `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
	cause   error
}

func NewError(code ErrorCode, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

func WrapError(code ErrorCode, message string, cause error) *AppError {
	return &AppError{Code: code, Message: message, cause: cause}
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.cause == nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
}

func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func CodeOf(err error) ErrorCode {
	if err == nil {
		return ""
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return CodeInternal
}

func HTTPStatusOf(err error) int {
	switch CodeOf(err) {
	case "":
		return http.StatusOK
	case CodeInvalidArgument:
		return http.StatusBadRequest
	case CodeUnauthenticated:
		return http.StatusUnauthorized
	case CodePermissionDenied:
		return http.StatusForbidden
	case CodeNotFound:
		return http.StatusNotFound
	case CodeConflict:
		return http.StatusConflict
	case CodeFailedPrecondition:
		return http.StatusPreconditionFailed
	case CodeUnavailable:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
