package notification

import (
	"context"

	"github.com/shareinto/paas/internal/shared"
)

type Repository interface {
	CreateTemplate(ctx context.Context, template NotificationTemplate) error
	FindTemplateByEventType(ctx context.Context, eventType string) (NotificationTemplate, error)

	CreateChannel(ctx context.Context, channel NotificationChannel) error
	GetDefaultChannel(ctx context.Context) (NotificationChannel, error)

	CreateNotification(ctx context.Context, notification Notification) error
	UpdateNotification(ctx context.Context, notification Notification) error
	FindNotificationByDedupeKey(ctx context.Context, dedupeKey string) (Notification, error)
	GetNotification(ctx context.Context, id shared.ID) (Notification, error)
	ListNotifications(ctx context.Context, page shared.PageRequest) (shared.PageResult[Notification], error)
}

type Sender interface {
	Send(ctx context.Context, channel NotificationChannel, message Message) error
}

type Message struct {
	Title   string
	Content string
}
