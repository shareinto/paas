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
	return &Service{repo: opts.Repository, builds: opts.BuildQuery, apps: opts.ApplicationQuery, envs: opts.EnvironmentQuery, gitops: opts.GitOpsDeployment, permission: opts.PermissionChecker, audit: audit, events: events, ids: ids, clock: clock}
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

func (s *Service) HandleBuildSucceeded(ctx context.Context, payload BuildSucceededPayload) (Release, Freight, error) {
	if payload.BuildRunID.IsZero() || payload.ApplicationID.IsZero() {
		return Release{}, Freight{}, shared.NewError(shared.CodeInvalidArgument, "build_run_id and application_id are required")
	}
	if existing, err := s.repo.FindReleaseByBuildRun(ctx, payload.BuildRunID); err == nil {
		freights, _ := s.repo.ListFreightsByApplication(ctx, existing.ApplicationID, shared.PageRequest{Page: 1, PageSize: 1})
		if len(freights.Items) > 0 {
			return existing, freights.Items[0], nil
		}
	}
	run, err := s.buildsOrError().GetBuildRun(ctx, payload.BuildRunID)
	if err != nil {
		return Release{}, Freight{}, err
	}
	artifactIDs := payload.BuildArtifactIDs
	if len(artifactIDs) == 0 && !payload.BuildArtifactID.IsZero() {
		artifactIDs = []shared.ID{payload.BuildArtifactID}
	}
	artifacts, err := s.resolveBuildArtifacts(ctx, payload.BuildRunID, artifactIDs)
	if err != nil {
		return Release{}, Freight{}, err
	}
	if run.ApplicationID != payload.ApplicationID {
		return Release{}, Freight{}, shared.NewError(shared.CodeInvalidArgument, "build payload ownership mismatch")
	}
	for _, artifact := range artifacts {
		if artifact.BuildRunID != run.ID || artifact.ApplicationID != run.ApplicationID {
			return Release{}, Freight{}, shared.NewError(shared.CodeInvalidArgument, "build payload ownership mismatch")
		}
	}
	primary := primaryArtifact(artifacts)
	now := s.clock.Now()
	releaseID, err := s.ids.NewID("release")
	if err != nil {
		return Release{}, Freight{}, err
	}
	freightID, err := s.ids.NewID("freight")
	if err != nil {
		return Release{}, Freight{}, err
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
	release := Release{ID: releaseID, TenantID: run.TenantID, ProjectID: run.ProjectID, ApplicationID: run.ApplicationID, PipelineID: pipelineID, PipelineName: pipelineName, PipelineDisplayName: pipelineDisplayName, BuildRunID: run.ID, BuildArtifactID: primary.ID, Version: releaseVersion(commit, run.ID), CommitSHA: commit, ImageURI: imageURI, ImageDigest: imageDigest, Status: ReleaseReady, CreatedAt: now}
	freight := Freight{ID: freightID, TenantID: run.TenantID, ProjectID: run.ProjectID, ApplicationID: run.ApplicationID, PipelineID: pipelineID, PipelineName: pipelineName, PipelineDisplayName: pipelineDisplayName, Name: release.Version, Status: FreightAvailable, CreatedAt: now}
	if err := s.repo.CreateRelease(ctx, release); err != nil {
		return Release{}, Freight{}, err
	}
	if err := s.repo.CreateFreight(ctx, freight); err != nil {
		return Release{}, Freight{}, err
	}
	for _, artifact := range artifacts {
		itemID, err := s.ids.NewID("freight_item")
		if err != nil {
			return Release{}, Freight{}, err
		}
		itemType := FreightItemImage
		if artifact.ID == primary.ID {
			itemType = FreightItemApplicationRelease
		}
		name := artifact.SourceKey
		if name == "" {
			name = release.Version
		}
		item := FreightItem{ID: itemID, TenantID: run.TenantID, ProjectID: run.ProjectID, FreightID: freight.ID, ApplicationID: run.ApplicationID, ReleaseID: release.ID, BuildArtifactID: artifact.ID, SourceKey: artifact.SourceKey, Type: itemType, Name: name, URI: artifact.URI, Digest: artifact.Digest, CreatedAt: now}
		if err := s.repo.CreateFreightItem(ctx, item); err != nil {
			return Release{}, Freight{}, err
		}
	}
	if _, err := s.ensureDefaultFlow(ctx, run.ApplicationID); err != nil {
		return Release{}, Freight{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{Action: "freight.create", ResourceType: "freight", ResourceID: freight.ID, Result: "succeeded", Summary: "创建可交付变更包", OccurredAt: now})
	_ = s.publish(ctx, "FreightCreated", now, map[string]any{"freight_id": freight.ID, "release_id": release.ID, "application_id": run.ApplicationID})
	return release, freight, nil
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
	promotion, err := s.newPromotion(ctx, freight, stage, env, input.Actor.ID, strings.TrimSpace(input.Message), false, "")
	if err != nil {
		return Promotion{}, err
	}
	if isProdStage(stage.Name) {
		return s.createApproval(ctx, promotion)
	}
	return s.applyPromotion(ctx, promotion)
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
		artifacts = append(artifacts, GitOpsArtifactSpec{Name: item.Name, SourceKey: item.SourceKey, URI: item.URI, Digest: item.Digest, IsPrimary: i == 0 || item.Type == FreightItemApplicationRelease})
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
