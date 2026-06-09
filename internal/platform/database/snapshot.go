package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/shareinto/paas/internal/shared"
)

type SnapshotStore struct {
	db     *sql.DB
	module string
}

func NewSnapshotStore(db *sql.DB, module string) *SnapshotStore {
	return &SnapshotStore{db: db, module: module}
}

func (s *SnapshotStore) Load(ctx context.Context, target any) error {
	var payload []byte
	err := s.db.QueryRowContext(ctx, "SELECT payload FROM repository_snapshots WHERE module = ?", s.module).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return shared.WrapError(shared.CodeUnavailable, "load repository snapshot failed", err)
	}
	if len(payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return shared.WrapError(shared.CodeInternal, "decode repository snapshot failed", err)
	}
	return nil
}

func (s *SnapshotStore) Save(ctx context.Context, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return shared.WrapError(shared.CodeInternal, "encode repository snapshot failed", err)
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO repository_snapshots(module, payload, updated_at)
VALUES (?, ?, ?)
ON DUPLICATE KEY UPDATE payload = VALUES(payload), updated_at = VALUES(updated_at)`,
		s.module, string(payload), time.Now().UTC())
	if err != nil {
		return shared.WrapError(shared.CodeUnavailable, "save repository snapshot failed", err)
	}
	return nil
}
