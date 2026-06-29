package gitops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/shareinto/paas/internal/modules/clusteragent"
	"github.com/shareinto/paas/internal/modules/delivery"
	"github.com/shareinto/paas/internal/shared"
)

const (
	defaultPlatformTemplateName    = "java"
	defaultPlatformTemplateContent = "containers: []"
	defaultTemplateActorID         = shared.ID("usr_admin")
)

type Service struct {
	repo            Repository
	manifest        ManifestRepositoryPort
	manifestRepoURL string
	apps            ApplicationQuery
	workloads       WorkloadQuery
	audit           AuditLogger
	ids             shared.IDGenerator
	clock           shared.Clock
}

type Options struct {
	Repository      Repository
	ManifestRepo    ManifestRepositoryPort
	ManifestRepoURL string
	Application     ApplicationQuery
	Workload        WorkloadQuery
	Audit           AuditLogger
	IDGenerator     shared.IDGenerator
	Clock           shared.Clock
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
	return &Service{
		repo:            opts.Repository,
		manifest:        opts.ManifestRepo,
		manifestRepoURL: strings.TrimSpace(opts.ManifestRepoURL),
		apps:            opts.Application,
		workloads:       opts.Workload,
		audit:           audit,
		ids:             ids,
		clock:           clock,
	}
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
	template := DeploymentTemplate{ID: id, Name: name, Content: content, CurrentVersion: 1, CreatedAt: now, UpdatedAt: now}
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

func (s *Service) UpdatePlatformTemplate(ctx context.Context, content string, actorID shared.ID) (DeploymentTemplateRevision, error) {
	result := validateTemplate(content)
	if !result.Valid {
		return DeploymentTemplateRevision{}, shared.NewError(shared.CodeInvalidArgument, strings.Join(result.Errors, "; "))
	}
	template, err := s.repo.FindPlatformTemplate(ctx, defaultPlatformTemplateName)
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
	_ = s.audit.Log(ctx, AuditEvent{ActorID: actorID, Action: "deployment_template.platform.update", ResourceType: "deployment_template", ResourceID: template.ID, Result: "succeeded", Summary: "更新平台部署模板并生成新版本", OccurredAt: now})
	return revision, nil
}

func (s *Service) GetPlatformTemplateRevision(ctx context.Context) (DeploymentTemplate, DeploymentTemplateRevision, error) {
	template, err := s.repo.FindPlatformTemplate(ctx, defaultPlatformTemplateName)
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
	stageKey := normalizeStageKey(spec.StageKey)
	if stageKey == "" {
		return delivery.GitOpsPromotionResult{}, shared.NewError(shared.CodeInvalidArgument, "target_stage_key is required")
	}
	bindings, err := s.promotionTargetBindings(stageKey, spec.TargetClusters)
	if err != nil {
		return delivery.GitOpsPromotionResult{}, err
	}
	template, err := s.repo.FindPlatformTemplate(ctx, defaultPlatformTemplateName)
	if err != nil {
		if shared.CodeOf(err) == shared.CodeNotFound {
			template, err = s.EnsurePlatformTemplate(ctx, defaultPlatformTemplateName, defaultPlatformTemplateContent)
		}
		if err != nil {
			return delivery.GitOpsPromotionResult{}, err
		}
	}
	validation := validateTemplate(template.Content)
	if !validation.Valid {
		return delivery.GitOpsPromotionResult{}, shared.NewError(shared.CodeFailedPrecondition, strings.Join(validation.Errors, "; "))
	}
	revision, err := s.repo.GetCurrentTemplateRevision(ctx, template.ID)
	if err != nil {
		return delivery.GitOpsPromotionResult{}, err
	}
	artifacts := normalizePromotionArtifacts(spec)
	artifact := primaryPromotionArtifact(artifacts)
	if artifact.URI == "" && strings.TrimSpace(spec.ImageURI) != "" {
		artifact = delivery.GitOpsArtifactSpec{URI: spec.ImageURI, Digest: spec.ImageDigest, IsPrimary: true}
	}
	if len(artifacts) == 0 && artifact.URI == "" {
		return delivery.GitOpsPromotionResult{}, shared.NewError(shared.CodeInvalidArgument, "promotion artifacts is required")
	}
	if len(artifacts) == 0 {
		artifacts = []delivery.GitOpsArtifactSpec{artifact}
	}
	type targetRecord struct {
		binding          ClusterBindingRef
		deployment       Deployment
		manifestRevision ManifestRevision
		eventID          shared.ID
		valuesPath       string
	}
	records := make([]targetRecord, 0, len(bindings))
	files := make([]CommitFile, 0, len(bindings)*2)
	message := fmt.Sprintf("paas: deploy %s to %s", app.Name, stageKey)
	now := s.clock.Now()
	changeType := "deploy"
	if spec.IsRollback {
		changeType = "rollback"
	}
	for _, binding := range bindings {
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
		resolvedArtifacts, err := resolvePromotionArtifactsForBinding(artifacts, binding)
		if err != nil {
			return delivery.GitOpsPromotionResult{}, err
		}
		resolvedPrimary := primaryPromotionArtifact(resolvedArtifacts)
		repository, tag := imageRepositoryTag(resolvedPrimary)
		valuesPath := manifestPathForBinding(app.Name, stageKey, binding)
		manifests, workloadSummary, err := s.renderK8sManifests(ctx, app, stageKey, binding, deploymentID, resolvedArtifacts)
		if err != nil {
			return delivery.GitOpsPromotionResult{}, err
		}
		argoPath := argoApplicationPathForBinding(app.Name, stageKey, binding)
		manifestDir := fmt.Sprintf("apps/%s/%s", app.Name, stageKey)
		argo := renderArgoApplication(app, stageKey, binding, deploymentID, s.manifestRepoURL, manifestDir)
		files = append(files, CommitFile{Path: valuesPath, Content: manifests}, CommitFile{Path: argoPath, Content: argo})
		manifestRevision := ManifestRevision{ID: manifestRevisionID, DeploymentID: deploymentID, PromotionID: spec.PromotionID, ApplicationID: app.ID, StageKey: stageKey, TemplateRevisionID: revision.ID, Path: valuesPath, ChangeType: changeType, CreatedAt: now}
		stageConfigs := s.collectStageConfigs(ctx, app.ID, resolvedArtifacts, stageKey)
		configHash := ComputeStageConfigHash(revision.ID, stageConfigs)
		deployment := Deployment{ID: deploymentID, TenantID: app.TenantID, ProjectID: app.ProjectID, ApplicationID: app.ID, StageKey: stageKey, ClusterBindingID: binding.ID, PromotionID: spec.PromotionID, FreightID: spec.FreightID, ManifestRevisionID: manifestRevision.ID, ImageRepository: repository, ImageTag: tag, ImageDigest: resolvedPrimary.Digest, WorkloadSummary: workloadSummary, ConfigHash: configHash, Status: DeploymentPending, CreatedAt: now, UpdatedAt: now}
		records = append(records, targetRecord{binding: binding, deployment: deployment, manifestRevision: manifestRevision, eventID: eventID, valuesPath: valuesPath})
	}
	result, err := s.manifest.CommitFiles(ctx, CommitSpec{Branch: "main", Message: message, Files: files})
	if err != nil {
		for _, record := range records {
			record.deployment.ManifestRevisionID = ""
			_ = s.recordFailedDeployment(ctx, record.deployment, record.eventID, "提交部署清单失败："+err.Error())
		}
		return delivery.GitOpsPromotionResult{}, err
	}
	manifestRef := result.CommitSHA
	for i := range records {
		records[i].manifestRevision.CommitSHA = result.CommitSHA
	}
	for _, record := range records {
		if err := s.repo.CreateDeployment(ctx, record.deployment); err != nil {
			return delivery.GitOpsPromotionResult{}, err
		}
		if err := s.repo.CreateManifestRevision(ctx, record.manifestRevision); err != nil {
			return delivery.GitOpsPromotionResult{}, err
		}
		_ = s.repo.CreateDeploymentEvent(ctx, DeploymentEvent{ID: record.eventID, DeploymentID: record.deployment.ID, Status: record.deployment.Status, Message: "清单变更已提交", OccurredAt: now})
		_ = s.audit.Log(ctx, AuditEvent{TenantID: app.TenantID, ProjectID: app.ProjectID, Action: "manifest_revision.create", ResourceType: "manifest_revision", ResourceID: record.manifestRevision.ID, Result: "succeeded", Summary: "提交部署清单变更", OccurredAt: now})
		_ = s.audit.Log(ctx, AuditEvent{TenantID: app.TenantID, ProjectID: app.ProjectID, Action: "deployment.create", ResourceType: "deployment", ResourceID: record.deployment.ID, Result: "succeeded", Summary: "创建部署记录", OccurredAt: now})
	}
	return delivery.GitOpsPromotionResult{ManifestRevision: manifestRef}, nil
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

func (s *Service) promotionTargetBindings(stageKey string, targets []delivery.GitOpsPromotionTargetCluster) ([]ClusterBindingRef, error) {
	if len(targets) == 0 {
		return nil, shared.NewError(shared.CodeInvalidArgument, "promotion target cluster is required")
	}
	if len(targets) > 1 {
		return nil, shared.NewError(shared.CodeInvalidArgument, "一个环境只能绑定一个集群")
	}
	out := make([]ClusterBindingRef, 0, len(targets))
	for _, target := range targets {
		if target.ClusterID.IsZero() || strings.TrimSpace(target.Namespace) == "" {
			return nil, shared.NewError(shared.CodeInvalidArgument, "promotion target cluster is invalid")
		}
		out = append(out, ClusterBindingRef{
			ID:          target.ClusterID,
			StageKey:    stageKey,
			ClusterID:   target.ClusterID,
			ClusterName: strings.TrimSpace(target.ClusterName),
			Namespace:   strings.TrimSpace(target.Namespace),
			Labels:      cleanStringMap(target.Labels),
			Active:      true,
		})
	}
	return out, nil
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
		artifact.Variants = normalizeImageVariants(artifact.Variants)
		out = append(out, artifact)
	}
	if len(out) == 0 && strings.TrimSpace(spec.ImageURI) != "" {
		repository, tag := splitImage(spec.ImageURI)
		out = append(out, delivery.GitOpsArtifactSpec{URI: strings.TrimSpace(spec.ImageURI), Repository: repository, Tag: tag, Digest: strings.TrimSpace(spec.ImageDigest), IsPrimary: true})
	}
	return out
}

func normalizeImageVariants(variants []delivery.GitOpsImageVariant) []delivery.GitOpsImageVariant {
	out := make([]delivery.GitOpsImageVariant, 0, len(variants))
	for _, variant := range variants {
		variant.URI = strings.TrimSpace(variant.URI)
		variant.Repository = strings.TrimSpace(variant.Repository)
		variant.Tag = strings.TrimSpace(variant.Tag)
		variant.Digest = strings.TrimSpace(variant.Digest)
		if variant.Repository == "" || variant.Tag == "" {
			repository, tag := splitImage(variant.URI)
			if variant.Repository == "" {
				variant.Repository = repository
			}
			if variant.Tag == "" {
				variant.Tag = tag
			}
		}
		if variant.URI == "" && variant.Repository != "" {
			variant.URI = variant.Repository
			if variant.Tag != "" {
				variant.URI += ":" + variant.Tag
			}
		}
		variant.SelectorLabels = cleanStringMap(variant.SelectorLabels)
		out = append(out, variant)
	}
	return out
}

func resolvePromotionArtifactsForBinding(artifacts []delivery.GitOpsArtifactSpec, binding ClusterBindingRef) ([]delivery.GitOpsArtifactSpec, error) {
	out := make([]delivery.GitOpsArtifactSpec, 0, len(artifacts))
	for _, artifact := range artifacts {
		if len(artifact.Variants) == 0 {
			out = append(out, artifact)
			continue
		}
		matches := make([]delivery.GitOpsImageVariant, 0, 1)
		for _, variant := range artifact.Variants {
			if labelsMatch(binding.Labels, variant.SelectorLabels) {
				matches = append(matches, variant)
			}
		}
		if len(matches) != 1 {
			return nil, shared.NewError(shared.CodeFailedPrecondition, "当前版本不适用于目标环境，请联系平台管理员检查运行时镜像配置")
		}
		match := matches[0]
		artifact.URI = match.URI
		artifact.Repository = match.Repository
		artifact.Tag = match.Tag
		artifact.Digest = match.Digest
		artifact.Variants = nil
		out = append(out, artifact)
	}
	return out, nil
}

func labelsMatch(clusterLabels map[string]string, selector map[string]string) bool {
	if len(selector) == 0 {
		return len(clusterLabels) == 0
	}
	for key, value := range selector {
		if clusterLabels[key] != value {
			return false
		}
	}
	return true
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

func (s *Service) getWorkload(ctx context.Context, applicationID shared.ID, workloadID shared.ID) (WorkloadRef, error) {
	if s.workloads == nil {
		return WorkloadRef{ID: workloadID, ApplicationID: applicationID, Name: workloadID.String(), WorkloadType: "Deployment"}, nil
	}
	return s.workloads.GetWorkload(ctx, applicationID, workloadID)
}

func (s *Service) getWorkloadStageConfig(ctx context.Context, workloadID shared.ID, stageKey string) (WorkloadStageConfigRef, error) {
	if s.workloads == nil {
		return WorkloadStageConfigRef{}, shared.NewError(shared.CodeNotFound, "workload stage config not found")
	}
	return s.workloads.GetWorkloadStageConfig(ctx, workloadID, stageKey)
}

func (s *Service) getWorkloadDefaultConfig(ctx context.Context, workloadID shared.ID) (WorkloadStageConfigRef, error) {
	if s.workloads == nil {
		return WorkloadStageConfigRef{}, shared.NewError(shared.CodeNotFound, "workload default config not found")
	}
	return s.workloads.GetWorkloadDefaultConfig(ctx, workloadID)
}

func normalizeContainerName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "app"
	}
	return name
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

func (s *Service) DeleteApplicationManifests(ctx context.Context, applicationID shared.ID) error {
	app, err := s.apps.GetApplication(ctx, applicationID)
	if err != nil {
		return err
	}
	stageKeys, err := s.deployedStageKeys(ctx, applicationID)
	if err != nil {
		return err
	}
	if len(stageKeys) == 0 {
		return nil
	}
	paths := make([]string, 0, len(stageKeys)*2)
	for _, stageKey := range stageKeys {
		paths = append(paths, manifestPath(app.Name, stageKey), argoApplicationPath(app.Name, stageKey))
	}
	existing, err := s.existingManifestPaths(ctx, paths)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		if _, err := s.manifest.DeleteFiles(ctx, DeleteFilesSpec{
			Branch:  "main",
			Message: fmt.Sprintf("paas: delete %s manifests", app.Name),
			Paths:   existing,
		}); err != nil {
			return err
		}
	}

	keepFiles := argoApplicationStageKeepFiles(stageKeys)
	if len(keepFiles) == 0 {
		return nil
	}
	_, err = s.manifest.CommitFiles(ctx, CommitSpec{
		Branch:  "main",
		Message: fmt.Sprintf("paas: keep %s manifest stage directories", app.Name),
		Files:   keepFiles,
	})
	return err
}

