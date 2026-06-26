package delivery

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/shared"
)

const deliverySystemActorID shared.ID = "system"

type Service struct {
	repo          Repository
	builds        BuildQuery
	apps          ApplicationQuery
	workloads     WorkloadQuery
	runtimeStates StageRuntimeStateQuery
	stageSync     StageSync
	clusters      ClusterQuery
	gitops        GitOpsDeploymentCommand
	permission    PermissionChecker
	audit         AuditLogger
	events        EventPublisher
	ids           shared.IDGenerator
	clock         shared.Clock
}

type Options struct {
	Repository             Repository
	BuildQuery             BuildQuery
	ApplicationQuery       ApplicationQuery
	WorkloadQuery          WorkloadQuery
	StageRuntimeStateQuery StageRuntimeStateQuery
	StageSync              StageSync
	ClusterQuery           ClusterQuery
	GitOpsDeployment       GitOpsDeploymentCommand
	PermissionChecker      PermissionChecker
	Audit                  AuditLogger
	EventPublisher         EventPublisher
	IDGenerator            shared.IDGenerator
	Clock                  shared.Clock
}

func NewService(opts Options) *Service {
	audit := opts.Audit
	if audit == nil {
		audit = NoopAuditLogger{}
	}
	events := opts.EventPublisher
	if events == nil {
		events = NoopEventPublisher{}
	}
	ids := opts.IDGenerator
	if ids == nil {
		ids = shared.RandomIDGenerator{}
	}
	clock := opts.Clock
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Service{repo: opts.Repository, builds: opts.BuildQuery, apps: opts.ApplicationQuery, workloads: opts.WorkloadQuery, runtimeStates: opts.StageRuntimeStateQuery, stageSync: opts.StageSync, clusters: opts.ClusterQuery, gitops: opts.GitOpsDeployment, permission: opts.PermissionChecker, audit: audit, events: events, ids: ids, clock: clock}
}

type CreatePromotionInput struct {
	Actor             identityaccess.Subject `json:"actor"`
	FreightID         shared.ID              `json:"freight_id"`
	TargetStageKey    string                 `json:"target_stage_key"`
	TargetClusterIDs  []shared.ID            `json:"target_cluster_ids"`
	NamespaceOverride string                 `json:"namespace_override"`
	Message           string                 `json:"message"`
	AutoPublish       *bool                  `json:"auto_publish,omitempty"`
}

type CreateRollbackPromotionInput struct {
	Actor            identityaccess.Subject `json:"actor"`
	TargetFreightID  shared.ID              `json:"target_freight_id"`
	CurrentFreightID shared.ID              `json:"current_freight_id"`
	TargetStageKey   string                 `json:"target_stage_key"`
	Message          string                 `json:"message"`
}

type ApprovalInput struct {
	Actor       identityaccess.Subject `json:"actor"`
	PromotionID shared.ID              `json:"promotion_id"`
	Comment     string                 `json:"comment"`
}

type SaveDeliveryFlowTemplateStageInput struct {
	Actor                identityaccess.Subject          `json:"actor"`
	TenantID             shared.ID                       `json:"tenant_id"`
	StageKey             string                          `json:"stage_key"`
	NewStageKey          string                          `json:"new_stage_key,omitempty"`
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
}

type ReplaceDeliveryFlowTemplateGraphInput struct {
	Actor            identityaccess.Subject               `json:"actor"`
	TenantID         shared.ID                            `json:"tenant_id"`
	Stages           []SaveDeliveryFlowTemplateStageInput `json:"stages"`
	Edges            []DeliveryFlowTemplateEdgeInput      `json:"edges"`
	DeletedStageKeys []string                             `json:"deleted_stage_keys"`
}

type DeliveryFlowTemplateEdgeInput struct {
	FromStageKey string `json:"from_stage_key"`
	ToStageKey   string `json:"to_stage_key"`
}

type StageTemplateActionInput struct {
	Actor    identityaccess.Subject `json:"actor"`
	TenantID shared.ID              `json:"tenant_id"`
	StageKey string                 `json:"stage_key"`
}

type ReplaceStageClusterBindingsInput struct {
	Actor    identityaccess.Subject     `json:"actor"`
	TenantID shared.ID                  `json:"tenant_id"`
	StageKey string                     `json:"stage_key"`
	Clusters []StageClusterBindingInput `json:"clusters"`
}

type StageClusterBindingInput struct {
	ClusterID   shared.ID `json:"cluster_id"`
	ClusterName string    `json:"cluster_name"`
}

type FreightApprovalInput struct {
	Actor          identityaccess.Subject `json:"actor"`
	FreightID      shared.ID              `json:"freight_id"`
	TargetStageKey string                 `json:"target_stage_key"`
	Decision       FreightApprovalStatus  `json:"decision"`
	Comment        string                 `json:"comment"`
}

type StageVerificationInput struct {
	Actor         identityaccess.Subject  `json:"actor"`
	ApplicationID shared.ID               `json:"application_id"`
	StageKey      string                  `json:"stage_key"`
	FreightID     shared.ID               `json:"freight_id"`
	Status        StageVerificationStatus `json:"status"`
	Comment       string                  `json:"comment"`
	SyncStatus    string                  `json:"sync_status"`
	HealthStatus  string                  `json:"health_status"`
	AgentStatus   string                  `json:"agent_status"`
}

type CreateFreightInput struct {
	Actor             identityaccess.Subject   `json:"actor"`
	ApplicationID     shared.ID                `json:"application_id"`
	Name              string                   `json:"name"`
	Description       string                   `json:"description"`
	SourceFingerprint string                   `json:"source_fingerprint"`
	Items             []CreateFreightItemInput `json:"items"`
}

type ArchiveFreightInput struct {
	Actor     identityaccess.Subject `json:"actor"`
	FreightID shared.ID              `json:"freight_id"`
}

var defaultTemplateStages = []struct {
	key                  string
	displayName          string
	requiresApproval     bool
	requiresVerification bool
	approveRoles         []string
	verifyRoles          []string
}{
	{key: "dev", displayName: "开发", verifyRoles: []string{"developer", "operator"}},
	{key: "test", displayName: "测试", verifyRoles: []string{"developer", "operator"}},
	{key: "staging", displayName: "预发", verifyRoles: []string{"operator"}},
	{key: "prod", displayName: "生产", requiresApproval: true, requiresVerification: true, approveRoles: []string{"prod_approver"}, verifyRoles: []string{"operator", "prod_approver"}},
}

var deliveryFlowColumnColors = []string{"#ED204E", "#FD5352", "#FE7537", "#e78a00", "#DFC546", "#9bce22", "#84DF75", "#1CAC77", "#1bc1a7", "#1DCECA", "#0DAFD3", "#3882EA", "#2D5EDC", "#6380E1", "#7851AA", "#A9499D", "#D0469D", "#E573A2", "#f1619b", "#FE43A3", "#6a7382"}

type CreateFreightItemInput struct {
	WorkloadID      shared.ID       `json:"workload_id"`
	ContainerName   string          `json:"container_name"`
	SourceType      FreightItemType `json:"source_type"`
	ReleaseID       shared.ID       `json:"release_id"`
	BuildArtifactID shared.ID       `json:"build_artifact_id"`
	ImageRef        string          `json:"image_ref"`
}

type FreightCreationContext struct {
	EnabledWorkloads                   []WorkloadRef                  `json:"enabled_workloads"`
	LatestReleasesByWorkload           map[shared.ID]Release          `json:"latest_releases_by_workload"`
	LatestArtifactsByWorkload          map[shared.ID]BuildArtifactRef `json:"latest_artifacts_by_workload"`
	LatestReleasesByWorkloadContainer  map[string]Release             `json:"latest_releases_by_workload_container"`
	LatestArtifactsByWorkloadContainer map[string]BuildArtifactRef    `json:"latest_artifacts_by_workload_container"`
	StageEligibility                   map[shared.ID][]shared.ID      `json:"stage_eligibility"`
	Stages                             []FreightCreationStage         `json:"stages"`
}

type FreightCreationStage struct {
	ID               shared.ID `json:"id"`
	StageKey         string    `json:"stage_key"`
	Name             string    `json:"name"`
	ApprovalRequired bool      `json:"approval_required"`
}

func (s *Service) HandleBuildSucceeded(ctx context.Context, payload BuildSucceededPayload) (Release, error) {
	targets := buildSucceededTargets(payload)
	if payload.BuildRunID.IsZero() || payload.ApplicationID.IsZero() || len(targets) == 0 {
		return Release{}, shared.NewError(shared.CodeInvalidArgument, "build_run_id, application_id and workload_ids are required")
	}
	run, err := s.buildsOrError().GetBuildRun(ctx, payload.BuildRunID)
	if err != nil {
		return Release{}, err
	}
	artifactIDs := payload.BuildArtifactIDs
	if len(artifactIDs) == 0 && !payload.BuildArtifactID.IsZero() {
		artifactIDs = []shared.ID{payload.BuildArtifactID}
	}
	artifacts, err := s.resolveBuildArtifacts(ctx, payload.BuildRunID, artifactIDs)
	if err != nil {
		return Release{}, err
	}
	if run.ApplicationID != payload.ApplicationID {
		return Release{}, shared.NewError(shared.CodeInvalidArgument, "build payload ownership mismatch")
	}
	for _, artifact := range artifacts {
		if artifact.BuildRunID != run.ID || artifact.ApplicationID != run.ApplicationID {
			return Release{}, shared.NewError(shared.CodeInvalidArgument, "build payload ownership mismatch")
		}
	}
	primary := primaryArtifact(artifacts)
	releases := make([]Release, 0, len(targets))
	for _, target := range uniqueWorkloadTargets(targets) {
		if target.WorkloadID.IsZero() {
			return Release{}, shared.NewError(shared.CodeInvalidArgument, "workload_id is required")
		}
		release, err := s.ensureBuildReleaseForWorkload(ctx, payload, run, artifacts, primary, target.WorkloadID, target.ContainerName)
		if err != nil {
			return Release{}, err
		}
		releases = append(releases, release)
	}
	if _, err := s.ensureDefaultFlow(ctx, run.ApplicationID); err != nil {
		return Release{}, err
	}
	if freight, ok, err := s.autoCreateFreightFromBuildSucceeded(ctx, run.ApplicationID); err != nil {
		_ = s.audit.Log(ctx, AuditEvent{ActorID: deliverySystemActorID, Action: "freight.auto_create", ResourceType: "application", ResourceID: run.ApplicationID, Result: "failed", Summary: "构建成功后自动创建 Freight 失败", Details: map[string]string{"reason": err.Error()}, OccurredAt: s.clock.Now()})
	} else if ok {
		if _, err := s.createManualPromotionsForEligibleStages(ctx, run.ApplicationID, freight.ID, deliverySystemActorID, "build_succeeded"); err != nil {
			_ = s.audit.Log(ctx, AuditEvent{ActorID: deliverySystemActorID, Action: "promotion.auto_create", ResourceType: "freight", ResourceID: freight.ID, Result: "failed", Summary: "构建成功后自动创建手动 Promotion 失败", Details: map[string]string{"reason": err.Error()}, OccurredAt: s.clock.Now()})
		}
	}
	return releases[0], nil
}

func (s *Service) ensureBuildReleaseForWorkload(ctx context.Context, payload BuildSucceededPayload, run BuildRunRef, artifacts []BuildArtifactRef, primary BuildArtifactRef, workloadID shared.ID, targetContainerName string) (Release, error) {
	containerName := normalizeContainerName(firstNonEmpty(targetContainerName, payload.ContainerName, primary.ContainerName))
	if existing, err := s.repo.FindReleaseByBuildRunWorkloadAndContainer(ctx, run.ID, workloadID, containerName); err == nil {
		return existing, nil
	} else if shared.CodeOf(err) != shared.CodeNotFound {
		return Release{}, err
	}
	for _, artifact := range artifacts {
		if !artifact.WorkloadID.IsZero() && artifact.WorkloadID != workloadID {
			return Release{}, shared.NewError(shared.CodeInvalidArgument, "build payload ownership mismatch")
		}
	}
	now := s.clock.Now()
	bundleID, err := s.ids.NewID("image_bundle")
	if err != nil {
		return Release{}, err
	}
	bundle := ImageBundle{ID: bundleID, TenantID: run.TenantID, ProjectID: run.ProjectID, ApplicationID: run.ApplicationID, WorkloadID: workloadID, ContainerName: containerName, BuildRunID: run.ID, CommitSHA: firstNonEmpty(payload.CommitSHA, run.CommitSHA), CreatedAt: now}
	if err := s.repo.CreateImageBundle(ctx, bundle); err != nil {
		return Release{}, err
	}
	for _, artifact := range artifacts {
		imageID, err := s.ids.NewID("image_bundle_image")
		if err != nil {
			return Release{}, err
		}
		repository, tag := splitImageRepositoryTag(artifact.URI)
		image := ImageBundleImage{
			ID:                     imageID,
			BundleID:               bundle.ID,
			BuildArtifactID:        artifact.ID,
			RuntimeEnvironmentID:   shared.ID(artifact.Metadata["runtime_environment_id"]),
			RuntimeEnvironmentName: artifact.Metadata["runtime_environment_name"],
			ContainerName:          containerName,
			URI:                    artifact.URI,
			ImageRepository:        repository,
			ImageTag:               tag,
			Digest:                 artifact.Digest,
			SelectorLabels:         cleanStringMap(artifact.SelectorLabels),
			IsPrimary:              artifact.IsPrimary || artifact.ID == primary.ID,
			CreatedAt:              now,
		}
		if err := s.repo.CreateImageBundleImage(ctx, image); err != nil {
			return Release{}, err
		}
	}
	releaseID, err := s.ids.NewID("release")
	if err != nil {
		return Release{}, err
	}
	commit := firstNonEmpty(payload.CommitSHA, run.CommitSHA)
	imageURI := firstNonEmpty(payload.ImageURI, primary.URI)
	imageDigest := firstNonEmpty(payload.ImageDigest, primary.Digest)
	pipelineID := run.PipelineID
	if pipelineID.IsZero() {
		pipelineID = payload.PipelineID
	}
	pipelineName := firstNonEmpty(run.PipelineName, payload.PipelineName)
	pipelineDisplayName := firstNonEmpty(run.PipelineDisplayName, payload.PipelineDisplayName)
	imageRepository, imageTag := splitImageRepositoryTag(imageURI)
	release := Release{ID: releaseID, TenantID: run.TenantID, ProjectID: run.ProjectID, ApplicationID: run.ApplicationID, WorkloadID: workloadID, ContainerName: containerName, PipelineID: pipelineID, PipelineName: pipelineName, PipelineDisplayName: pipelineDisplayName, BuildRunID: run.ID, BuildArtifactID: primary.ID, ImageBundleID: bundle.ID, Version: releaseVersion(commit, run.ID), CommitSHA: commit, ImageURI: imageURI, ImageRepository: imageRepository, ImageTag: imageTag, ImageDigest: imageDigest, SourceType: ReleaseSourcePipelineArtifact, Status: ReleaseReady, CreatedAt: now}
	if err := s.repo.CreateRelease(ctx, release); err != nil {
		return Release{}, err
	}
	return release, nil
}

