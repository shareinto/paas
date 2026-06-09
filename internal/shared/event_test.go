package shared_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/shareinto/paas/internal/shared"
)

func TestNewDomainEvent(t *testing.T) {
	occurredAt := time.Date(2026, 5, 30, 1, 2, 3, 0, time.FixedZone("CST", 8*60*60))

	event, err := shared.NewDomainEvent(shared.ID("evt_1"), "ApplicationCreated", occurredAt, map[string]string{"app_id": "app_1"})
	if err != nil {
		t.Fatalf("NewDomainEvent() error = %v", err)
	}

	if event.EventID != "evt_1" || event.EventType != "ApplicationCreated" {
		t.Fatalf("unexpected event identity: %+v", event)
	}
	if event.OccurredAt.Location() != time.UTC {
		t.Fatalf("OccurredAt should be normalized to UTC")
	}

	var payload map[string]string
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		t.Fatalf("payload should be json: %v", err)
	}
	if payload["app_id"] != "app_1" {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestNewDomainEventValidation(t *testing.T) {
	if _, err := shared.NewDomainEvent("", "SomethingHappened", time.Now(), nil); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("empty event id should be invalid_argument, got %v", err)
	}
	if _, err := shared.NewDomainEvent("evt_1", "", time.Now(), nil); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("empty event type should be invalid_argument, got %v", err)
	}
	if _, err := shared.NewDomainEvent("evt_1", "BadPayload", time.Now(), make(chan int)); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("unmarshalable payload should be invalid_argument, got %v", err)
	}
}
