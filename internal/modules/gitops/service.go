package gitops

import (
	"context"
	"fmt"
	"strings"

	"github.com/shareinto/paas/internal/modules/clusteragent"
	"github.com/shareinto/paas/internal/modules/delivery"
	"github.com/shareinto/paas/internal/shared"
)

type Service struct {
	repo     Repository
	manifest ManifestRepositoryPort
	apps     ApplicationQuery
	envs     EnvironmentQuery
	audit    AuditLogger
	ids      shared.IDGenerator
	clock    shared.Clock
}

type Options struct {
	Repository   Repository
	ManifestRepo ManifestRepositoryPort
	Application  ApplicationQuery
	Environment  EnvironmentQuery
	Audit        AuditLogger
	IDGenerator  shared.IDGenerator
	Clock        shared.Clock
}

func NewService(opts Options) *Service {
	ids := opts.IDGenerator
	if ids == nil {
		ids = shared.RandomIDGenerator{}
	}
	clock := opts.Clock
	if clock == nil {
		clock = shared.SystemClock{}
	}
	audit := opts.Audit
	if audit == nil {
		audit = NoopAuditLogger{}
	}
	return &Service{repo: opts.Repository, manifest: opts.ManifestRepo, apps: opts.Application, envs: opts.Environment, audit: audit, ids: ids, clock: clock}
}

