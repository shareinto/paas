package delivery

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/shared"
)

type Service struct {
	repo       Repository
	builds     BuildQuery
	apps       ApplicationQuery
	workloads  WorkloadQuery
	envs       EnvironmentQuery
	gitops     GitOpsDeploymentCommand
	permission PermissionChecker
	audit      AuditLogger
	events     EventPublisher
	ids        shared.IDGenerator
	clock      shared.Clock
}

type Options struct {
	Repository        Repository
	BuildQuery        BuildQuery
	ApplicationQuery  ApplicationQuery
	WorkloadQuery     WorkloadQuery
	EnvironmentQuery  EnvironmentQuery
	GitOpsDeployment  GitOpsDeploymentCommand
	PermissionChecker PermissionChecker
	Audit             AuditLogger
	EventPublisher    EventPublisher
	IDGenerator       shared.IDGenerator
	Clock             shared.Clock
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
	return &Service{repo: opts.Repository, builds: opts.BuildQuery, apps: opts.ApplicationQuery, workloads: opts.WorkloadQuery, envs: opts.EnvironmentQuery, gitops: opts.GitOpsDeployment, permission: opts.PermissionChecker, audit: audit, events: events, ids: ids, clock: clock}
}

type CreatePromotionInput struct {
	Actor               identityaccess.Subject `json:"actor"`
	FreightID           shared.ID              `json:"freight_id"`
	TargetEnvironmentID shared.ID              `json:"target_environment_id"`
	Message             string                 `json:"message"`
}

type CreateRollbackPromotionInput struct {
	Actor               identityaccess.Subject `json:"actor"`
	TargetFreightID     shared.ID              `json:"target_freight_id"`
	CurrentFreightID    shared.ID              `json:"current_freight_id"`
	TargetEnvironmentID shared.ID              `json:"target_environment_id"`
	Message             string                 `json:"message"`
}

type ApprovalInput struct {
	Actor       identityaccess.Subject `json:"actor"`
	PromotionID shared.ID              `json:"promotion_id"`
	Comment     string                 `json:"comment"`
}

type CreateFreightInput struct {
	Actor         identityaccess.Subject   `json:"actor"`
	ApplicationID shared.ID                `json:"application_id"`
	Name          string                   `json:"name"`
	Description   string                   `json:"description"`
	Items         []CreateFreightItemInput `json:"items"`
}

type CreateFreightItemInput struct {
	WorkloadID      shared.ID       `json:"workload_id"`
	SourceType      FreightItemType `json:"source_type"`
	ReleaseID       shared.ID       `json:"release_id"`
	BuildArtifactID shared.ID       `json:"build_artifact_id"`
	ImageRef        string          `json:"image_ref"`
}

type FreightCreationContext struct {
	EnabledWorkloads          []WorkloadRef                  `json:"enabled_workloads"`
	LatestReleasesByWorkload  map[shared.ID]Release          `json:"latest_releases_by_workload"`
	LatestArtifactsByWorkload map[shared.ID]BuildArtifactRef `json:"latest_artifacts_by_workload"`
	StageEligibility          map[shared.ID][]shared.ID      `json:"stage_eligibility"`
}

func (s *Service) HandleBuildSucceeded(ctx context.Context, payload BuildSucceededPayload) (Release, error) {
	if payload.BuildRunID.IsZero() || payload.ApplicationID.IsZero() || payload.WorkloadID.IsZero() {
		return Release{}, shared.NewError(shared.CodeInvalidArgument, "build_run_id, application_id and workload_id are required")
	}
	if existing, err := s.repo.FindReleaseByBuildRun(ctx, payload.BuildRunID); err == nil {
		return existing, nil
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
		if artifact.BuildRunID != run.ID || artifact.ApplicationID != run.ApplicationID || artifact.WorkloadID != payload.WorkloadID {
			return Release{}, shared.NewError(shared.CodeInvalidArgument, "build payload ownership mismatch")
		}
	}
	primary := primaryArtifact(artifacts)
	if strings.TrimSpace(primary.Digest) == "" || strings.TrimSpace(run.CommitSHA) == "" {
		return Release{}, shared.NewError(shared.CodeFailedPrecondition, "pipeline artifact requires digest and commit")
	}
	now := s.clock.Now()
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
	release := Release{ID: releaseID, TenantID: run.TenantID, ProjectID: run.ProjectID, ApplicationID: run.ApplicationID, WorkloadID: payload.WorkloadID, PipelineID: pipelineID, PipelineName: pipelineName, PipelineDisplayName: pipelineDisplayName, BuildRunID: run.ID, BuildArtifactID: primary.ID, Version: releaseVersion(commit, run.ID), CommitSHA: commit, ImageURI: imageURI, ImageRepository: imageRepository, ImageTag: imageTag, ImageDigest: imageDigest, SourceType: ReleaseSourcePipelineArtifact, Status: ReleaseReady, CreatedAt: now}
	if err := s.repo.CreateRelease(ctx, release); err != nil {
		return Release{}, err
	}
	if _, err := s.ensureDefaultFlow(ctx, run.ApplicationID); err != nil {
		return Release{}, err
	}
	return release, nil
}

