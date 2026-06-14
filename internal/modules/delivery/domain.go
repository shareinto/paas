package delivery

import (
	"strings"
	"time"

	"github.com/shareinto/paas/internal/shared"
)

type ReleaseStatus string

const (
	ReleaseReady    ReleaseStatus = "ready"
	ReleaseDisabled ReleaseStatus = "disabled"
)

type Release struct {
	ID                  shared.ID          `json:"id"`
	TenantID            shared.ID          `json:"tenant_id"`
	ProjectID           shared.ID          `json:"project_id"`
	ApplicationID       shared.ID          `json:"application_id"`
	WorkloadID          shared.ID          `json:"workload_id"`
	PipelineID          shared.ID          `json:"pipeline_id"`
	PipelineName        string             `json:"pipeline_name"`
	PipelineDisplayName string             `json:"pipeline_display_name"`
	BuildRunID          shared.ID          `json:"build_run_id"`
	BuildArtifactID     shared.ID          `json:"build_artifact_id"`
	ImageBundleID       shared.ID          `json:"image_bundle_id"`
	Version             string             `json:"version"`
	CommitSHA           string             `json:"commit_sha"`
	ImageURI            string             `json:"image_uri"`
	ImageRepository     string             `json:"image_repository"`
	ImageTag            string             `json:"image_tag"`
	ImageDigest         string             `json:"image_digest"`
	SourceType          string             `json:"source_type"`
	Status              ReleaseStatus      `json:"status"`
	BundleImages        []ImageBundleImage `json:"bundle_images,omitempty"`
	CreatedAt           time.Time          `json:"created_at"`
}

type ImageBundle struct {
	ID            shared.ID `json:"id"`
	TenantID      shared.ID `json:"tenant_id"`
	ProjectID     shared.ID `json:"project_id"`
	ApplicationID shared.ID `json:"application_id"`
	WorkloadID    shared.ID `json:"workload_id"`
	BuildRunID    shared.ID `json:"build_run_id"`
	CommitSHA     string    `json:"commit_sha"`
	CreatedAt     time.Time `json:"created_at"`
}

type ImageBundleImage struct {
	ID                     shared.ID         `json:"id"`
	BundleID               shared.ID         `json:"bundle_id"`
	BuildArtifactID        shared.ID         `json:"build_artifact_id"`
	RuntimeEnvironmentID   shared.ID         `json:"runtime_environment_id"`
	RuntimeEnvironmentName string            `json:"runtime_environment_name"`
	URI                    string            `json:"uri"`
	ImageRepository        string            `json:"image_repository"`
	ImageTag               string            `json:"image_tag"`
	Digest                 string            `json:"digest"`
	SelectorLabels         map[string]string `json:"selector_labels"`
	IsPrimary              bool              `json:"is_primary"`
	CreatedAt              time.Time         `json:"created_at"`
}

type FreightStatus string

const (
	FreightAvailable FreightStatus = "available"
	FreightArchived  FreightStatus = "archived"
)

type Freight struct {
	ID                  shared.ID     `json:"id"`
	TenantID            shared.ID     `json:"tenant_id"`
	ProjectID           shared.ID     `json:"project_id"`
	ApplicationID       shared.ID     `json:"application_id"`
	PipelineID          shared.ID     `json:"pipeline_id"`
	PipelineName        string        `json:"pipeline_name"`
	PipelineDisplayName string        `json:"pipeline_display_name"`
	Name                string        `json:"name"`
	Status              FreightStatus `json:"status"`
	CreatedAt           time.Time     `json:"created_at"`
}

type FreightDetail struct {
	Freight Freight       `json:"freight"`
	Items   []FreightItem `json:"items"`
}

type FreightItemType string

const (
	FreightItemPipelineArtifact   FreightItemType = "pipeline_artifact"
	FreightItemCustomImage        FreightItemType = "custom_image"
	FreightItemApplicationRelease FreightItemType = "application_release"
	FreightItemImage              FreightItemType = "image"
	FreightItemConfig             FreightItemType = "config"
	FreightItemMigration          FreightItemType = "migration"
)