func (s *Service) deployedStageKeys(ctx context.Context, applicationID shared.ID) ([]string, error) {
	seen := map[string]struct{}{}
	var out []string
	page := shared.PageRequest{Page: 1, PageSize: 100}
	for {
		result, err := s.repo.ListDeployments(ctx, applicationID, page)
		if err != nil {
			return nil, err
		}
		for _, deployment := range result.Items {
			stageKey := normalizeStageKey(deployment.StageKey)
			if stageKey == "" {
				continue
			}
			if _, ok := seen[stageKey]; ok {
				continue
			}
			seen[stageKey] = struct{}{}
			out = append(out, stageKey)
		}
		if len(result.Items) == 0 || int64(page.Page*page.PageSize) >= result.Total {
			return out, nil
		}
		page.Page++
	}
}

func (s *Service) existingManifestPaths(ctx context.Context, paths []string) ([]string, error) {
	existing := make([]string, 0, len(paths))
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if _, err := s.manifest.ReadFile(ctx, path, "main"); err != nil {
			if shared.CodeOf(err) == shared.CodeNotFound {
				continue
			}
			return nil, err
		}
		existing = append(existing, path)
	}
	return existing, nil
}

func argoApplicationStageKeepFiles(stageKeys []string) []CommitFile {
	files := make([]CommitFile, 0, len(stageKeys))
	for _, stageKey := range stageKeys {
		stageKey = normalizeStageKey(stageKey)
		if stageKey == "" {
			continue
		}
		files = append(files, CommitFile{
			Path:    argoApplicationStageKeepPath(stageKey),
			Content: "# keep stage directory for Argo CD app-of-apps\n",
		})
	}
	return files
}

