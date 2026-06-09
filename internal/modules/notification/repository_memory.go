package notification

import (
	"context"
	"sort"
	"sync"

	"github.com/shareinto/paas/internal/shared"
)

type MemoryRepository struct {
	mu              sync.RWMutex
	templates       map[string]NotificationTemplate
	channels        map[shared.ID]NotificationChannel
	defaultChannel  shared.ID
	notifications   map[shared.ID]Notification
	notificationIDs []shared.ID
	byDedupe        map[string]shared.ID
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		templates: map[string]NotificationTemplate{}, channels: map[shared.ID]NotificationChannel{},
		notifications: map[shared.ID]Notification{}, byDedupe: map[string]shared.ID{},
	}
}

func (r *MemoryRepository) CreateTemplate(_ context.Context, template NotificationTemplate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.templates[template.EventType]; ok {
		return shared.NewError(shared.CodeConflict, "notification template already exists")
	}
	r.templates[template.EventType] = template
	return nil
}

func (r *MemoryRepository) FindTemplateByEventType(_ context.Context, eventType string) (NotificationTemplate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	template, ok := r.templates[eventType]
	if !ok {
		return NotificationTemplate{}, shared.NewError(shared.CodeNotFound, "notification template not found")
	}
	return template, nil
}

func (r *MemoryRepository) CreateChannel(_ context.Context, channel NotificationChannel) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.channels[channel.ID]; ok {
		return shared.NewError(shared.CodeConflict, "notification channel already exists")
	}
	r.channels[channel.ID] = channel
	if r.defaultChannel.IsZero() {
		r.defaultChannel = channel.ID
	}
	return nil
}

func (r *MemoryRepository) GetDefaultChannel(_ context.Context) (NotificationChannel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.defaultChannel.IsZero() {
		return NotificationChannel{}, shared.NewError(shared.CodeNotFound, "notification channel not found")
	}
	return r.channels[r.defaultChannel], nil
}

func (r *MemoryRepository) CreateNotification(_ context.Context, notification Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.notifications[notification.ID]; ok {
		return shared.NewError(shared.CodeConflict, "notification already exists")
	}
	if _, ok := r.byDedupe[notification.DedupeKey]; ok {
		return shared.NewError(shared.CodeConflict, "notification already exists for event")
	}
	r.notifications[notification.ID] = notification
	r.notificationIDs = append(r.notificationIDs, notification.ID)
	r.byDedupe[notification.DedupeKey] = notification.ID
	return nil
}

func (r *MemoryRepository) UpdateNotification(_ context.Context, notification Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.notifications[notification.ID]; !ok {
		return shared.NewError(shared.CodeNotFound, "notification not found")
	}
	r.notifications[notification.ID] = notification
	return nil
}

func (r *MemoryRepository) FindNotificationByDedupeKey(_ context.Context, dedupe string) (Notification, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byDedupe[dedupe]
	if !ok {
		return Notification{}, shared.NewError(shared.CodeNotFound, "notification not found")
	}
	return r.notifications[id], nil
}

func (r *MemoryRepository) GetNotification(_ context.Context, id shared.ID) (Notification, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	notification, ok := r.notifications[id]
	if !ok {
		return Notification{}, shared.NewError(shared.CodeNotFound, "notification not found")
	}
	return notification, nil
}

func (r *MemoryRepository) ListNotifications(_ context.Context, page shared.PageRequest) (shared.PageResult[Notification], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	page = page.Normalize()
	items := make([]Notification, 0, len(r.notificationIDs))
	for _, id := range r.notificationIDs {
		items = append(items, r.notifications[id])
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	start := page.Offset()
	if start > len(items) {
		start = len(items)
	}
	end := start + page.PageSize
	if end > len(items) {
		end = len(items)
	}
	return shared.NewPageResult(items[start:end], int64(len(items)), page), nil
}
