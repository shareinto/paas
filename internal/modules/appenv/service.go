package appenv

import (
	"context"
	"strings"
	"time"

	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/modules/tenantproject"
	"github.com/shareinto/paas/internal/shared"
)

var defaultEnvironmentNames = []string{"dev", "test", "staging", "prod"}

type Service struct {
	repo                Repository
	projects            ProjectQuery
	sourceRepos         SourceRepositoryQuery
	jenkinsTemplates    JenkinsTemplateQuery
	buildEnvironments   BuildEnvironmentQuery
	runtimeEnvironments RuntimeEnvironmentQuery
	buildPipelines      BuildPipelineProvisioner
	clusters            ClusterPlacementQuery
	gitops              GitOpsEnvironmentProvisioner
	permission          PermissionChecker
	audit               AuditLogger
	events              EventPublisher
	ids                 shared.IDGenerator
	clock               shared.Clock
}

type Options struct {
	Repository                   Repository
	ProjectQuery                 ProjectQuery
	SourceRepositoryQuery        SourceRepositoryQuery
	JenkinsTemplateQuery         JenkinsTemplateQuery
	BuildEnvironmentQuery        BuildEnvironmentQuery
	RuntimeEnvironmentQuery      RuntimeEnvironmentQuery
	BuildPipelineProvisioner     BuildPipelineProvisioner
	ClusterPlacementQuery        ClusterPlacementQuery
	GitOpsEnvironmentProvisioner GitOpsEnvironmentProvisioner
	PermissionChecker            PermissionChecker
	Audit                        AuditLogger
	EventPublisher               EventPublisher
	IDGenerator                  shared.IDGenerator
	Clock                        shared.Clock
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
	return &Service{
		repo:                opts.Repository,
		projects:            opts.ProjectQuery,
		sourceRepos:         opts.SourceRepositoryQuery,
		jenkinsTemplates:    opts.JenkinsTemplateQuery,
		buildEnvironments:   opts.BuildEnvironmentQuery,
		runtimeEnvironments: opts.RuntimeEnvironmentQuery,
		buildPipelines:      opts.BuildPipelineProvisioner,
		clusters:            opts.ClusterPlacementQuery,
		gitops:              opts.GitOpsEnvironmentProvisioner,
		permission:          opts.PermissionChecker,
		audit:               audit,
		events:              events,
		ids:                 ids,
		clock:               clock,
	}
}

type CreateApplicationInput struct {
	Actor                 identityaccess.Subject         `json:"actor"`
	ProjectID             shared.ID                      `json:"project_id"`
	Name                  string                         `json:"name"`
	DisplayName           string                         `json:"display_name"`
	Description           string                         `json:"description"`
	Sources               []CreateApplicationSourceInput `json:"sources"`
	SourceRepositoryID    shared.ID                      `json:"source_repository_id,omitempty"`
	JenkinsTemplateID     shared.ID                      `json:"jenkins_template_id,omitempty"`
	RuntimeEnvironmentID  shared.ID                      `json:"runtime_environment_id,omitempty"`
	RuntimeEnvironmentIDs []shared.ID                    `json:"runtime_environment_ids,omitempty"`
	RuntimeOverrides      BuildSpec                      `json:"runtime_overrides,omitempty"`
	BuildSpec             BuildSpec                      `json:"build_spec,omitempty"`
}

type CreateApplicationSourceInput struct {
	Key                string    `json:"key"`
	DisplayName        string    `json:"display_name"`
	SourceRepositoryID shared.ID `json:"source_repository_id"`
	JenkinsTemplateID  shared.ID `json:"jenkins_template_id"`
	BuildEnvironmentID shared.ID `json:"build_environment_id"`
	SourcePath         string    `json:"source_path"`
	BuildSpec          BuildSpec `json:"build_spec"`
	DefaultRef         string    `json:"default_ref"`
	IsPrimary          bool      `json:"is_primary"`
}

type UpdateApplicationInput struct {
	Actor                 identityaccess.Subject         `json:"actor"`
	ApplicationID         shared.ID                      `json:"application_id"`
	DisplayName           string                         `json:"display_name"`
	Description           string                         `json:"description"`
	Disabled              bool                           `json:"disabled"`
	Sources               []CreateApplicationSourceInput `json:"sources,omitempty"`
	RuntimeEnvironmentID  shared.ID                      `json:"runtime_environment_id,omitempty"`
	RuntimeEnvironmentIDs []shared.ID                    `json:"runtime_environment_ids,omitempty"`
	RuntimeOverrides      BuildSpec                      `json:"runtime_overrides,omitempty"`
}

type UpdateEnvironmentStateInput struct {
	EnvironmentID shared.ID         `json:"environment_id"`
	Status        EnvironmentStatus `json:"status"`
	Message       string            `json:"message"`
	ReportedAt    *time.Time        `json:"reported_at,omitempty"`
}