type FreightItem struct {
	ID              shared.ID          `json:"id"`
	TenantID        shared.ID          `json:"tenant_id"`
	ProjectID       shared.ID          `json:"project_id"`
	FreightID       shared.ID          `json:"freight_id"`
	ApplicationID   shared.ID          `json:"application_id"`
	WorkloadID      shared.ID          `json:"workload_id"`
	ReleaseID       shared.ID          `json:"release_id"`
	BuildArtifactID shared.ID          `json:"build_artifact_id"`
	ImageBundleID   shared.ID          `json:"image_bundle_id"`
	SourceType      FreightItemType    `json:"source_type"`
	SourceKey       string             `json:"source_key"`
	Type            FreightItemType    `json:"type"`
	Name            string             `json:"name"`
	URI             string             `json:"uri"`
	ImageRef        string             `json:"image_ref"`
	ImageRepository string             `json:"image_repository"`
	ImageTag        string             `json:"image_tag"`
	Digest          string             `json:"digest"`
	BundleImages    []ImageBundleImage `json:"bundle_images,omitempty"`
	CreatedAt       time.Time          `json:"created_at"`
}

type DeliveryFlow struct {
	ID            shared.ID `json:"id"`
	TenantID      shared.ID `json:"tenant_id"`
	ProjectID     shared.ID `json:"project_id"`
	ApplicationID shared.ID `json:"application_id"`
	Name          string    `json:"name"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type DeliveryFlowTemplateStageStatus string

const (
	DeliveryFlowTemplateStageEnabled  DeliveryFlowTemplateStageStatus = "enabled"
	DeliveryFlowTemplateStageDisabled DeliveryFlowTemplateStageStatus = "disabled"
)

type DeliveryFlowTemplate struct {
	ID        shared.ID                   `json:"id"`
	TenantID  shared.ID                   `json:"tenant_id"`
	Name      string                      `json:"name"`
	Stages    []DeliveryFlowTemplateStage `json:"stages,omitempty"`
	Edges     []DeliveryFlowTemplateEdge  `json:"edges,omitempty"`
	CreatedAt time.Time                   `json:"created_at"`
	UpdatedAt time.Time                   `json:"updated_at"`
}

type DeliveryFlowTemplateStage struct {
	ID                   shared.ID                       `json:"id"`
	TenantID             shared.ID                       `json:"tenant_id"`
	TemplateID           shared.ID                       `json:"template_id"`
	StageKey             string                          `json:"stage_key"`
	DisplayName          string                          `json:"display_name"`
	Color                string                          `json:"color"`
	Order                int                             `json:"order"`
	LayoutColumn         int                             `json:"layout_column"`
	LayoutRow            int                             `json:"layout_row"`
	Status               DeliveryFlowTemplateStageStatus `json:"status"`
	RequiresApproval     bool                            `json:"requires_approval"`
	RequiresVerification bool                            `json:"requires_verification"`
	ApproveRoles         []string                        `json:"approve_roles"`
	VerifyRoles          []string                        `json:"verify_roles"`
	CreatedAt            time.Time                       `json:"created_at"`
	UpdatedAt            time.Time                       `json:"updated_at"`
}

type DeliveryFlowTemplateEdge struct {
	ID           shared.ID `json:"id"`
	TenantID     shared.ID `json:"tenant_id"`
	TemplateID   shared.ID `json:"template_id"`
	FromStageKey string    `json:"from_stage_key"`
	ToStageKey   string    `json:"to_stage_key"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type StageClusterBindingStatus string

const (
	StageClusterBindingActive   StageClusterBindingStatus = "active"
	StageClusterBindingDisabled StageClusterBindingStatus = "disabled"
)

type StageClusterBinding struct {
	ID          shared.ID                 `json:"id"`
	TenantID    shared.ID                 `json:"tenant_id"`
	StageKey    string                    `json:"stage_key"`
	ClusterID   shared.ID                 `json:"cluster_id"`
	ClusterName string                    `json:"cluster_name"`
	Status      StageClusterBindingStatus `json:"status"`
	CreatedAt   time.Time                 `json:"created_at"`
	UpdatedAt   time.Time                 `json:"updated_at"`
}

type FreightApprovalStatus string

const (
	FreightApprovalPending  FreightApprovalStatus = "pending"
	FreightApprovalApproved FreightApprovalStatus = "approved"
	FreightApprovalRejected FreightApprovalStatus = "rejected"
)

type FreightApproval struct {
	ID             shared.ID             `json:"id"`
	TenantID       shared.ID             `json:"tenant_id"`
	ProjectID      shared.ID             `json:"project_id"`
	ApplicationID  shared.ID             `json:"application_id"`
	FreightID      shared.ID             `json:"freight_id"`
	TargetStageKey string                `json:"target_stage_key"`
	ApproverID     shared.ID             `json:"approver_id"`
	Status         FreightApprovalStatus `json:"status"`
	Comment        string                `json:"comment"`
	CreatedAt      time.Time             `json:"created_at"`
	UpdatedAt      time.Time             `json:"updated_at"`
}

type StageVerificationStatus string

const (
	StageVerificationPassed StageVerificationStatus = "passed"
	StageVerificationFailed StageVerificationStatus = "failed"
)

type StageVerification struct {
	ID            shared.ID               `json:"id"`
	TenantID      shared.ID               `json:"tenant_id"`
	ProjectID     shared.ID               `json:"project_id"`
	ApplicationID shared.ID               `json:"application_id"`
	StageKey      string                  `json:"stage_key"`
	FreightID     shared.ID               `json:"freight_id"`
	VerifierID    shared.ID               `json:"verifier_id"`
	Status        StageVerificationStatus `json:"status"`
	Comment       string                  `json:"comment"`
	SyncStatus    string                  `json:"sync_status"`
	HealthStatus  string                  `json:"health_status"`
	AgentStatus   string                  `json:"agent_status"`
	CreatedAt     time.Time               `json:"created_at"`
	UpdatedAt     time.Time               `json:"updated_at"`
}

type AppStage struct {
	TenantID              shared.ID                       `json:"tenant_id"`
	ProjectID             shared.ID                       `json:"project_id"`
	ApplicationID         shared.ID                       `json:"application_id"`
	DeliveryStageID       shared.ID                       `json:"delivery_stage_id"`
	EnvironmentID         shared.ID                       `json:"environment_id"`
	StageKey              string                          `json:"stage_key"`
	DisplayName           string                          `json:"display_name"`
	Color                 string                          `json:"color"`
	Order                 int                             `json:"order"`
	LayoutColumn          int                             `json:"layout_column"`
	LayoutRow             int                             `json:"layout_row"`
	Status                DeliveryFlowTemplateStageStatus `json:"status"`
	RequiresApproval      bool                            `json:"requires_approval"`
	RequiresVerification  bool                            `json:"requires_verification"`
	ApproveRoles          []string                        `json:"approve_roles"`
	VerifyRoles           []string                        `json:"verify_roles"`
	ClusterPoolSize       int                             `json:"cluster_pool_size"`
	BoundClusterID        shared.ID                       `json:"bound_cluster_id,omitempty"`
	BoundClusterName      string                          `json:"bound_cluster_name,omitempty"`
	CurrentFreightID      shared.ID                       `json:"current_freight_id,omitempty"`
	CurrentFreightVersion string                          `json:"current_freight_version,omitempty"`
	SyncStatus            string                          `json:"sync_status,omitempty"`
	HealthStatus          string                          `json:"health_status,omitempty"`
	UpstreamStageKeys     []string                        `json:"upstream_stage_keys,omitempty"`
	DownstreamStageKeys   []string                        `json:"downstream_stage_keys,omitempty"`
}

type DeliveryStage struct {
	ID               shared.ID `json:"id"`
	TenantID         shared.ID `json:"tenant_id"`
	ProjectID        shared.ID `json:"project_id"`
	ApplicationID    shared.ID `json:"application_id"`
	DeliveryFlowID   shared.ID `json:"delivery_flow_id"`
	EnvironmentID    shared.ID `json:"environment_id"`
	Name             string    `json:"name"`
	Order            int       `json:"order"`
	RequiresApproval bool      `json:"requires_approval"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type PromotionStatus string

const (
	PromotionCreated          PromotionStatus = "created"
	PromotionPendingApproval  PromotionStatus = "pending_approval"
	PromotionApproved         PromotionStatus = "approved"
	PromotionRejected         PromotionStatus = "rejected"
	PromotionManifestUpdating PromotionStatus = "manifest_updating"
	PromotionManifestUpdated  PromotionStatus = "manifest_updated"
	PromotionSyncing          PromotionStatus = "syncing"
	PromotionHealthy          PromotionStatus = "healthy"
	PromotionFailed           PromotionStatus = "failed"
	PromotionAborted          PromotionStatus = "aborted"
)

var AllowedPromotionStatuses = []string{
	string(PromotionCreated),
	string(PromotionPendingApproval),
	string(PromotionApproved),
	string(PromotionRejected),
	string(PromotionManifestUpdating),
	string(PromotionManifestUpdated),
	string(PromotionSyncing),
	string(PromotionHealthy),
	string(PromotionFailed),
	string(PromotionAborted),
}

type Promotion struct {
	ID                    shared.ID       `json:"id"`
	TenantID              shared.ID       `json:"tenant_id"`
	ProjectID             shared.ID       `json:"project_id"`
	ApplicationID         shared.ID       `json:"application_id"`
	FreightID             shared.ID       `json:"freight_id"`
	TargetStageID         shared.ID       `json:"target_stage_id"`
	TargetEnvironmentID   shared.ID       `json:"target_environment_id"`
	TargetStageKey        string          `json:"target_stage_key"`
	NamespaceOverride     string          `json:"namespace_override"`
	Status                PromotionStatus `json:"status"`
	IsRollback            bool            `json:"is_rollback"`
	RollbackFromFreightID shared.ID       `json:"rollback_from_freight_id"`
	CreatedBy             shared.ID       `json:"created_by"`
	ApprovedBy            shared.ID       `json:"approved_by"`
	Message               string          `json:"message"`
	ManifestRevision      string          `json:"manifest_revision"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
	CompletedAt           *time.Time      `json:"completed_at,omitempty"`
}

type PromotionApprovalStatus string

const (
	PromotionApprovalPending  PromotionApprovalStatus = "pending"
	PromotionApprovalApproved PromotionApprovalStatus = "approved"
	PromotionApprovalRejected PromotionApprovalStatus = "rejected"
)

type PromotionApproval struct {
	ID          shared.ID               `json:"id"`
	TenantID    shared.ID               `json:"tenant_id"`
	ProjectID   shared.ID               `json:"project_id"`
	PromotionID shared.ID               `json:"promotion_id"`
	ApproverID  shared.ID               `json:"approver_id"`
	Status      PromotionApprovalStatus `json:"status"`
	Comment     string                  `json:"comment"`
	CreatedAt   time.Time               `json:"created_at"`
	UpdatedAt   time.Time               `json:"updated_at"`
}

type BuildSucceededPayload struct {
	BuildRunID          shared.ID   `json:"build_run_id"`
	ApplicationID       shared.ID   `json:"application_id"`
	WorkloadID          shared.ID   `json:"workload_id"`
	WorkloadIDs         []shared.ID `json:"workload_ids"`
	PipelineID          shared.ID   `json:"pipeline_id"`
	PipelineName        string      `json:"pipeline_name"`
	PipelineDisplayName string      `json:"pipeline_display_name"`
	BuildArtifactIDs    []shared.ID `json:"build_artifact_ids"`
	CommitSHA           string      `json:"commit_sha"`
	BuildArtifactID     shared.ID   `json:"build_artifact_id,omitempty"`
	ImageURI            string      `json:"image_uri,omitempty"`
	ImageDigest         string      `json:"image_digest,omitempty"`
}

const ReleaseSourcePipelineArtifact = "pipeline_artifact"

func terminalPromotion(status PromotionStatus) bool {
	return status == PromotionHealthy || status == PromotionFailed || status == PromotionAborted || status == PromotionRejected
}

func isProdStage(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), "prod")
}
