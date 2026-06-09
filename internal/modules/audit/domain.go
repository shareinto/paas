package audit

import (
	"strings"
	"time"

	"github.com/shareinto/paas/internal/shared"
)

type Result string

const (
	ResultSucceeded Result = "succeeded"
	ResultFailed    Result = "failed"
	ResultDenied    Result = "denied"
)

type AuditLog struct {
	ID           shared.ID         `json:"id"`
	ActorID      shared.ID         `json:"actor_id"`
	ActorType    string            `json:"actor_type"`
	SubjectType  string            `json:"subject_type"`
	TenantID     shared.ID         `json:"tenant_id"`
	ProjectID    shared.ID         `json:"project_id"`
	ResourceType string            `json:"resource_type"`
	ResourceID   shared.ID         `json:"resource_id"`
	Action       string            `json:"action"`
	Result       Result            `json:"result"`
	Summary      string            `json:"summary"`
	Details      map[string]string `json:"details,omitempty"`
	OccurredAt   time.Time         `json:"occurred_at"`
	CreatedAt    time.Time         `json:"created_at"`
}

type Query struct {
	TenantID     shared.ID
	ProjectID    shared.ID
	ActorID      shared.ID
	ResourceType string
	ResourceID   shared.ID
	Action       string
	From         *time.Time
	To           *time.Time
}

func normalizeLog(log AuditLog) (AuditLog, error) {
	log.ActorType = trimOrDefault(log.ActorType, "user")
	log.SubjectType = trimOrDefault(log.SubjectType, log.ActorType)
	log.Action = strings.TrimSpace(log.Action)
	log.ResourceType = strings.TrimSpace(log.ResourceType)
	log.Summary = strings.TrimSpace(log.Summary)
	if log.Action == "" {
		return AuditLog{}, shared.NewError(shared.CodeInvalidArgument, "audit action is required")
	}
	if log.ResourceType == "" {
		return AuditLog{}, shared.NewError(shared.CodeInvalidArgument, "audit resource_type is required")
	}
	if log.Result == "" {
		log.Result = ResultSucceeded
	}
	if log.Result != ResultSucceeded && log.Result != ResultFailed && log.Result != ResultDenied {
		return AuditLog{}, shared.NewError(shared.CodeInvalidArgument, "unsupported audit result")
	}
	log.Details = sanitizeDetails(log.Details)
	return log, nil
}

func trimOrDefault(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func sanitizeDetails(details map[string]string) map[string]string {
	if len(details) == 0 {
		return nil
	}
	out := make(map[string]string, len(details))
	for key, value := range details {
		normalized := strings.ToLower(strings.TrimSpace(key))
		switch {
		case strings.Contains(normalized, "secret"),
			strings.Contains(normalized, "token"),
			strings.Contains(normalized, "password"),
			strings.Contains(normalized, "credential"),
			strings.Contains(normalized, "kubeconfig"):
			out[key] = "[REDACTED]"
		default:
			out[key] = value
		}
	}
	return out
}