func (s *Service) ListDeployments(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[Deployment], error) {
	return s.repo.ListDeployments(ctx, applicationID, page)
}

func (s *Service) GetLatestDeploymentForStage(ctx context.Context, appID shared.ID, stageKey string) (Deployment, error) {
	return s.repo.GetLatestDeploymentForStage(ctx, appID, stageKey)
}

func ComputeStageConfigHash(templateRevisionID shared.ID, workloadConfigs []WorkloadStageConfigRef) string {
	raw, _ := json.Marshal(struct {
		TemplateRevisionID shared.ID                `json:"t"`
		Configs            []WorkloadStageConfigRef `json:"c"`
	}{templateRevisionID, workloadConfigs})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:8])
}

// RenderExpectedManifest renders the expected manifests.yaml for a stage using current config + the given freight items.
func (s *Service) RenderExpectedManifest(ctx context.Context, appID shared.ID, stageKey string, freightItems []delivery.FreightItem) (expected string, currentManifest string, err error) {
	app, err := s.apps.GetApplication(ctx, appID)
	if err != nil {
		return "", "", err
	}
	deploy, err := s.repo.GetLatestDeploymentForStage(ctx, appID, stageKey)
	if err != nil {
		return "", "", err
	}
	path := manifestPath(app.Name, stageKey)
	currentManifest, _ = s.manifest.ReadFile(ctx, path, "main")

	binding := ClusterBindingRef{ID: deploy.ClusterBindingID, StageKey: stageKey, Namespace: defaultNamespaceForStage(app, stageKey)}
	artifacts := make([]delivery.GitOpsArtifactSpec, 0, len(freightItems))
	for i, item := range freightItems {
		artifacts = append(artifacts, delivery.GitOpsArtifactSpec{
			WorkloadID:    item.WorkloadID,
			ContainerName: item.ContainerName,
			URI:           item.URI,
			Repository:    item.ImageRepository,
			Tag:           item.ImageTag,
			Digest:        item.Digest,
			IsPrimary:     i == 0,
		})
	}
	expected, _, err = s.renderK8sManifests(ctx, app, stageKey, binding, deploy.ID, artifacts)
	if err != nil {
		return "", "", err
	}
	return expected, currentManifest, nil
}

