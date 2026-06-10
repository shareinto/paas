package notification

import (
	"context"
	"database/sql"
	"time"

	"github.com/shareinto/paas/internal/platform/database"
	"github.com/shareinto/paas/internal/shared"
)

type MySQLRepository struct {
	db *sql.DB
}

func NewMySQLRepository(_ context.Context, db *sql.DB) (*MySQLRepository, error) {
	return &MySQLRepository{db: db}, nil
}

func (r *MySQLRepository) CreateTemplate(ctx context.Context, template NotificationTemplate) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO notification_templates (id, event_type, title_template, content_template, enabled, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
		template.ID, template.EventType, template.TitleTemplate, template.ContentTemplate, template.Enabled, mysqlTime(template.CreatedAt), mysqlTime(template.UpdatedAt))
	return database.ConflictOrUnavailable(err, "notification template already exists", "create notification template failed")
}

func (r *MySQLRepository) FindTemplateByEventType(ctx context.Context, eventType string) (NotificationTemplate, error) {
	template, err := scanTemplate(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, event_type, title_template, content_template, enabled, created_at, updated_at
FROM notification_templates WHERE event_type = ?`, eventType))
	if err != nil {
		return NotificationTemplate{}, database.NotFound(err, "notification template not found")
	}
	return template, nil
}

func (r *MySQLRepository) CreateChannel(ctx context.Context, channel NotificationChannel) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO notification_channels (id, name, type, target, enabled, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
		channel.ID, channel.Name, channel.Type, channel.Target, channel.Enabled, mysqlTime(channel.CreatedAt), mysqlTime(channel.UpdatedAt))
	return database.ConflictOrUnavailable(err, "notification channel already exists", "create notification channel failed")
}

func (r *MySQLRepository) GetDefaultChannel(ctx context.Context) (NotificationChannel, error) {
	channel, err := scanChannel(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, name, type, target, enabled, created_at, updated_at
FROM notification_channels WHERE enabled = 1 ORDER BY created_at ASC, id ASC LIMIT 1`))
	if err != nil {
		return NotificationChannel{}, database.NotFound(err, "notification channel not found")
	}
	return channel, nil
}

func (r *MySQLRepository) CreateNotification(ctx context.Context, notification Notification) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO notifications (
  id, tenant_id, project_id, event_type, dedupe_key, channel_id, title, content,
  status, attempts, error_message, created_at, updated_at, sent_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		notification.ID, notification.TenantID, notification.ProjectID, notification.EventType,
		notification.DedupeKey, notification.ChannelID, notification.Title, notification.Content,
		notification.Status, notification.Attempts, notification.ErrorMessage,
		mysqlTime(notification.CreatedAt), mysqlTime(notification.UpdatedAt), mysqlTimePtr(notification.SentAt))
	return database.ConflictOrUnavailable(err, "notification already exists for event", "create notification failed")
}

func (r *MySQLRepository) UpdateNotification(ctx context.Context, notification Notification) error {
	result, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
UPDATE notifications
SET status = ?, attempts = ?, error_message = ?, updated_at = ?, sent_at = ?
WHERE id = ?`,
		notification.Status, notification.Attempts, notification.ErrorMessage, mysqlTime(notification.UpdatedAt), mysqlTimePtr(notification.SentAt), notification.ID)
	if err != nil {
		return database.WrapUnavailable(err, "update notification failed")
	}
	return database.RequireAffected(result, "notification not found")
}

func (r *MySQLRepository) FindNotificationByDedupeKey(ctx context.Context, dedupeKey string) (Notification, error) {
	notification, err := scanNotification(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, tenant_id, project_id, event_type, dedupe_key, channel_id, title, content,
       status, attempts, error_message, created_at, updated_at, sent_at
FROM notifications WHERE dedupe_key = ?`, dedupeKey))
	if err != nil {
		return Notification{}, database.NotFound(err, "notification not found")
	}
	return notification, nil
}

func (r *MySQLRepository) GetNotification(ctx context.Context, id shared.ID) (Notification, error) {
	notification, err := scanNotification(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, tenant_id, project_id, event_type, dedupe_key, channel_id, title, content,
       status, attempts, error_message, created_at, updated_at, sent_at
FROM notifications WHERE id = ?`, id))
	if err != nil {
		return Notification{}, database.NotFound(err, "notification not found")
	}
	return notification, nil
}

func (r *MySQLRepository) ListNotifications(ctx context.Context, page shared.PageRequest) (shared.PageResult[Notification], error) {
	var total int64
	if err := database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, "SELECT COUNT(*) FROM notifications").Scan(&total); err != nil {
		return shared.PageResult[Notification]{}, database.WrapUnavailable(err, "count notifications failed")
	}
	page, limit, offset := database.LimitOffset(page)
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, `
SELECT id, tenant_id, project_id, event_type, dedupe_key, channel_id, title, content,
       status, attempts, error_message, created_at, updated_at, sent_at
FROM notifications ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return shared.PageResult[Notification]{}, database.WrapUnavailable(err, "list notifications failed")
	}
	defer rows.Close()
	items := []Notification{}
	for rows.Next() {
		notification, err := scanNotification(rows)
		if err != nil {
			return shared.PageResult[Notification]{}, err
		}
		items = append(items, notification)
	}
	if err := rows.Err(); err != nil {
		return shared.PageResult[Notification]{}, database.WrapUnavailable(err, "list notifications failed")
	}
	return shared.NewPageResult(items, total, page), nil
}

type notificationScanner interface {
	Scan(dest ...any) error
}

func scanTemplate(scanner notificationScanner) (NotificationTemplate, error) {
	var template NotificationTemplate
	err := scanner.Scan(&template.ID, &template.EventType, &template.TitleTemplate, &template.ContentTemplate, &template.Enabled, &template.CreatedAt, &template.UpdatedAt)
	return template, err
}

func scanChannel(scanner notificationScanner) (NotificationChannel, error) {
	var channel NotificationChannel
	err := scanner.Scan(&channel.ID, &channel.Name, &channel.Type, &channel.Target, &channel.Enabled, &channel.CreatedAt, &channel.UpdatedAt)
	return channel, err
}

func scanNotification(scanner notificationScanner) (Notification, error) {
	var notification Notification
	err := scanner.Scan(&notification.ID, &notification.TenantID, &notification.ProjectID, &notification.EventType, &notification.DedupeKey, &notification.ChannelID, &notification.Title, &notification.Content, &notification.Status, &notification.Attempts, &notification.ErrorMessage, &notification.CreatedAt, &notification.UpdatedAt, &notification.SentAt)
	return notification, err
}

func mysqlTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value
}

func mysqlTimePtr(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return *value
}
