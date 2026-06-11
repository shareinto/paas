package audit

import (
	"context"

	"github.com/shareinto/paas/internal/modules/appenv"
	"github.com/shareinto/paas/internal/modules/build"
	"github.com/shareinto/paas/internal/modules/clusteragent"
	"github.com/shareinto/paas/internal/modules/delivery"
	"github.com/shareinto/paas/internal/modules/gitops"
	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/modules/sourcerepository"
	"github.com/shareinto/paas/internal/modules/tenantproject"
)

type IdentityAccessLogger struct{ Logger Logger }
type TenantProjectLogger struct{ Logger Logger }
type SourceRepositoryLogger struct{ Logger Logger }
type ApplicationEnvironmentLogger struct{ Logger Logger }
type BuildLogger struct{ Logger Logger }
type DeliveryLogger struct{ Logger Logger }
type GitOpsLogger struct{ Logger Logger }
type ClusterAgentLogger struct{ Logger Logger }

func (l IdentityAccessLogger) Log(ctx context.Context, event identityaccess.AuditEvent) error {
	return logIfConfigured(ctx, l.Logger, AuditLog{ActorID: event.ActorID, ResourceType: event.ResourceType, ResourceID: event.ResourceID, Action: event.Action, Result: Result(event.Result), Summary: event.Summary, OccurredAt: event.OccurredAt})
}

func (l TenantProjectLogger) Log(ctx context.Context, event tenantproject.AuditEvent) error {
	return logIfConfigured(ctx, l.Logger, AuditLog{ActorID: event.ActorID, ResourceType: event.ResourceType, ResourceID: event.ResourceID, Action: event.Action, Result: Result(event.Result), Summary: event.Summary, OccurredAt: event.OccurredAt})
}

func (l SourceRepositoryLogger) Log(ctx context.Context, event sourcerepository.AuditEvent) error {
	return logIfConfigured(ctx, l.Logger, AuditLog{ActorID: event.ActorID, ResourceType: event.ResourceType, ResourceID: event.ResourceID, Action: event.Action, Result: Result(event.Result), Summary: event.Summary, OccurredAt: event.OccurredAt})
}

func (l ApplicationEnvironmentLogger) Log(ctx context.Context, event appenv.AuditEvent) error {
	return logIfConfigured(ctx, l.Logger, AuditLog{ActorID: event.ActorID, ResourceType: event.ResourceType, ResourceID: event.ResourceID, Action: event.Action, Result: Result(event.Result), Summary: event.Summary, OccurredAt: event.OccurredAt})
}

func (l BuildLogger) Log(ctx context.Context, event build.AuditEvent) error {
	return logIfConfigured(ctx, l.Logger, AuditLog{ActorID: event.ActorID, ResourceType: event.ResourceType, ResourceID: event.ResourceID, Action: event.Action, Result: Result(event.Result), Summary: event.Summary, Details: event.Details, OccurredAt: event.OccurredAt})
}

func (l DeliveryLogger) Log(ctx context.Context, event delivery.AuditEvent) error {
	return logIfConfigured(ctx, l.Logger, AuditLog{ActorID: event.ActorID, ResourceType: event.ResourceType, ResourceID: event.ResourceID, Action: event.Action, Result: Result(event.Result), Summary: event.Summary, Details: event.Details, OccurredAt: event.OccurredAt})
}

func (l GitOpsLogger) Log(ctx context.Context, event gitops.AuditEvent) error {
	return logIfConfigured(ctx, l.Logger, AuditLog{ActorID: event.ActorID, TenantID: event.TenantID, ProjectID: event.ProjectID, ResourceType: event.ResourceType, ResourceID: event.ResourceID, Action: event.Action, Result: Result(event.Result), Summary: event.Summary, OccurredAt: event.OccurredAt})
}

func (l ClusterAgentLogger) Log(ctx context.Context, event clusteragent.AuditEvent) error {
	return logIfConfigured(ctx, l.Logger, AuditLog{ActorID: event.ActorID, TenantID: event.TenantID, ResourceType: event.ResourceType, ResourceID: event.ResourceID, Action: event.Action, Result: Result(event.Result), Summary: event.Summary, OccurredAt: event.OccurredAt})
}

func logIfConfigured(ctx context.Context, logger Logger, log AuditLog) error {
	if logger == nil {
		return nil
	}
	return logger.Log(ctx, log)
}
