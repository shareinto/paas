package shared_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/shareinto/paas/internal/shared"
)

func TestErrorCodeAndHTTPStatus(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		code   shared.ErrorCode
		status int
	}{
		{name: "nil", err: nil, code: "", status: http.StatusOK},
		{name: "invalid", err: shared.NewError(shared.CodeInvalidArgument, "bad input"), code: shared.CodeInvalidArgument, status: http.StatusBadRequest},
		{name: "unauthenticated", err: shared.NewError(shared.CodeUnauthenticated, "login required"), code: shared.CodeUnauthenticated, status: http.StatusUnauthorized},
		{name: "permission denied", err: shared.NewError(shared.CodePermissionDenied, "denied"), code: shared.CodePermissionDenied, status: http.StatusForbidden},
		{name: "not found", err: shared.NewError(shared.CodeNotFound, "missing"), code: shared.CodeNotFound, status: http.StatusNotFound},
		{name: "conflict", err: shared.NewError(shared.CodeConflict, "exists"), code: shared.CodeConflict, status: http.StatusConflict},
		{name: "failed precondition", err: shared.NewError(shared.CodeFailedPrecondition, "not ready"), code: shared.CodeFailedPrecondition, status: http.StatusPreconditionFailed},
		{name: "unavailable", err: shared.NewError(shared.CodeUnavailable, "down"), code: shared.CodeUnavailable, status: http.StatusServiceUnavailable},
		{name: "plain error", err: errors.New("plain"), code: shared.CodeInternal, status: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shared.CodeOf(tt.err); got != tt.code {
				t.Fatalf("CodeOf() = %q, want %q", got, tt.code)
			}
			if got := shared.HTTPStatusOf(tt.err); got != tt.status {
				t.Fatalf("HTTPStatusOf() = %d, want %d", got, tt.status)
			}
		})
	}
}

func TestWrapErrorUnwrapsCause(t *testing.T) {
	cause := errors.New("database failed")
	err := shared.WrapError(shared.CodeUnavailable, "store unavailable", cause)

	if !errors.Is(err, cause) {
		t.Fatalf("wrapped error should unwrap cause")
	}
	if got := shared.CodeOf(err); got != shared.CodeUnavailable {
		t.Fatalf("CodeOf() = %q, want %q", got, shared.CodeUnavailable)
	}
}

func TestAppErrorStringForms(t *testing.T) {
	err := shared.NewError(shared.CodeNotFound, "missing")
	if err.Error() != "not_found: missing" {
		t.Fatalf("Error() = %q", err.Error())
	}

	wrapped := shared.WrapError(shared.CodeUnavailable, "db failed", errors.New("timeout"))
	if wrapped.Error() != "unavailable: db failed: timeout" {
		t.Fatalf("wrapped Error() = %q", wrapped.Error())
	}

	var nilErr *shared.AppError
	if nilErr.Error() != "" || nilErr.Unwrap() != nil {
		t.Fatalf("nil AppError should produce empty string and nil unwrap")
	}
}