func uniqueIDs(ids []shared.ID) []shared.ID {
	out := make([]shared.ID, 0, len(ids))
	seen := map[shared.ID]struct{}{}
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func buildSucceededTargets(payload BuildSucceededPayload) []WorkloadTarget {
	if len(payload.WorkloadTargets) > 0 {
		return payload.WorkloadTargets
	}
	containerName := normalizeContainerName(payload.ContainerName)
	out := make([]WorkloadTarget, 0, len(payload.WorkloadIDs)+1)
	if !payload.WorkloadID.IsZero() {
		out = append(out, WorkloadTarget{WorkloadID: payload.WorkloadID, ContainerName: containerName})
	}
	for _, workloadID := range payload.WorkloadIDs {
		out = append(out, WorkloadTarget{WorkloadID: workloadID, ContainerName: containerName})
	}
	return out
}

func uniqueWorkloadTargets(targets []WorkloadTarget) []WorkloadTarget {
	out := make([]WorkloadTarget, 0, len(targets))
	seen := map[string]struct{}{}
	for _, target := range targets {
		if target.WorkloadID.IsZero() {
			continue
		}
		containerName := normalizeContainerName(target.ContainerName)
		key := freightTargetKey(target.WorkloadID, containerName)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, WorkloadTarget{WorkloadID: target.WorkloadID, ContainerName: containerName})
	}
	return out
}

func (s *Service) CreatePromotion(ctx context.Context, input CreatePromotionInput) (Promotion, error) {
	freight, err := s.repo.GetFreight(ctx, input.FreightID)
	if err != nil {
		return Promotion{}, err
	}
	if freight.Status == FreightArchived {
		return Promotion{}, shared.NewError(shared.CodeFailedPrecondition, "freight is archived")
	}
	stageKey := normalizeStageKey(input.TargetStageKey)
	app, stage, targetClusters, err := s.validateStagePromotionTarget(ctx, freight.ApplicationID, stageKey, input.TargetClusterIDs, input.NamespaceOverride)
	namespaceOverride := ""
	if strings.TrimSpace(input.NamespaceOverride) != "" {
		namespaceOverride = normalizeKubernetesNamespace(input.NamespaceOverride)
	}
	if err != nil {
		return Promotion{}, err
	}
	if err := s.check(ctx, input.Actor, app, "deployment:create"); err != nil {
		return Promotion{}, err
	}
	if err := s.validateFreightComplete(ctx, freight.ApplicationID, freight.ID); err != nil {
		return Promotion{}, err
	}
	autoPublish := true
	if input.AutoPublish != nil {
		autoPublish = *input.AutoPublish
	}
	if existing, ok, err := s.findPendingPromotion(ctx, freight.ApplicationID, freight.ID, stage.ID, stageKey); err != nil {
		return Promotion{}, err
	} else if ok {
		if autoPublish && canPublishExistingPromotion(existing.Status) {
			return s.applyPromotion(ctx, existing, targetClusters)
		}
		return existing, nil
	}
	if err := s.validateStageOrder(ctx, freight, stage); err != nil {
		return Promotion{}, err
	}
	promotion, err := s.newPromotion(ctx, freight, stage, stageKey, namespaceOverride, input.Actor.ID, strings.TrimSpace(input.Message), false, "", autoPublish)
	if err != nil {
		return Promotion{}, err
	}
	if promotion.Status == PromotionPendingApproval {
		return s.createApproval(ctx, promotion)
	}
	if !promotion.AutoPublish {
		return promotion, nil
	}
	return s.applyPromotion(ctx, promotion, targetClusters)
}

func (s *Service) findPendingPromotion(ctx context.Context, applicationID shared.ID, freightID shared.ID, stageID shared.ID, stageKey string) (Promotion, bool, error) {
	promotions, err := s.repo.ListPromotionsByApplication(ctx, applicationID, shared.PageRequest{Page: 1, PageSize: 1000})
	if err != nil {
		return Promotion{}, false, err
	}
	stageKey = normalizeStageKey(stageKey)
	for _, promotion := range promotions.Items {
		if promotion.FreightID != freightID {
			continue
		}
		promotionStageKey := normalizeStageKey(promotion.TargetStageKey)
		sameStage := promotionStageKey != "" && promotionStageKey == stageKey
		if !sameStage && !stageID.IsZero() && promotion.TargetStageID == stageID {
			sameStage = true
		}
		if sameStage && pendingPromotion(promotion.Status) {
			return promotion, true, nil
		}
	}
	return Promotion{}, false, nil
}

func pendingPromotion(status PromotionStatus) bool {
	switch status {
	case PromotionCreated, PromotionPendingApproval, PromotionApproved, PromotionManifestUpdating, PromotionSyncing:
		return true
	default:
		return false
	}
}

func canPublishExistingPromotion(status PromotionStatus) bool {
	return status == PromotionCreated || status == PromotionApproved
}

func (s *Service) CreateFreight(ctx context.Context, input CreateFreightInput) (Freight, error) {
	return s.createFreight(ctx, input, true)
}

func (s *Service) createFreight(ctx context.Context, input CreateFreightInput, checkPermission bool) (Freight, error) {
	app, err := s.appsOrError().GetApplication(ctx, input.ApplicationID)
	if err != nil {
		return Freight{}, err
	}
	if checkPermission {
		if err := s.check(ctx, input.Actor, app, "freight:create"); err != nil {
			return Freight{}, err
		}
	}
	workloads, err := s.workloadsOrError().ListEnabledWorkloads(ctx, app.ID)
	if err != nil {
		return Freight{}, err
	}
	if len(workloads) == 0 {
		return Freight{}, shared.NewError(shared.CodeFailedPrecondition, "enabled workload is required")
	}
	required := freightTargets(workloads)
	if len(input.Items) != len(required) {
		return Freight{}, shared.NewError(shared.CodeFailedPrecondition, "freight must include every enabled workload container")
	}
	enabled := map[shared.ID]WorkloadRef{}
	for _, workload := range workloads {
		enabled[workload.ID] = workload
	}
	seen := map[string]struct{}{}
	now := s.clock.Now()
	freightID, err := s.ids.NewID("freight")
	if err != nil {
		return Freight{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = freightDisplayName(now)
	}
	freight := Freight{ID: freightID, TenantID: app.TenantID, ProjectID: app.ProjectID, ApplicationID: app.ID, Name: name, SourceFingerprint: strings.TrimSpace(input.SourceFingerprint), Status: FreightAvailable, CreatedAt: now}
	items := make([]FreightItem, 0, len(input.Items))
	tagRisk := false
	for _, itemInput := range input.Items {
		if itemInput.WorkloadID.IsZero() {
			return Freight{}, shared.NewError(shared.CodeInvalidArgument, "workload_id is required")
		}
		workload, ok := enabled[itemInput.WorkloadID]
		if !ok {
			return Freight{}, shared.NewError(shared.CodeInvalidArgument, "freight item workload is not enabled")
		}
		containerName := normalizeContainerName(itemInput.ContainerName)
		targetKey := freightTargetKey(itemInput.WorkloadID, containerName)
		if _, ok := required[targetKey]; !ok {
			return Freight{}, shared.NewError(shared.CodeInvalidArgument, "freight item workload container is not enabled")
		}
		if _, ok := seen[targetKey]; ok {
			return Freight{}, shared.NewError(shared.CodeConflict, "freight item workload container is duplicated")
		}
		seen[targetKey] = struct{}{}
		itemID, err := s.ids.NewID("freight_item")
		if err != nil {
			return Freight{}, err
		}
		item := FreightItem{ID: itemID, TenantID: app.TenantID, ProjectID: app.ProjectID, FreightID: freight.ID, ApplicationID: app.ID, WorkloadID: workload.ID, ContainerName: containerName, SourceType: itemInput.SourceType, Type: itemInput.SourceType, Name: freightItemDisplayName(workload, containerName), CreatedAt: now}
		switch itemInput.SourceType {
		case FreightItemPipelineArtifact, "":
			pipelineItem, err := s.pipelineFreightItem(ctx, item, itemInput)
			if err != nil {
				return Freight{}, err
			}
			items = append(items, pipelineItem)
		case FreightItemCustomImage:
			image, err := parseCustomImageRef(itemInput.ImageRef)
			if err != nil {
				return Freight{}, err
			}
			item.URI = image.ref
			item.ImageRef = image.ref
			item.ImageRepository = image.repository
			item.ImageTag = image.tag
			item.Digest = image.digest
			if image.digest == "" {
				tagRisk = true
			}
			items = append(items, item)
		default:
			return Freight{}, shared.NewError(shared.CodeInvalidArgument, "freight item source_type is not supported")
		}
	}
	if len(seen) != len(required) {
		return Freight{}, shared.NewError(shared.CodeFailedPrecondition, "freight must include every enabled workload container")
	}
	if err := s.repo.CreateFreight(ctx, freight); err != nil {
		return Freight{}, err
	}
	for _, item := range items {
		if err := s.repo.CreateFreightItem(ctx, item); err != nil {
			return Freight{}, err
		}
	}
	details := map[string]string{"item_count": fmt.Sprintf("%d", len(items))}
	if tagRisk {
		details["custom_image_tag_risk"] = "true"
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "freight.create", ResourceType: "freight", ResourceID: freight.ID, Result: "succeeded", Summary: "创建可交付变更包", Details: details, OccurredAt: now})
	if tagRisk {
		_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "freight.custom_image_risk", ResourceType: "freight", ResourceID: freight.ID, Result: "succeeded", Summary: "记录自定义镜像 tag 漂移风险", Details: map[string]string{"custom_image_tag_risk": "true"}, OccurredAt: now})
	}
	_ = s.publish(ctx, "FreightCreated", now, map[string]any{"freight_id": freight.ID, "application_id": app.ID})
	return freight, nil
}

func (s *Service) autoCreateFreightFromBuildSucceeded(ctx context.Context, applicationID shared.ID) (Freight, bool, error) {
	workloads, err := s.workloadsOrError().ListEnabledWorkloads(ctx, applicationID)
	if err != nil {
		return Freight{}, false, err
	}
	required := freightTargets(workloads)
	if len(required) == 0 {
		return Freight{}, false, shared.NewError(shared.CodeFailedPrecondition, "enabled workload is required")
	}
	releases, err := s.repo.ListReleasesByApplication(ctx, applicationID, shared.PageRequest{Page: 1, PageSize: 1000})
	if err != nil {
		return Freight{}, false, err
	}
	latest := map[string]Release{}
	for _, release := range releases.Items {
		key := freightTargetKey(release.WorkloadID, release.ContainerName)
		if _, ok := required[key]; !ok {
			continue
		}
		if _, ok := latest[key]; ok {
			continue
		}
		latest[key] = release
	}
	inputItems := make([]CreateFreightItemInput, 0, len(required))
	releaseByKey := map[string]Release{}
	missing := make([]string, 0)
	for key := range required {
		release, ok := latest[key]
		if !ok {
			missing = append(missing, key)
			continue
		}
		releaseByKey[key] = release
		inputItems = append(inputItems, CreateFreightItemInput{
			WorkloadID:    release.WorkloadID,
			ContainerName: release.ContainerName,
			SourceType:    FreightItemPipelineArtifact,
			ReleaseID:     release.ID,
		})
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return Freight{}, false, shared.NewError(shared.CodeFailedPrecondition, "missing successful build artifact for "+strings.Join(missing, ", "))
	}
	sort.Slice(inputItems, func(i, j int) bool {
		return freightTargetKey(inputItems[i].WorkloadID, inputItems[i].ContainerName) < freightTargetKey(inputItems[j].WorkloadID, inputItems[j].ContainerName)
	})
	if existing, ok, err := s.findFreightByReleaseSet(ctx, applicationID, releaseByKey); err != nil {
		return Freight{}, false, err
	} else if ok {
		return existing, true, nil
	}
	freight, err := s.createFreight(ctx, CreateFreightInput{
		Actor:             identityaccess.Subject{Type: identityaccess.SubjectServiceAccount, ID: deliverySystemActorID},
		ApplicationID:     applicationID,
		SourceFingerprint: autoFreightSourceFingerprint(releaseByKey),
		Items:             inputItems,
	}, false)
	if err != nil {
		return Freight{}, false, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: deliverySystemActorID, Action: "freight.auto_create", ResourceType: "freight", ResourceID: freight.ID, Result: "succeeded", Summary: "构建成功后自动创建 Freight", Details: map[string]string{"item_count": fmt.Sprintf("%d", len(inputItems))}, OccurredAt: s.clock.Now()})
	return freight, true, nil
}

func (s *Service) findFreightByReleaseSet(ctx context.Context, applicationID shared.ID, releases map[string]Release) (Freight, bool, error) {
	result, err := s.repo.ListFreightsByApplication(ctx, applicationID, shared.PageRequest{Page: 1, PageSize: 1000})
	if err != nil {
		return Freight{}, false, err
	}
	expected := map[string]shared.ID{}
	for key, release := range releases {
		expected[key] = release.BuildArtifactID
	}
	for _, freight := range result.Items {
		items, err := s.repo.ListFreightItems(ctx, freight.ID)
		if err != nil {
			return Freight{}, false, err
		}
		if len(items) != len(expected) {
			continue
		}
		matched := true
		for _, item := range items {
			key := freightTargetKey(item.WorkloadID, item.ContainerName)
			if expected[key].IsZero() || item.BuildArtifactID != expected[key] {
				matched = false
				break
			}
		}
		if matched {
			return freight, true, nil
		}
	}
	return Freight{}, false, nil
}

func autoFreightSourceFingerprint(releases map[string]Release) string {
	parts := make([]string, 0, len(releases))
	for key, release := range releases {
		parts = append(parts, fmt.Sprintf("%s:pipeline:%s:%s", key, release.BuildArtifactID, firstNonEmpty(release.ImageTag, release.Version)))
	}
	sort.Strings(parts)
	return strings.Join(parts, "|")
}

func freightDisplayName(t time.Time) string {
	return t.Format("20060102-150405")
}

func (s *Service) GetDeliveryFlowTemplate(ctx context.Context, tenantID shared.ID) (DeliveryFlowTemplate, error) {
	return s.ensureDeliveryFlowTemplate(ctx, tenantID)
}