type BindEnvironmentClusterInput struct {
	Actor         identityaccess.Subject `json:"actor"`
	EnvironmentID shared.ID              `json:"environment_id"`
	ClusterID     shared.ID              `json:"cluster_id"`
	ClusterName   string                 `json:"cluster_name"`
	Namespace     string                 `json:"namespace"`
}

type SetEnvironmentConfigInput struct {
	Actor         identityaccess.Subject `json:"actor"`
	EnvironmentID shared.ID              `json:"environment_id"`
	Key           string                 `json:"key"`
	Value         string                 `json:"value"`
}

type SetEnvironmentSecretInput struct {
	Actor         identityaccess.Subject `json:"actor"`
	EnvironmentID shared.ID              `json:"environment_id"`
	Key           string                 `json:"key"`
	SecretRef     string                 `json:"secret_ref"`
}

func (s *Service) CreateApplication(ctx context.Context, input CreateApplicationInput) (Application, error) {
	project, err := s.requireProject(ctx, input.ProjectID)
	if err != nil {
		return Application{}, err
	}
	if err := s.check(ctx, input.Actor, identityaccess.ResourceScope{Kind: identityaccess.ScopeProject, TenantID: project.TenantID, ProjectID: project.ID}, "application:create"); err != nil {
		return Application{}, err
	}
	name := normalizeApplicationName(input.Name)
	if err := validateApplicationName(name); err != nil {
		return Application{}, err
	}
	var sources []ApplicationSource
	if len(input.Sources) > 0 {
		sources, err = s.prepareApplicationSources(ctx, project, input.Sources, nil, BuildSpec{})
		if err != nil {
			return Application{}, err
		}
	}
	appID, err := s.ids.NewID("app")
	if err != nil {
		return Application{}, err
	}
	now := s.clock.Now()
	app := Application{
		ID:          appID,
		TenantID:    project.TenantID,
		ProjectID:   project.ID,
		Name:        name,
		DisplayName: normalizeDisplayName(input.DisplayName, name),
		Description: strings.TrimSpace(input.Description),
		Status:      ApplicationStatusActive,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	sourceKeys := make([]string, 0, len(sources))
	for i := range sources {
		sourceID, err := s.ids.NewID("app_source")
		if err != nil {
			return Application{}, err
		}
		sources[i].ID = sourceID
		sources[i].ApplicationID = app.ID
		sources[i].TenantID = project.TenantID
		sources[i].ProjectID = project.ID
		sources[i].CreatedAt = now
		sources[i].UpdatedAt = now
		sourceKeys = append(sourceKeys, sources[i].Key)
	}
	if err := s.repo.CreateApplication(ctx, app); err != nil {
		return Application{}, err
	}
	for _, source := range sources {
		if err := s.repo.CreateApplicationSource(ctx, source); err != nil {
			_ = s.repo.DeleteApplicationData(ctx, app.ID)
			return Application{}, err
		}
	}
	if err := s.createDefaultEnvironments(ctx, app, ApplicationSource{}); err != nil {
		_ = s.repo.DeleteApplicationData(ctx, app.ID)
		return Application{}, err
	}
	sourceRepositoryID := shared.ID("")
	if len(sources) > 0 {
		sourceRepositoryID = sources[0].SourceRepositoryID
	}
	if err := s.publish(ctx, "ApplicationCreated", now, ApplicationCreatedPayload{ApplicationID: app.ID, TenantID: app.TenantID, ProjectID: app.ProjectID, SourceRepositoryID: sourceRepositoryID, SourceKeys: sourceKeys, Name: app.Name}); err != nil {
		_ = s.repo.DeleteApplicationData(ctx, app.ID)
		return Application{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "application.create", ResourceType: "application", ResourceID: app.ID, Result: "succeeded", Summary: "创建应用", OccurredAt: now})
	return app, nil
}

func (s *Service) prepareApplicationSources(ctx context.Context, project tenantproject.Project, inputs []CreateApplicationSourceInput, runtimeEnvironments []RuntimeEnvironmentRef, runtimeOverrides BuildSpec) ([]ApplicationSource, error) {
	if len(inputs) == 0 {
		return nil, shared.NewError(shared.CodeInvalidArgument, "sources is required")
	}
	sources := make([]ApplicationSource, 0, len(inputs))
	seen := map[string]struct{}{}
	primaryIndex := -1
	for i, input := range inputs {
		key := normalizeSourceKey(input.Key)
		if key == "" && len(inputs) == 1 {
			key = "main"
		}
		if err := validateSourceKey(key); err != nil {
			return nil, err
		}
		if _, ok := seen[key]; ok {
			return nil, shared.NewError(shared.CodeConflict, "source key already exists")
		}
		seen[key] = struct{}{}
		sourceRepo, err := s.requireSourceRepository(ctx, project, input.SourceRepositoryID)
		if err != nil {
			return nil, err
		}
		spec := input.BuildSpec
		if strings.TrimSpace(spec.SourcePath) == "" {
			spec.SourcePath = input.SourcePath
		}
		if strings.TrimSpace(input.DefaultRef) != "" {
			spec.DefaultRef = strings.TrimSpace(input.DefaultRef)
		}
		buildEnvironmentID, err := s.applyBuildEnvironment(ctx, input.BuildEnvironmentID, &spec)
		if err != nil {
			return nil, err
		}
		if len(runtimeEnvironments) > 0 {
			applyRuntimeEnvironment(runtimeEnvironments[0], runtimeOverrides, &spec)
		}
		spec, err = s.validateBuildSpec(spec, sourceRepo.DefaultBranch)
		if err != nil {
			return nil, err
		}
		templateID := shared.ID("")
		if buildEnvironmentID.IsZero() {
			templateID, err = s.requireEnabledJenkinsTemplate(ctx, input.JenkinsTemplateID)
			if err != nil {
				return nil, err
			}
		} else if !input.JenkinsTemplateID.IsZero() && input.JenkinsTemplateID != buildEnvironmentID {
			templateID, err = s.requireEnabledJenkinsTemplate(ctx, input.JenkinsTemplateID)
			if err != nil {
				return nil, err
			}
		}
		isPrimary := input.IsPrimary
		if isPrimary {
			if primaryIndex >= 0 {
				return nil, shared.NewError(shared.CodeInvalidArgument, "only one source can be primary")
			}
			primaryIndex = i
		}
		sources = append(sources, ApplicationSource{
			TenantID:           project.TenantID,
			ProjectID:          project.ID,
			Key:                key,
			DisplayName:        normalizeDisplayName(input.DisplayName, key),
			SourceRepositoryID: sourceRepo.ID,
			JenkinsTemplateID:  templateID,
			BuildEnvironmentID: buildEnvironmentID,
			SourcePath:         spec.SourcePath,
			BuildSpec:          spec,
			IsPrimary:          isPrimary,
		})
	}
	if primaryIndex < 0 {
		sources[0].IsPrimary = true
	} else if primaryIndex != 0 {
		primary := sources[primaryIndex]
		copy(sources[1:primaryIndex+1], sources[0:primaryIndex])
		sources[0] = primary
	}
	return sources, nil
}

func (s *Service) ensureBuildPipeline(ctx context.Context, applicationID shared.ID) error {
	if s.buildPipelines == nil {
		return nil
	}
	if err := s.buildPipelines.EnsureBuildPipeline(ctx, applicationID); err != nil {
		switch shared.CodeOf(err) {
		case shared.CodeInvalidArgument, shared.CodeNotFound, shared.CodeFailedPrecondition:
			return err
		default:
			return shared.WrapError(shared.CodeUnavailable, "jenkins pipeline provision failed", err)
		}
	}
	return nil
}

func (s *Service) UpdateApplication(ctx context.Context, input UpdateApplicationInput) (Application, error) {
	app, err := s.repo.GetApplication(ctx, input.ApplicationID)
	if err != nil {
		return Application{}, err
	}
	if err := s.check(ctx, input.Actor, identityaccess.ResourceScope{Kind: identityaccess.ScopeApplication, TenantID: app.TenantID, ProjectID: app.ProjectID, ApplicationID: app.ID}, "application:update"); err != nil {
		return Application{}, err
	}
	project, err := s.requireProject(ctx, app.ProjectID)
	if err != nil {
		return Application{}, err
	}
	if len(input.Sources) > 0 {
		sources, err := s.prepareApplicationSources(ctx, project, input.Sources, nil, BuildSpec{})
		if err != nil {
			return Application{}, err
		}
		existingSources, _ := s.repo.ListApplicationSources(ctx, app.ID)
		existingByKey := map[string]ApplicationSource{}
		for _, source := range existingSources {
			existingByKey[source.Key] = source
		}
		now := s.clock.Now()
		for i := range sources {
			if existing, ok := existingByKey[sources[i].Key]; ok {
				sources[i].ID = existing.ID
				sources[i].CreatedAt = existing.CreatedAt
			} else {
				sourceID, err := s.ids.NewID("app_source")
				if err != nil {
					return Application{}, err
				}
				sources[i].ID = sourceID
				sources[i].CreatedAt = now
			}
			sources[i].ApplicationID = app.ID
			sources[i].TenantID = app.TenantID
			sources[i].ProjectID = app.ProjectID
			sources[i].UpdatedAt = now
		}
		if err := s.repo.ReplaceApplicationSources(ctx, app.ID, sources); err != nil {
			return Application{}, err
		}
	}
	app.DisplayName = normalizeDisplayName(input.DisplayName, app.Name)
	app.Description = strings.TrimSpace(input.Description)
	app.Status = ApplicationStatusActive
	if input.Disabled {
		app.Status = ApplicationStatusDisabled
	}
	app.UpdatedAt = s.clock.Now()
	if err := s.repo.UpdateApplication(ctx, app); err != nil {
		return Application{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "application.update", ResourceType: "application", ResourceID: app.ID, Result: "succeeded", Summary: "更新应用", OccurredAt: app.UpdatedAt})
	return app, nil
}

func (s *Service) DeleteApplication(ctx context.Context, actor identityaccess.Subject, applicationID shared.ID) error {
	app, err := s.repo.GetApplication(ctx, applicationID)
	if err != nil {
		return err
	}
	if err := s.check(ctx, actor, identityaccess.ResourceScope{Kind: identityaccess.ScopeApplication, TenantID: app.TenantID, ProjectID: app.ProjectID, ApplicationID: app.ID}, "application:delete"); err != nil {
		return err
	}
	now := s.clock.Now()
	if s.buildPipelines != nil {
		if err := s.buildPipelines.DeleteBuildPipeline(ctx, app.ID); err != nil {
			return err
		}
	}
	if err := s.repo.DeleteApplicationData(ctx, app.ID); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: actor.ID, Action: "application.delete", ResourceType: "application", ResourceID: app.ID, Result: "succeeded", Summary: "删除应用", OccurredAt: now})
	return nil
}

