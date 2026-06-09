package delivery

import (
	"context"
	"time"

	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/shared"
)

type Repository interface {
	CreateRelease(ctx context.Context, release Release) error
	GetRelease(ctx context.Context, id shared.ID) (Release, error)
	FindReleaseByBuildRun(ctx context.Context, buildRunID shared.ID) (Release, error)

	CreateFreight(ctx context.Context, freight Freight) error
	GetFreight(ctx context.Context, id shared.ID) (Freight, error)
	ListFreightsByApplication(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[Freight], error)
	CreateFreightItem(ctx context.Context, item FreightItem) error
	ListFreightItems(ctx context.Context, freightID shared.ID) ([]FreightItem, error)

	CreateDeliveryFlow(ctx context.Context, flow DeliveryFlow) error
	GetDeliveryFlow(ctx context.Context, id shared.ID) (DeliveryFlow, error)
	FindDeliveryFlowByApplication(ctx context.Context, applicationID shared.ID) (DeliveryFlow, error)
	CreateDeliveryStage(ctx context.Context, stage DeliveryStage) error
	GetDeliveryStage(ctx context.Context, id shared.ID) (DeliveryStage, error)
	FindDeliveryStageByEnvironment(ctx context.Context, applicationID shared.ID, environmentID shared.ID) (DeliveryStage, error)
	ListDeliveryStages(ctx context.Context, flowID shared.ID) ([]DeliveryStage, error)

	CreatePromotion(ctx context.Context, promotion Promotion) error
	UpdatePromotion(ctx context.Context, promotion Promotion) error
	GetPromotion(ctx context.Context, id shared.ID) (Promotion, error)
	ListPromotionsByApplication(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[Promotion], error)
	CreatePromotionApproval(ctx context.Context, approval PromotionApproval) error
	UpdatePromotionApproval(ctx context.Context, approval PromotionApproval) error
	GetPromotionApproval(ctx context.Context, promotionID shared.ID) (PromotionApproval, error)
}

type ApplicationRef struct {
	ID        shared.ID
	TenantID  shared.ID
	ProjectID shared.ID
	Name      string
}

type EnvironmentRef struct {
	ID            shared.ID
	TenantID      shared.ID
	ProjectID     shared.ID
	ApplicationID shared.ID
	Name          string
	Status        string
	BindingActive bool
}

type BuildRunRef struct {
	ID                  shared.ID
	TenantID            shared.ID
	ProjectID           shared.ID
	ApplicationID       shared.ID
	PipelineID          shared.ID
	PipelineName        string
	PipelineDisplayName string
	CommitSHA           string
}

type BuildArtifactRef struct {
	ID            shared.ID
	BuildRunID    shared.ID
	ApplicationID shared.ID
	SourceKey     string
	URI           string
	Digest        string
	IsPrimary     bool
}

type BuildQuery interface {
	GetBuildRun(ctx context.Context, id shared.ID) (BuildRunRef, error)
	GetBuildArtifact(ctx context.Context, id shared.ID) (BuildArtifactRef, error)
	ListBuildArtifacts(ctx context.Context, buildRunID shared.ID) ([]BuildArtifactRef, error)
}

type ApplicationQuery interface {
	GetApplication(ctx context.Context, id shared.ID) (ApplicationRef, error)
}

type EnvironmentQuery interface {
	ListEnvironments(ctx context.Context, applicationID shared.ID) ([]EnvironmentRef, error)
	GetEnvironment(ctx context.Context, id shared.ID) (EnvironmentRef, error)
}

type GitOpsDeploymentCommand interface {
	ApplyPromotion(ctx context.Context, spec GitOpsPromotionSpec) (GitOpsPromotionResult, error)
}

type GitOpsPromotionSpec struct {
	PromotionID   shared.ID
	FreightID     shared.ID
	ApplicationID shared.ID
	EnvironmentID shared.ID
	Artifacts     []GitOpsArtifactSpec
	IsRollback    bool
	ImageURI      string
	ImageDigest   string
}

type GitOpsArtifactSpec struct {
	Name      string
	SourceKey string
	URI       string
	Digest    string
	IsPrimary bool
}

type GitOpsPromotionResult struct {
	ManifestRevision string
}

type PermissionChecker interface {
	Check(ctx context.Context, subject identityaccess.Subject, resource identityaccess.ResourceScope, action identityaccess.Permission) error
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

type ReleaseQuery interface {
	GetRelease(ctx context.Context, id shared.ID) (Release, error)
}

type FreightQuery interface {
	GetFreight(ctx context.Context, id shared.ID) (Freight, error)
	ListFreights(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[Freight], error)
}

type PromotionCommand interface {
	CreatePromotion(ctx context.Context, input CreatePromotionInput) (Promotion, error)
	AbortPromotion(ctx context.Context, actor identityaccess.Subject, promotionID shared.ID) (Promotion, error)
	CreateRollbackPromotion(ctx context.Context, input CreateRollbackPromotionInput) (Promotion, error)
}

type PromotionQuery interface {
	GetPromotion(ctx context.Context, id shared.ID) (Promotion, error)
	ListPromotions(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[Promotion], error)
}

type ApprovalCommand interface {
	ApprovePromotion(ctx context.Context, input ApprovalInput) (Promotion, error)
	RejectPromotion(ctx context.Context, input ApprovalInput) (Promotion, error)
}