func (s *Service) ReplaceDeliveryFlowTemplateGraph(ctx context.Context, input ReplaceDeliveryFlowTemplateGraphInput) (DeliveryFlowTemplate, error) {
	if input.TenantID.IsZero() {
		return DeliveryFlowTemplate{}, shared.NewError(shared.CodeInvalidArgument, "tenant_id is required")
	}
	if err := s.check(ctx, input.Actor, ApplicationRef{TenantID: input.TenantID}, "tenant:update"); err != nil {
		return DeliveryFlowTemplate{}, err
	}
	if len(input.Stages) == 0 {
		return DeliveryFlowTemplate{}, shared.NewError(shared.CodeInvalidArgument, "stage is required")
	}
	template, err := s.ensureDeliveryFlowTemplate(ctx, input.TenantID)
	if err != nil {
		return DeliveryFlowTemplate{}, err
	}
	if err := validateTemplateGraphInput(input.Stages, input.Edges); err != nil {
		return DeliveryFlowTemplate{}, err
	}
	now := s.clock.Now()
	stages := make([]DeliveryFlowTemplateStage, 0, len(input.Stages))
	stageByKey := map[string]DeliveryFlowTemplateStage{}
	for idx, stageInput := range input.Stages {
		stageKey := normalizeStageKey(stageInput.StageKey)
		if stageKey == "" {
			return DeliveryFlowTemplate{}, shared.NewError(shared.CodeInvalidArgument, "stage_key is required")
		}
		if _, ok := stageByKey[stageKey]; ok {
			return DeliveryFlowTemplate{}, shared.NewError(shared.CodeConflict, "stage is duplicated")
		}
		status := stageInput.Status
		if status == "" {
			status = DeliveryFlowTemplateStageEnabled
		}
		if status != DeliveryFlowTemplateStageEnabled && status != DeliveryFlowTemplateStageDisabled {
			return DeliveryFlowTemplate{}, shared.NewError(shared.CodeInvalidArgument, "stage status is not supported")
		}
		order := stageInput.Order
		if order <= 0 {
			order = idx + 1
		}
		stage := DeliveryFlowTemplateStage{TenantID: input.TenantID, TemplateID: template.ID, StageKey: stageKey, DisplayName: firstNonEmpty(strings.TrimSpace(stageInput.DisplayName), stageKey), Color: colorForStageInput(stageInput.Color, stageInput.LayoutColumn), Order: order, LayoutColumn: stageInput.LayoutColumn, LayoutRow: stageInput.LayoutRow, Status: status, RequiresApproval: stageInput.RequiresApproval, RequiresVerification: stageInput.RequiresVerification, ApproveRoles: cleanRoles(stageInput.ApproveRoles), VerifyRoles: cleanRoles(stageInput.VerifyRoles), UpdatedAt: now}
		if existing, err := s.repo.FindDeliveryFlowTemplateStage(ctx, input.TenantID, stageKey); err == nil {
			stage.ID = existing.ID
			stage.CreatedAt = existing.CreatedAt
			if err := s.repo.UpdateDeliveryFlowTemplateStage(ctx, stage); err != nil {
				if shared.CodeOf(err) != shared.CodeNotFound {
					return DeliveryFlowTemplate{}, err
				}
				id, err := s.ids.NewID("delivery_flow_stage_template")
				if err != nil {
					return DeliveryFlowTemplate{}, err
				}
				stage.ID = id
				stage.CreatedAt = now
				if err := s.repo.CreateDeliveryFlowTemplateStage(ctx, stage); err != nil {
					return DeliveryFlowTemplate{}, err
				}
			}
		} else if shared.CodeOf(err) == shared.CodeNotFound {
			id, err := s.ids.NewID("delivery_flow_stage_template")
			if err != nil {
				return DeliveryFlowTemplate{}, err
			}
			stage.ID = id
			stage.CreatedAt = now
			if err := s.repo.CreateDeliveryFlowTemplateStage(ctx, stage); err != nil {
				return DeliveryFlowTemplate{}, err
			}
		} else {
			return DeliveryFlowTemplate{}, err
		}
		stages = append(stages, stage)
		stageByKey[stageKey] = stage
	}
	edges, err := s.buildTemplateEdges(input.TenantID, template.ID, input.Edges, stageByKey, now)
	if err != nil {
		return DeliveryFlowTemplate{}, err
	}
	if err := s.repo.ReplaceDeliveryFlowTemplateEdges(ctx, template.ID, edges); err != nil {
		return DeliveryFlowTemplate{}, err
	}
	payloadStageKeys := map[string]struct{}{}
	for _, stage := range stages {
		payloadStageKeys[stage.StageKey] = struct{}{}
	}
	for _, rawStageKey := range input.DeletedStageKeys {
		stageKey := normalizeStageKey(rawStageKey)
		if stageKey == "" {
			continue
		}
		if _, stillPresent := payloadStageKeys[stageKey]; stillPresent {
			continue
		}
		if err := s.repo.DeleteDeliveryFlowTemplateStage(ctx, input.TenantID, stageKey); err != nil && shared.CodeOf(err) != shared.CodeNotFound {
			return DeliveryFlowTemplate{}, err
		}
	}
	stageKeys := make([]string, 0, len(stages))
	for _, stage := range stages {
		stageKeys = append(stageKeys, stage.StageKey)
	}
	if s.stageSync != nil {
		if err := s.stageSync.SyncApplicationStages(ctx, SyncApplicationStagesInput{TenantID: input.TenantID, StageKeys: stageKeys}); err != nil {
			return DeliveryFlowTemplate{}, err
		}
	}
	saved, err := s.ensureDeliveryFlowTemplate(ctx, input.TenantID)
	if err != nil {
		return DeliveryFlowTemplate{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "delivery_flow_template.graph.replace", ResourceType: "delivery_flow_template", ResourceID: template.ID, Result: "succeeded", Summary: "保存交付流 DAG 模板", Details: map[string]string{"stage_count": fmt.Sprintf("%d", len(saved.Stages)), "edge_count": fmt.Sprintf("%d", len(saved.Edges))}, OccurredAt: now})
	return saved, nil
}

func (s *Service) SaveDeliveryFlowTemplateStage(ctx context.Context, input SaveDeliveryFlowTemplateStageInput) (DeliveryFlowTemplateStage, error) {
	stageKey := normalizeStageKey(input.StageKey)
	if input.TenantID.IsZero() || stageKey == "" {
		return DeliveryFlowTemplateStage{}, shared.NewError(shared.CodeInvalidArgument, "tenant_id and stage_key are required")
	}
	if newKey := normalizeStageKey(input.NewStageKey); newKey != "" && newKey != stageKey {
		return DeliveryFlowTemplateStage{}, shared.NewError(shared.CodeInvalidArgument, "stage key cannot be changed")
	}
	if err := s.check(ctx, input.Actor, ApplicationRef{TenantID: input.TenantID}, "tenant:update"); err != nil {
		return DeliveryFlowTemplateStage{}, err
	}
	template, err := s.ensureDeliveryFlowTemplate(ctx, input.TenantID)
	if err != nil {
		return DeliveryFlowTemplateStage{}, err
	}
	now := s.clock.Now()
	status := input.Status
	if status == "" {
		status = DeliveryFlowTemplateStageEnabled
	}
	if status != DeliveryFlowTemplateStageEnabled && status != DeliveryFlowTemplateStageDisabled {
		return DeliveryFlowTemplateStage{}, shared.NewError(shared.CodeInvalidArgument, "stage status is not supported")
	}
	stage := DeliveryFlowTemplateStage{
		TenantID:             input.TenantID,
		TemplateID:           template.ID,
		StageKey:             stageKey,
		DisplayName:          firstNonEmpty(strings.TrimSpace(input.DisplayName), stageKey),
		Color:                colorForStageInput(input.Color, input.LayoutColumn),
		Order:                input.Order,
		LayoutColumn:         input.LayoutColumn,
		LayoutRow:            input.LayoutRow,
		Status:               status,
		RequiresApproval:     input.RequiresApproval,
		RequiresVerification: input.RequiresVerification,
		ApproveRoles:         cleanRoles(input.ApproveRoles),
		VerifyRoles:          cleanRoles(input.VerifyRoles),
		UpdatedAt:            now,
	}
	if stage.Order <= 0 {
		stage.Order = len(template.Stages) + 1
	}
	existing, err := s.repo.FindDeliveryFlowTemplateStage(ctx, input.TenantID, stageKey)
	if err == nil {
		stage.ID = existing.ID
		stage.CreatedAt = existing.CreatedAt
		if input.LayoutColumn == 0 && input.LayoutRow == 0 {
			stage.LayoutColumn = existing.LayoutColumn
			stage.LayoutRow = existing.LayoutRow
			stage.Color = colorForStageInput(input.Color, existing.LayoutColumn)
		}
		if err := s.repo.UpdateDeliveryFlowTemplateStage(ctx, stage); err != nil {
			return DeliveryFlowTemplateStage{}, err
		}
		_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "delivery_flow_template.stage.update", ResourceType: "delivery_flow_template_stage", ResourceID: stage.ID, Result: "succeeded", Summary: "更新交付流 Stage 模板", OccurredAt: now})
		return stage, nil
	}
	if shared.CodeOf(err) != shared.CodeNotFound {
		return DeliveryFlowTemplateStage{}, err
	}
	id, err := s.ids.NewID("delivery_flow_stage_template")
	if err != nil {
		return DeliveryFlowTemplateStage{}, err
	}
	stage.ID = id
	stage.CreatedAt = now
	if input.LayoutColumn == 0 && input.LayoutRow == 0 {
		stage.LayoutColumn = len(template.Stages)
		stage.Color = colorForStageInput(input.Color, stage.LayoutColumn)
	}
	if err := s.repo.CreateDeliveryFlowTemplateStage(ctx, stage); err != nil {
		return DeliveryFlowTemplateStage{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "delivery_flow_template.stage.create", ResourceType: "delivery_flow_template_stage", ResourceID: stage.ID, Result: "succeeded", Summary: "创建交付流 Stage 模板", OccurredAt: now})
	return stage, nil
}

func (s *Service) DeleteDeliveryFlowTemplateStage(ctx context.Context, input StageTemplateActionInput) (DeliveryFlowTemplateStage, error) {
	stage, err := s.repo.FindDeliveryFlowTemplateStage(ctx, input.TenantID, normalizeStageKey(input.StageKey))
	if err != nil {
		return DeliveryFlowTemplateStage{}, err
	}
	if err := s.check(ctx, input.Actor, ApplicationRef{TenantID: input.TenantID}, "tenant:update"); err != nil {
		return DeliveryFlowTemplateStage{}, err
	}
	now := s.clock.Now()
	if err := s.repo.DeleteDeliveryFlowTemplateStage(ctx, input.TenantID, stage.StageKey); err != nil {
		return DeliveryFlowTemplateStage{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "delivery_flow_template.stage.delete", ResourceType: "delivery_flow_template_stage", ResourceID: stage.ID, Result: "succeeded", Summary: "删除交付流 Stage 模板", OccurredAt: now})
	return stage, nil
}

func (s *Service) ReplaceStageClusterBindings(ctx context.Context, input ReplaceStageClusterBindingsInput) ([]StageClusterBinding, error) {
	stageKey := normalizeStageKey(input.StageKey)
	if input.TenantID.IsZero() || stageKey == "" {
		return nil, shared.NewError(shared.CodeInvalidArgument, "tenant_id and stage_key are required")
	}
	if err := s.check(ctx, input.Actor, ApplicationRef{TenantID: input.TenantID}, "cluster:manage"); err != nil {
		return nil, err
	}
	if _, err := s.ensureDeliveryFlowTemplate(ctx, input.TenantID); err != nil {
		return nil, err
	}
	stage, err := s.repo.FindDeliveryFlowTemplateStage(ctx, input.TenantID, stageKey)
	if err != nil {
		return nil, err
	}
	if stage.Status == DeliveryFlowTemplateStageDisabled {
		return nil, shared.NewError(shared.CodeFailedPrecondition, "disabled stage cannot bind clusters")
	}
	if len(input.Clusters) > 1 {
		return nil, shared.NewError(shared.CodeInvalidArgument, "stage can bind at most one cluster")
	}
	now := s.clock.Now()
	bindings := make([]StageClusterBinding, 0, len(input.Clusters))
	seen := map[shared.ID]struct{}{}
	for _, cluster := range input.Clusters {
		if cluster.ClusterID.IsZero() || strings.TrimSpace(cluster.ClusterName) == "" {
			return nil, shared.NewError(shared.CodeInvalidArgument, "cluster_id and cluster_name are required")
		}
		if _, ok := seen[cluster.ClusterID]; ok {
			return nil, shared.NewError(shared.CodeConflict, "cluster is duplicated")
		}
		seen[cluster.ClusterID] = struct{}{}
		id, err := s.ids.NewID("stage_cluster_binding")
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, StageClusterBinding{ID: id, TenantID: input.TenantID, StageKey: stageKey, ClusterID: cluster.ClusterID, ClusterName: strings.TrimSpace(cluster.ClusterName), Status: StageClusterBindingActive, CreatedAt: now, UpdatedAt: now})
	}
	if err := s.repo.ReplaceStageClusterBindings(ctx, input.TenantID, stageKey, bindings); err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "stage_cluster_binding.replace", ResourceType: "delivery_flow_template_stage", ResourceID: stage.ID, Result: "succeeded", Summary: "保存 Stage 集群池绑定", Details: map[string]string{"stage_key": stageKey, "cluster_count": fmt.Sprintf("%d", len(bindings))}, OccurredAt: now})
	return bindings, nil
}

func (s *Service) ListStageClusterBindings(ctx context.Context, tenantID shared.ID, stageKey string) ([]StageClusterBinding, error) {
	normalized := normalizeStageKey(stageKey)
	if tenantID.IsZero() || normalized == "" {
		return nil, shared.NewError(shared.CodeInvalidArgument, "tenant_id and stage_key are required")
	}
	if _, err := s.ensureDeliveryFlowTemplate(ctx, tenantID); err != nil {
		return nil, err
	}
	if _, err := s.repo.FindDeliveryFlowTemplateStage(ctx, tenantID, normalized); err != nil {
		return nil, err
	}
	return s.repo.ListStageClusterBindings(ctx, tenantID, normalized)
}