func (s *Service) GetApplication(ctx context.Context, id shared.ID) (Application, error) {
	return s.repo.GetApplication(ctx, id)
}

func (s *Service) ListApplicationsByProject(ctx context.Context, projectID shared.ID, page shared.PageRequest) (shared.PageResult[Application], error) {
	if _, err := s.requireProject(ctx, projectID); err != nil {
		return shared.PageResult[Application]{}, err
	}
	return s.repo.ListApplicationsByProject(ctx, projectID, page)
}

func (s *Service) SyncRuntimeEnvironmentSnapshot(ctx context.Context, input RuntimeEnvironmentSnapshotInput) (int, error) {
	if input.Environment.ID.IsZero() {
		return 0, shared.NewError(shared.CodeInvalidArgument, "runtime environment id is required")
	}
	if strings.TrimSpace(input.Environment.RuntimeBaseImage) == "" {
		return 0, shared.NewError(shared.CodeInvalidArgument, "runtime_base_image is required")
	}
	now := s.clock.Now()
	applications := []Application{}
	total := 0
	for page := 1; ; page++ {
		result, err := s.repo.ListApplicationsByRuntimeEnvironment(ctx, input.Environment.ID, shared.PageRequest{Page: page, PageSize: 100})
		if err != nil {
			return total, err
		}
		if len(result.Items) == 0 {
			break
		}
		applications = append(applications, result.Items...)
		if int64(page*100) >= result.Total {
			break
		}
	}
	for _, app := range applications {
		updated := false
		for i := range app.RuntimeEnvironments {
			if app.RuntimeEnvironments[i].ID != input.Environment.ID {
				continue
			}
			app.RuntimeEnvironments[i] = ApplicationRuntimeEnvironment{
				ID:                 input.Environment.ID,
				Name:               input.Environment.Name,
				RuntimeBaseImage:   input.Environment.RuntimeBaseImage,
				ArtifactDeployPath: input.Environment.ArtifactDeployPath,
				DockerfilePath:     input.Environment.DockerfilePath,
			}
			updated = true
		}
		if !updated {
			continue
		}
		if app.RuntimeEnvironmentID == input.Environment.ID {
			sources, err := s.repo.ListApplicationSources(ctx, app.ID)
			if err != nil && shared.CodeOf(err) != shared.CodeNotFound {
				return total, err
			}
			for i := range sources {
				sources[i].BuildSpec.RuntimeBaseImage = input.Environment.RuntimeBaseImage
				sources[i].BuildSpec.ArtifactDeployPath = input.Environment.ArtifactDeployPath
				sources[i].UpdatedAt = now
			}
			if len(sources) > 0 {
				if err := s.repo.ReplaceApplicationSources(ctx, app.ID, sources); err != nil {
					return total, err
				}
			}
		}
		app.UpdatedAt = now
		if err := s.repo.UpdateApplication(ctx, app); err != nil {
			return total, err
		}
		_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "application.runtime_environment.sync", ResourceType: "application", ResourceID: app.ID, Result: "succeeded", Summary: "同步运行时环境快照", OccurredAt: now})
		total++
	}
	return total, nil
}

