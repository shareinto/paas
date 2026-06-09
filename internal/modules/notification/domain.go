package notification

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/shareinto/paas/internal/shared"
)

type NotificationStatus string

const (
	NotificationPending   NotificationStatus = "pending"
	NotificationSending   NotificationStatus = "sending"
	NotificationSucceeded NotificationStatus = "succeeded"
	NotificationFailed    NotificationStatus = "failed"
)

type ChannelType string

const (
	ChannelWebhook ChannelType = "webhook"
	ChannelEmail   ChannelType = "email"
	ChannelFake    ChannelType = "fake"
)

type Notification struct {
	ID           shared.ID          `json:"id"`
	TenantID     shared.ID          `json:"tenant_id"`
	ProjectID    shared.ID          `json:"project_id"`
	EventType    string             `json:"event_type"`
	DedupeKey    string             `json:"dedupe_key"`
	ChannelID    shared.ID          `json:"channel_id"`
	Title        string             `json:"title"`
	Content      string             `json:"content"`
	Status       NotificationStatus `json:"status"`
	Attempts     int                `json:"attempts"`
	ErrorMessage string             `json:"error_message"`
	CreatedAt    time.Time          `json:"created_at"`
	UpdatedAt    time.Time          `json:"updated_at"`
	SentAt       *time.Time         `json:"sent_at,omitempty"`
}

type NotificationTemplate struct {
	ID              shared.ID `json:"id"`
	EventType       string    `json:"event_type"`
	TitleTemplate   string    `json:"title_template"`
	ContentTemplate string    `json:"content_template"`
	Enabled         bool      `json:"enabled"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type NotificationChannel struct {
	ID        shared.ID   `json:"id"`
	Name      string      `json:"name"`
	Type      ChannelType `json:"type"`
	Target    string      `json:"target"`
	Enabled   bool        `json:"enabled"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
}

type Event struct {
	Type      string         `json:"type"`
	TenantID  shared.ID      `json:"tenant_id"`
	ProjectID shared.ID      `json:"project_id"`
	Payload   map[string]any `json:"payload"`
}

var supportedEvents = map[string]struct{}{
	"BuildFailed":        {},
	"PromotionCreated":   {},
	"PromotionApproved":  {},
	"PromotionRejected":  {},
	"DeploymentFailed":   {},
	"ClusterUnreachable": {},
}

func supportedEvent(eventType string) bool {
	_, ok := supportedEvents[eventType]
	return ok
}

func dedupeKey(event Event) string {
	payload, _ := json.Marshal(event.Payload)
	sum := sha256.Sum256([]byte(event.Type + "|" + string(event.TenantID) + "|" + string(event.ProjectID) + "|" + string(payload)))
	return hex.EncodeToString(sum[:])
}

func normalizeChannel(channel NotificationChannel) (NotificationChannel, error) {
	channel.Name = strings.TrimSpace(channel.Name)
	channel.Target = strings.TrimSpace(channel.Target)
	if channel.Name == "" {
		return NotificationChannel{}, shared.NewError(shared.CodeInvalidArgument, "notification channel name is required")
	}
	if channel.Type == "" {
		channel.Type = ChannelFake
	}
	if channel.Type != ChannelFake && channel.Target == "" {
		return NotificationChannel{}, shared.NewError(shared.CodeInvalidArgument, "notification channel target is required")
	}
	return channel, nil
}
