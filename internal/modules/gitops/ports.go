package gitops

import (
	"context"
	"time"

	"github.com/shareinto/paas/internal/shared"
)

type Repository interface {
	CreateTemplate(ctx context.Context, template DeploymentTemplate) error
	UpdateTemplate(ctx context.Context, template DeploymentTemplate) error
	GetTemplate(ctx context.Context, id shared.ID) (DeploymentTemplate, error)
	FindPlatformTemplate(ctx context.Context, name string) (DeploymentTemplate, error)
	FindApplicationTemplate(ctx context.Context, applicationID shared.ID) (DeploymentTemplate, error)
	CreateTemplateRevision(ctx context.Context, revision DeploymentTemplateRevision) error
	GetCurrentTemplateRevision(ctx context.Context, templateID shared.ID) (DeploymentTemplateRevision, error)

	CreateManifestRevision(ctx context.Context, revision ManifestRevision) error
	GetManifestRevision(ctx context.Context, id shared.ID) (ManifestRevision, error)

	CreateDeployment(ctx context.Context, deployment Deployment) error
	UpdateDeployment(ctx context.Context, deployment Deployment) error
	GetDeployment(ctx context.Context, id shared.ID) (Deployment, error)
	FindDeploymentByPromotion(ctx context.Context, promotionID shared.ID) (Deployment, error)
	ListDeployments(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[Deployment], error)
	CreateDeploymentEvent(ctx context.Context, event DeploymentEvent) error
}

type ManifestRepositoryPort interface {
	ReadFile(ctx context.Context, path string, ref string) (string, error)
	CommitFiles(ctx context.Context, spec CommitSpec) (CommitResult, error)
	CreateMergeRequest(ctx context.Context, spec MergeRequestSpec) (MergeRequestResult, error)
	GetMergeRequest(ctx context.Context, mrID string) (MergeRequest, error)
	CreateTag(ctx context.Context, name string, ref string) (TagResult, error)
}

type CommitFile struct {
	Path    string
	Content string
}

type CommitSpec struct {
	Branch      string
	StartBranch string
	Message     string
	Files       []CommitFile
}

type CommitResult struct {
	CommitSHA string
}

type MergeRequestSpec struct {
	SourceBranch string
	TargetBranch string
	Title        string
	Files        []CommitFile
}

type MergeRequestResult struct {
	ID        string
	CommitSHA string
	WebURL    string
}

type MergeRequest struct {
	ID     string
	State  string
	WebURL string
	Merged bool
}

type TagResult struct {
	Name string
	Ref  string
}

type ApplicationQuery interface {
	GetApplication(ctx context.Context, id shared.ID) (ApplicationRef, error)
}

type ApplicationRef struct {
	ID        shared.ID
	TenantID  shared.ID
	ProjectID shared.ID
	Name      string
}

type WorkloadQuery interface {
	GetWorkload(ctx context.Context, applicationID shared.ID, workloadID shared.ID) (WorkloadRef, error)
	GetWorkloadStageConfig(ctx context.Context, workloadID shared.ID, stageKey string) (WorkloadStageConfigRef, error)
	GetWorkloadDefaultConfig(ctx context.Context, workloadID shared.ID) (WorkloadStageConfigRef, error)
}

type AuditLogger interface {
	Log(ctx context.Context, event AuditEvent) error
}

type AuditEvent struct {
	ActorID      shared.ID
	TenantID     shared.ID
	ProjectID    shared.ID
	Action       string
	ResourceType string
	ResourceID   shared.ID
	Result       string
	Summary      string
	OccurredAt   time.Time
}
