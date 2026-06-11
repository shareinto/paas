package gitops

import (
	"context"
	"fmt"
	"strings"

	"github.com/shareinto/paas/internal/modules/clusteragent"
	"github.com/shareinto/paas/internal/modules/delivery"
	"github.com/shareinto/paas/internal/shared"
	"gopkg.in/yaml.v3"
)

type Service struct {
	repo      Repository
	manifest  ManifestRepositoryPort
	apps      ApplicationQuery
	envs      EnvironmentQuery
	workloads WorkloadQuery
	audit     AuditLogger
	ids       shared.IDGenerator
	clock     shared.Clock
}

type Options struct {
	Repository   Repository
	ManifestRepo ManifestRepositoryPort
	Application  ApplicationQuery
	Environment  EnvironmentQuery
	Workload     WorkloadQuery
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
	return &Service{repo: opts.Repository, manifest: opts.ManifestRepo, apps: opts.Application, envs: opts.Environment, workloads: opts.Workload, audit: audit, ids: ids, clock: clock}
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
	artifacts := normalizePromotionArtifacts(spec)
	artifact := primaryPromotionArtifact(artifacts)
	if artifact.URI == "" && strings.TrimSpace(spec.ImageURI) != "" {
		artifact = delivery.GitOpsArtifactSpec{URI: spec.ImageURI, Digest: spec.ImageDigest, IsPrimary: true}
	}
	if artifact.URI == "" {
		return delivery.GitOpsPromotionResult{}, shared.NewError(shared.CodeInvalidArgument, "promotion artifacts is required")
	}
	if len(artifacts) == 0 {
		artifacts = []delivery.GitOpsArtifactSpec{artifact}
	}
	repository, tag := imageRepositoryTag(artifact)
	valuesPath := manifestPath(app.Name, env.Name)
	values, workloadSummary, err := s.renderPromotionValues(ctx, app, env, binding, artifacts, template.Content)
	if err != nil {
		return delivery.GitOpsPromotionResult{}, err
	}
	argoPath := argoApplicationPath(app.Name, env.Name)
	argo := renderArgoApplication(app, env, binding, valuesPath)
	files := []CommitFile{{Path: valuesPath, Content: values}, {Path: argoPath, Content: argo}}
	message := fmt.Sprintf("paas: deploy %s to %s", app.Name, env.Name)
	now := s.clock.Now()
	manifestRevision := ManifestRevision{ID: manifestRevisionID, DeploymentID: deploymentID, PromotionID: spec.PromotionID, ApplicationID: app.ID, EnvironmentID: env.ID, TemplateRevisionID: revision.ID, Path: valuesPath, ChangeType: "deploy", CreatedAt: now}
	if spec.IsRollback {
		manifestRevision.ChangeType = "rollback"
	}
	deployment := Deployment{ID: deploymentID, TenantID: app.TenantID, ProjectID: app.ProjectID, ApplicationID: app.ID, EnvironmentID: env.ID, ClusterBindingID: binding.ID, PromotionID: spec.PromotionID, FreightID: spec.FreightID, ManifestRevisionID: manifestRevision.ID, ImageRepository: repository, ImageTag: tag, ImageDigest: artifact.Digest, WorkloadSummary: workloadSummary, Status: DeploymentPending, CreatedAt: now, UpdatedAt: now}
	if commitDirectly(env.Name) {
		result, err := s.manifest.CommitFiles(ctx, CommitSpec{Branch: "main", Message: message, Files: files})
		if err != nil {
			deployment.ManifestRevisionID = ""
			_ = s.recordFailedDeployment(ctx, deployment, eventID, "提交部署清单失败："+err.Error())
			return delivery.GitOpsPromotionResult{}, err
		}
		manifestRevision.CommitSHA = result.CommitSHA
	} else {
		mr, err := s.manifest.CreateMergeRequest(ctx, MergeRequestSpec{SourceBranch: "paas/" + string(spec.PromotionID), TargetBranch: "main", Title: message, Files: files})
		if err != nil {
			deployment.ManifestRevisionID = ""
			_ = s.recordFailedDeployment(ctx, deployment, eventID, "创建合并请求失败："+err.Error())
			return delivery.GitOpsPromotionResult{}, err
		}
		manifestRevision.MergeRequestID = mr.ID
		manifestRevision.CommitSHA = mr.CommitSHA
	}
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

func normalizePromotionArtifacts(spec delivery.GitOpsPromotionSpec) []delivery.GitOpsArtifactSpec {
	out := make([]delivery.GitOpsArtifactSpec, 0, len(spec.Artifacts))
	for _, artifact := range spec.Artifacts {
		artifact.URI = strings.TrimSpace(artifact.URI)
		artifact.Repository = strings.TrimSpace(artifact.Repository)
		artifact.Tag = strings.TrimSpace(artifact.Tag)
		artifact.Digest = strings.TrimSpace(artifact.Digest)
		if artifact.Repository == "" || artifact.Tag == "" {
			repository, tag := splitImage(artifact.URI)
			if artifact.Repository == "" {
				artifact.Repository = repository
			}
			if artifact.Tag == "" {
				artifact.Tag = tag
			}
		}
		if artifact.URI == "" && artifact.Repository != "" {
			artifact.URI = artifact.Repository
			if artifact.Tag != "" {
				artifact.URI += ":" + artifact.Tag
			}
		}
		out = append(out, artifact)
	}
	if len(out) == 0 && strings.TrimSpace(spec.ImageURI) != "" {
		repository, tag := splitImage(spec.ImageURI)
		out = append(out, delivery.GitOpsArtifactSpec{URI: strings.TrimSpace(spec.ImageURI), Repository: repository, Tag: tag, Digest: strings.TrimSpace(spec.ImageDigest), IsPrimary: true})
	}
	return out
}

func imageRepositoryTag(artifact delivery.GitOpsArtifactSpec) (string, string) {
	repository := strings.TrimSpace(artifact.Repository)
	tag := strings.TrimSpace(artifact.Tag)
	if repository == "" || tag == "" {
		parsedRepository, parsedTag := splitImage(artifact.URI)
		if repository == "" {
			repository = parsedRepository
		}
		if tag == "" {
			tag = parsedTag
		}
	}
	return repository, tag
}

func (s *Service) renderPromotionValues(ctx context.Context, app ApplicationRef, env EnvironmentRef, binding ClusterBindingRef, artifacts []delivery.GitOpsArtifactSpec, template string) (string, string, error) {
	primary := primaryPromotionArtifact(artifacts)
	repository, tag := imageRepositoryTag(primary)
	values := map[string]any{
		"application": app.Name,
		"environment": env.Name,
		"namespace":   binding.Namespace,
		"image":       imageValues(repository, tag, primary.Digest),
		"template":    template,
	}
	workloadValues := map[string]any{}
	summary := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		if artifact.WorkloadID.IsZero() {
			continue
		}
		workload, err := s.getWorkload(ctx, app.ID, artifact.WorkloadID)
		if err != nil {
			return "", "", err
		}
		config, err := s.getWorkloadEnvironmentConfig(ctx, artifact.WorkloadID, env.ID)
		if err != nil && shared.CodeOf(err) != shared.CodeNotFound {
			return "", "", err
		}
		repository, tag := imageRepositoryTag(artifact)
		name := strings.TrimSpace(workload.Name)
		if name == "" {
			name = artifact.WorkloadID.String()
		}
		workloadValues[name] = renderWorkloadValues(workload, config, repository, tag, artifact.Digest)
		summary = append(summary, fmt.Sprintf("%s=%s", name, imageSummary(repository, tag, artifact.Digest)))
	}
	if len(workloadValues) > 0 {
		values["workloads"] = workloadValues
	}
	raw, err := yaml.Marshal(values)
	if err != nil {
		return "", "", shared.NewError(shared.CodeInternal, "render values failed: "+err.Error())
	}
	return string(raw), strings.Join(summary, "\n"), nil
}