func defaultNamespaceForStage(app ApplicationRef, stageKey string) string {
	base := firstNonEmpty(app.ProjectName, app.ProjectID.String())
	stageKey = normalizeStageKey(stageKey)
	if stageKey == "" {
		return normalizeKubernetesNamespace(base)
	}
	return normalizeKubernetesNamespace(base + "-" + stageKey)
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

func (s *Service) PreviewPromotionManifest(ctx context.Context, spec delivery.GitOpsPromotionSpec) (expected string, currentManifest string, err error) {
	app, err := s.apps.GetApplication(ctx, spec.ApplicationID)
	if err != nil {
		return "", "", err
	}
	stageKey := normalizeStageKey(spec.StageKey)
	if stageKey == "" {
		return "", "", shared.NewError(shared.CodeInvalidArgument, "target_stage_key is required")
	}
	bindings, err := s.promotionTargetBindings(stageKey, spec.TargetClusters)
	if err != nil {
		return "", "", err
	}
	template, err := s.repo.FindPlatformTemplate(ctx, defaultPlatformTemplateName)
	if err != nil {
		if shared.CodeOf(err) == shared.CodeNotFound {
			template, err = s.EnsurePlatformTemplate(ctx, defaultPlatformTemplateName, defaultPlatformTemplateContent)
		}
		if err != nil {
			return "", "", err
		}
	}
	validation := validateTemplate(template.Content)
	if !validation.Valid {
		return "", "", shared.NewError(shared.CodeFailedPrecondition, strings.Join(validation.Errors, "; "))
	}
	artifacts := normalizePromotionArtifacts(spec)
	if len(artifacts) == 0 && strings.TrimSpace(spec.ImageURI) != "" {
		repository, tag := splitImage(spec.ImageURI)
		artifacts = []delivery.GitOpsArtifactSpec{{URI: strings.TrimSpace(spec.ImageURI), Repository: repository, Tag: tag, Digest: strings.TrimSpace(spec.ImageDigest), IsPrimary: true}}
	}
	if len(artifacts) == 0 {
		return "", "", shared.NewError(shared.CodeInvalidArgument, "promotion artifacts is required")
	}
	deploymentID := spec.PromotionID
	if latest, latestErr := s.repo.GetLatestDeploymentForStage(ctx, spec.ApplicationID, stageKey); latestErr == nil && !latest.ID.IsZero() {
		deploymentID = latest.ID
	}
	var expectedFiles []string
	var currentFiles []string
	for _, binding := range bindings {
		resolvedArtifacts, err := resolvePromotionArtifactsForBinding(artifacts, binding)
		if err != nil {
			return "", "", err
		}
		manifests, _, err := s.renderK8sManifests(ctx, app, stageKey, binding, deploymentID, resolvedArtifacts)
		if err != nil {
			return "", "", err
		}
		expectedFiles = append(expectedFiles, manifests)
		path := manifestPathForBinding(app.Name, stageKey, binding)
		current, readErr := s.manifest.ReadFile(ctx, path, "main")
		if readErr != nil && shared.CodeOf(readErr) != shared.CodeNotFound {
			return "", "", readErr
		}
		currentFiles = append(currentFiles, current)
	}
	return strings.Join(expectedFiles, "\n---\n"), strings.Join(currentFiles, "\n---\n"), nil
}

func (s *Service) collectStageConfigs(ctx context.Context, appID shared.ID, artifacts []delivery.GitOpsArtifactSpec, stageKey string) []WorkloadStageConfigRef {
	var configs []WorkloadStageConfigRef
	for _, artifact := range artifacts {
		if artifact.WorkloadID.IsZero() {
			continue
		}
		config, err := s.resolveWorkloadConfig(ctx, artifact.WorkloadID, stageKey)
		if err != nil {
			continue
		}
		configs = append(configs, config)
	}
	return configs
}

func (s *Service) ComputeStageConfigHashForFreightItems(ctx context.Context, templateRevisionID shared.ID, stageKey string, items []delivery.FreightItem) string {
	artifacts := make([]delivery.GitOpsArtifactSpec, 0, len(items))
	for _, item := range items {
		artifacts = append(artifacts, delivery.GitOpsArtifactSpec{
			WorkloadID:    item.WorkloadID,
			ContainerName: item.ContainerName,
			URI:           item.URI,
			Repository:    item.ImageRepository,
			Tag:           item.ImageTag,
			Digest:        item.Digest,
		})
	}
	return ComputeStageConfigHash(templateRevisionID, s.collectStageConfigs(ctx, "", artifacts, stageKey))
}

func (s *Service) resolveWorkloadConfig(ctx context.Context, workloadID shared.ID, stageKey string) (WorkloadStageConfigRef, error) {
	defaultConfig, defaultErr := s.getWorkloadDefaultConfig(ctx, workloadID)
	stageConfig, stageErr := s.getWorkloadStageConfig(ctx, workloadID, stageKey)
	switch {
	case stageErr == nil && defaultErr == nil:
		return mergeWorkloadConfig(defaultConfig, stageConfig), nil
	case stageErr == nil:
		return stageConfig, nil
	case defaultErr == nil:
		return defaultConfig, nil
	case shared.CodeOf(stageErr) != shared.CodeNotFound:
		return WorkloadStageConfigRef{}, stageErr
	default:
		return WorkloadStageConfigRef{}, defaultErr
	}
}

func mergeWorkloadConfig(base WorkloadStageConfigRef, override WorkloadStageConfigRef) WorkloadStageConfigRef {
	out := base
	if override.Replicas > 0 {
		out.Replicas = override.Replicas
	}
	if len(override.ServicePorts) > 0 {
		out.ServicePorts = override.ServicePorts
	}
	if override.ResourceRequests.CPU != "" || override.ResourceRequests.Memory != "" {
		out.ResourceRequests = override.ResourceRequests
	}
	if override.ResourceLimits.CPU != "" || override.ResourceLimits.Memory != "" {
		out.ResourceLimits = override.ResourceLimits
	}
	if len(override.Probes) > 0 {
		out.Probes = override.Probes
	}
	if len(override.EnvVars) > 0 {
		out.EnvVars = override.EnvVars
	}
	if len(override.IngressHosts) > 0 {
		out.IngressHosts = override.IngressHosts
	}
	if len(override.SecretRefs) > 0 {
		out.SecretRefs = override.SecretRefs
	}
	if len(override.ConfigFiles) > 0 {
		out.ConfigFiles = override.ConfigFiles
	}
	if len(override.WritableDirs) > 0 {
		out.WritableDirs = override.WritableDirs
	}
	if len(override.VolumeMounts) > 0 {
		out.VolumeMounts = override.VolumeMounts
	}
	if len(override.InitContainers) > 0 {
		out.InitContainers = override.InitContainers
	}
	out.ValuesOverride = mergeValuesOverride(base.ValuesOverride, override.ValuesOverride)
	return out
}

func mergeValuesOverride(base map[string]any, override map[string]any) map[string]any {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}
	out := deepCopyMap(base)
	for key, value := range override {
		if key == "containers" {
			out[key] = mergeContainerOverrides(out[key], value)
			continue
		}
		if baseMap, ok := out[key].(map[string]any); ok {
			if overrideMap, ok := value.(map[string]any); ok {
				out[key] = mergeValuesOverride(baseMap, overrideMap)
				continue
			}
		}
		out[key] = value
	}
	return out
}

