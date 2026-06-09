package audit

import (
	"context"

	"github.com/shareinto/paas/internal/shared"
)

type Repository interface {
	Append(ctx context.Context, log AuditLog) error
	Get(ctx context.Context, id shared.ID) (AuditLog, error)
	List(ctx context.Context, query Query, page shared.PageRequest) (shared.PageResult[AuditLog], error)
}

type Logger interface {
	Log(ctx context.Context, log AuditLog) error
}
