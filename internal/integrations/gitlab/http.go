package gitlab

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/shareinto/paas/internal/shared"
)

func encodeBody(body any) (io.Reader, string, error) {
	if body == nil {
		return nil, "", nil
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, "", err
	}
	return bytes.NewReader(data), "application/json", nil
}

func decodeResponse(resp *http.Response, target any) error {
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return mapHTTPError(resp.StatusCode, gitLabErrorMessage(resp.StatusCode, body))
	}
	if target == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func retryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || status == http.StatusBadGateway || status == http.StatusServiceUnavailable || status == http.StatusGatewayTimeout
}

func mapHTTPError(status int, message string) error {
	return shared.NewError(mapStatus(status), message)
}

func mapStatus(status int) shared.ErrorCode {
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return shared.CodeFailedPrecondition
	case http.StatusUnauthorized:
		return shared.CodeUnavailable
	case http.StatusForbidden:
		return shared.CodePermissionDenied
	case http.StatusNotFound:
		return shared.CodeNotFound
	case http.StatusConflict:
		return shared.CodeConflict
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return shared.CodeUnavailable
	default:
		return shared.CodeInternal
	}
}

func gitLabErrorMessage(status int, body []byte) string {
	text := strings.TrimSpace(string(body))
	if text == "" {
		return fmt.Sprintf("gitlab request failed: status=%d", status)
	}
	return fmt.Sprintf("gitlab request failed: status=%d body=%s", status, text)
}

func isAlreadyExistsError(err error) bool {
	var appErr *shared.AppError
	if !errors.As(err, &appErr) {
		return false
	}
	if appErr.Code != shared.CodeConflict && appErr.Code != shared.CodeFailedPrecondition {
		return false
	}
	message := strings.ToLower(appErr.Message)
	return strings.Contains(message, "has already been taken") || strings.Contains(message, "already exists")
}
