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
	FindReleaseByBuildRunAndWorkload(ctx context.Context, buildRunID shared.ID, workloadID shared.ID) (Release, error)
	ListReleasesByApplication(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[Release], error)
	CreateImageBundle(ctx context.Context, bundle ImageBundle) error
	CreateImageBundleImage(ctx context.Context, image ImageBundleImage) error
	GetImageBundle(ctx context.Context, id shared.ID) (ImageBundle, error)
	ListImageBundleImages(ctx context.Context, bundleID shared.ID) ([]ImageBundleImage, error)

	CreateFreight(ctx context.Context, freight Freight) error
	GetFreight(ctx context.Context, id shared.ID) (Freight, error)
	ListFreightsByApplication(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[Freight], error)
	UpdateFreightStatus(ctx context.Context, id shared.ID, status FreightStatus) error
	CreateFreightItem(ctx context.Context, item FreightItem) error
	ListFreightItems(ctx context.Context, freightID shared.ID) ([]FreightItem, error)

	CreateDeliveryFlow(ctx context.Context, flow DeliveryFlow) error
	GetDeliveryFlow(ctx context.Context, id shared.ID) (DeliveryFlow, error)
	FindDeliveryFlowByApplication(ctx context.Context, applicationID shared.ID) (DeliveryFlow, error)
	CreateDeliveryStage(ctx context.Context, stage DeliveryStage) error
	GetDeliveryStage(ctx context.Context, id shared.ID) (DeliveryStage, error)
	FindDeliveryStageByName(ctx context.Context, applicationID shared.ID, name string) (DeliveryStage, error)
	ListDeliveryStages(ctx context.Context, flowID shared.ID) ([]DeliveryStage, error)

	CreateDeliveryFlowTemplate(ctx context.Context, template DeliveryFlowTemplate) error
	FindDeliveryFlowTemplateByTenant(ctx context.Context, tenantID shared.ID) (DeliveryFlowTemplate, error)
	CreateDeliveryFlowTemplateStage(ctx context.Context, stage DeliveryFlowTemplateStage) error
	UpdateDeliveryFlowTemplateStage(ctx context.Context, stage DeliveryFlowTemplateStage) error
	DeleteDeliveryFlowTemplateStage(ctx context.Context, tenantID shared.ID, stageKey string) error
	FindDeliveryFlowTemplateStage(ctx context.Context, tenantID shared.ID, stageKey string) (DeliveryFlowTemplateStage, error)
	ListDeliveryFlowTemplateStages(ctx context.Context, templateID shared.ID) ([]DeliveryFlowTemplateStage, error)
	ReplaceDeliveryFlowTemplateEdges(ctx context.Context, templateID shared.ID, edges []DeliveryFlowTemplateEdge) error
	ListDeliveryFlowTemplateEdges(ctx context.Context, templateID shared.ID) ([]DeliveryFlowTemplateEdge, error)
	ReplaceStageClusterBindings(ctx context.Context, tenantID shared.ID, stageKey string, bindings []StageClusterBinding) error
	ListStageClusterBindings(ctx context.Context, tenantID shared.ID, stageKey string) ([]StageClusterBinding, error)
	CreateFreightApproval(ctx context.Context, approval FreightApproval) error
	FindFreightApproval(ctx context.Context, freightID shared.ID, targetStageKey string) (FreightApproval, error)
	CreateStageVerification(ctx context.Context, verification StageVerification) error
	FindStageVerification(ctx context.Context, applicationID shared.ID, stageKey string, freightID shared.ID) (StageVerification, error)

	CreatePromotion(ctx context.Context, promotion Promotion) error
	UpdatePromotion(ctx context.Context, promotion Promotion) error
	GetPromotion(ctx context.Context, id shared.ID) (Promotion, error)
	ListPromotionsByApplication(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[Promotion], error)
	CreatePromotionApproval(ctx context.Context, approval PromotionApproval) error
	UpdatePromotionApproval(ctx context.Context, approval PromotionApproval) error
	GetPromotionApproval(ctx context.Context, promotionID shared.ID) (PromotionApproval, error)
}

type ApplicationRef struct {
	ID          shared.ID
	TenantID    shared.ID
	ProjectID   shared.ID
	ProjectName string
	Name        string
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
	Status              string
}

type BuildArtifactRef struct {
	ID             shared.ID
	BuildRunID     shared.ID
	ApplicationID  shared.ID
	WorkloadID     shared.ID
	SourceKey      string
	URI            string
	Digest         string
	IsPrimary      bool
	SelectorLabels map[string]string
	Metadata       map[string]string
}

type WorkloadRef struct {
	ID            shared.ID `json:"id"`
	TenantID      shared.ID `json:"tenant_id"`
	ProjectID     shared.ID `json:"project_id"`
	ApplicationID shared.ID `json:"application_id"`
	Name          string    `json:"name"`
	DisplayName   string    `json:"display_name"`
	Status        string    `json:"status"`
}

type WorkloadQuery interface {
	ListEnabledWorkloads(ctx context.Context, applicationID shared.ID) ([]WorkloadRef, error)
}

type BuildQuery interface {
	GetBuildRun(ctx context.Context, id shared.ID) (BuildRunRef, error)
	ListBuildRuns(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[BuildRunRef], error)
	GetBuildArtifact(ctx context.Context, id shared.ID) (BuildArtifactRef, error)
	ListBuildArtifacts(ctx context.Context, buildRunID shared.ID) ([]BuildArtifactRef, error)
}

type ApplicationQuery interface {
	GetApplication(ctx context.Context, id shared.ID) (ApplicationRef, error)
}

type StageRuntimeState struct {
	ApplicationID  shared.ID
	StageKey       string
	SyncStatus     string
	HealthStatus   string
	OperationState string
	Message        string
}

type StageRuntimeStateQuery interface {
	ListStageRuntimeStates(ctx context.Context, applicationID shared.ID) (map[string]StageRuntimeState, error)
}

type SyncApplicationStagesInput struct {
	TenantID  shared.ID
	StageKeys []string
}

type StageSync interface {
	SyncApplicationStages(ctx context.Context, input SyncApplicationStagesInput) error
}

type ClusterRef struct {
	ID       shared.ID
	TenantID shared.ID
	Name     string
	Labels   map[string]string
}

type ClusterQuery interface {
	GetCluster(ctx context.Context, id shared.ID) (ClusterRef, error)
}

type GitOpsDeploymentCommand interface {
	ApplyPromotion(ctx context.Context, spec GitOpsPromotionSpec) (GitOpsPromotionResult, error)
}

type GitOpsPromotionSpec struct {
	PromotionID    shared.ID
	FreightID      shared.ID
	ApplicationID  shared.ID
	StageKey       string
	TargetClusters []GitOpsPromotionTargetCluster
	Artifacts      []GitOpsArtifactSpec
	IsRollback     bool
	ImageURI       string
	ImageDigest    string
}

type GitOpsPromotionTargetCluster struct {
	ClusterID   shared.ID
	ClusterName string
	Namespace   string
	Labels      map[string]string
}

type GitOpsArtifactSpec struct {
	WorkloadID shared.ID
	Name       string
	SourceKey  string
	URI        string
	Repository string
	Tag        string
	Digest     string
	IsPrimary  bool
	Variants   []GitOpsImageVariant
}

type GitOpsImageVariant struct {
	URI                    string
	Repository             string
	Tag                    string
	Digest                 string
	RuntimeEnvironmentID   shared.ID
	RuntimeEnvironmentName string
	SelectorLabels         map[string]string
	IsPrimary              bool
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
	Details      map[string]string
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