func (s *Service) getWorkload(ctx context.Context, applicationID shared.ID, workloadID shared.ID) (WorkloadRef, error) {
	if s.workloads == nil {
		return WorkloadRef{ID: workloadID, ApplicationID: applicationID, Name: workloadID.String(), WorkloadType: "Deployment"}, nil
	}
	return s.workloads.GetWorkload(ctx, applicationID, workloadID)
}

func (s *Service) getWorkloadEnvironmentConfig(ctx context.Context, workloadID shared.ID, environmentID shared.ID) (WorkloadEnvironmentConfigRef, error) {
	if s.workloads == nil {
		return WorkloadEnvironmentConfigRef{}, shared.NewError(shared.CodeNotFound, "workload environment config not found")
	}
	return s.workloads.GetWorkloadEnvironmentConfig(ctx, workloadID, environmentID)
}

func imageValues(repository string, tag string, digest string) map[string]any {
	return map[string]any{"repository": repository, "tag": tag, "digest": strings.TrimSpace(digest)}
}

func renderWorkloadValues(workload WorkloadRef, config WorkloadEnvironmentConfigRef, repository string, tag string, digest string) map[string]any {
	values := map[string]any{
		"kind":  normalizeWorkloadKind(workload.WorkloadType),
		"image": imageValues(repository, tag, digest),
	}
	if config.Replicas > 0 {
		values["replicas"] = config.Replicas
	}
	if len(config.ServicePorts) > 0 {
		values["servicePorts"] = config.ServicePorts
	}
	resources := map[string]any{}
	if config.ResourceRequests.CPU != "" || config.ResourceRequests.Memory != "" {
		resources["requests"] = config.ResourceRequests
	}
	if config.ResourceLimits.CPU != "" || config.ResourceLimits.Memory != "" {
		resources["limits"] = config.ResourceLimits
	}
	if len(resources) > 0 {
		values["resources"] = resources
	}
	if len(config.Probes) > 0 {
		values["probes"] = config.Probes
	}
	if len(config.EnvVars) > 0 {
		values["env"] = config.EnvVars
	}
	if len(config.IngressHosts) > 0 {
		values["ingressHosts"] = config.IngressHosts
	}
	if len(config.SecretRefs) > 0 {
		values["secretRefs"] = config.SecretRefs
	}
	if len(config.ConfigFiles) > 0 {
		values["configFiles"] = config.ConfigFiles
	}
	if len(config.WritableDirs) > 0 {
		values["writableDirs"] = config.WritableDirs
	}
	if len(config.VolumeMounts) > 0 {
		values["volumeMounts"] = config.VolumeMounts
	}
	if len(config.InitContainers) > 0 {
		values["initContainers"] = config.InitContainers
	}
	for key, value := range config.ValuesOverride {
		if strings.TrimSpace(key) == "" || key == "image" {
			continue
		}
		values[key] = value
	}
	return values
}

func normalizeWorkloadKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "statefulset":
		return "StatefulSet"
	default:
		return "Deployment"
	}
}

func imageSummary(repository string, tag string, digest string) string {
	image := strings.TrimSpace(repository)
	if strings.TrimSpace(tag) != "" {
		image += ":" + strings.TrimSpace(tag)
	}
	if strings.TrimSpace(digest) != "" {
		image += "@" + strings.TrimSpace(digest)
	}
	return image
}

func (s *Service) recordFailedDeployment(ctx context.Context, deployment Deployment, eventID shared.ID, message string) error {
	now := s.clock.Now()
	deployment.Status = DeploymentFailed
	deployment.Message = strings.TrimSpace(message)
	deployment.UpdatedAt = now
	deployment.CompletedAt = &now
	if err := s.repo.CreateDeployment(ctx, deployment); err != nil {
		return err
	}
	return s.repo.CreateDeploymentEvent(ctx, DeploymentEvent{ID: eventID, DeploymentID: deployment.ID, Status: DeploymentFailed, Message: deployment.Message, OccurredAt: now})
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