func (s *Service) ListAppStages(ctx context.Context, applicationID shared.ID) ([]AppStage, error) {
	app, err := s.appsOrError().GetApplication(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	template, err := s.ensureDeliveryFlowTemplate(ctx, app.TenantID)
	if err != nil {
		return nil, err
	}
	currentFreights, err := s.currentFreightsByStage(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	runtimeStates := map[string]StageRuntimeState{}
	if s.runtimeStates != nil {
		runtimeStates, err = s.runtimeStates.ListStageRuntimeStates(ctx, applicationID)
		if err != nil {
			return nil, err
		}
	}
	out := make([]AppStage, 0, len(template.Stages))
	upstreams := map[string][]string{}
	downstreams := map[string][]string{}
	for _, edge := range template.Edges {
		upstreams[edge.ToStageKey] = append(upstreams[edge.ToStageKey], edge.FromStageKey)
		downstreams[edge.FromStageKey] = append(downstreams[edge.FromStageKey], edge.ToStageKey)
	}
	for _, stage := range template.Stages {
		bindings, err := s.repo.ListStageClusterBindings(ctx, app.TenantID, stage.StageKey)
		if err != nil {
			return nil, err
		}
		active := 0
		for _, binding := range bindings {
			if binding.Status == StageClusterBindingActive {
				active++
			}
		}
		var boundClusterID shared.ID
		boundClusterName := ""
		for _, binding := range bindings {
			if binding.Status == StageClusterBindingActive {
				boundClusterID = binding.ClusterID
				boundClusterName = binding.ClusterName
				break
			}
		}
		currentFreight := currentFreights[stage.StageKey]
		runtimeState := runtimeStates[stage.StageKey]
		var verificationStatus StageVerificationStatus
		if stage.RequiresVerification && currentFreight.ID != "" {
			if v, err := s.repo.FindStageVerification(ctx, applicationID, stage.StageKey, currentFreight.ID); err == nil {
				verificationStatus = v.Status
			}
		}
		out = append(out, AppStage{
			TenantID:              app.TenantID,
			ProjectID:             app.ProjectID,
			ApplicationID:         app.ID,
			StageKey:              stage.StageKey,
			DisplayName:           stage.DisplayName,
			Color:                 stage.Color,
			Order:                 stage.Order,
			LayoutColumn:          stage.LayoutColumn,
			LayoutRow:             stage.LayoutRow,
			Status:                stage.Status,
			RequiresApproval:      stage.RequiresApproval,
			RequiresVerification:  stage.RequiresVerification,
			ApproveRoles:          append([]string(nil), stage.ApproveRoles...),
			VerifyRoles:           append([]string(nil), stage.VerifyRoles...),
			ClusterPoolSize:       active,
			BoundClusterID:        boundClusterID,
			BoundClusterName:      boundClusterName,
			CurrentFreightID:      currentFreight.ID,
			CurrentFreightVersion: currentFreight.Name,
			SyncStatus:            runtimeState.SyncStatus,
			HealthStatus:          runtimeState.HealthStatus,
			OperationState:        runtimeState.OperationState,
			RuntimeMessage:        runtimeState.Message,
			UpstreamStageKeys:     append([]string(nil), upstreams[stage.StageKey]...),
			DownstreamStageKeys:   append([]string(nil), downstreams[stage.StageKey]...),
			VerificationStatus:    verificationStatus,
		})
	}
	return out, nil
}

func (s *Service) currentFreightsByStage(ctx context.Context, applicationID shared.ID) (map[string]Freight, error) {
	out := map[string]Freight{}
	promotionAt := map[string]time.Time{}
	for page := 1; ; page++ {
		result, err := s.repo.ListPromotionsByApplication(ctx, applicationID, shared.PageRequest{Page: page, PageSize: 100})
		if err != nil {
			return nil, err
		}
		for _, promotion := range result.Items {
			stageKey := normalizeStageKey(promotion.TargetStageKey)
			if stageKey == "" || !promotionStagePassed(promotion.Status) {
				continue
			}
			if currentPromotionTime, ok := promotionAt[stageKey]; ok && !promotion.CreatedAt.After(currentPromotionTime) {
				continue
			}
			freight, err := s.repo.GetFreight(ctx, promotion.FreightID)
			if err != nil {
				if shared.CodeOf(err) == shared.CodeNotFound {
					continue
				}
				return nil, err
			}
			out[stageKey] = freight
			promotionAt[stageKey] = promotion.CreatedAt
		}
		if int64(page*result.PageSize) >= result.Total {
			break
		}
	}
	return out, nil
}

func promotionStagePassed(status PromotionStatus) bool {
	return status == PromotionManifestUpdated || status == PromotionSyncing || status == PromotionHealthy
}

func (s *Service) CompleteFreightApproval(ctx context.Context, input FreightApprovalInput) (FreightApproval, error) {
	freight, err := s.repo.GetFreight(ctx, input.FreightID)
	if err != nil {
		return FreightApproval{}, err
	}
	if freight.Status == FreightArchived {
		return FreightApproval{}, shared.NewError(shared.CodeFailedPrecondition, "freight is archived")
	}
	app, err := s.appsOrError().GetApplication(ctx, freight.ApplicationID)
	if err != nil {
		return FreightApproval{}, err
	}
	if err := s.check(ctx, input.Actor, app, "deployment:approve"); err != nil {
		return FreightApproval{}, err
	}
	stageKey := normalizeStageKey(input.TargetStageKey)
	if stageKey == "" {
		return FreightApproval{}, shared.NewError(shared.CodeInvalidArgument, "target_stage_key is required")
	}
	if input.Decision != FreightApprovalApproved && input.Decision != FreightApprovalRejected {
		return FreightApproval{}, shared.NewError(shared.CodeInvalidArgument, "approval decision is not supported")
	}
	if existing, err := s.repo.FindFreightApproval(ctx, freight.ID, stageKey); err == nil {
		return existing, nil
	} else if shared.CodeOf(err) != shared.CodeNotFound {
		return FreightApproval{}, err
	}
	id, err := s.ids.NewID("freight_approval")
	if err != nil {
		return FreightApproval{}, err
	}
	now := s.clock.Now()
	approval := FreightApproval{ID: id, TenantID: freight.TenantID, ProjectID: freight.ProjectID, ApplicationID: freight.ApplicationID, FreightID: freight.ID, TargetStageKey: stageKey, ApproverID: input.Actor.ID, Status: input.Decision, Comment: strings.TrimSpace(input.Comment), CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateFreightApproval(ctx, approval); err != nil {
		return FreightApproval{}, err
	}
	action := "freight.approve"
	summary := "审批通过 Freight"
	if input.Decision == FreightApprovalRejected {
		action = "freight.reject"
		summary = "审批拒绝 Freight"
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: action, ResourceType: "freight", ResourceID: freight.ID, Result: "succeeded", Summary: summary, OccurredAt: now})
	return approval, nil
}

func (s *Service) CompleteStageVerification(ctx context.Context, input StageVerificationInput) (StageVerification, error) {
	app, err := s.appsOrError().GetApplication(ctx, input.ApplicationID)
	if err != nil {
		return StageVerification{}, err
	}
	if err := s.check(ctx, input.Actor, app, "deployment:approve"); err != nil {
		return StageVerification{}, err
	}
	stageKey := normalizeStageKey(input.StageKey)
	if stageKey == "" || input.FreightID.IsZero() {
		return StageVerification{}, shared.NewError(shared.CodeInvalidArgument, "stage_key and freight_id are required")
	}
	if input.Status != StageVerificationPassed && input.Status != StageVerificationFailed {
		return StageVerification{}, shared.NewError(shared.CodeInvalidArgument, "verification status is not supported")
	}
	if err := s.requireDeploymentRecordForVerification(ctx, app.ID, stageKey, input.FreightID); err != nil {
		return StageVerification{}, err
	}
	now := s.clock.Now()
	if existing, err := s.repo.FindStageVerification(ctx, app.ID, stageKey, input.FreightID); err == nil {
		existing.VerifierID = input.Actor.ID
		existing.Status = input.Status
		existing.Comment = strings.TrimSpace(input.Comment)
		existing.SyncStatus = strings.TrimSpace(input.SyncStatus)
		existing.HealthStatus = strings.TrimSpace(input.HealthStatus)
		existing.AgentStatus = strings.TrimSpace(input.AgentStatus)
		existing.UpdatedAt = now
		if err := s.repo.UpdateStageVerification(ctx, existing); err != nil {
			return StageVerification{}, err
		}
		_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "stage.verify", ResourceType: "stage_verification", ResourceID: existing.ID, Result: "succeeded", Summary: "更新人工验证", OccurredAt: now})
		s.triggerAutoPromotionsAfterPassedVerification(ctx, existing)
		_ = s.publish(ctx, "StageVerificationCompleted", s.clock.Now(), map[string]any{"application_id": existing.ApplicationID, "stage_key": existing.StageKey, "freight_id": existing.FreightID, "verification_id": existing.ID, "status": existing.Status})
		return existing, nil
	} else if shared.CodeOf(err) != shared.CodeNotFound {
		return StageVerification{}, err
	}
	id, err := s.ids.NewID("stage_verification")
	if err != nil {
		return StageVerification{}, err
	}
	verification := StageVerification{ID: id, TenantID: app.TenantID, ProjectID: app.ProjectID, ApplicationID: app.ID, StageKey: stageKey, FreightID: input.FreightID, VerifierID: input.Actor.ID, Status: input.Status, Comment: strings.TrimSpace(input.Comment), SyncStatus: strings.TrimSpace(input.SyncStatus), HealthStatus: strings.TrimSpace(input.HealthStatus), AgentStatus: strings.TrimSpace(input.AgentStatus), CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateStageVerification(ctx, verification); err != nil {
		return StageVerification{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "stage.verify", ResourceType: "stage_verification", ResourceID: verification.ID, Result: "succeeded", Summary: "完成人工验证", OccurredAt: now})
	s.triggerAutoPromotionsAfterPassedVerification(ctx, verification)
	_ = s.publish(ctx, "StageVerificationCompleted", s.clock.Now(), map[string]any{"application_id": verification.ApplicationID, "stage_key": verification.StageKey, "freight_id": verification.FreightID, "verification_id": verification.ID, "status": verification.Status})
	return verification, nil
}

func (s *Service) triggerAutoPromotionsAfterPassedVerification(ctx context.Context, verification StageVerification) {
	if verification.Status != StageVerificationPassed {
		return
	}
	if _, err := s.createManualPromotionsForEligibleStages(ctx, verification.ApplicationID, verification.FreightID, deliverySystemActorID, "stage_verification_passed:"+verification.StageKey); err != nil {
		_ = s.audit.Log(ctx, AuditEvent{ActorID: deliverySystemActorID, Action: "promotion.auto_create", ResourceType: "freight", ResourceID: verification.FreightID, Result: "failed", Summary: "验证通过后自动创建手动 Promotion 失败", Details: map[string]string{"stage_key": verification.StageKey, "reason": err.Error()}, OccurredAt: s.clock.Now()})
	}
}

func (s *Service) GetFreightCreationContext(ctx context.Context, applicationID shared.ID) (FreightCreationContext, error) {
	workloads, err := s.workloadsOrError().ListEnabledWorkloads(ctx, applicationID)
	if err != nil {
		return FreightCreationContext{}, err
	}
	_ = s.backfillMissingReleasesFromSucceededBuilds(ctx, applicationID, workloads)
	releases, err := s.repo.ListReleasesByApplication(ctx, applicationID, shared.PageRequest{Page: 1, PageSize: 1000})
	if err != nil {
		return FreightCreationContext{}, err
	}
	ctxOut := FreightCreationContext{
		EnabledWorkloads:                   workloads,
		LatestReleasesByWorkload:           map[shared.ID]Release{},
		LatestArtifactsByWorkload:          map[shared.ID]BuildArtifactRef{},
		LatestReleasesByWorkloadContainer:  map[string]Release{},
		LatestArtifactsByWorkloadContainer: map[string]BuildArtifactRef{},
		StageEligibility:                   map[shared.ID][]shared.ID{},
	}
	for _, release := range releases.Items {
		key := freightTargetKey(release.WorkloadID, release.ContainerName)
		if _, ok := ctxOut.LatestReleasesByWorkloadContainer[key]; ok {
			continue
		}
		release, err = s.enrichReleaseBundleImages(ctx, release)
		if err != nil {
			return FreightCreationContext{}, err
		}
		ctxOut.LatestReleasesByWorkloadContainer[key] = release
		if _, ok := ctxOut.LatestReleasesByWorkload[release.WorkloadID]; !ok {
			ctxOut.LatestReleasesByWorkload[release.WorkloadID] = release
		}
		if artifact, err := s.buildsOrError().GetBuildArtifact(ctx, release.BuildArtifactID); err == nil {
			ctxOut.LatestArtifactsByWorkloadContainer[key] = artifact
			if _, ok := ctxOut.LatestArtifactsByWorkload[release.WorkloadID]; !ok {
				ctxOut.LatestArtifactsByWorkload[release.WorkloadID] = artifact
			}
		}
	}
	stages, err := s.ListAppStages(ctx, applicationID)
	if err == nil {
		ctxOut.Stages = make([]FreightCreationStage, 0, len(stages))
		for _, stage := range stages {
			stageKey := normalizeStageKey(stage.StageKey)
			if stageKey == "" {
				continue
			}
			ctxOut.Stages = append(ctxOut.Stages, FreightCreationStage{ID: shared.ID(stageKey), StageKey: stageKey, Name: stage.DisplayName, ApprovalRequired: stage.RequiresApproval})
			freights, err := s.ListEligibleFreights(ctx, applicationID, shared.ID(stageKey))
			if err != nil {
				continue
			}
			ids := make([]shared.ID, 0, len(freights))
			for _, freight := range freights {
				ids = append(ids, freight.ID)
			}
			ctxOut.StageEligibility[shared.ID(stageKey)] = ids
		}
	}
	return ctxOut, nil
}

func (s *Service) backfillMissingReleasesFromSucceededBuilds(ctx context.Context, applicationID shared.ID, workloads []WorkloadRef) error {
	if len(workloads) == 0 {
		return nil
	}
	builds := s.buildsOrError()
	runs, err := builds.ListBuildRuns(ctx, applicationID, shared.PageRequest{Page: 1, PageSize: 100})
	if err != nil {
		if shared.CodeOf(err) == shared.CodeFailedPrecondition {
			return nil
		}
		return err
	}
	enabledWorkloads := map[shared.ID]struct{}{}
	for _, workload := range workloads {
		if !workload.ID.IsZero() {
			enabledWorkloads[workload.ID] = struct{}{}
		}
	}
	for _, run := range runs.Items {
		if !strings.EqualFold(strings.TrimSpace(run.Status), "succeeded") {
			continue
		}
		artifacts, err := builds.ListBuildArtifacts(ctx, run.ID)
		if err != nil || len(artifacts) == 0 {
			continue
		}
		workloadIDs := make([]shared.ID, 0, len(workloads))
		for _, artifact := range artifacts {
			if artifact.ApplicationID != applicationID {
				continue
			}
			if !artifact.WorkloadID.IsZero() {
				if _, ok := enabledWorkloads[artifact.WorkloadID]; ok {
					workloadIDs = append(workloadIDs, artifact.WorkloadID)
				}
			}
		}
		if len(workloadIDs) == 0 && len(workloads) == 1 {
			workloadIDs = append(workloadIDs, workloads[0].ID)
		}
		if len(workloadIDs) == 0 {
			continue
		}
		artifactIDs := make([]shared.ID, 0, len(artifacts))
		for _, artifact := range artifacts {
			artifactIDs = append(artifactIDs, artifact.ID)
		}
		if _, err := s.HandleBuildSucceeded(ctx, BuildSucceededPayload{BuildRunID: run.ID, ApplicationID: applicationID, WorkloadIDs: uniqueIDs(workloadIDs), BuildArtifactIDs: artifactIDs}); err != nil {
			continue
		}
	}
	return nil
}

func (s *Service) ListEligibleFreights(ctx context.Context, applicationID shared.ID, stageID shared.ID) ([]Freight, error) {
	stage, err := s.deliveryStageByIDOrKey(ctx, applicationID, stageID)
	if err != nil {
		return nil, err
	}
	result, err := s.repo.ListFreightsByApplication(ctx, applicationID, shared.PageRequest{Page: 1, PageSize: 1000})
	if err != nil {
		return nil, err
	}
	out := []Freight{}
	for _, freight := range result.Items {
		if err := s.validateFreightComplete(ctx, applicationID, freight.ID); err != nil {
			continue
		}
		if err := s.validateStageOrder(ctx, freight, stage); err != nil {
			continue
		}
		out = append(out, freight)
	}
	return out, nil
}

func (s *Service) createManualPromotionsForEligibleStages(ctx context.Context, applicationID shared.ID, freightID shared.ID, actorID shared.ID, trigger string) ([]Promotion, error) {
	freight, err := s.repo.GetFreight(ctx, freightID)
	if err != nil {
		return nil, err
	}
	if freight.ApplicationID != applicationID {
		return nil, shared.NewError(shared.CodeInvalidArgument, "freight does not belong to application")
	}
	if err := s.validateFreightComplete(ctx, applicationID, freight.ID); err != nil {
		return nil, err
	}
	stages, err := s.ListAppStages(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	created := make([]Promotion, 0)
	for _, appStage := range stages {
		stageKey := normalizeStageKey(appStage.StageKey)
		if stageKey == "" || appStage.Status == DeliveryFlowTemplateStageDisabled {
			continue
		}
		_, stage, _, err := s.validateStagePromotionTarget(ctx, applicationID, stageKey, nil, "")
		if err != nil {
			s.auditAutoPromotionSkip(ctx, freight.ID, stageKey, trigger, err)
			continue
		}
		if exists, err := s.hasPromotionForFreightStage(ctx, applicationID, freight.ID, stage.ID, stageKey); err != nil {
			return created, err
		} else if exists {
			continue
		}
		if err := s.validateStageOrder(ctx, freight, stage); err != nil {
			s.auditAutoPromotionSkip(ctx, freight.ID, stageKey, trigger, err)
			continue
		}
		promotion, err := s.newPromotion(ctx, freight, stage, stageKey, "", actorID, "自动创建，等待手动发布", false, "", false)
		if err != nil {
			return created, err
		}
		if promotion.Status == PromotionPendingApproval {
			promotion, err = s.createApproval(ctx, promotion)
			if err != nil {
				return created, err
			}
		}
		created = append(created, promotion)
		_ = s.audit.Log(ctx, AuditEvent{ActorID: actorID, Action: "promotion.auto_create", ResourceType: "promotion", ResourceID: promotion.ID, Result: "succeeded", Summary: "自动创建手动发布 Promotion", Details: map[string]string{"freight_id": freight.ID.String(), "stage_key": stageKey, "trigger": trigger}, OccurredAt: s.clock.Now()})
	}
	return created, nil
}

func (s *Service) hasPromotionForFreightStage(ctx context.Context, applicationID shared.ID, freightID shared.ID, stageID shared.ID, stageKey string) (bool, error) {
	promotions, err := s.repo.ListPromotionsByApplication(ctx, applicationID, shared.PageRequest{Page: 1, PageSize: 1000})
	if err != nil {
		return false, err
	}
	stageKey = normalizeStageKey(stageKey)
	for _, promotion := range promotions.Items {
		if promotion.FreightID != freightID {
			continue
		}
		promotionStageKey := normalizeStageKey(promotion.TargetStageKey)
		sameStage := promotionStageKey != "" && promotionStageKey == stageKey
		if !sameStage && !stageID.IsZero() && promotion.TargetStageID == stageID {
			sameStage = true
		}
		if sameStage {
			return true, nil
		}
	}
	return false, nil
}

func (s *Service) auditAutoPromotionSkip(ctx context.Context, freightID shared.ID, stageKey string, trigger string, err error) {
	if err == nil {
		return
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: deliverySystemActorID, Action: "promotion.auto_skip", ResourceType: "freight", ResourceID: freightID, Result: "skipped", Summary: "自动创建 Promotion 跳过不可达 Stage", Details: map[string]string{"stage_key": stageKey, "trigger": trigger, "reason": err.Error()}, OccurredAt: s.clock.Now()})
}

func (s *Service) deliveryStageByIDOrKey(ctx context.Context, applicationID shared.ID, stageIDOrKey shared.ID) (DeliveryStage, error) {
	stage, err := s.repo.GetDeliveryStage(ctx, stageIDOrKey)
	if err == nil {
		if stage.ApplicationID != applicationID {
			return DeliveryStage{}, shared.NewError(shared.CodeInvalidArgument, "stage does not belong to application")
		}
		return stage, nil
	}
	if shared.CodeOf(err) != shared.CodeNotFound {
		return DeliveryStage{}, err
	}
	stageKey := normalizeStageKey(string(stageIDOrKey))
	if stageKey == "" {
		return DeliveryStage{}, err
	}
	flow, flowErr := s.ensureDefaultFlow(ctx, applicationID)
	if flowErr != nil {
		return DeliveryStage{}, flowErr
	}
	stages, listErr := s.repo.ListDeliveryStages(ctx, flow.ID)
	if listErr != nil {
		return DeliveryStage{}, listErr
	}
	for _, stage := range stages {
		if normalizeStageKey(stage.Name) == stageKey {
			return stage, nil
		}
	}
	return DeliveryStage{}, err
}

func (s *Service) CreateRollbackPromotion(ctx context.Context, input CreateRollbackPromotionInput) (Promotion, error) {
	target, err := s.repo.GetFreight(ctx, input.TargetFreightID)
	if err != nil {
		return Promotion{}, err
	}
	if target.Status == FreightArchived {
		return Promotion{}, shared.NewError(shared.CodeFailedPrecondition, "freight is archived")
	}
	if !input.CurrentFreightID.IsZero() {
		if _, err := s.repo.GetFreight(ctx, input.CurrentFreightID); err != nil {
			return Promotion{}, err
		}
	}
	stageKey := normalizeStageKey(input.TargetStageKey)
	app, stage, targetClusters, err := s.validateStagePromotionTarget(ctx, target.ApplicationID, stageKey, nil, "")
	if err != nil {
		return Promotion{}, err
	}
	if err := s.check(ctx, input.Actor, app, "deployment:rollback"); err != nil {
		return Promotion{}, err
	}
	promotion, err := s.newPromotion(ctx, target, stage, stageKey, "", input.Actor.ID, strings.TrimSpace(input.Message), true, input.CurrentFreightID, true)
	if err != nil {
		return Promotion{}, err
	}
	if promotion.Status == PromotionPendingApproval {
		return s.createApproval(ctx, promotion)
	}
	return s.applyPromotion(ctx, promotion, targetClusters)
}

func (s *Service) ApprovePromotion(ctx context.Context, input ApprovalInput) (Promotion, error) {
	promotion, err := s.repo.GetPromotion(ctx, input.PromotionID)
	if err != nil {
		return Promotion{}, err
	}
	if promotion.Status != PromotionPendingApproval {
		return Promotion{}, shared.NewError(shared.CodeFailedPrecondition, "promotion is not pending approval")
	}
	app, err := s.appsOrError().GetApplication(ctx, promotion.ApplicationID)
	if err != nil {
		return Promotion{}, err
	}
	if err := s.check(ctx, input.Actor, app, "deployment:approve"); err != nil {
		return Promotion{}, err
	}
	approval, err := s.repo.GetPromotionApproval(ctx, promotion.ID)
	if err != nil {
		return Promotion{}, err
	}
	now := s.clock.Now()
	approval.Status = PromotionApprovalApproved
	approval.ApproverID = input.Actor.ID
	approval.Comment = strings.TrimSpace(input.Comment)
	approval.UpdatedAt = now
	if err := s.repo.UpdatePromotionApproval(ctx, approval); err != nil {
		return Promotion{}, err
	}
	promotion.Status = PromotionApproved
	promotion.ApprovedBy = input.Actor.ID
	promotion.UpdatedAt = now
	if err := s.repo.UpdatePromotion(ctx, promotion); err != nil {
		return Promotion{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "promotion.approve", ResourceType: "promotion", ResourceID: promotion.ID, Result: "succeeded", Summary: "审批通过发布晋级", OccurredAt: now})
	if !promotion.AutoPublish {
		return promotion, nil
	}
	_, _, targetClusters, err := s.validateStagePromotionTarget(ctx, promotion.ApplicationID, promotion.TargetStageKey, nil, promotion.NamespaceOverride)
	if err != nil {
		return Promotion{}, err
	}
	return s.applyPromotion(ctx, promotion, targetClusters)
}

func (s *Service) RejectPromotion(ctx context.Context, input ApprovalInput) (Promotion, error) {
	promotion, err := s.repo.GetPromotion(ctx, input.PromotionID)
	if err != nil {
		return Promotion{}, err
	}
	if promotion.Status != PromotionPendingApproval {
		return Promotion{}, shared.NewError(shared.CodeFailedPrecondition, "promotion is not pending approval")
	}
	app, err := s.appsOrError().GetApplication(ctx, promotion.ApplicationID)
	if err != nil {
		return Promotion{}, err
	}
	if err := s.check(ctx, input.Actor, app, "deployment:approve"); err != nil {
		return Promotion{}, err
	}
	approval, err := s.repo.GetPromotionApproval(ctx, promotion.ID)
	if err != nil {
		return Promotion{}, err
	}
	now := s.clock.Now()
	approval.Status = PromotionApprovalRejected
	approval.ApproverID = input.Actor.ID
	approval.Comment = strings.TrimSpace(input.Comment)
	approval.UpdatedAt = now
	promotion.Status = PromotionRejected
	promotion.Message = strings.TrimSpace(input.Comment)
	promotion.UpdatedAt = now
	promotion.CompletedAt = &now
	if err := s.repo.UpdatePromotionApproval(ctx, approval); err != nil {
		return Promotion{}, err
	}
	if err := s.repo.UpdatePromotion(ctx, promotion); err != nil {
		return Promotion{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "promotion.reject", ResourceType: "promotion", ResourceID: promotion.ID, Result: "succeeded", Summary: "拒绝发布晋级", OccurredAt: now})
	return promotion, nil
}

func (s *Service) PublishPromotion(ctx context.Context, input ApprovalInput) (Promotion, error) {
	promotion, err := s.repo.GetPromotion(ctx, input.PromotionID)
	if err != nil {
		return Promotion{}, err
	}
	if promotion.Status != PromotionCreated && promotion.Status != PromotionApproved {
		return Promotion{}, shared.NewError(shared.CodeFailedPrecondition, "promotion is not pending publish")
	}
	app, err := s.appsOrError().GetApplication(ctx, promotion.ApplicationID)
	if err != nil {
		return Promotion{}, err
	}
	if err := s.check(ctx, input.Actor, app, "deployment:create"); err != nil {
		return Promotion{}, err
	}
	promotion.AutoPublish = false
	if strings.TrimSpace(input.Comment) != "" {
		promotion.Message = strings.TrimSpace(input.Comment)
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "promotion.publish", ResourceType: "promotion", ResourceID: promotion.ID, Result: "succeeded", Summary: "提交发布晋级", OccurredAt: s.clock.Now()})
	_, _, targetClusters, err := s.validateStagePromotionTarget(ctx, promotion.ApplicationID, promotion.TargetStageKey, nil, promotion.NamespaceOverride)
	if err != nil {
		return Promotion{}, err
	}
	return s.applyPromotion(ctx, promotion, targetClusters)
}

