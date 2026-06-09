package gitops

import "context"

type NoopAuditLogger struct{}

func (NoopAuditLogger) Log(context.Context, AuditEvent) error { return nil }
