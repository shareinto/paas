package audit

import (
	"context"

	"github.com/shareinto/paas/internal/shared"
)

type Service struct {
	repo  Repository
	ids   shared.IDGenerator
	clock shared.Clock
}

type Options struct {
	Repository  Repository
	IDGenerator shared.IDGenerator
	Clock       shared.Clock
}

func NewService(opts Options) *Service {
	ids := opts.IDGenerator
	if ids == nil {
		ids = shared.RandomIDGenerator{}
	}
	clock := opts.Clock
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Service{repo: opts.Repository, ids: ids, clock: clock}
}

func (s *Service) Log(ctx context.Context, log AuditLog) error {
	if s.repo == nil {
		return shared.NewError(shared.CodeFailedPrecondition, "audit repository is required")
	}
	normalized, err := normalizeLog(log)
	if err != nil {
		return err
	}
	now := s.clock.Now()
	if normalized.ID.IsZero() {
		normalized.ID, err = s.ids.NewID("audit")
		if err != nil {
			return err
		}
	}
	if normalized.OccurredAt.IsZero() {
		normalized.OccurredAt = now
	}
	if normalized.CreatedAt.IsZero() {
		normalized.CreatedAt = now
	}
	return s.repo.Append(ctx, normalized)
}

func (s *Service) Get(ctx context.Context, id shared.ID) (AuditLog, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) List(ctx context.Context, query Query, page shared.PageRequest) (shared.PageResult[AuditLog], error) {
	return s.repo.List(ctx, query, page)
}