func (s *Service) RejectPendingPromotion(ctx context.Context, input ApprovalInput) (Promotion, error) {
	promotion, err := s.repo.GetPromotion(ctx, input.PromotionID)
	if err != nil {
		return Promotion{}, err
	}
	if promotion.Status != PromotionCreated && promotion.Status != PromotionApproved {
		return Promotion{}, shared.NewError(shared.CodeFailedPrecondition, "promotion is not pending publish")
	}
	app, err := s.appsOrError().GetApplication(ctx, promotion.ApplicationID)
	if err != nil {
		return Promotion{}, err
	}
	if err := s.check(ctx, input.Actor, app, "deployment:create"); err != nil {
		return Promotion{}, err
	}
	now := s.clock.Now()
	promotion.Status = PromotionRejected
	promotion.Message = strings.TrimSpace(input.Comment)
	promotion.UpdatedAt = now
	promotion.CompletedAt = &now
	if err := s.repo.UpdatePromotion(ctx, promotion); err != nil {
		return Promotion{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "promotion.reject_publish", ResourceType: "promotion", ResourceID: promotion.ID, Result: "succeeded", Summary: "驳回待发布晋级", OccurredAt: now})
	return promotion, nil
}

func (s *Service) AbortPromotion(ctx context.Context, actor identityaccess.Subject, promotionID shared.ID) (Promotion, error) {
	promotion, err := s.repo.GetPromotion(ctx, promotionID)
	if err != nil {
		return Promotion{}, err
	}
	if terminalPromotion(promotion.Status) {
		return promotion, nil
	}
	app, err := s.appsOrError().GetApplication(ctx, promotion.ApplicationID)
	if err != nil {
		return Promotion{}, err
	}
	if err := s.check(ctx, actor, app, "deployment:create"); err != nil {
		return Promotion{}, err
	}
	now := s.clock.Now()
	promotion.Status = PromotionAborted
	promotion.UpdatedAt = now
	promotion.CompletedAt = &now
	if err := s.repo.UpdatePromotion(ctx, promotion); err != nil {
		return Promotion{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: actor.ID, Action: "promotion.abort", ResourceType: "promotion", ResourceID: promotion.ID, Result: "succeeded", Summary: "中止发布晋级", OccurredAt: now})
	return promotion, nil
}