func (s *Service) EnsurePlatformTemplate(ctx context.Context, name string, content string) (DeploymentTemplate, error) {
	if existing, err := s.repo.FindPlatformTemplate(ctx, name); err == nil {
		return existing, nil
	}
	if result := validateTemplate(content); !result.Valid {
		return DeploymentTemplate{}, shared.NewError(shared.CodeInvalidArgument, strings.Join(result.Errors, "; "))
	}
	id, err := s.ids.NewID("deployment_template")
	if err != nil {
		return DeploymentTemplate{}, err
	}
	revisionID, err := s.ids.NewID("deployment_template_revision")
	if err != nil {
		return DeploymentTemplate{}, err
	}
	now := s.clock.Now()
	template := DeploymentTemplate{ID: id, Name: name, Scope: TemplateScopePlatform, Content: content, CurrentVersion: 1, CreatedAt: now, UpdatedAt: now}
	revision := DeploymentTemplateRevision{ID: revisionID, TemplateID: id, Version: 1, Content: content, CreatedAt: now}
	if err := s.repo.CreateTemplate(ctx, template); err != nil {
		return DeploymentTemplate{}, err
	}
	if err := s.repo.CreateTemplateRevision(ctx, revision); err != nil {
		return DeploymentTemplate{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{Action: "deployment_template.platform.create", ResourceType: "deployment_template", ResourceID: template.ID, Result: "succeeded", Summary: "创建平台基础部署模板", OccurredAt: now})
	return template, nil
}

func (s *Service) CreateApplicationTemplate(ctx context.Context, applicationID shared.ID, platformTemplateName string, actorID shared.ID) (DeploymentTemplate, error) {
	if existing, err := s.repo.FindApplicationTemplate(ctx, applicationID); err == nil {
		return existing, nil
	}
	app, err := s.apps.GetApplication(ctx, applicationID)
	if err != nil {
		return DeploymentTemplate{}, err
	}
	base, err := s.repo.FindPlatformTemplate(ctx, platformTemplateName)
	if err != nil {
		return DeploymentTemplate{}, err
	}
	id, err := s.ids.NewID("deployment_template")
	if err != nil {
		return DeploymentTemplate{}, err
	}
	revisionID, err := s.ids.NewID("deployment_template_revision")
	if err != nil {
		return DeploymentTemplate{}, err
	}
	now := s.clock.Now()
	template := DeploymentTemplate{ID: id, TenantID: app.TenantID, ProjectID: app.ProjectID, ApplicationID: app.ID, Name: app.Name + "-template", Scope: TemplateScopeApplication, Content: base.Content, CurrentVersion: 1, CreatedAt: now, UpdatedAt: now}
	revision := DeploymentTemplateRevision{ID: revisionID, TemplateID: id, Version: 1, Content: template.Content, CreatedBy: actorID, CreatedAt: now}
	if err := s.repo.CreateTemplate(ctx, template); err != nil {
		return DeploymentTemplate{}, err
	}
	if err := s.repo.CreateTemplateRevision(ctx, revision); err != nil {
		return DeploymentTemplate{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: actorID, TenantID: app.TenantID, ProjectID: app.ProjectID, Action: "deployment_template.application.create", ResourceType: "deployment_template", ResourceID: template.ID, Result: "succeeded", Summary: "创建应用部署模板", OccurredAt: now})
	return template, nil
}

func (s *Service) UpdateApplicationTemplate(ctx context.Context, applicationID shared.ID, content string, actorID shared.ID) (DeploymentTemplateRevision, error) {
	result := validateTemplate(content)
	if !result.Valid {
		return DeploymentTemplateRevision{}, shared.NewError(shared.CodeInvalidArgument, strings.Join(result.Errors, "; "))
	}
	template, err := s.repo.FindApplicationTemplate(ctx, applicationID)
	if err != nil {
		return DeploymentTemplateRevision{}, err
	}
	revisionID, err := s.ids.NewID("deployment_template_revision")
	if err != nil {
		return DeploymentTemplateRevision{}, err
	}
	now := s.clock.Now()
	template.Content = content
	template.CurrentVersion++
	template.UpdatedAt = now
	revision := DeploymentTemplateRevision{ID: revisionID, TemplateID: template.ID, Version: template.CurrentVersion, Content: content, CreatedBy: actorID, CreatedAt: now}
	if err := s.repo.UpdateTemplate(ctx, template); err != nil {
		return DeploymentTemplateRevision{}, err
	}
	if err := s.repo.CreateTemplateRevision(ctx, revision); err != nil {
		return DeploymentTemplateRevision{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: actorID, TenantID: template.TenantID, ProjectID: template.ProjectID, Action: "deployment_template.update", ResourceType: "deployment_template", ResourceID: template.ID, Result: "succeeded", Summary: "更新应用部署模板并生成新版本", OccurredAt: now})
	return revision, nil
}

func (s *Service) GetApplicationTemplate(ctx context.Context, applicationID shared.ID) (DeploymentTemplate, DeploymentTemplateRevision, error) {
	template, err := s.repo.FindApplicationTemplate(ctx, applicationID)
	if err != nil {
		return DeploymentTemplate{}, DeploymentTemplateRevision{}, err
	}
	revision, err := s.repo.GetCurrentTemplateRevision(ctx, template.ID)
	if err != nil {
		return DeploymentTemplate{}, DeploymentTemplateRevision{}, err
	}
	return template, revision, nil
}

func (s *Service) ValidateTemplate(_ context.Context, content string) ValidationResult {
	return validateTemplate(content)
}

func (s *Service) ApplyPromotion(ctx context.Context, spec delivery.GitOpsPromotionSpec) (delivery.GitOpsPromotionResult, error) {
	app, err := s.apps.GetApplication(ctx, spec.ApplicationID)
	if err != nil {
		return delivery.GitOpsPromotionResult{}, err
	}
	env, err := s.envs.GetEnvironment(ctx, spec.EnvironmentID)
	if err != nil {
		return delivery.GitOpsPromotionResult{}, err
	}
	binding, err := s.envs.GetActiveBinding(ctx, env.ID)
	if err != nil {
		return delivery.GitOpsPromotionResult{}, err
	}
	template, err := s.repo.FindApplicationTemplate(ctx, app.ID)
	if err != nil {
		return delivery.GitOpsPromotionResult{}, err
	}
	validation := validateTemplate(template.Content)
	if !validation.Valid {
		return delivery.GitOpsPromotionResult{}, shared.NewError(shared.CodeFailedPrecondition, strings.Join(validation.Errors, "; "))
	}
	revision, err := s.repo.GetCurrentTemplateRevision(ctx, template.ID)
	if err != nil {
		return delivery.GitOpsPromotionResult{}, err
	}
	deploymentID, err := s.ids.NewID("deployment")
	if err != nil {
		return delivery.GitOpsPromotionResult{}, err
	}
	manifestRevisionID, err := s.ids.NewID("manifest_revision")
	if err != nil {
		return delivery.GitOpsPromotionResult{}, err
	}
	eventID, err := s.ids.NewID("deployment_event")
	if err != nil {
		return delivery.GitOpsPromotionResult{}, err
	}
	artifact := primaryPromotionArtifact(spec.Artifacts)
	if artifact.URI == "" && strings.TrimSpace(spec.ImageURI) != "" {
		artifact = delivery.GitOpsArtifactSpec{URI: spec.ImageURI, Digest: spec.ImageDigest, IsPrimary: true}
	}
	if artifact.URI == "" {
		return delivery.GitOpsPromotionResult{}, shared.NewError(shared.CodeInvalidArgument, "promotion artifacts is required")
	}
	repository, tag := splitImage(artifact.URI)
	valuesPath := manifestPath(app.Name, env.Name)
	values := renderValues(app, env, binding, repository, tag, artifact.Digest, template.Content)
	argoPath := argoApplicationPath(app.Name, env.Name)
	argo := renderArgoApplication(app, env, binding, valuesPath)
	files := []CommitFile{{Path: valuesPath, Content: values}, {Path: argoPath, Content: argo}}
	message := fmt.Sprintf("paas: deploy %s to %s", app.Name, env.Name)
	now := s.clock.Now()
	manifestRevision := ManifestRevision{ID: manifestRevisionID, DeploymentID: deploymentID, PromotionID: spec.PromotionID, ApplicationID: app.ID, EnvironmentID: env.ID, TemplateRevisionID: revision.ID, Path: valuesPath, ChangeType: "deploy", CreatedAt: now}
	if spec.IsRollback {
		manifestRevision.ChangeType = "rollback"
	}
	if commitDirectly(env.Name) {
		result, err := s.manifest.CommitFiles(ctx, CommitSpec{Branch: "main", Message: message, Files: files})
		if err != nil {
			return delivery.GitOpsPromotionResult{}, err
		}
		manifestRevision.CommitSHA = result.CommitSHA
	} else {
		mr, err := s.manifest.CreateMergeRequest(ctx, MergeRequestSpec{SourceBranch: "paas/" + string(spec.PromotionID), TargetBranch: "main", Title: message, Files: files})
		if err != nil {
			return delivery.GitOpsPromotionResult{}, err
		}
		manifestRevision.MergeRequestID = mr.ID
		manifestRevision.CommitSHA = mr.CommitSHA
	}
	deployment := Deployment{ID: deploymentID, TenantID: app.TenantID, ProjectID: app.ProjectID, ApplicationID: app.ID, EnvironmentID: env.ID, ClusterBindingID: binding.ID, PromotionID: spec.PromotionID, FreightID: spec.FreightID, ManifestRevisionID: manifestRevision.ID, ImageRepository: repository, ImageTag: tag, ImageDigest: artifact.Digest, Status: DeploymentPending, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateDeployment(ctx, deployment); err != nil {
		return delivery.GitOpsPromotionResult{}, err
	}
	if err := s.repo.CreateManifestRevision(ctx, manifestRevision); err != nil {
		return delivery.GitOpsPromotionResult{}, err
	}
	_ = s.repo.CreateDeploymentEvent(ctx, DeploymentEvent{ID: eventID, DeploymentID: deployment.ID, Status: deployment.Status, Message: "清单变更已提交", OccurredAt: now})
	_ = s.audit.Log(ctx, AuditEvent{TenantID: app.TenantID, ProjectID: app.ProjectID, Action: "manifest_revision.create", ResourceType: "manifest_revision", ResourceID: manifestRevision.ID, Result: "succeeded", Summary: "提交部署清单变更", OccurredAt: now})
	_ = s.audit.Log(ctx, AuditEvent{TenantID: app.TenantID, ProjectID: app.ProjectID, Action: "deployment.create", ResourceType: "deployment", ResourceID: deployment.ID, Result: "succeeded", Summary: "创建部署记录", OccurredAt: now})
	return delivery.GitOpsPromotionResult{ManifestRevision: firstNonEmpty(manifestRevision.CommitSHA, manifestRevision.MergeRequestID)}, nil
}

func (s *Service) UpdateFromAgent(ctx context.Context, report clusteragent.StatusReport) error {
	for _, appStatus := range report.Applications {
		if appStatus.DeploymentID.IsZero() {
			continue
		}
		deployment, err := s.repo.GetDeployment(ctx, appStatus.DeploymentID)
		if err != nil {
			continue
		}
		status := mapAgentStatus(appStatus)
		if deployment.Status == status && deployment.Message == appStatus.Message {
			continue
		}
		deployment.Status = status
		deployment.Message = appStatus.Message
		now := s.clock.Now()
		deployment.UpdatedAt = now
		if status == DeploymentSucceeded || status == DeploymentFailed || status == DeploymentDegraded {
			deployment.CompletedAt = &now
		}
		if err := s.repo.UpdateDeployment(ctx, deployment); err != nil {
			return err
		}
		eventID, err := s.ids.NewID("deployment_event")
		if err != nil {
			return err
		}
		if err := s.repo.CreateDeploymentEvent(ctx, DeploymentEvent{ID: eventID, DeploymentID: deployment.ID, Status: status, Message: appStatus.Message, OccurredAt: now}); err != nil {
			return err
		}
	}
	return nil
}

func primaryPromotionArtifact(artifacts []delivery.GitOpsArtifactSpec) delivery.GitOpsArtifactSpec {
	for _, artifact := range artifacts {
		if artifact.IsPrimary {
			return artifact
		}
	}
	if len(artifacts) > 0 {
		return artifacts[0]
	}
	return delivery.GitOpsArtifactSpec{}
}

func (s *Service) GetDeployment(ctx context.Context, id shared.ID) (Deployment, error) {
	return s.repo.GetDeployment(ctx, id)
}

func (s *Service) ListDeployments(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[Deployment], error) {
	return s.repo.ListDeployments(ctx, applicationID, page)
}

func renderValues(app ApplicationRef, env EnvironmentRef, binding ClusterBindingRef, repo string, tag string, digest string, template string) string {
	return fmt.Sprintf("application: %s\nenvironment: %s\nnamespace: %s\nimage:\n  repository: %s\n  tag: %s\n  digest: %s\ntemplate: |\n%s\n", app.Name, env.Name, binding.Namespace, repo, tag, digest, indent(template, "  "))
}

func renderArgoApplication(app ApplicationRef, env EnvironmentRef, binding ClusterBindingRef, valuesPath string) string {
	return fmt.Sprintf("apiVersion: argoproj.io/v1alpha1\nkind: Application\nmetadata:\n  name: %s-%s\nspec:\n  destination:\n    namespace: %s\n  source:\n    path: %s\n", app.Name, env.Name, binding.Namespace, valuesPath)
}

func indent(value string, prefix string) string {
	lines := strings.Split(value, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

func mapAgentStatus(status clusteragent.ApplicationStatus) DeploymentStatus {
	sync := strings.ToLower(status.SyncStatus)
	health := strings.ToLower(status.HealthStatus)
	operation := strings.ToLower(status.OperationState)
	switch {
	case sync == "synced" && health == "healthy":
		return DeploymentSucceeded
	case health == "degraded":
		return DeploymentDegraded
	case operation == "failed" || sync == "unknown" && health == "missing":
		return DeploymentFailed
	case operation == "running" || sync == "outofsync":
		return DeploymentSyncing
	case health == "progressing":
		return DeploymentProgressing
	default:
		return DeploymentUnknown
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
