package tenantproject

import (
	"context"
	"time"

	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/shared"
)

type Repository interface {
	CreateTenant(ctx context.Context, tenant Tenant) error
	UpdateTenant(ctx context.Context, tenant Tenant) error
	GetTenant(ctx context.Context, id shared.ID) (Tenant, error)
	FindTenantByName(ctx context.Context, name string) (Tenant, error)
	ListTenants(ctx context.Context, page shared.PageRequest) (shared.PageResult[Tenant], error)

	SaveTenantMember(ctx context.Context, member TenantMember) error
	DeleteTenantMember(ctx context.Context, tenantID shared.ID, userID shared.ID) error
	GetTenantMember(ctx context.Context, tenantID shared.ID, userID shared.ID) (TenantMember, error)
	ListTenantMembers(ctx context.Context, tenantID shared.ID) ([]TenantMember, error)

	CreateProject(ctx context.Context, project Project) error
	UpdateProject(ctx context.Context, project Project) error
	DeleteProject(ctx context.Context, id shared.ID) error
	GetProject(ctx context.Context, id shared.ID) (Project, error)
	FindProjectByTenantAndName(ctx context.Context, tenantID shared.ID, name string) (Project, error)
	ListProjectsByTenant(ctx context.Context, tenantID shared.ID, page shared.PageRequest) (shared.PageResult[Project], error)
}

type AuditLogger interface {
	Log(ctx context.Context, event AuditEvent) error
}

type AuditEvent struct {
	ActorID      shared.ID
	Action       string
	ResourceType string
	ResourceID   shared.ID
	Result       string
	Summary      string
	OccurredAt   time.Time
}

type EventPublisher interface {
	Publish(ctx context.Context, event shared.DomainEvent) error
}

type TenantQuery interface {
	GetTenant(ctx context.Context, id shared.ID) (Tenant, error)
	ListTenants(ctx context.Context, page shared.PageRequest) (shared.PageResult[Tenant], error)
}

type ProjectQuery interface {
	GetProject(ctx context.Context, id shared.ID) (Project, error)
	GetProjectTenantID(ctx context.Context, projectID shared.ID) (shared.ID, error)
	ListProjectsByTenant(ctx context.Context, tenantID shared.ID, page shared.PageRequest) (shared.PageResult[Project], error)
}

type ProjectMembershipQuery interface {
	GetTenantMember(ctx context.Context, tenantID shared.ID, userID shared.ID) (TenantMember, error)
	IsTenantMember(ctx context.Context, tenantID shared.ID, userID shared.ID) (bool, error)
	ListTenantMembers(ctx context.Context, tenantID shared.ID) ([]TenantMember, error)
}

type PermissionChecker interface {
	Check(ctx context.Context, subject identityaccess.Subject, resource identityaccess.ResourceScope, action identityaccess.Permission) error
}

type ProjectDeletionGuard interface {
	PrepareProjectDeletion(ctx context.Context, actor identityaccess.Subject, project Project) error
}