func (s *Service) GetRelease(ctx context.Context, id shared.ID) (Release, error) {
	return s.repo.GetRelease(ctx, id)
}
func (s *Service) GetFreight(ctx context.Context, id shared.ID) (Freight, error) {
	return s.repo.GetFreight(ctx, id)
}
func (s *Service) GetFreightDetail(ctx context.Context, id shared.ID) (FreightDetail, error) {
	freight, err := s.repo.GetFreight(ctx, id)
	if err != nil {
		return FreightDetail{}, err
	}
	items, err := s.repo.ListFreightItems(ctx, id)
	if err != nil {
		return FreightDetail{}, err
	}
	items, err = s.enrichFreightItemBundleImages(ctx, items)
	if err != nil {
		return FreightDetail{}, err
	}
	return FreightDetail{Freight: freight, Items: items}, nil
}
func (s *Service) ListFreights(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[Freight], error) {
	return s.repo.ListFreightsByApplication(ctx, applicationID, page)
}

func (s *Service) ArchiveFreight(ctx context.Context, input ArchiveFreightInput) (Freight, error) {
	freight, err := s.repo.GetFreight(ctx, input.FreightID)
	if err != nil {
		return Freight{}, err
	}
	app, err := s.appsOrError().GetApplication(ctx, freight.ApplicationID)
	if err != nil {
		return Freight{}, err
	}
	if err := s.check(ctx, input.Actor, app, "freight:delete"); err != nil {
		return Freight{}, err
	}
	if freight.Status == FreightArchived {
		return freight, nil
	}
	current, err := s.currentFreightsByStage(ctx, freight.ApplicationID)
	if err != nil {
		return Freight{}, err
	}
	for _, currentFreight := range current {
		if currentFreight.ID == freight.ID {
			return Freight{}, shared.NewError(shared.CodeFailedPrecondition, "freight is currently used by stage")
		}
	}
	if err := s.ensureNoUnfinishedPromotion(ctx, freight); err != nil {
		return Freight{}, err
	}
	if err := s.repo.UpdateFreightStatus(ctx, freight.ID, FreightArchived); err != nil {
		return Freight{}, err
	}
	freight.Status = FreightArchived
	now := s.clock.Now()
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "freight.archive", ResourceType: "freight", ResourceID: freight.ID, Result: "succeeded", Summary: "归档可交付变更包", OccurredAt: now})
	_ = s.publish(ctx, "FreightArchived", now, map[string]any{"freight_id": freight.ID, "application_id": freight.ApplicationID})
	return freight, nil
}

func (s *Service) ensureNoUnfinishedPromotion(ctx context.Context, freight Freight) error {
	for page := 1; ; page++ {
		result, err := s.repo.ListPromotionsByApplication(ctx, freight.ApplicationID, shared.PageRequest{Page: page, PageSize: 100})
		if err != nil {
			return err
		}
		for _, promotion := range result.Items {
			if promotion.FreightID != freight.ID {
				continue
			}
			if !terminalPromotion(promotion.Status) && !promotionStagePassed(promotion.Status) {
				return shared.NewError(shared.CodeFailedPrecondition, "freight has unfinished promotion")
			}
		}
		if int64(page*result.PageSize) >= result.Total {
			return nil
		}
	}
}

func (s *Service) GetPromotion(ctx context.Context, id shared.ID) (Promotion, error) {
	return s.repo.GetPromotion(ctx, id)
}
func (s *Service) ListPromotions(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[Promotion], error) {
	return s.repo.ListPromotionsByApplication(ctx, applicationID, page)
}

func (s *Service) enrichReleaseBundleImages(ctx context.Context, release Release) (Release, error) {
	if release.ImageBundleID.IsZero() {
		return release, nil
	}
	images, err := s.repo.ListImageBundleImages(ctx, release.ImageBundleID)
	if err != nil {
		return Release{}, err
	}
	release.BundleImages = images
	return release, nil
}

func (s *Service) enrichFreightItemBundleImages(ctx context.Context, items []FreightItem) ([]FreightItem, error) {
	for i := range items {
		if items[i].ImageBundleID.IsZero() {
			continue
		}
		images, err := s.repo.ListImageBundleImages(ctx, items[i].ImageBundleID)
		if err != nil {
			return nil, err
		}
		items[i].BundleImages = images
	}
	return items, nil
}