func (s *Service) GetApplicationSource(ctx context.Context, applicationID shared.ID) (ApplicationSource, error) {
	return s.repo.GetApplicationSource(ctx, applicationID)
}

func (s *Service) ListApplicationSources(ctx context.Context, applicationID shared.ID) ([]ApplicationSource, error) {
	return s.repo.ListApplicationSources(ctx, applicationID)
}

func (s *Service) ListEnvironments(ctx context.Context, applicationID shared.ID) ([]Environment, error) {
	if _, err := s.repo.GetApplication(ctx, applicationID); err != nil {
		return nil, err
	}
	return s.repo.ListEnvironmentsByApplication(ctx, applicationID)
}

func (s *Service) GetEnvironment(ctx context.Context, id shared.ID) (Environment, error) {
	return s.repo.GetEnvironment(ctx, id)
}

func (s *Service) BindEnvironmentCluster(ctx context.Context, input BindEnvironmentClusterInput) (EnvironmentClusterBinding, error) {
	env, err := s.repo.GetEnvironment(ctx, input.EnvironmentID)
	if err != nil {
		return EnvironmentClusterBinding{}, err
	}
	if err := s.check(ctx, input.Actor, identityaccess.ResourceScope{Kind: identityaccess.ScopeEnvironment, TenantID: env.TenantID, ProjectID: env.ProjectID, ApplicationID: env.ApplicationID, EnvironmentID: env.ID}, "environment:update"); err != nil {
		return EnvironmentClusterBinding{}, err
	}
	candidate := ClusterCandidate{ClusterID: input.ClusterID, ClusterName: strings.TrimSpace(input.ClusterName), Namespace: strings.TrimSpace(input.Namespace)}
	if candidate.ClusterID.IsZero() || candidate.ClusterName == "" || candidate.Namespace == "" {
		return EnvironmentClusterBinding{}, shared.NewError(shared.CodeInvalidArgument, "cluster_id, cluster_name and namespace are required")
	}
	return s.createBindingAndProvision(ctx, env, candidate)
}