func (s *Service) CreatePromotion(ctx context.Context, input CreatePromotionInput) (Promotion, error) {
	freight, err := s.repo.GetFreight(ctx, input.FreightID)
	if err != nil {
		return Promotion{}, err
	}
	app, env, stage, err := s.validatePromotionTarget(ctx, freight.ApplicationID, input.TargetEnvironmentID)
	if err != nil {
		return Promotion{}, err
	}
	if err := s.check(ctx, input.Actor, app, "deployment:create"); err != nil {
		return Promotion{}, err
	}
	if err := s.validateFreightComplete(ctx, freight.ApplicationID, freight.ID); err != nil {
		return Promotion{}, err
	}
	if err := s.validateStageOrder(ctx, freight, stage); err != nil {
		return Promotion{}, err
	}
	promotion, err := s.newPromotion(ctx, freight, stage, env, input.Actor.ID, strings.TrimSpace(input.Message), false, "")
	if err != nil {
		return Promotion{}, err
	}
	if isProdStage(stage.Name) {
		return s.createApproval(ctx, promotion)
	}
	return s.applyPromotion(ctx, promotion)
}

func (s *Service) CreateFreight(ctx context.Context, input CreateFreightInput) (Freight, error) {
	app, err := s.appsOrError().GetApplication(ctx, input.ApplicationID)
	if err != nil {
		return Freight{}, err
	}
	if err := s.check(ctx, input.Actor, app, "freight:create"); err != nil {
		return Freight{}, err
	}
	workloads, err := s.workloadsOrError().ListEnabledWorkloads(ctx, app.ID)
	if err != nil {
		return Freight{}, err
	}
	if len(workloads) == 0 {
		return Freight{}, shared.NewError(shared.CodeFailedPrecondition, "enabled workload is required")
	}
	if len(input.Items) != len(workloads) {
		return Freight{}, shared.NewError(shared.CodeFailedPrecondition, "freight must include every enabled workload")
	}
	enabled := map[shared.ID]WorkloadRef{}
	for _, workload := range workloads {
		enabled[workload.ID] = workload
	}
	seen := map[shared.ID]struct{}{}
	now := s.clock.Now()
	freightID, err := s.ids.NewID("freight")
	if err != nil {
		return Freight{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = freightID.String()
	}
	freight := Freight{ID: freightID, TenantID: app.TenantID, ProjectID: app.ProjectID, ApplicationID: app.ID, Name: name, Status: FreightAvailable, CreatedAt: now}
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
		if _, ok := seen[itemInput.WorkloadID]; ok {
			return Freight{}, shared.NewError(shared.CodeConflict, "freight item workload is duplicated")
		}
		seen[itemInput.WorkloadID] = struct{}{}
		itemID, err := s.ids.NewID("freight_item")
		if err != nil {
			return Freight{}, err
		}
		item := FreightItem{ID: itemID, TenantID: app.TenantID, ProjectID: app.ProjectID, FreightID: freight.ID, ApplicationID: app.ID, WorkloadID: workload.ID, SourceType: itemInput.SourceType, Type: itemInput.SourceType, Name: firstNonEmpty(workload.DisplayName, workload.Name), CreatedAt: now}
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
	if len(seen) != len(enabled) {
		return Freight{}, shared.NewError(shared.CodeFailedPrecondition, "freight must include every enabled workload")
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

func (s *Service) GetFreightCreationContext(ctx context.Context, applicationID shared.ID) (FreightCreationContext, error) {
	workloads, err := s.workloadsOrError().ListEnabledWorkloads(ctx, applicationID)
	if err != nil {
		return FreightCreationContext{}, err
	}
	releases, err := s.repo.ListReleasesByApplication(ctx, applicationID, shared.PageRequest{Page: 1, PageSize: 1000})
	if err != nil {
		return FreightCreationContext{}, err
	}
	ctxOut := FreightCreationContext{
		EnabledWorkloads:          workloads,
		LatestReleasesByWorkload:  map[shared.ID]Release{},
		LatestArtifactsByWorkload: map[shared.ID]BuildArtifactRef{},
		StageEligibility:          map[shared.ID][]shared.ID{},
	}
	for _, release := range releases.Items {
		if _, ok := ctxOut.LatestReleasesByWorkload[release.WorkloadID]; ok {
			continue
		}
		ctxOut.LatestReleasesByWorkload[release.WorkloadID] = release
		if artifact, err := s.buildsOrError().GetBuildArtifact(ctx, release.BuildArtifactID); err == nil {
			ctxOut.LatestArtifactsByWorkload[release.WorkloadID] = artifact
		}
	}
	if flow, err := s.ensureDefaultFlow(ctx, applicationID); err == nil {
		if stages, err := s.repo.ListDeliveryStages(ctx, flow.ID); err == nil {
			for _, stage := range stages {
				freights, err := s.ListEligibleFreights(ctx, applicationID, stage.ID)
				if err != nil {
					continue
				}
				ids := make([]shared.ID, 0, len(freights))
				for _, freight := range freights {
					ids = append(ids, freight.ID)
				}
				ctxOut.StageEligibility[stage.ID] = ids
			}
		}
	}
	return ctxOut, nil
}

func (s *Service) ListEligibleFreights(ctx context.Context, applicationID shared.ID, stageID shared.ID) ([]Freight, error) {
	stage, err := s.repo.GetDeliveryStage(ctx, stageID)
	if err != nil {
		return nil, err
	}
	if stage.ApplicationID != applicationID {
		return nil, shared.NewError(shared.CodeInvalidArgument, "stage does not belong to application")
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

func (s *Service) CreateRollbackPromotion(ctx context.Context, input CreateRollbackPromotionInput) (Promotion, error) {
	target, err := s.repo.GetFreight(ctx, input.TargetFreightID)
	if err != nil {
		return Promotion{}, err
	}
	if !input.CurrentFreightID.IsZero() {
		if _, err := s.repo.GetFreight(ctx, input.CurrentFreightID); err != nil {
			return Promotion{}, err
		}
	}
	app, env, stage, err := s.validatePromotionTarget(ctx, target.ApplicationID, input.TargetEnvironmentID)
	if err != nil {
		return Promotion{}, err
	}
	if err := s.check(ctx, input.Actor, app, "deployment:rollback"); err != nil {
		return Promotion{}, err
	}
	promotion, err := s.newPromotion(ctx, target, stage, env, input.Actor.ID, strings.TrimSpace(input.Message), true, input.CurrentFreightID)
	if err != nil {
		return Promotion{}, err
	}
	if isProdStage(stage.Name) {
		return s.createApproval(ctx, promotion)
	}
	return s.applyPromotion(ctx, promotion)
}

func (s *Service) ApprovePromotion(ctx context.Context, input ApprovalInput) (Promotion, error) {
	promotion, err := s.repo.GetPromotion(ctx, input.PromotionID)
	if err != nil {
		return Promotion{}, err
	}
	if promotion.Status != PromotionPendingApproval {
		return Promotion{}, shared.NewError(shared.CodeFailedPrecondition, "promotion is not pending approval")
	}
	if promotion.CreatedBy == input.Actor.ID {
		return Promotion{}, shared.NewError(shared.CodeFailedPrecondition, "self approval is not allowed")
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
	return s.applyPromotion(ctx, promotion)
}

func (s *Service) RejectPromotion(ctx context.Context, input ApprovalInput) (Promotion, error) {
	promotion, err := s.repo.GetPromotion(ctx, input.PromotionID)
	if err != nil {
		return Promotion{}, err
	}
	if promotion.Status != PromotionPendingApproval {
		return Promotion{}, shared.NewError(shared.CodeFailedPrecondition, "promotion is not pending approval")
	}
	if promotion.CreatedBy == input.Actor.ID {
		return Promotion{}, shared.NewError(shared.CodeFailedPrecondition, "self approval is not allowed")
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
func (s *Service) ListFreights(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[Freight], error) {
	return s.repo.ListFreightsByApplication(ctx, applicationID, page)
}
func (s *Service) GetPromotion(ctx context.Context, id shared.ID) (Promotion, error) {
	return s.repo.GetPromotion(ctx, id)
}
func (s *Service) ListPromotions(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[Promotion], error) {
	return s.repo.ListPromotionsByApplication(ctx, applicationID, page)
}

func (s *Service) ensureDefaultFlow(ctx context.Context, applicationID shared.ID) (DeliveryFlow, error) {
	if flow, err := s.repo.FindDeliveryFlowByApplication(ctx, applicationID); err == nil {
		return flow, nil
	}
	app, err := s.appsOrError().GetApplication(ctx, applicationID)
	if err != nil {
		return DeliveryFlow{}, err
	}
	envs, err := s.envsOrError().ListEnvironments(ctx, applicationID)
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
	order := map[string]int{"dev": 0, "test": 1, "staging": 2, "prod": 3}
	seen := map[string]struct{}{}
	for _, env := range envs {
		idx, ok := order[env.Name]
		if !ok {
			continue
		}
		if _, ok := seen[env.Name]; ok {
			continue
		}
		seen[env.Name] = struct{}{}
		stageID, err := s.ids.NewID("delivery_stage")
		if err != nil {
			return DeliveryFlow{}, err
		}
		stage := DeliveryStage{ID: stageID, TenantID: app.TenantID, ProjectID: app.ProjectID, ApplicationID: app.ID, DeliveryFlowID: flow.ID, EnvironmentID: env.ID, Name: env.Name, Order: idx, RequiresApproval: isProdStage(env.Name), CreatedAt: now, UpdatedAt: now}
		if err := s.repo.CreateDeliveryStage(ctx, stage); err != nil {
			return DeliveryFlow{}, err
		}
	}
	return flow, nil
}

func (s *Service) validatePromotionTarget(ctx context.Context, applicationID shared.ID, environmentID shared.ID) (ApplicationRef, EnvironmentRef, DeliveryStage, error) {
	app, err := s.appsOrError().GetApplication(ctx, applicationID)
	if err != nil {
		return ApplicationRef{}, EnvironmentRef{}, DeliveryStage{}, err
	}
	env, err := s.envsOrError().GetEnvironment(ctx, environmentID)
	if err != nil {
		return ApplicationRef{}, EnvironmentRef{}, DeliveryStage{}, err
	}
	if env.ApplicationID != applicationID {
		return ApplicationRef{}, EnvironmentRef{}, DeliveryStage{}, shared.NewError(shared.CodeInvalidArgument, "environment does not belong to application")
	}
	if env.Status == "pending_cluster_binding" || !env.BindingActive {
		return ApplicationRef{}, EnvironmentRef{}, DeliveryStage{}, shared.NewError(shared.CodeFailedPrecondition, "environment has no active cluster binding")
	}
	if _, err := s.ensureDefaultFlow(ctx, applicationID); err != nil {
		return ApplicationRef{}, EnvironmentRef{}, DeliveryStage{}, err
	}
	stage, err := s.repo.FindDeliveryStageByEnvironment(ctx, applicationID, environmentID)
	if err != nil {
		return ApplicationRef{}, EnvironmentRef{}, DeliveryStage{}, err
	}
	return app, env, stage, nil
}

func (s *Service) newPromotion(ctx context.Context, freight Freight, stage DeliveryStage, env EnvironmentRef, actorID shared.ID, message string, rollback bool, from shared.ID) (Promotion, error) {
	id, err := s.ids.NewID("promotion")
	if err != nil {
		return Promotion{}, err
	}
	now := s.clock.Now()
	promotion := Promotion{ID: id, TenantID: freight.TenantID, ProjectID: freight.ProjectID, ApplicationID: freight.ApplicationID, FreightID: freight.ID, TargetStageID: stage.ID, TargetEnvironmentID: env.ID, Status: PromotionCreated, IsRollback: rollback, RollbackFromFreightID: from, CreatedBy: actorID, Message: message, CreatedAt: now, UpdatedAt: now}
	if isProdStage(stage.Name) {
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

func (s *Service) applyPromotion(ctx context.Context, promotion Promotion) (Promotion, error) {
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
		artifacts = append(artifacts, GitOpsArtifactSpec{WorkloadID: item.WorkloadID, Name: item.Name, SourceKey: item.SourceKey, URI: item.URI, Repository: item.ImageRepository, Tag: item.ImageTag, Digest: item.Digest, IsPrimary: i == 0 || item.Type == FreightItemApplicationRelease})
	}
	result, err := s.gitopsOrError().ApplyPromotion(ctx, GitOpsPromotionSpec{PromotionID: promotion.ID, FreightID: promotion.FreightID, ApplicationID: promotion.ApplicationID, EnvironmentID: promotion.TargetEnvironmentID, Artifacts: artifacts, IsRollback: promotion.IsRollback})
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
	_ = s.publish(ctx, "PromotionManifestUpdated", now, map[string]any{"promotion_id": promotion.ID, "freight_id": promotion.FreightID})
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
		item.ReleaseID = release.ID
		item.BuildArtifactID = release.BuildArtifactID
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
	if strings.TrimSpace(artifact.Digest) == "" {
		return FreightItem{}, shared.NewError(shared.CodeFailedPrecondition, "pipeline artifact requires digest and commit")
	}
	run, err := s.buildsOrError().GetBuildRun(ctx, artifact.BuildRunID)
	if err != nil {
		return FreightItem{}, err
	}
	if strings.TrimSpace(run.CommitSHA) == "" {
		return FreightItem{}, shared.NewError(shared.CodeFailedPrecondition, "pipeline artifact requires digest and commit")
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
	enabled := map[shared.ID]struct{}{}
	for _, workload := range workloads {
		enabled[workload.ID] = struct{}{}
	}
	items, err := s.repo.ListFreightItems(ctx, freightID)
	if err != nil {
		return err
	}
	if len(items) != len(enabled) {
		return shared.NewError(shared.CodeFailedPrecondition, "freight must include every enabled workload")
	}
	seen := map[shared.ID]struct{}{}
	for _, item := range items {
		if _, ok := enabled[item.WorkloadID]; !ok {
			return shared.NewError(shared.CodeFailedPrecondition, "freight item workload is not enabled")
		}
		if _, ok := seen[item.WorkloadID]; ok {
			return shared.NewError(shared.CodeFailedPrecondition, "freight item workload is duplicated")
		}
		seen[item.WorkloadID] = struct{}{}
	}
	if len(seen) != len(enabled) {
		return shared.NewError(shared.CodeFailedPrecondition, "freight must include every enabled workload")
	}
	return nil
}

func (s *Service) validateStageOrder(ctx context.Context, freight Freight, target DeliveryStage) error {
	if target.Order <= 0 {
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
	for _, stage := range stages {
		if stage.Order >= target.Order {
			continue
		}
		promotions, err := s.repo.ListPromotionsByApplication(ctx, freight.ApplicationID, shared.PageRequest{Page: 1, PageSize: 1000})
		if err != nil {
			return err
		}
		found := false
		for _, promotion := range promotions.Items {
			if promotion.FreightID == freight.ID && promotion.TargetStageID == stage.ID && (promotion.Status == PromotionManifestUpdated || promotion.Status == PromotionHealthy || promotion.Status == PromotionSyncing) {
				found = true
				break
			}
		}
		if !found {
			return shared.NewError(shared.CodeFailedPrecondition, "freight has not passed previous stage")
		}
	}
	return nil
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
func (s *Service) envsOrError() EnvironmentQuery {
	if s.envs == nil {
		return failingEnvQuery{}
	}
	return s.envs
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

type failingBuildQuery struct{}

func (failingBuildQuery) GetBuildRun(context.Context, shared.ID) (BuildRunRef, error) {
	return BuildRunRef{}, shared.NewError(shared.CodeFailedPrecondition, "build query port is required")
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

type failingEnvQuery struct{}

func (failingEnvQuery) ListEnvironments(context.Context, shared.ID) ([]EnvironmentRef, error) {
	return nil, shared.NewError(shared.CodeFailedPrecondition, "environment query port is required")
}
func (failingEnvQuery) GetEnvironment(context.Context, shared.ID) (EnvironmentRef, error) {
	return EnvironmentRef{}, shared.NewError(shared.CodeFailedPrecondition, "environment query port is required")
}

type failingGitOps struct{}

func (failingGitOps) ApplyPromotion(context.Context, GitOpsPromotionSpec) (GitOpsPromotionResult, error) {
	return GitOpsPromotionResult{}, shared.NewError(shared.CodeFailedPrecondition, "gitops deployment command is required")
}