func (s *Service) ensureDefaultFlow(ctx context.Context, applicationID shared.ID) (DeliveryFlow, error) {
	if flow, err := s.repo.FindDeliveryFlowByApplication(ctx, applicationID); err == nil {
		if err := s.syncDeliveryStages(ctx, flow); err != nil {
			return DeliveryFlow{}, err
		}
		return flow, nil
	}
	app, err := s.appsOrError().GetApplication(ctx, applicationID)
	if err != nil {
		return DeliveryFlow{}, err
	}
	flowID, err := s.ids.NewID("delivery_flow")
	if err != nil {
		return DeliveryFlow{}, err
	}
	now := s.clock.Now()
	flow := DeliveryFlow{ID: flowID, TenantID: app.TenantID, ProjectID: app.ProjectID, ApplicationID: app.ID, Name: "默认交付流", CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateDeliveryFlow(ctx, flow); err != nil {
		return DeliveryFlow{}, err
	}
	if err := s.syncDeliveryStages(ctx, flow); err != nil {
		return DeliveryFlow{}, err
	}
	return flow, nil
}

func (s *Service) syncDeliveryStages(ctx context.Context, flow DeliveryFlow) error {
	app, err := s.appsOrError().GetApplication(ctx, flow.ApplicationID)
	if err != nil {
		return err
	}
	template, err := s.ensureDeliveryFlowTemplate(ctx, app.TenantID)
	if err != nil {
		return err
	}
	for _, stage := range template.Stages {
		stageKey := normalizeStageKey(stage.StageKey)
		if stageKey == "" || stage.Status == DeliveryFlowTemplateStageDisabled {
			continue
		}
		if _, err := s.repo.FindDeliveryStageByName(ctx, flow.ApplicationID, stageKey); err == nil {
			continue
		} else if shared.CodeOf(err) != shared.CodeNotFound {
			return err
		}
		stageID, err := s.ids.NewID("delivery_stage")
		if err != nil {
			return err
		}
		now := s.clock.Now()
		deliveryStage := DeliveryStage{ID: stageID, TenantID: app.TenantID, ProjectID: app.ProjectID, ApplicationID: app.ID, DeliveryFlowID: flow.ID, Name: stageKey, Order: stage.Order, RequiresApproval: stage.RequiresApproval, CreatedAt: now, UpdatedAt: now}
		if err := s.repo.CreateDeliveryStage(ctx, deliveryStage); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ensureDeliveryFlowTemplate(ctx context.Context, tenantID shared.ID) (DeliveryFlowTemplate, error) {
	if tenantID.IsZero() {
		return DeliveryFlowTemplate{}, shared.NewError(shared.CodeInvalidArgument, "tenant_id is required")
	}
	if template, err := s.repo.FindDeliveryFlowTemplateByTenant(ctx, tenantID); err == nil {
		stages, err := s.repo.ListDeliveryFlowTemplateStages(ctx, template.ID)
		if err != nil {
			return DeliveryFlowTemplate{}, err
		}
		edges, err := s.repo.ListDeliveryFlowTemplateEdges(ctx, template.ID)
		if err != nil {
			return DeliveryFlowTemplate{}, err
		}
		template.Stages = stages
		template.Edges = edges
		normalizeTemplateLayout(&template)
		return template, nil
	} else if shared.CodeOf(err) != shared.CodeNotFound {
		return DeliveryFlowTemplate{}, err
	}
	templateID, err := s.ids.NewID("delivery_flow_template")
	if err != nil {
		return DeliveryFlowTemplate{}, err
	}
	now := s.clock.Now()
	template := DeliveryFlowTemplate{ID: templateID, TenantID: tenantID, Name: "默认交付流模板", CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateDeliveryFlowTemplate(ctx, template); err != nil {
		if existing, findErr := s.repo.FindDeliveryFlowTemplateByTenant(ctx, tenantID); findErr == nil {
			stages, err := s.repo.ListDeliveryFlowTemplateStages(ctx, existing.ID)
			if err != nil {
				return DeliveryFlowTemplate{}, err
			}
			edges, err := s.repo.ListDeliveryFlowTemplateEdges(ctx, existing.ID)
			if err != nil {
				return DeliveryFlowTemplate{}, err
			}
			existing.Stages = stages
			existing.Edges = edges
			normalizeTemplateLayout(&existing)
			return existing, nil
		}
		return DeliveryFlowTemplate{}, err
	}
	for idx, item := range defaultTemplateStages {
		stageID, err := s.ids.NewID("delivery_flow_stage_template")
		if err != nil {
			return DeliveryFlowTemplate{}, err
		}
		stage := DeliveryFlowTemplateStage{
			ID:                   stageID,
			TenantID:             tenantID,
			TemplateID:           template.ID,
			StageKey:             item.key,
			DisplayName:          item.displayName,
			Color:                colorForLayoutColumn(idx),
			Order:                idx + 1,
			LayoutColumn:         idx,
			LayoutRow:            0,
			Status:               DeliveryFlowTemplateStageEnabled,
			RequiresApproval:     item.requiresApproval,
			RequiresVerification: item.requiresVerification,
			ApproveRoles:         append([]string(nil), item.approveRoles...),
			VerifyRoles:          append([]string(nil), item.verifyRoles...),
			CreatedAt:            now,
			UpdatedAt:            now,
		}
		if err := s.repo.CreateDeliveryFlowTemplateStage(ctx, stage); err != nil {
			return DeliveryFlowTemplate{}, err
		}
		template.Stages = append(template.Stages, stage)
	}
	defaultEdges := []DeliveryFlowTemplateEdgeInput{{FromStageKey: "dev", ToStageKey: "test"}, {FromStageKey: "test", ToStageKey: "staging"}, {FromStageKey: "staging", ToStageKey: "prod"}}
	stageByKey := map[string]DeliveryFlowTemplateStage{}
	for _, stage := range template.Stages {
		stageByKey[stage.StageKey] = stage
	}
	edges, err := s.buildTemplateEdges(tenantID, template.ID, defaultEdges, stageByKey, now)
	if err != nil {
		return DeliveryFlowTemplate{}, err
	}
	if err := s.repo.ReplaceDeliveryFlowTemplateEdges(ctx, template.ID, edges); err != nil {
		return DeliveryFlowTemplate{}, err
	}
	template.Edges = edges
	return template, nil
}

func (s *Service) validateStagePromotionTarget(ctx context.Context, applicationID shared.ID, stageKey string, clusterIDs []shared.ID, namespaceOverride string) (ApplicationRef, DeliveryStage, []GitOpsPromotionTargetCluster, error) {
	if stageKey == "" {
		return ApplicationRef{}, DeliveryStage{}, nil, shared.NewError(shared.CodeInvalidArgument, "target_stage_key is required")
	}
	if len(clusterIDs) > 1 {
		return ApplicationRef{}, DeliveryStage{}, nil, shared.NewError(shared.CodeInvalidArgument, "stage can target only one bound cluster")
	}
	app, err := s.appsOrError().GetApplication(ctx, applicationID)
	if err != nil {
		return ApplicationRef{}, DeliveryStage{}, nil, err
	}
	template, err := s.ensureDeliveryFlowTemplate(ctx, app.TenantID)
	if err != nil {
		return ApplicationRef{}, DeliveryStage{}, nil, err
	}
	var templateStage DeliveryFlowTemplateStage
	found := false
	for _, stage := range template.Stages {
		if stage.StageKey == stageKey {
			templateStage = stage
			found = true
			break
		}
	}
	if !found {
		return ApplicationRef{}, DeliveryStage{}, nil, shared.NewError(shared.CodeNotFound, "stage template not found")
	}
	if templateStage.Status == DeliveryFlowTemplateStageDisabled {
		return ApplicationRef{}, DeliveryStage{}, nil, shared.NewError(shared.CodeFailedPrecondition, "stage is disabled")
	}
	if _, err := s.ensureDefaultFlow(ctx, applicationID); err != nil {
		return ApplicationRef{}, DeliveryStage{}, nil, err
	}
	stage, err := s.repo.FindDeliveryStageByName(ctx, applicationID, stageKey)
	if err != nil {
		return ApplicationRef{}, DeliveryStage{}, nil, err
	}
	bindings, err := s.repo.ListStageClusterBindings(ctx, app.TenantID, stageKey)
	if err != nil {
		return ApplicationRef{}, DeliveryStage{}, nil, err
	}
	pool := map[shared.ID]StageClusterBinding{}
	for _, binding := range bindings {
		if binding.Status == StageClusterBindingActive {
			pool[binding.ClusterID] = binding
		}
	}
	if len(clusterIDs) == 0 {
		if len(pool) == 0 {
			return ApplicationRef{}, DeliveryStage{}, nil, shared.NewError(shared.CodeFailedPrecondition, "stage has no bound cluster")
		}
		if len(pool) > 1 {
			return ApplicationRef{}, DeliveryStage{}, nil, shared.NewError(shared.CodeConflict, "stage has multiple bound clusters")
		}
		for clusterID := range pool {
			clusterIDs = []shared.ID{clusterID}
		}
	}
	namespace := strings.TrimSpace(namespaceOverride)
	if namespace == "" {
		namespace = firstNonEmpty(app.ProjectName, app.ProjectID.String())
	}
	namespace = normalizeKubernetesNamespace(namespace)
	targets := make([]GitOpsPromotionTargetCluster, 0, len(clusterIDs))
	seen := map[shared.ID]struct{}{}
	for _, id := range clusterIDs {
		if id.IsZero() {
			return ApplicationRef{}, DeliveryStage{}, nil, shared.NewError(shared.CodeInvalidArgument, "target_cluster_ids is invalid")
		}
		if _, ok := seen[id]; ok {
			return ApplicationRef{}, DeliveryStage{}, nil, shared.NewError(shared.CodeConflict, "target cluster is duplicated")
		}
		seen[id] = struct{}{}
		binding, ok := pool[id]
		if !ok {
			return ApplicationRef{}, DeliveryStage{}, nil, shared.NewError(shared.CodeInvalidArgument, "target cluster is not bound to stage")
		}
		labels := map[string]string(nil)
		if s.clusters != nil {
			cluster, err := s.clusters.GetCluster(ctx, id)
			if err != nil {
				return ApplicationRef{}, DeliveryStage{}, nil, err
			}
			if cluster.TenantID != app.TenantID {
				return ApplicationRef{}, DeliveryStage{}, nil, shared.NewError(shared.CodePermissionDenied, "cluster belongs to another tenant")
			}
			labels = cleanStringMap(cluster.Labels)
		}
		targets = append(targets, GitOpsPromotionTargetCluster{ClusterID: id, ClusterName: binding.ClusterName, Namespace: namespace, Labels: labels})
	}
	return app, stage, targets, nil
}

func normalizeKubernetesNamespace(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if r == '-' || r == '_' || r == '.' {
			if b.Len() > 0 && !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
			continue
		}
		if b.Len() > 0 && !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	namespace := strings.Trim(b.String(), "-")
	if len(namespace) > 63 {
		namespace = strings.Trim(namespace[:63], "-")
	}
	if namespace == "" {
		return "default"
	}
	return namespace
}

func (s *Service) requireDeploymentRecordForVerification(ctx context.Context, applicationID shared.ID, stageKey string, freightID shared.ID) error {
	promotions, err := s.repo.ListPromotionsByApplication(ctx, applicationID, shared.PageRequest{Page: 1, PageSize: 1000})
	if err != nil {
		return err
	}
	for _, promotion := range promotions.Items {
		promotionStageKey := normalizeStageKey(promotion.TargetStageKey)
		if promotionStageKey == "" {
			stage, err := s.repo.GetDeliveryStage(ctx, promotion.TargetStageID)
			if err == nil {
				promotionStageKey = normalizeStageKey(stage.Name)
			}
		}
		if promotion.FreightID == freightID && promotionStageKey == stageKey && (promotion.Status == PromotionManifestUpdated || promotion.Status == PromotionSyncing || promotion.Status == PromotionHealthy) {
			return nil
		}
	}
	return shared.NewError(shared.CodeFailedPrecondition, "stage has no deployment record for freight")
}

func (s *Service) newPromotion(ctx context.Context, freight Freight, stage DeliveryStage, stageKey string, namespaceOverride string, actorID shared.ID, message string, rollback bool, from shared.ID, autoPublish bool) (Promotion, error) {
	id, err := s.ids.NewID("promotion")
	if err != nil {
		return Promotion{}, err
	}
	now := s.clock.Now()
	promotion := Promotion{ID: id, TenantID: freight.TenantID, ProjectID: freight.ProjectID, ApplicationID: freight.ApplicationID, FreightID: freight.ID, TargetStageID: stage.ID, TargetStageKey: stageKey, NamespaceOverride: namespaceOverride, Status: PromotionCreated, AutoPublish: autoPublish, IsRollback: rollback, RollbackFromFreightID: from, CreatedBy: actorID, Message: message, CreatedAt: now, UpdatedAt: now}
	requiresApproval := isProdStage(stage.Name)
	if templateStage, err := s.templateStageForPromotion(ctx, freight.TenantID, stageKey); err == nil {
		requiresApproval = templateStage.RequiresApproval
	}
	if requiresApproval {
		promotion.Status = PromotionPendingApproval
	}
	if err := s.repo.CreatePromotion(ctx, promotion); err != nil {
		return Promotion{}, err
	}
	action := "promotion.create"
	summary := "创建发布晋级"
	if rollback {
		action = "promotion.rollback"
		summary = "创建回滚晋级"
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: actorID, Action: action, ResourceType: "promotion", ResourceID: promotion.ID, Result: "succeeded", Summary: summary, OccurredAt: now})
	return promotion, nil
}

func (s *Service) templateStageForPromotion(ctx context.Context, tenantID shared.ID, stageKey string) (DeliveryFlowTemplateStage, error) {
	template, err := s.ensureDeliveryFlowTemplate(ctx, tenantID)
	if err != nil {
		return DeliveryFlowTemplateStage{}, err
	}
	normalized := normalizeStageKey(stageKey)
	for _, stage := range template.Stages {
		if stage.StageKey == normalized {
			return stage, nil
		}
	}
	return DeliveryFlowTemplateStage{}, shared.NewError(shared.CodeNotFound, "stage template not found")
}

func (s *Service) createApproval(ctx context.Context, promotion Promotion) (Promotion, error) {
	id, err := s.ids.NewID("promotion_approval")
	if err != nil {
		return Promotion{}, err
	}
	now := s.clock.Now()
	approval := PromotionApproval{ID: id, TenantID: promotion.TenantID, ProjectID: promotion.ProjectID, PromotionID: promotion.ID, Status: PromotionApprovalPending, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreatePromotionApproval(ctx, approval); err != nil {
		return Promotion{}, err
	}
	return promotion, nil
}

func (s *Service) applyPromotion(ctx context.Context, promotion Promotion, targetClusters []GitOpsPromotionTargetCluster) (Promotion, error) {
	items, err := s.repo.ListFreightItems(ctx, promotion.FreightID)
	if err != nil {
		return Promotion{}, err
	}
	if len(items) == 0 {
		return Promotion{}, shared.NewError(shared.CodeFailedPrecondition, "freight has no items")
	}
	now := s.clock.Now()
	promotion.Status = PromotionManifestUpdating
	promotion.UpdatedAt = now
	if err := s.repo.UpdatePromotion(ctx, promotion); err != nil {
		return Promotion{}, err
	}
	artifacts := make([]GitOpsArtifactSpec, 0, len(items))
	for i, item := range items {
		spec := GitOpsArtifactSpec{WorkloadID: item.WorkloadID, ContainerName: normalizeContainerName(item.ContainerName), Name: item.Name, SourceKey: item.SourceKey, URI: item.URI, Repository: item.ImageRepository, Tag: item.ImageTag, Digest: item.Digest, IsPrimary: i == 0 || item.Type == FreightItemApplicationRelease}
		if !item.ImageBundleID.IsZero() {
			images, err := s.repo.ListImageBundleImages(ctx, item.ImageBundleID)
			if err != nil {
				return Promotion{}, err
			}
			spec.Variants = imageBundleVariants(images)
		}
		artifacts = append(artifacts, spec)
	}
	result, err := s.gitopsOrError().ApplyPromotion(ctx, GitOpsPromotionSpec{PromotionID: promotion.ID, FreightID: promotion.FreightID, ApplicationID: promotion.ApplicationID, StageKey: promotion.TargetStageKey, TargetClusters: targetClusters, Artifacts: artifacts, IsRollback: promotion.IsRollback})
	if err != nil {
		promotion.Status = PromotionFailed
		promotion.Message = strings.TrimSpace(err.Error())
		promotion.UpdatedAt = s.clock.Now()
		promotion.CompletedAt = &promotion.UpdatedAt
		_ = s.repo.UpdatePromotion(ctx, promotion)
		return Promotion{}, err
	}
	now = s.clock.Now()
	promotion.Status = PromotionManifestUpdated
	promotion.ManifestRevision = strings.TrimSpace(result.ManifestRevision)
	promotion.UpdatedAt = now
	if err := s.repo.UpdatePromotion(ctx, promotion); err != nil {
		return Promotion{}, err
	}
	if !promotion.IsRollback {
		if _, err := s.createManualPromotionsForEligibleStages(ctx, promotion.ApplicationID, promotion.FreightID, deliverySystemActorID, "promotion_applied:"+promotion.TargetStageKey); err != nil {
			_ = s.audit.Log(ctx, AuditEvent{ActorID: deliverySystemActorID, Action: "promotion.auto_create", ResourceType: "freight", ResourceID: promotion.FreightID, Result: "failed", Summary: "发布完成后自动创建手动 Promotion 失败", Details: map[string]string{"stage_key": promotion.TargetStageKey, "reason": err.Error()}, OccurredAt: s.clock.Now()})
		}
	}
	_ = s.publish(ctx, "PromotionManifestUpdated", s.clock.Now(), map[string]any{"application_id": promotion.ApplicationID, "stage_key": promotion.TargetStageKey, "promotion_id": promotion.ID, "freight_id": promotion.FreightID})
	return promotion, nil
}

func (s *Service) pipelineFreightItem(ctx context.Context, item FreightItem, input CreateFreightItemInput) (FreightItem, error) {
	if input.ReleaseID.IsZero() && input.BuildArtifactID.IsZero() {
		return FreightItem{}, shared.NewError(shared.CodeInvalidArgument, "release_id or build_artifact_id is required")
	}
	var release Release
	var err error
	if !input.ReleaseID.IsZero() {
		release, err = s.repo.GetRelease(ctx, input.ReleaseID)
		if err != nil {
			return FreightItem{}, err
		}
		if release.ApplicationID != item.ApplicationID || release.WorkloadID != item.WorkloadID {
			return FreightItem{}, shared.NewError(shared.CodeInvalidArgument, "pipeline artifact does not belong to workload")
		}
		if normalizeContainerName(release.ContainerName) != normalizeContainerName(item.ContainerName) {
			return FreightItem{}, shared.NewError(shared.CodeInvalidArgument, "pipeline artifact does not belong to workload container")
		}
		item.ReleaseID = release.ID
		item.BuildArtifactID = release.BuildArtifactID
		item.ImageBundleID = release.ImageBundleID
		item.URI = release.ImageURI
		item.ImageRef = release.ImageURI
		item.ImageRepository = release.ImageRepository
		item.ImageTag = release.ImageTag
		item.Digest = release.ImageDigest
		item.SourceKey = release.PipelineName
		return item, nil
	}
	artifact, err := s.buildsOrError().GetBuildArtifact(ctx, input.BuildArtifactID)
	if err != nil {
		return FreightItem{}, err
	}
	if artifact.ApplicationID != item.ApplicationID || artifact.WorkloadID != item.WorkloadID {
		return FreightItem{}, shared.NewError(shared.CodeInvalidArgument, "pipeline artifact does not belong to workload")
	}
	if normalizeContainerName(artifact.ContainerName) != normalizeContainerName(item.ContainerName) {
		return FreightItem{}, shared.NewError(shared.CodeInvalidArgument, "pipeline artifact does not belong to workload container")
	}
	repository, tag := splitImageRepositoryTag(artifact.URI)
	item.BuildArtifactID = artifact.ID
	item.URI = artifact.URI
	item.ImageRef = artifact.URI
	item.ImageRepository = repository
	item.ImageTag = tag
	item.Digest = artifact.Digest
	item.SourceKey = artifact.SourceKey
	return item, nil
}

func (s *Service) validateFreightComplete(ctx context.Context, applicationID shared.ID, freightID shared.ID) error {
	workloads, err := s.workloadsOrError().ListEnabledWorkloads(ctx, applicationID)
	if err != nil {
		return err
	}
	required := freightTargets(workloads)
	enabled := map[shared.ID]struct{}{}
	for _, workload := range workloads {
		enabled[workload.ID] = struct{}{}
	}
	items, err := s.repo.ListFreightItems(ctx, freightID)
	if err != nil {
		return err
	}
	if len(items) != len(required) {
		return shared.NewError(shared.CodeFailedPrecondition, "freight must include every enabled workload container")
	}
	seen := map[string]struct{}{}
	for _, item := range items {
		if _, ok := enabled[item.WorkloadID]; !ok {
			return shared.NewError(shared.CodeFailedPrecondition, "freight item workload is not enabled")
		}
		key := freightTargetKey(item.WorkloadID, item.ContainerName)
		if _, ok := required[key]; !ok {
			return shared.NewError(shared.CodeFailedPrecondition, "freight item workload container is not enabled")
		}
		if _, ok := seen[key]; ok {
			return shared.NewError(shared.CodeFailedPrecondition, "freight item workload container is duplicated")
		}
		seen[key] = struct{}{}
	}
	if len(seen) != len(required) {
		return shared.NewError(shared.CodeFailedPrecondition, "freight must include every enabled workload container")
	}
	return nil
}

func freightTargets(workloads []WorkloadRef) map[string]WorkloadContainerRef {
	out := map[string]WorkloadContainerRef{}
	for _, workload := range workloads {
		containers := workload.Containers
		if len(containers) == 0 {
			containers = []WorkloadContainerRef{{Name: "app"}}
		}
		for _, container := range containers {
			name := normalizeContainerName(container.Name)
			out[freightTargetKey(workload.ID, name)] = WorkloadContainerRef{Name: name, PipelineID: container.PipelineID}
		}
	}
	return out
}

func freightTargetKey(workloadID shared.ID, containerName string) string {
	return workloadID.String() + "/" + normalizeContainerName(containerName)
}

func normalizeContainerName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "app"
	}
	return name
}

func freightItemDisplayName(workload WorkloadRef, containerName string) string {
	base := firstNonEmpty(workload.DisplayName, workload.Name)
	if normalizeContainerName(containerName) == "app" {
		return base
	}
	return base + " / " + normalizeContainerName(containerName)
}

func (s *Service) validateStageOrder(ctx context.Context, freight Freight, target DeliveryStage) error {
	app, err := s.appsOrError().GetApplication(ctx, freight.ApplicationID)
	if err != nil {
		return err
	}
	template, err := s.ensureDeliveryFlowTemplate(ctx, app.TenantID)
	if err != nil {
		return err
	}
	stageByKey := map[string]DeliveryFlowTemplateStage{}
	for _, stage := range template.Stages {
		stageByKey[stage.StageKey] = stage
	}
	parentKeys := []string{}
	for _, edge := range template.Edges {
		if edge.ToStageKey == normalizeStageKey(target.Name) {
			parentKeys = append(parentKeys, edge.FromStageKey)
		}
	}
	if len(parentKeys) == 0 {
		return nil
	}
	flow, err := s.repo.FindDeliveryFlowByApplication(ctx, freight.ApplicationID)
	if err != nil {
		return err
	}
	stages, err := s.repo.ListDeliveryStages(ctx, flow.ID)
	if err != nil {
		return err
	}
	deliveryStageByName := map[string]DeliveryStage{}
	for _, stage := range stages {
		deliveryStageByName[normalizeStageKey(stage.Name)] = stage
	}
	promotions, err := s.repo.ListPromotionsByApplication(ctx, freight.ApplicationID, shared.PageRequest{Page: 1, PageSize: 1000})
	if err != nil {
		return err
	}
	for _, parentKey := range parentKeys {
		parentStage, ok := deliveryStageByName[parentKey]
		if !ok {
			return shared.NewError(shared.CodeFailedPrecondition, "freight has not passed previous stage")
		}
		found := false
		for _, promotion := range promotions.Items {
			promotionStageKey := normalizeStageKey(promotion.TargetStageKey)
			if promotionStageKey == "" {
				promotionStageKey = normalizeStageKey(parentStage.Name)
			}
			if promotion.FreightID == freight.ID && promotionStageKey == parentKey && (promotion.Status == PromotionManifestUpdated || promotion.Status == PromotionHealthy || promotion.Status == PromotionSyncing) {
				found = true
				break
			}
		}
		if !found {
			return shared.NewError(shared.CodeFailedPrecondition, "freight has not passed previous stage")
		}
		if stageByKey[parentKey].RequiresVerification {
			verification, err := s.repo.FindStageVerification(ctx, freight.ApplicationID, parentKey, freight.ID)
			if err != nil {
				return shared.NewError(shared.CodeFailedPrecondition, "freight has not passed previous stage")
			}
			if verification.Status != StageVerificationPassed {
				return shared.NewError(shared.CodeFailedPrecondition, "freight has not passed previous stage")
			}
		}
	}
	return nil
}

func (s *Service) buildTemplateEdges(tenantID shared.ID, templateID shared.ID, inputs []DeliveryFlowTemplateEdgeInput, stageByKey map[string]DeliveryFlowTemplateStage, now time.Time) ([]DeliveryFlowTemplateEdge, error) {
	edges := make([]DeliveryFlowTemplateEdge, 0, len(inputs))
	seen := map[string]struct{}{}
	graph := map[string][]string{}
	for _, input := range inputs {
		from := normalizeStageKey(input.FromStageKey)
		to := normalizeStageKey(input.ToStageKey)
		if from == "" || to == "" {
			return nil, shared.NewError(shared.CodeInvalidArgument, "edge stage key is required")
		}
		if from == to {
			return nil, shared.NewError(shared.CodeInvalidArgument, "edge cannot point to itself")
		}
		fromStage, ok := stageByKey[from]
		if !ok {
			return nil, shared.NewError(shared.CodeInvalidArgument, "edge source stage not found")
		}
		toStage, ok := stageByKey[to]
		if !ok {
			return nil, shared.NewError(shared.CodeInvalidArgument, "edge target stage not found")
		}
		if fromStage.Status == DeliveryFlowTemplateStageDisabled || toStage.Status == DeliveryFlowTemplateStageDisabled {
			return nil, shared.NewError(shared.CodeInvalidArgument, "edge cannot reference disabled stage")
		}
		key := from + "->" + to
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		id, err := s.ids.NewID("delivery_flow_template_edge")
		if err != nil {
			return nil, err
		}
		edges = append(edges, DeliveryFlowTemplateEdge{ID: id, TenantID: tenantID, TemplateID: templateID, FromStageKey: from, ToStageKey: to, CreatedAt: now, UpdatedAt: now})
		graph[from] = append(graph[from], to)
	}
	if hasCycle(graph, stageByKey) {
		return nil, shared.NewError(shared.CodeInvalidArgument, "delivery flow template graph has cycle")
	}
	return edges, nil
}

func validateTemplateGraphInput(stages []SaveDeliveryFlowTemplateStageInput, edges []DeliveryFlowTemplateEdgeInput) error {
	stageByKey := map[string]DeliveryFlowTemplateStage{}
	for _, input := range stages {
		key := normalizeStageKey(input.StageKey)
		if key == "" {
			return shared.NewError(shared.CodeInvalidArgument, "stage_key is required")
		}
		if input.LayoutColumn < 0 {
			return shared.NewError(shared.CodeInvalidArgument, "layout_column is invalid")
		}
		if _, ok := stageByKey[key]; ok {
			return shared.NewError(shared.CodeConflict, "stage is duplicated")
		}
		status := input.Status
		if status == "" {
			status = DeliveryFlowTemplateStageEnabled
		}
		stageByKey[key] = DeliveryFlowTemplateStage{StageKey: key, Status: status}
	}
	graph := map[string][]string{}
	seen := map[string]struct{}{}
	for _, input := range edges {
		from := normalizeStageKey(input.FromStageKey)
		to := normalizeStageKey(input.ToStageKey)
		if from == "" || to == "" || from == to {
			return shared.NewError(shared.CodeInvalidArgument, "edge is invalid")
		}
		fromStage, ok := stageByKey[from]
		if !ok {
			return shared.NewError(shared.CodeInvalidArgument, "edge source stage not found")
		}
		toStage, ok := stageByKey[to]
		if !ok {
			return shared.NewError(shared.CodeInvalidArgument, "edge target stage not found")
		}
		if fromStage.Status == DeliveryFlowTemplateStageDisabled || toStage.Status == DeliveryFlowTemplateStageDisabled {
			return shared.NewError(shared.CodeInvalidArgument, "edge cannot reference disabled stage")
		}
		key := from + "->" + to
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		graph[from] = append(graph[from], to)
	}
	if hasCycle(graph, stageByKey) {
		return shared.NewError(shared.CodeInvalidArgument, "delivery flow template graph has cycle")
	}
	return nil
}

func hasCycle(graph map[string][]string, stages map[string]DeliveryFlowTemplateStage) bool {
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var visit func(string) bool
	visit = func(node string) bool {
		if visiting[node] {
			return true
		}
		if visited[node] {
			return false
		}
		visiting[node] = true
		for _, next := range graph[node] {
			if visit(next) {
				return true
			}
		}
		visiting[node] = false
		visited[node] = true
		return false
	}
	for key := range stages {
		if visit(key) {
			return true
		}
	}
	return false
}

func (s *Service) check(ctx context.Context, actor identityaccess.Subject, app ApplicationRef, action identityaccess.Permission) error {
	if actor.ID.IsZero() {
		return shared.NewError(shared.CodeUnauthenticated, "actor is required")
	}
	if s.permission == nil {
		return nil
	}
	return s.permission.Check(ctx, actor, identityaccess.ResourceScope{Kind: identityaccess.ScopeApplication, TenantID: app.TenantID, ProjectID: app.ProjectID, ApplicationID: app.ID}, action)
}

func (s *Service) publish(ctx context.Context, eventType string, occurredAt time.Time, payload any) error {
	id, err := s.ids.NewID("evt")
	if err != nil {
		return err
	}
	event, err := shared.NewDomainEvent(id, eventType, occurredAt, payload)
	if err != nil {
		return err
	}
	return s.events.Publish(ctx, event)
}

func (s *Service) buildsOrError() BuildQuery {
	if s.builds == nil {
		return failingBuildQuery{}
	}
	return s.builds
}
func (s *Service) appsOrError() ApplicationQuery {
	if s.apps == nil {
		return failingAppQuery{}
	}
	return s.apps
}
func (s *Service) workloadsOrError() WorkloadQuery {
	if s.workloads == nil {
		return failingWorkloadQuery{}
	}
	return s.workloads
}
func (s *Service) gitopsOrError() GitOpsDeploymentCommand {
	if s.gitops == nil {
		return failingGitOps{}
	}
	return s.gitops
}

func (s *Service) resolveBuildArtifacts(ctx context.Context, buildRunID shared.ID, ids []shared.ID) ([]BuildArtifactRef, error) {
	if len(ids) == 0 {
		artifacts, err := s.buildsOrError().ListBuildArtifacts(ctx, buildRunID)
		if err != nil {
			return nil, err
		}
		if len(artifacts) == 0 {
			return nil, shared.NewError(shared.CodeInvalidArgument, "build artifacts is required")
		}
		return artifacts, nil
	}
	artifacts := make([]BuildArtifactRef, 0, len(ids))
	for _, id := range ids {
		artifact, err := s.buildsOrError().GetBuildArtifact(ctx, id)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	if len(artifacts) == 0 {
		return nil, shared.NewError(shared.CodeInvalidArgument, "build artifacts is required")
	}
	return artifacts, nil
}

func primaryArtifact(artifacts []BuildArtifactRef) BuildArtifactRef {
	for _, artifact := range artifacts {
		if artifact.IsPrimary {
			return artifact
		}
	}
	if len(artifacts) > 0 {
		return artifacts[0]
	}
	return BuildArtifactRef{}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func imageBundleVariants(images []ImageBundleImage) []GitOpsImageVariant {
	variants := make([]GitOpsImageVariant, 0, len(images))
	for _, image := range images {
		variants = append(variants, GitOpsImageVariant{
			URI:                    image.URI,
			Repository:             image.ImageRepository,
			Tag:                    image.ImageTag,
			Digest:                 image.Digest,
			RuntimeEnvironmentID:   image.RuntimeEnvironmentID,
			RuntimeEnvironmentName: image.RuntimeEnvironmentName,
			SelectorLabels:         cleanStringMap(image.SelectorLabels),
			IsPrimary:              image.IsPrimary,
		})
	}
	return variants
}

func cleanStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := map[string]string{}
	for key, value := range values {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func releaseVersion(commit string, fallback shared.ID) string {
	if commit = strings.TrimSpace(commit); commit != "" {
		if len(commit) > 12 {
			commit = commit[:12]
		}
		return commit
	}
	return fmt.Sprintf("build-%s", fallback)
}

type parsedImageRef struct {
	ref        string
	repository string
	tag        string
	digest     string
}

func parseCustomImageRef(value string) (parsedImageRef, error) {
	ref := strings.TrimSpace(value)
	if ref == "" || strings.ContainsAny(ref, " \t\r\n") || strings.HasPrefix(ref, "-") {
		return parsedImageRef{}, shared.NewError(shared.CodeInvalidArgument, "custom image ref is invalid")
	}
	repository, tag := splitImageRepositoryTag(ref)
	digest := ""
	if at := strings.LastIndex(ref, "@"); at >= 0 {
		repository = ref[:at]
		digest = ref[at+1:]
		tag = ""
		if !strings.HasPrefix(digest, "sha256:") || len(strings.TrimPrefix(digest, "sha256:")) < 16 {
			return parsedImageRef{}, shared.NewError(shared.CodeInvalidArgument, "custom image digest is invalid")
		}
	}
	if repository == "" || strings.Contains(repository, "://") || strings.HasPrefix(repository, "/") || strings.HasSuffix(repository, "/") {
		return parsedImageRef{}, shared.NewError(shared.CodeInvalidArgument, "custom image ref is invalid")
	}
	if digest == "" && tag == "" {
		return parsedImageRef{}, shared.NewError(shared.CodeInvalidArgument, "custom image tag or digest is required")
	}
	return parsedImageRef{ref: ref, repository: repository, tag: tag, digest: digest}, nil
}

func splitImageRepositoryTag(image string) (string, string) {
	image = strings.TrimSpace(image)
	if at := strings.LastIndex(image, "@"); at >= 0 {
		return image[:at], ""
	}
	lastSlash := strings.LastIndex(image, "/")
	lastColon := strings.LastIndex(image, ":")
	if lastColon > lastSlash {
		return image[:lastColon], image[lastColon+1:]
	}
	return image, ""
}

func normalizeStageKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return ""
	}
	return value
}

func normalizeTemplateLayout(template *DeliveryFlowTemplate) {
	if len(template.Stages) == 0 {
		return
	}
	needsFallback := len(template.Stages) > 1
	for _, stage := range template.Stages {
		if stage.LayoutColumn != 0 || stage.LayoutRow != 0 {
			needsFallback = false
			break
		}
	}
	if needsFallback {
		depths := templateStageDepths(template.Stages, template.Edges)
		rowByColumn := map[int]int{}
		for idx := range template.Stages {
			column := depths[template.Stages[idx].StageKey]
			row := rowByColumn[column]
			rowByColumn[column] = row + 1
			template.Stages[idx].LayoutColumn = column
			template.Stages[idx].LayoutRow = symmetricRow(row)
		}
	}
	for idx := range template.Stages {
		if template.Stages[idx].LayoutColumn < 0 {
			template.Stages[idx].LayoutColumn = 0
		}
		if strings.TrimSpace(template.Stages[idx].Color) == "" {
			template.Stages[idx].Color = colorForLayoutColumn(template.Stages[idx].LayoutColumn)
		}
	}
}

func templateStageDepths(stages []DeliveryFlowTemplateStage, edges []DeliveryFlowTemplateEdge) map[string]int {
	stageKeys := map[string]struct{}{}
	for _, stage := range stages {
		stageKeys[stage.StageKey] = struct{}{}
	}
	parents := map[string][]string{}
	for _, edge := range edges {
		if _, ok := stageKeys[edge.FromStageKey]; !ok {
			continue
		}
		if _, ok := stageKeys[edge.ToStageKey]; !ok {
			continue
		}
		parents[edge.ToStageKey] = append(parents[edge.ToStageKey], edge.FromStageKey)
	}
	memo := map[string]int{}
	visiting := map[string]bool{}
	var depthOf func(string) int
	depthOf = func(key string) int {
		if value, ok := memo[key]; ok {
			return value
		}
		if visiting[key] {
			return 0
		}
		visiting[key] = true
		maxDepth := 0
		for _, parent := range parents[key] {
			if depth := depthOf(parent) + 1; depth > maxDepth {
				maxDepth = depth
			}
		}
		visiting[key] = false
		memo[key] = maxDepth
		return maxDepth
	}
	for _, stage := range stages {
		depthOf(stage.StageKey)
	}
	return memo
}

func symmetricRow(index int) int {
	if index <= 0 {
		return 0
	}
	value := (index + 1) / 2
	if index%2 == 1 {
		return -value
	}
	return value
}

func colorForLayoutColumn(column int) string {
	if column < 0 {
		column = 0
	}
	return deliveryFlowColumnColors[column%len(deliveryFlowColumnColors)]
}

func colorForStageInput(color string, column int) string {
	if value := strings.TrimSpace(color); value != "" {
		return value
	}
	return colorForLayoutColumn(column)
}

func cleanRoles(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		role := strings.TrimSpace(value)
		if role == "" {
			continue
		}
		if _, ok := seen[role]; ok {
			continue
		}
		seen[role] = struct{}{}
		out = append(out, role)
	}
	return out
}

type failingBuildQuery struct{}

func (failingBuildQuery) GetBuildRun(context.Context, shared.ID) (BuildRunRef, error) {
	return BuildRunRef{}, shared.NewError(shared.CodeFailedPrecondition, "build query port is required")
}
func (failingBuildQuery) ListBuildRuns(context.Context, shared.ID, shared.PageRequest) (shared.PageResult[BuildRunRef], error) {
	return shared.PageResult[BuildRunRef]{}, shared.NewError(shared.CodeFailedPrecondition, "build query port is required")
}
func (failingBuildQuery) GetBuildArtifact(context.Context, shared.ID) (BuildArtifactRef, error) {
	return BuildArtifactRef{}, shared.NewError(shared.CodeFailedPrecondition, "build query port is required")
}
func (failingBuildQuery) ListBuildArtifacts(context.Context, shared.ID) ([]BuildArtifactRef, error) {
	return nil, shared.NewError(shared.CodeFailedPrecondition, "build query port is required")
}

type failingAppQuery struct{}

func (failingAppQuery) GetApplication(context.Context, shared.ID) (ApplicationRef, error) {
	return ApplicationRef{}, shared.NewError(shared.CodeFailedPrecondition, "application query port is required")
}

type failingWorkloadQuery struct{}

func (failingWorkloadQuery) ListEnabledWorkloads(context.Context, shared.ID) ([]WorkloadRef, error) {
	return nil, shared.NewError(shared.CodeFailedPrecondition, "workload query port is required")
}

type failingGitOps struct{}

func (failingGitOps) ApplyPromotion(context.Context, GitOpsPromotionSpec) (GitOpsPromotionResult, error) {
	return GitOpsPromotionResult{}, shared.NewError(shared.CodeFailedPrecondition, "gitops deployment command is required")
}
