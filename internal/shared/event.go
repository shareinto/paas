package shared

import (
	"encoding/json"
	"strings"
	"time"
)

type DomainEvent struct {
	EventID    ID              `json:"event_id"`
	EventType  string          `json:"event_type"`
	OccurredAt time.Time       `json:"occurred_at"`
	Payload    json.RawMessage `json:"payload"`
}

func NewDomainEvent(id ID, eventType string, occurredAt time.Time, payload any) (DomainEvent, error) {
	if id.IsZero() {
		return DomainEvent{}, NewError(CodeInvalidArgument, "event_id is required")
	}
	if strings.TrimSpace(eventType) == "" {
		return DomainEvent{}, NewError(CodeInvalidArgument, "event_type is required")
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return DomainEvent{}, WrapError(CodeInvalidArgument, "payload must be json serializable", err)
	}

	return DomainEvent{
		EventID:    id,
		EventType:  eventType,
		OccurredAt: occurredAt.UTC(),
		Payload:    raw,
	}, nil
}