func mergeContainerOverrides(base any, override any) any {
	baseItems := mapByContainerName(base)
	for _, item := range mapSlice(override) {
		name := strings.TrimSpace(fmt.Sprint(item["name"]))
		if name == "" {
			continue
		}
		if existing, ok := baseItems[name]; ok {
			baseItems[name] = mergeContainerMap(existing, item)
		} else {
			baseItems[name] = deepCopyMap(item)
		}
	}
	names := make([]string, 0, len(baseItems))
	for name := range baseItems {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]map[string]any, 0, len(names))
	for _, name := range names {
		out = append(out, baseItems[name])
	}
	return out
}

func mergeContainerMap(base map[string]any, override map[string]any) map[string]any {
	out := mergeValuesOverride(base, override)
	for _, spec := range []struct {
		key      string
		identity string
	}{
		{key: "config_files", identity: "mount_path"},
		{key: "writable_dirs", identity: "mount_path"},
		{key: "env_vars", identity: "name"},
		{key: "secret_refs", identity: "name"},
		{key: "volume_mounts", identity: "mount_path"},
		{key: "probes", identity: "name"},
	} {
		if _, ok := override[spec.key]; ok {
			out[spec.key] = mergeNamedItems(base[spec.key], override[spec.key], spec.identity)
		}
	}
	return out
}

