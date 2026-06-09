package audit

import (
	"context"
	"database/sql"

	"github.com/shareinto/paas/internal/platform/database"
)

type MySQLRepository struct {
	*MemoryRepository
	store *database.SnapshotStore
}

type auditSnapshot struct {
	Logs []AuditLog
}

func NewMySQLRepository(ctx context.Context, db *sql.DB) (*MySQLRepository, error) {
	repo := &MySQLRepository{MemoryRepository: NewMemoryRepository(), store: database.NewSnapshotStore(db, "audit")}
	var snapshot auditSnapshot
	if err := repo.store.Load(ctx, &snapshot); err != nil {
		return nil, err
	}
	for _, log := range snapshot.Logs {
		repo.logs[log.ID] = log
		repo.order = append(repo.order, log.ID)
		repo.sealed[log.ID] = struct{}{}
	}
	return repo, nil
}

func (r *MySQLRepository) snapshot() auditSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := auditSnapshot{Logs: make([]AuditLog, 0, len(r.order))}
	for _, id := range r.order {
		out.Logs = append(out.Logs, r.logs[id])
	}
	return out
}

func (r *MySQLRepository) Append(ctx context.Context, log AuditLog) error {
	if err := r.MemoryRepository.Append(ctx, log); err != nil {
		return err
	}
	return r.store.Save(ctx, r.snapshot())
}
