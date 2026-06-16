package sourcerepository

import (
	"context"
	"time"

	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/modules/tenantproject"
	"github.com/shareinto/paas/internal/shared"
)

type Repository interface {
	CreateSourceRepository(ctx context.Context, repository SourceRepository) error
	UpdateSourceRepository(ctx context.Context, repository SourceRepository) error
	DeleteSourceRepository(ctx context.Context, id shared.ID) error
	GetSourceRepository(ctx context.Context, id shared.ID) (SourceRepository, error)
	FindSourceRepositoryByProjectAndName(ctx context.Context, projectID shared.ID, name string) (SourceRepository, error)
	ListSourceRepositoriesByProject(ctx context.Context, projectID shared.ID, page shared.PageRequest) (shared.PageResult[SourceRepository], error)
	ListAssociatedApplications(ctx context.Context, sourceRepositoryID shared.ID) ([]AssociatedApplication, error)
	SetAssociatedApplications(ctx context.Context, sourceRepositoryID shared.ID, applications []AssociatedApplication) error

	CreateMigration(ctx context.Context, migration RepositoryMigration) error
	UpdateMigration(ctx context.Context, migration RepositoryMigration) error
	GetMigration(ctx context.Context, id shared.ID) (RepositoryMigration, error)
	ListMigrationsByRepository(ctx context.Context, sourceRepositoryID shared.ID, page shared.PageRequest) (shared.PageResult[RepositoryMigration], error)

	CreatePermissionSyncJob(ctx context.Context, job RepositoryPermissionSyncJob) error
	UpdatePermissionSyncJob(ctx context.Context, job RepositoryPermissionSyncJob) error
	GetPermissionSyncJob(ctx context.Context, id shared.ID) (RepositoryPermissionSyncJob, error)
}

type GitSourceRepositoryPort interface {
	ResolveProjectByHTTPURL(ctx context.Context, httpURL string) (GitProject, error)
	CreateProject(ctx context.Context, spec GitProjectSpec) (GitProject, error)
	DeleteProject(ctx context.Context, gitProjectID string) error
	InitializeRepository(ctx context.Context, gitProjectID string, defaultBranch string) error
	ProtectBranch(ctx context.Context, gitProjectID string, branch string) error
	ConfigureWebhook(ctx context.Context, gitProjectID string, callbackURL string) error
	SyncMembers(ctx context.Context, gitProjectID string, members []GitMemberAccess) error
	MirrorRepository(ctx context.Context, spec GitMirrorSpec) error
	VerifyRepository(ctx context.Context, gitProjectID string) error
	ListFiles(ctx context.Context, gitProjectID string, ref string) ([]RepositoryFile, error)
	ListTree(ctx context.Context, gitProjectID string, ref string, path string) ([]RepositoryTreeItem, error)
	ListBranches(ctx context.Context, gitProjectID string) ([]RepositoryBranch, error)
}

type GitProjectSpec struct {
	TenantID       shared.ID
	TenantName     string
	ProjectID      shared.ID
	ProjectName    string
	RepositoryID   shared.ID
	RepositoryName string
	DefaultBranch  string
}

type GitProject struct {
	ID      string
	HTTPURL string
	SSHURL  string
}

type GitAccessLevel string

const (
	GitAccessOwner      GitAccessLevel = "Owner"
	GitAccessMaintainer GitAccessLevel = "Maintainer"
	GitAccessDeveloper  GitAccessLevel = "Developer"
	GitAccessReporter   GitAccessLevel = "Reporter"
)

type GitMemberAccess struct {
	UserID shared.ID
	RoleID identityaccess.RoleID
	Access GitAccessLevel
}

type GitMirrorSpec struct {
	SourceURL    string
	GitProjectID string
}

type RepositoryFile struct {
	Path string
	Type string
}

type PermissionChecker interface {
	Check(ctx context.Context, subject identityaccess.Subject, resource identityaccess.ResourceScope, action identityaccess.Permission) error
}

type ProjectQuery interface {
	GetProject(ctx context.Context, id shared.ID) (tenantproject.Project, error)
	GetTenant(ctx context.Context, id shared.ID) (tenantproject.Tenant, error)
}

type ProjectMembershipQuery interface {
	ListTenantMembers(ctx context.Context, tenantID shared.ID) ([]tenantproject.TenantMember, error)
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