func mergeNamedItems(base any, override any, identity string) any {
	items := map[string]map[string]any{}
	for _, item := range mapSlice(base) {
		key := strings.TrimSpace(fmt.Sprint(item[identity]))
		if key == "" {
			continue
		}
		items[key] = deepCopyMap(item)
	}
	for _, item := range mapSlice(override) {
		key := strings.TrimSpace(fmt.Sprint(item[identity]))
		if key == "" {
			continue
		}
		if existing, ok := items[key]; ok {
			items[key] = mergeValuesOverride(existing, item)
		} else {
			items[key] = deepCopyMap(item)
		}
	}
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]map[string]any, 0, len(keys))
	for _, key := range keys {
		out = append(out, items[key])
	}
	return out
}

func mapByContainerName(raw any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, item := range mapSlice(raw) {
		name := strings.TrimSpace(fmt.Sprint(item["name"]))
		if name == "" {
			continue
		}
		out[name] = deepCopyMap(item)
	}
	return out
}

func mapSlice(raw any) []map[string]any {
	var out []map[string]any
	switch items := raw.(type) {
	case []map[string]any:
		for _, item := range items {
			out = append(out, item)
		}
	case []any:
		for _, item := range items {
			if m, ok := item.(map[string]any); ok {
				out = append(out, m)
			}
		}
	}
	return out
}

