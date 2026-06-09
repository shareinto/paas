package delivery

import (
	"context"

	"github.com/shareinto/paas/internal/shared"
)

type NoopAuditLogger struct{}

func (NoopAuditLogger) Log(context.Context, AuditEvent) error {
	return nil
}

type NoopEventPublisher struct{}

func (NoopEventPublisher) Publish(context.Context, shared.DomainEvent) error {
	return nil
}
