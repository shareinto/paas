package notification

import (
	"context"
	"database/sql"

	"github.com/shareinto/paas/internal/platform/database"
	"github.com/shareinto/paas/internal/shared"
)

type MySQLRepository struct {
	*MemoryRepository
	store *database.SnapshotStore
}

type notificationSnapshot struct {
	Templates      []NotificationTemplate
	Channels       []NotificationChannel
	DefaultChannel shared.ID
	Notifications  []Notification
}

func NewMySQLRepository(ctx context.Context, db *sql.DB) (*MySQLRepository, error) {
	repo := &MySQLRepository{MemoryRepository: NewMemoryRepository(), store: database.NewSnapshotStore(db, "notification")}
	var snapshot notificationSnapshot
	if err := repo.store.Load(ctx, &snapshot); err != nil {
		return nil, err
	}
	for _, v := range snapshot.Templates {
		repo.templates[v.EventType] = v
	}
	for _, v := range snapshot.Channels {
		repo.channels[v.ID] = v
	}
	repo.defaultChannel = snapshot.DefaultChannel
	for _, v := range snapshot.Notifications {
		repo.notifications[v.ID] = v
		repo.notificationIDs = append(repo.notificationIDs, v.ID)
		repo.byDedupe[v.DedupeKey] = v.ID
	}
	return repo, nil
}

func (r *MySQLRepository) snapshot() notificationSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := notificationSnapshot{
		Templates: make([]NotificationTemplate, 0, len(r.templates)), Channels: make([]NotificationChannel, 0, len(r.channels)),
		DefaultChannel: r.defaultChannel, Notifications: make([]Notification, 0, len(r.notificationIDs)),
	}
	for _, v := range r.templates {
		out.Templates = append(out.Templates, v)
	}
	for _, v := range r.channels {
		out.Channels = append(out.Channels, v)
	}
	for _, id := range r.notificationIDs {
		out.Notifications = append(out.Notifications, r.notifications[id])
	}
	return out
}

func (r *MySQLRepository) persist(ctx context.Context) error { return r.store.Save(ctx, r.snapshot()) }
func (r *MySQLRepository) CreateTemplate(ctx context.Context, template NotificationTemplate) error {
	if err := r.MemoryRepository.CreateTemplate(ctx, template); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreateChannel(ctx context.Context, channel NotificationChannel) error {
	if err := r.MemoryRepository.CreateChannel(ctx, channel); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreateNotification(ctx context.Context, notification Notification) error {
	if err := r.MemoryRepository.CreateNotification(ctx, notification); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) UpdateNotification(ctx context.Context, notification Notification) error {
	if err := r.MemoryRepository.UpdateNotification(ctx, notification); err != nil {
		return err
	}
	return r.persist(ctx)
}