func deepCopyMap(in map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range in {
		if nested, ok := value.(map[string]any); ok {
			out[key] = deepCopyMap(nested)
			continue
		}
		if items, ok := value.([]any); ok {
			copyItems := make([]any, 0, len(items))
			for _, item := range items {
				if m, ok := item.(map[string]any); ok {
					copyItems = append(copyItems, deepCopyMap(m))
				} else {
					copyItems = append(copyItems, item)
				}
			}
			out[key] = copyItems
			continue
		}
		out[key] = value
	}
	return out
}

func renderArgoApplication(app ApplicationRef, stageKey string, binding ClusterBindingRef, deploymentID shared.ID, manifestRepoURL string, manifestDir string) string {
	name := fmt.Sprintf("%s-%s", app.Name, stageKey)
	return fmt.Sprintf(`apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: %s
  finalizers:
    - resources-finalizer.argocd.argoproj.io
  labels:
    paas.shareinto.com/application-id: %s
    paas.shareinto.com/stage-key: %s
    paas.shareinto.com/deployment-id: %s
spec:
  project: default
  destination:
    server: https://kubernetes.default.svc
    namespace: %s
  source:
    repoURL: %s
    targetRevision: main
    path: %s
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
      - ServerSideApply=true
`, name, app.ID, stageKey, deploymentID, binding.Namespace, manifestRepoURL, manifestDir)
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

func normalizeStageKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