func (s *Service) SetEnvironmentConfig(ctx context.Context, input SetEnvironmentConfigInput) (EnvironmentConfig, error) {
	env, err := s.repo.GetEnvironment(ctx, input.EnvironmentID)
	if err != nil {
		return EnvironmentConfig{}, err
	}
	if err := s.check(ctx, input.Actor, identityaccess.ResourceScope{Kind: identityaccess.ScopeEnvironment, TenantID: env.TenantID, ProjectID: env.ProjectID, ApplicationID: env.ApplicationID, EnvironmentID: env.ID}, "environment:update"); err != nil {
		return EnvironmentConfig{}, err
	}
	key := strings.TrimSpace(input.Key)
	if key == "" {
		return EnvironmentConfig{}, shared.NewError(shared.CodeInvalidArgument, "config key is required")
	}
	id, err := s.ids.NewID("env_config")
	if err != nil {
		return EnvironmentConfig{}, err
	}
	now := s.clock.Now()
	config := EnvironmentConfig{ID: id, TenantID: env.TenantID, ProjectID: env.ProjectID, ApplicationID: env.ApplicationID, EnvironmentID: env.ID, Key: key, Value: strings.TrimSpace(input.Value), CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateEnvironmentConfig(ctx, config); err != nil {
		return EnvironmentConfig{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "environment_config.update", ResourceType: "environment_config", ResourceID: config.ID, Result: "succeeded", Summary: "修改环境配置", OccurredAt: now})
	return config, nil
}

func (s *Service) SetEnvironmentSecret(ctx context.Context, input SetEnvironmentSecretInput) (EnvironmentSecret, error) {
	env, err := s.repo.GetEnvironment(ctx, input.EnvironmentID)
	if err != nil {
		return EnvironmentSecret{}, err
	}
	if err := s.check(ctx, input.Actor, identityaccess.ResourceScope{Kind: identityaccess.ScopeEnvironment, TenantID: env.TenantID, ProjectID: env.ProjectID, ApplicationID: env.ApplicationID, EnvironmentID: env.ID}, "secret:update"); err != nil {
		return EnvironmentSecret{}, err
	}
	key := strings.TrimSpace(input.Key)
	secretRef := strings.TrimSpace(input.SecretRef)
	if key == "" || secretRef == "" {
		return EnvironmentSecret{}, shared.NewError(shared.CodeInvalidArgument, "secret key and secret_ref are required")
	}
	id, err := s.ids.NewID("env_secret")
	if err != nil {
		return EnvironmentSecret{}, err
	}
	now := s.clock.Now()
	secret := EnvironmentSecret{ID: id, TenantID: env.TenantID, ProjectID: env.ProjectID, ApplicationID: env.ApplicationID, EnvironmentID: env.ID, Key: key, SecretRef: secretRef, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateEnvironmentSecret(ctx, secret); err != nil {
		return EnvironmentSecret{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "environment_secret.update", ResourceType: "environment_secret", ResourceID: secret.ID, Result: "succeeded", Summary: "修改环境密钥元数据", OccurredAt: now})
	return secret, nil
}

func (s *Service) GetEnvironmentState(ctx context.Context, environmentID shared.ID) (EnvironmentState, error) {
	return s.repo.GetEnvironmentState(ctx, environmentID)
}

func (s *Service) ListEnvironmentStates(ctx context.Context, applicationID shared.ID) ([]EnvironmentState, error) {
	if _, err := s.repo.GetApplication(ctx, applicationID); err != nil {
		return nil, err
	}
	return s.repo.ListEnvironmentStatesByApplication(ctx, applicationID)
}

func (s *Service) UpdateEnvironmentState(ctx context.Context, input UpdateEnvironmentStateInput) (EnvironmentState, error) {
	env, err := s.repo.GetEnvironment(ctx, input.EnvironmentID)
	if err != nil {
		return EnvironmentState{}, err
	}
	if err := shared.ValidateStatus(string(input.Status), AllowedEnvironmentStatuses); err != nil {
		return EnvironmentState{}, err
	}
	now := s.clock.Now()
	state := EnvironmentState{
		TenantID:       env.TenantID,
		ProjectID:      env.ProjectID,
		ApplicationID:  env.ApplicationID,
		EnvironmentID:  env.ID,
		Status:         input.Status,
		Message:        strings.TrimSpace(input.Message),
		LastReportedAt: input.ReportedAt,
		UpdatedAt:      now,
	}
	if err := s.repo.SaveEnvironmentState(ctx, state); err != nil {
		return EnvironmentState{}, err
	}
	_ = s.appendEnvironmentEvent(ctx, env, "environment_state.updated", state.Status, state.Message, now)
	return state, nil
}

func (s *Service) ListEnvironmentEvents(ctx context.Context, environmentID shared.ID, page shared.PageRequest) (shared.PageResult[EnvironmentEvent], error) {
	if _, err := s.repo.GetEnvironment(ctx, environmentID); err != nil {
		return shared.PageResult[EnvironmentEvent]{}, err
	}
	return s.repo.ListEnvironmentEvents(ctx, environmentID, page)
}

func (s *Service) createDefaultEnvironments(ctx context.Context, app Application, source ApplicationSource) error {
	for _, name := range defaultEnvironmentNames {
		envID, err := s.ids.NewID("env")
		if err != nil {
			return err
		}
		now := s.clock.Now()
		env := Environment{
			ID:            envID,
			TenantID:      app.TenantID,
			ProjectID:     app.ProjectID,
			ApplicationID: app.ID,
			Name:          name,
			DisplayName:   defaultEnvironmentDisplayName(name),
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := s.repo.CreateEnvironment(ctx, env); err != nil {
			return err
		}
		status := EnvironmentStatusPendingClusterBinding
		message := "等待绑定可用集群"
		if s.clusters != nil {
			candidate, ok, err := s.clusters.SelectCluster(ctx, env)
			if err != nil {
				return err
			}
			if ok {
				if _, err := s.createBindingAndProvision(ctx, env, candidate); err != nil {
					return err
				}
				continue
			}
		}
		state := EnvironmentState{TenantID: env.TenantID, ProjectID: env.ProjectID, ApplicationID: env.ApplicationID, EnvironmentID: env.ID, Status: status, Message: message, UpdatedAt: now}
		if err := s.repo.SaveEnvironmentState(ctx, state); err != nil {
			return err
		}
		if err := s.appendEnvironmentEvent(ctx, env, "environment.created", status, message, now); err != nil {
			return err
		}
		_ = source
	}
	return nil
}

func (s *Service) createBindingAndProvision(ctx context.Context, env Environment, candidate ClusterCandidate) (EnvironmentClusterBinding, error) {
	if candidate.ClusterID.IsZero() || strings.TrimSpace(candidate.ClusterName) == "" || strings.TrimSpace(candidate.Namespace) == "" {
		return EnvironmentClusterBinding{}, shared.NewError(shared.CodeInvalidArgument, "cluster candidate is incomplete")
	}
	app, err := s.repo.GetApplication(ctx, env.ApplicationID)
	if err != nil {
		return EnvironmentClusterBinding{}, err
	}
	source, sourceErr := s.repo.GetApplicationSource(ctx, env.ApplicationID)
	if sourceErr != nil && shared.CodeOf(sourceErr) != shared.CodeNotFound {
		return EnvironmentClusterBinding{}, sourceErr
	}
	id, err := s.ids.NewID("env_binding")
	if err != nil {
		return EnvironmentClusterBinding{}, err
	}
	now := s.clock.Now()
	binding := EnvironmentClusterBinding{
		ID:            id,
		TenantID:      env.TenantID,
		ProjectID:     env.ProjectID,
		ApplicationID: env.ApplicationID,
		EnvironmentID: env.ID,
		ClusterID:     candidate.ClusterID,
		ClusterName:   strings.TrimSpace(candidate.ClusterName),
		Namespace:     strings.TrimSpace(candidate.Namespace),
		Status:        EnvironmentClusterBindingActive,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if s.gitops != nil && shared.CodeOf(sourceErr) != shared.CodeNotFound {
		if err := s.gitops.ProvisionEnvironment(ctx, GitOpsEnvironmentSpec{
			TenantID:           env.TenantID,
			ProjectID:          env.ProjectID,
			ApplicationID:      env.ApplicationID,
			EnvironmentID:      env.ID,
			ApplicationName:    app.Name,
			EnvironmentName:    env.Name,
			SourceRepositoryID: source.SourceRepositoryID,
			SourcePath:         source.SourcePath,
			ClusterID:          candidate.ClusterID,
			Namespace:          strings.TrimSpace(candidate.Namespace),
		}); err != nil {
			return EnvironmentClusterBinding{}, err
		}
	}
	if err := s.repo.CreateEnvironmentClusterBinding(ctx, binding); err != nil {
		return EnvironmentClusterBinding{}, err
	}
	state := EnvironmentState{TenantID: env.TenantID, ProjectID: env.ProjectID, ApplicationID: env.ApplicationID, EnvironmentID: env.ID, Status: EnvironmentStatusClusterBound, Message: "已绑定集群，等待部署", UpdatedAt: now}
	if err := s.repo.SaveEnvironmentState(ctx, state); err != nil {
		return EnvironmentClusterBinding{}, err
	}
	if err := s.appendEnvironmentEvent(ctx, env, "environment.cluster_bound", state.Status, state.Message, now); err != nil {
		return EnvironmentClusterBinding{}, err
	}
	return binding, nil
}

func (s *Service) validateBuildSpec(input BuildSpec, defaultRef string) (BuildSpec, error) {
	sourcePath, err := normalizeRelativePath(input.SourcePath)
	if err != nil {
		return BuildSpec{}, err
	}
	if strings.TrimSpace(input.BuildCommand) == "" {
		return BuildSpec{}, shared.NewError(shared.CodeInvalidArgument, "build_command is required")
	}
	artifactCopyCommand := strings.TrimSpace(input.ArtifactCopyCommand)
	if artifactCopyCommand == "" {
		return BuildSpec{}, shared.NewError(shared.CodeInvalidArgument, "artifact_copy_command is required")
	}
	runtimeBaseImage := strings.TrimSpace(input.RuntimeBaseImage)
	if runtimeBaseImage == "" {
		return BuildSpec{}, shared.NewError(shared.CodeInvalidArgument, "runtime_base_image is required")
	}
	artifactDeployPath := strings.TrimSpace(input.ArtifactDeployPath)
	if artifactDeployPath != "" && (!strings.HasPrefix(artifactDeployPath, "/") || strings.Contains(artifactDeployPath, "..")) {
		return BuildSpec{}, shared.NewError(shared.CodeInvalidArgument, "artifact_deploy_path must be absolute and stay under runtime root")
	}
	ref := strings.TrimSpace(input.DefaultRef)
	if ref == "" {
		ref = strings.TrimSpace(defaultRef)
	}
	if ref == "" {
		ref = "main"
	}
	return BuildSpec{
		SourcePath:          sourcePath,
		BuildCommand:        strings.TrimSpace(input.BuildCommand),
		ArtifactCopyCommand: artifactCopyCommand,
		RuntimeBaseImage:    runtimeBaseImage,
		ArtifactDeployPath:  artifactDeployPath,
		DefaultRef:          ref,
	}, nil
}

func (s *Service) requireProject(ctx context.Context, projectID shared.ID) (tenantproject.Project, error) {
	if projectID.IsZero() {
		return tenantproject.Project{}, shared.NewError(shared.CodeInvalidArgument, "project_id is required")
	}
	if s.projects == nil {
		return tenantproject.Project{}, shared.NewError(shared.CodeFailedPrecondition, "project query port is required")
	}
	return s.projects.GetProject(ctx, projectID)
}

func (s *Service) requireSourceRepository(ctx context.Context, project tenantproject.Project, sourceRepositoryID shared.ID) (SourceRepositoryRef, error) {
	if sourceRepositoryID.IsZero() {
		return SourceRepositoryRef{}, shared.NewError(shared.CodeInvalidArgument, "source_repository_id is required")
	}
	if s.sourceRepos == nil {
		return SourceRepositoryRef{}, shared.NewError(shared.CodeFailedPrecondition, "source repository query port is required")
	}
	repository, err := s.sourceRepos.GetSourceRepository(ctx, sourceRepositoryID)
	if err != nil {
		return SourceRepositoryRef{}, err
	}
	if repository.TenantID != project.TenantID || repository.ProjectID != project.ID {
		return SourceRepositoryRef{}, shared.NewError(shared.CodeInvalidArgument, "source repository does not belong to project")
	}
	if strings.TrimSpace(repository.Status) != "" && repository.Status != "ready" {
		return SourceRepositoryRef{}, shared.NewError(shared.CodeFailedPrecondition, "source repository is not ready")
	}
	return repository, nil
}

func (s *Service) requireEnabledJenkinsTemplate(ctx context.Context, templateID shared.ID) (shared.ID, error) {
	if s.jenkinsTemplates == nil {
		if templateID.IsZero() {
			return "java-unified-v1", nil
		}
		return templateID, nil
	}
	var (
		template JenkinsTemplateRef
		err      error
	)
	if templateID.IsZero() {
		template, err = s.jenkinsTemplates.FindDefaultJenkinsJobTemplate(ctx)
	} else {
		template, err = s.jenkinsTemplates.GetJenkinsJobTemplate(ctx, templateID)
	}
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(template.Status) != "enabled" {
		return "", shared.NewError(shared.CodeFailedPrecondition, "jenkins template is disabled")
	}
	return template.ID, nil
}

func (s *Service) applyBuildEnvironment(ctx context.Context, environmentID shared.ID, spec *BuildSpec) (shared.ID, error) {
	if s.buildEnvironments == nil {
		return environmentID, nil
	}
	var (
		environment BuildEnvironmentRef
		err         error
	)
	if environmentID.IsZero() {
		environment, err = s.buildEnvironments.FindDefaultBuildEnvironment(ctx)
	} else {
		environment, err = s.buildEnvironments.GetBuildEnvironment(ctx, environmentID)
	}
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(environment.Status) != "enabled" {
		return "", shared.NewError(shared.CodeFailedPrecondition, "build environment is disabled")
	}
	return environment.ID, nil
}

func (s *Service) requireEnabledRuntimeEnvironment(ctx context.Context, environmentID shared.ID) (RuntimeEnvironmentRef, error) {
	if s.runtimeEnvironments == nil {
		return RuntimeEnvironmentRef{ID: environmentID}, nil
	}
	var (
		environment RuntimeEnvironmentRef
		err         error
	)
	if environmentID.IsZero() {
		environment, err = s.runtimeEnvironments.FindDefaultRuntimeEnvironment(ctx)
	} else {
		environment, err = s.runtimeEnvironments.GetRuntimeEnvironment(ctx, environmentID)
	}
	if err != nil {
		return RuntimeEnvironmentRef{}, err
	}
	if strings.TrimSpace(environment.Status) != "enabled" {
		return RuntimeEnvironmentRef{}, shared.NewError(shared.CodeFailedPrecondition, "runtime environment is disabled")
	}
	return environment, nil
}

func (s *Service) requireEnabledRuntimeEnvironments(ctx context.Context, primaryID shared.ID, ids []shared.ID) ([]RuntimeEnvironmentRef, error) {
	seen := map[shared.ID]struct{}{}
	ordered := make([]shared.ID, 0, len(ids)+1)
	if !primaryID.IsZero() {
		ordered = append(ordered, primaryID)
		seen[primaryID] = struct{}{}
	}
	for _, id := range ids {
		if id.IsZero() {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ordered = append(ordered, id)
	}
	if len(ordered) == 0 {
		environment, err := s.requireEnabledRuntimeEnvironment(ctx, "")
		if err != nil {
			return nil, err
		}
		return []RuntimeEnvironmentRef{environment}, nil
	}
	out := make([]RuntimeEnvironmentRef, 0, len(ordered))
	for _, id := range ordered {
		environment, err := s.requireEnabledRuntimeEnvironment(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, environment)
	}
	return out, nil
}

func applicationRuntimeEnvironments(environments []RuntimeEnvironmentRef) []ApplicationRuntimeEnvironment {
	out := make([]ApplicationRuntimeEnvironment, 0, len(environments))
	for _, environment := range environments {
		if environment.ID.IsZero() && strings.TrimSpace(environment.Name) == "" && strings.TrimSpace(environment.RuntimeBaseImage) == "" {
			continue
		}
		out = append(out, ApplicationRuntimeEnvironment{ID: environment.ID, Name: environment.Name, RuntimeBaseImage: environment.RuntimeBaseImage, ArtifactDeployPath: environment.ArtifactDeployPath, DockerfilePath: environment.DockerfilePath})
	}
	return out
}

func applyRuntimeEnvironment(environment RuntimeEnvironmentRef, overrides BuildSpec, spec *BuildSpec) {
	if !environment.ID.IsZero() && strings.TrimSpace(environment.RuntimeBaseImage) != "" {
		spec.RuntimeBaseImage = environment.RuntimeBaseImage
		spec.ArtifactDeployPath = environment.ArtifactDeployPath
	}
	if strings.TrimSpace(overrides.ArtifactDeployPath) != "" {
		spec.ArtifactDeployPath = strings.TrimSpace(overrides.ArtifactDeployPath)
	}
}

func (s *Service) check(ctx context.Context, actor identityaccess.Subject, resource identityaccess.ResourceScope, action identityaccess.Permission) error {
	if actor.ID.IsZero() {
		return shared.NewError(shared.CodeUnauthenticated, "actor is required")
	}
	if s.permission == nil {
		return nil
	}
	return s.permission.Check(ctx, actor, resource, action)
}

func (s *Service) appendEnvironmentEvent(ctx context.Context, env Environment, eventType string, status EnvironmentStatus, message string, occurredAt time.Time) error {
	id, err := s.ids.NewID("env_evt")
	if err != nil {
		return err
	}
	return s.repo.AppendEnvironmentEvent(ctx, EnvironmentEvent{ID: id, TenantID: env.TenantID, ProjectID: env.ProjectID, ApplicationID: env.ApplicationID, EnvironmentID: env.ID, Type: eventType, Status: status, Message: strings.TrimSpace(message), OccurredAt: occurredAt})
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

func defaultEnvironmentDisplayName(name string) string {
	switch name {
	case "dev":
		return "开发环境"
	case "test":
		return "测试环境"
	case "staging":
		return "预发环境"
	case "prod":
		return "生产环境"
	default:
		return name
	}
}
