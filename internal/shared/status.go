package shared

import "strings"

func ValidateStatus(status string, allowed []string) error {
	status = strings.TrimSpace(status)
	if status == "" {
		return NewError(CodeInvalidArgument, "status is required")
	}
	for _, candidate := range allowed {
		if status == candidate {
			return nil
		}
	}
	return NewError(CodeInvalidArgument, "status is not allowed")
}
