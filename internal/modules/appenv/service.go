package appenv

import (
	"context"
	"strings"
	"time"

	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/modules/tenantproject"
	"github.com/shareinto/paas/internal/shared"
)

type Service struct {
	repo                Repository
	projects            ProjectQuery
	sourceRepos         SourceRepositoryQuery
	jenkinsTemplates    JenkinsTemplateQuery
	buildEnvironments   BuildEnvironmentQuery
	runtimeEnvironments RuntimeEnvironmentQuery
	buildPipelines      BuildPipelineProvisioner
	buildPipelineCmd    BuildPipelineCommand
	buildPipelineQuery  BuildPipelineQuery
	manifestCleaner     ApplicationManifestCleaner
	permission          PermissionChecker
	audit               AuditLogger
	events              EventPublisher
	ids                 shared.IDGenerator
	clock               shared.Clock
}

type Options struct {
	Repository               Repository
	ProjectQuery             ProjectQuery
	SourceRepositoryQuery    SourceRepositoryQuery
	JenkinsTemplateQuery     JenkinsTemplateQuery
	BuildEnvironmentQuery    BuildEnvironmentQuery
	RuntimeEnvironmentQuery  RuntimeEnvironmentQuery
	BuildPipelineProvisioner BuildPipelineProvisioner
	BuildPipelineCommand     BuildPipelineCommand
	BuildPipelineQuery       BuildPipelineQuery
	ManifestCleaner          ApplicationManifestCleaner
	PermissionChecker        PermissionChecker
	Audit                    AuditLogger
	EventPublisher           EventPublisher
	IDGenerator              shared.IDGenerator
	Clock                    shared.Clock
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
		buildPipelineCmd:    opts.BuildPipelineCommand,
		buildPipelineQuery:  opts.BuildPipelineQuery,
		manifestCleaner:     opts.ManifestCleaner,
		permission:          opts.PermissionChecker,
		audit:               audit,
		events:              events,
		ids:                 ids,
		clock:               clock,
	}
}

func (s *Service) SetManifestCleaner(cleaner ApplicationManifestCleaner) {
	s.manifestCleaner = cleaner
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

type UpdateApplicationStageStateInput struct {
	ApplicationID shared.ID              `json:"application_id"`
	StageKey      string                 `json:"stage_key"`
	Status        ApplicationStageStatus `json:"status"`
	Message       string                 `json:"message"`
	ReportedAt    *time.Time             `json:"reported_at,omitempty"`
}

type CreateWorkloadInput struct {
	Actor           identityaccess.Subject `json:"actor"`
	ApplicationID   shared.ID              `json:"application_id"`
	Name            string                 `json:"name"`
	DisplayName     string                 `json:"display_name"`
	WorkloadType    WorkloadType           `json:"workload_type"`
	Description     string                 `json:"description"`
	ImageSourceMode string                 `json:"image_source_mode"`
	PipelineID      shared.ID              `json:"pipeline_id"`
}

type CreateWorkloadWithPipelineInput struct {
	Actor         identityaccess.Subject         `json:"actor"`
	ApplicationID shared.ID                      `json:"application_id"`
	Workload      CreateWorkloadInput            `json:"workload"`
	Pipeline      CreateBuildPipelineInput       `json:"pipeline"`
	DefaultConfig SaveWorkloadDefaultConfigInput `json:"default_config"`
}

type CreateWorkloadWithPipelineResult struct {
	Workload      Workload            `json:"workload"`
	Pipeline      BuildPipelineRef    `json:"pipeline"`
	DefaultConfig WorkloadStageConfig `json:"default_config"`
}

type UpdateWorkloadInput struct {
	Actor           identityaccess.Subject `json:"actor"`
	ApplicationID   shared.ID              `json:"application_id"`
	WorkloadID      shared.ID              `json:"workload_id"`
	Name            string                 `json:"name"`
	DisplayName     string                 `json:"display_name"`
	WorkloadType    WorkloadType           `json:"workload_type"`
	Description     string                 `json:"description"`
	ImageSourceMode string                 `json:"image_source_mode"`
	PipelineID      shared.ID              `json:"pipeline_id"`
}

type WorkloadStatusInput struct {
	Actor         identityaccess.Subject `json:"actor"`
	ApplicationID shared.ID              `json:"application_id"`
	WorkloadID    shared.ID              `json:"workload_id"`
}

type SaveWorkloadStageConfigInput struct {
	Actor            identityaccess.Subject  `json:"actor"`
	ApplicationID    shared.ID               `json:"application_id"`
	WorkloadID       shared.ID               `json:"workload_id"`
	StageKey         string                  `json:"stage_key"`
	Replicas         int                     `json:"replicas"`
	ServicePorts     []WorkloadServicePort   `json:"service_ports"`
	ResourceRequests WorkloadResourceList    `json:"resource_requests"`
	ResourceLimits   WorkloadResourceList    `json:"resource_limits"`
	Probes           []WorkloadProbe         `json:"probes"`
	IngressHosts     []WorkloadIngressHost   `json:"ingress_hosts"`
	EnvVars          []WorkloadEnvVar        `json:"env_vars"`
	SecretRefs       []WorkloadSecretRef     `json:"secret_refs"`
	ConfigFiles      []WorkloadConfigFile    `json:"config_files"`
	WritableDirs     []WorkloadWritableDir   `json:"writable_dirs"`
	VolumeMounts     []WorkloadVolumeMount   `json:"volume_mounts"`
	InitContainers   []WorkloadInitContainer `json:"init_containers"`
	ValuesOverride   map[string]any          `json:"values_override"`
}

type SaveWorkloadDefaultConfigInput struct {
	Actor            identityaccess.Subject  `json:"actor"`
	ApplicationID    shared.ID               `json:"application_id"`
	WorkloadID       shared.ID               `json:"workload_id"`
	Replicas         int                     `json:"replicas"`
	ServicePorts     []WorkloadServicePort   `json:"service_ports"`
	ResourceRequests WorkloadResourceList    `json:"resource_requests"`
	ResourceLimits   WorkloadResourceList    `json:"resource_limits"`
	Probes           []WorkloadProbe         `json:"probes"`
	IngressHosts     []WorkloadIngressHost   `json:"ingress_hosts"`
	EnvVars          []WorkloadEnvVar        `json:"env_vars"`
	SecretRefs       []WorkloadSecretRef     `json:"secret_refs"`
	ConfigFiles      []WorkloadConfigFile    `json:"config_files"`
	WritableDirs     []WorkloadWritableDir   `json:"writable_dirs"`
	VolumeMounts     []WorkloadVolumeMount   `json:"volume_mounts"`
	InitContainers   []WorkloadInitContainer `json:"init_containers"`
	ValuesOverride   map[string]any          `json:"values_override"`
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
	if s.manifestCleaner != nil {
		if err := s.manifestCleaner.DeleteApplicationManifests(ctx, app.ID); err != nil {
			return shared.WrapError(shared.CodeOf(err), "删除应用部署清单失败", err)
		}
	}
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
				SelectorLabels:     cleanStringMap(input.Environment.SelectorLabels),
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

func (s *Service) CreateWorkload(ctx context.Context, input CreateWorkloadInput) (Workload, error) {
	app, err := s.repo.GetApplication(ctx, input.ApplicationID)
	if err != nil {
		return Workload{}, err
	}
	if err := s.check(ctx, input.Actor, identityaccess.ResourceScope{Kind: identityaccess.ScopeApplication, TenantID: app.TenantID, ProjectID: app.ProjectID, ApplicationID: app.ID}, "application:update"); err != nil {
		return Workload{}, err
	}
	name := normalizeWorkloadName(input.Name)
	if err := validateWorkloadName(name); err != nil {
		return Workload{}, err
	}
	workloadType := normalizeWorkloadType(input.WorkloadType)
	if err := validateWorkloadType(workloadType); err != nil {
		return Workload{}, err
	}
	imageSourceMode, err := normalizeWorkloadImageSourceMode(input.ImageSourceMode)
	if err != nil {
		return Workload{}, err
	}
	if _, err := s.repo.FindWorkloadByApplicationAndName(ctx, app.ID, name); err == nil {
		return Workload{}, shared.NewError(shared.CodeConflict, "workload name already exists in application")
	} else if err != nil && shared.CodeOf(err) != shared.CodeNotFound {
		return Workload{}, err
	}
	pipelineID, err := s.normalizeWorkloadPipelineForImageSource(ctx, app.ID, imageSourceMode, input.PipelineID)
	if err != nil {
		return Workload{}, err
	}
	id, err := s.ids.NewID("workload")
	if err != nil {
		return Workload{}, err
	}
	now := s.clock.Now()
	workload := Workload{
		ID:              id,
		TenantID:        app.TenantID,
		ProjectID:       app.ProjectID,
		ApplicationID:   app.ID,
		Name:            name,
		DisplayName:     normalizeDisplayName(input.DisplayName, name),
		WorkloadType:    workloadType,
		Description:     strings.TrimSpace(input.Description),
		Status:          WorkloadStatusEnabled,
		ImageSourceMode: imageSourceMode,
		PipelineID:      pipelineID,
		CreatedBy:       input.Actor.ID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.repo.CreateWorkload(ctx, workload); err != nil {
		return Workload{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "workload.create", ResourceType: "workload", ResourceID: workload.ID, Result: "succeeded", Summary: "创建 Workload", OccurredAt: now})
	return workload, nil
}

func (s *Service) CreateWorkloadWithPipeline(ctx context.Context, input CreateWorkloadWithPipelineInput) (CreateWorkloadWithPipelineResult, error) {
	if s.buildPipelineCmd == nil {
		return CreateWorkloadWithPipelineResult{}, shared.NewError(shared.CodeFailedPrecondition, "build pipeline command port is required")
	}
	actor := firstSubject(input.Actor, input.Pipeline.Actor, input.Workload.Actor, input.DefaultConfig.Actor)
	if actor.ID.IsZero() {
		return CreateWorkloadWithPipelineResult{}, shared.NewError(shared.CodeUnauthenticated, "actor is required")
	}
	pipelineInput := input.Pipeline
	pipelineInput.Actor = actor
	pipelineInput.ApplicationID = input.ApplicationID
	pipeline, err := s.buildPipelineCmd.CreateBuildPipeline(ctx, pipelineInput)
	if err != nil {
		return CreateWorkloadWithPipelineResult{}, err
	}
	cleanupPipeline := func() {
		_ = s.buildPipelineCmd.DeleteBuildPipeline(ctx, actor, pipeline.ID)
	}
	workloadInput := input.Workload
	workloadInput.Actor = actor
	workloadInput.ApplicationID = input.ApplicationID
	workloadInput.PipelineID = pipeline.ID
	workload, err := s.CreateWorkload(ctx, workloadInput)
	if err != nil {
		cleanupPipeline()
		return CreateWorkloadWithPipelineResult{}, err
	}
	defaultConfigInput := input.DefaultConfig
	defaultConfigInput.Actor = actor
	defaultConfigInput.ApplicationID = input.ApplicationID
	defaultConfigInput.WorkloadID = workload.ID
	config, err := s.SaveWorkloadDefaultConfig(ctx, defaultConfigInput)
	if err != nil {
		_, _ = s.DeleteWorkload(ctx, WorkloadStatusInput{Actor: actor, ApplicationID: input.ApplicationID, WorkloadID: workload.ID})
		cleanupPipeline()
		return CreateWorkloadWithPipelineResult{}, err
	}
	return CreateWorkloadWithPipelineResult{Workload: workload, Pipeline: pipeline, DefaultConfig: config}, nil
}

func (s *Service) UpdateWorkload(ctx context.Context, input UpdateWorkloadInput) (Workload, error) {
	workload, err := s.requireWorkloadInApplication(ctx, input.ApplicationID, input.WorkloadID)
	if err != nil {
		return Workload{}, err
	}
	if err := s.check(ctx, input.Actor, identityaccess.ResourceScope{Kind: identityaccess.ScopeApplication, TenantID: workload.TenantID, ProjectID: workload.ProjectID, ApplicationID: workload.ApplicationID}, "application:update"); err != nil {
		return Workload{}, err
	}
	if workload.Status == WorkloadStatusDeleted {
		return Workload{}, shared.NewError(shared.CodeFailedPrecondition, "deleted workload cannot be updated")
	}
	name := workload.Name
	if strings.TrimSpace(input.Name) != "" {
		name = normalizeWorkloadName(input.Name)
	}
	if err := validateWorkloadName(name); err != nil {
		return Workload{}, err
	}
	workloadType := workload.WorkloadType
	if input.WorkloadType != "" {
		workloadType = normalizeWorkloadType(input.WorkloadType)
	}
	if err := validateWorkloadType(workloadType); err != nil {
		return Workload{}, err
	}
	imageSourceMode := workload.ImageSourceMode
	if strings.TrimSpace(input.ImageSourceMode) != "" {
		imageSourceMode, err = normalizeWorkloadImageSourceMode(input.ImageSourceMode)
		if err != nil {
			return Workload{}, err
		}
	}
	pipelineID := workload.PipelineID
	if imageSourceMode == "custom_image" {
		pipelineID = ""
	} else if strings.TrimSpace(string(input.PipelineID)) != "" {
		pipelineID, err = s.normalizeWorkloadPipeline(ctx, workload.ApplicationID, input.PipelineID)
		if err != nil {
			return Workload{}, err
		}
	}
	if name != workload.Name {
		if existing, err := s.repo.FindWorkloadByApplicationAndName(ctx, workload.ApplicationID, name); err == nil && existing.ID != workload.ID {
			return Workload{}, shared.NewError(shared.CodeConflict, "workload name already exists in application")
		} else if err != nil && shared.CodeOf(err) != shared.CodeNotFound {
			return Workload{}, err
		}
	}
	workload.Name = name
	workload.DisplayName = normalizeDisplayName(input.DisplayName, name)
	workload.Description = strings.TrimSpace(input.Description)
	workload.WorkloadType = workloadType
	workload.ImageSourceMode = imageSourceMode
	workload.PipelineID = pipelineID
	workload.UpdatedAt = s.clock.Now()
	if err := s.repo.UpdateWorkload(ctx, workload); err != nil {
		return Workload{}, err
	}
	return workload, nil
}

func (s *Service) normalizeWorkloadPipeline(ctx context.Context, applicationID shared.ID, pipelineID shared.ID) (shared.ID, error) {
	if pipelineID.IsZero() {
		return "", nil
	}
	if s.buildPipelineQuery == nil {
		return pipelineID, nil
	}
	pipeline, err := s.buildPipelineQuery.GetBuildPipeline(ctx, pipelineID)
	if err != nil {
		return "", err
	}
	if pipeline.ApplicationID != applicationID {
		return "", shared.NewError(shared.CodeInvalidArgument, "build pipeline does not belong to workload application")
	}
	if strings.EqualFold(pipeline.Status, "disabled") {
		return "", shared.NewError(shared.CodeFailedPrecondition, "build pipeline is disabled")
	}
	return pipeline.ID, nil
}

func (s *Service) normalizeWorkloadPipelineForImageSource(ctx context.Context, applicationID shared.ID, imageSourceMode string, pipelineID shared.ID) (shared.ID, error) {
	if imageSourceMode == "custom_image" {
		return "", nil
	}
	return s.normalizeWorkloadPipeline(ctx, applicationID, pipelineID)
}

func (s *Service) EnableWorkload(ctx context.Context, input WorkloadStatusInput) (Workload, error) {
	return s.changeWorkloadStatus(ctx, input, WorkloadStatusEnabled)
}

func (s *Service) DisableWorkload(ctx context.Context, input WorkloadStatusInput) (Workload, error) {
	return s.changeWorkloadStatus(ctx, input, WorkloadStatusDisabled)
}

func (s *Service) DeleteWorkload(ctx context.Context, input WorkloadStatusInput) (Workload, error) {
	return s.changeWorkloadStatus(ctx, input, WorkloadStatusDeleted)
}

func (s *Service) GetWorkload(ctx context.Context, applicationID shared.ID, workloadID shared.ID) (Workload, error) {
	return s.requireWorkloadInApplication(ctx, applicationID, workloadID)
}

func (s *Service) ListWorkloads(ctx context.Context, applicationID shared.ID) ([]Workload, error) {
	if _, err := s.repo.GetApplication(ctx, applicationID); err != nil {
		return nil, err
	}
	return s.repo.ListWorkloadsByApplication(ctx, applicationID)
}

func (s *Service) ListEnabledWorkloads(ctx context.Context, applicationID shared.ID) ([]Workload, error) {
	if _, err := s.repo.GetApplication(ctx, applicationID); err != nil {
		return nil, err
	}
	return s.repo.ListEnabledWorkloadsByApplication(ctx, applicationID)
}

func (s *Service) SaveWorkloadStageConfig(ctx context.Context, input SaveWorkloadStageConfigInput) (WorkloadStageConfig, error) {
	workload, err := s.requireWorkloadInApplication(ctx, input.ApplicationID, input.WorkloadID)
	if err != nil {
		return WorkloadStageConfig{}, err
	}
	if workload.Status == WorkloadStatusDeleted {
		return WorkloadStageConfig{}, shared.NewError(shared.CodeFailedPrecondition, "deleted workload cannot be configured")
	}
	stageKey, err := normalizeStageKey(input.StageKey)
	if err != nil {
		return WorkloadStageConfig{}, err
	}
	if err := s.check(ctx, input.Actor, identityaccess.ResourceScope{Kind: identityaccess.ScopeApplication, TenantID: workload.TenantID, ProjectID: workload.ProjectID, ApplicationID: workload.ApplicationID}, "application:update"); err != nil {
		return WorkloadStageConfig{}, err
	}
	if input.Replicas < 0 {
		return WorkloadStageConfig{}, shared.NewError(shared.CodeInvalidArgument, "replicas cannot be negative")
	}
	now := s.clock.Now()
	id, err := s.ids.NewID("workload_stage_config")
	if err != nil {
		return WorkloadStageConfig{}, err
	}
	if existing, err := s.repo.GetWorkloadStageConfig(ctx, workload.ID, stageKey); err == nil {
		id = existing.ID
	} else if err != nil && shared.CodeOf(err) != shared.CodeNotFound {
		return WorkloadStageConfig{}, err
	}
	config := WorkloadStageConfig{
		ID:               id,
		TenantID:         workload.TenantID,
		ProjectID:        workload.ProjectID,
		ApplicationID:    workload.ApplicationID,
		WorkloadID:       workload.ID,
		StageKey:         stageKey,
		Replicas:         input.Replicas,
		ServicePorts:     input.ServicePorts,
		ResourceRequests: input.ResourceRequests,
		ResourceLimits:   input.ResourceLimits,
		Probes:           input.Probes,
		IngressHosts:     input.IngressHosts,
		EnvVars:          input.EnvVars,
		SecretRefs:       input.SecretRefs,
		ConfigFiles:      input.ConfigFiles,
		WritableDirs:     input.WritableDirs,
		VolumeMounts:     input.VolumeMounts,
		InitContainers:   input.InitContainers,
		ValuesOverride:   input.ValuesOverride,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.repo.SaveWorkloadStageConfig(ctx, config); err != nil {
		return WorkloadStageConfig{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "workload_stage_config.update", ResourceType: "workload_stage_config", ResourceID: config.ID, Result: "succeeded", Summary: "修改 Workload Stage 部署配置", OccurredAt: now})
	return config, nil
}

func (s *Service) SaveWorkloadDefaultConfig(ctx context.Context, input SaveWorkloadDefaultConfigInput) (WorkloadStageConfig, error) {
	workload, err := s.requireWorkloadInApplication(ctx, input.ApplicationID, input.WorkloadID)
	if err != nil {
		return WorkloadStageConfig{}, err
	}
	if workload.Status == WorkloadStatusDeleted {
		return WorkloadStageConfig{}, shared.NewError(shared.CodeFailedPrecondition, "deleted workload cannot be configured")
	}
	if err := s.check(ctx, input.Actor, identityaccess.ResourceScope{Kind: identityaccess.ScopeApplication, TenantID: workload.TenantID, ProjectID: workload.ProjectID, ApplicationID: workload.ApplicationID}, "application:update"); err != nil {
		return WorkloadStageConfig{}, err
	}
	if input.Replicas < 0 {
		return WorkloadStageConfig{}, shared.NewError(shared.CodeInvalidArgument, "replicas cannot be negative")
	}
	now := s.clock.Now()
	id, err := s.ids.NewID("workload_default_config")
	if err != nil {
		return WorkloadStageConfig{}, err
	}
	if existing, err := s.repo.GetWorkloadDefaultConfig(ctx, workload.ID); err == nil {
		id = existing.ID
	} else if err != nil && shared.CodeOf(err) != shared.CodeNotFound {
		return WorkloadStageConfig{}, err
	}
	config := WorkloadStageConfig{
		ID:               id,
		TenantID:         workload.TenantID,
		ProjectID:        workload.ProjectID,
		ApplicationID:    workload.ApplicationID,
		WorkloadID:       workload.ID,
		Replicas:         input.Replicas,
		ServicePorts:     input.ServicePorts,
		ResourceRequests: input.ResourceRequests,
		ResourceLimits:   input.ResourceLimits,
		Probes:           input.Probes,
		IngressHosts:     input.IngressHosts,
		EnvVars:          input.EnvVars,
		SecretRefs:       input.SecretRefs,
		ConfigFiles:      input.ConfigFiles,
		WritableDirs:     input.WritableDirs,
		VolumeMounts:     input.VolumeMounts,
		InitContainers:   input.InitContainers,
		ValuesOverride:   input.ValuesOverride,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.repo.SaveWorkloadDefaultConfig(ctx, config); err != nil {
		return WorkloadStageConfig{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "workload_default_config.update", ResourceType: "workload_default_config", ResourceID: config.ID, Result: "succeeded", Summary: "修改 Workload 默认部署配置", OccurredAt: now})
	return config, nil
}

func (s *Service) GetWorkloadStageConfig(ctx context.Context, workloadID shared.ID, stageKey string) (WorkloadStageConfig, error) {
	normalized, err := normalizeStageKey(stageKey)
	if err != nil {
		return WorkloadStageConfig{}, err
	}
	return s.repo.GetWorkloadStageConfig(ctx, workloadID, normalized)
}

func (s *Service) GetWorkloadDefaultConfig(ctx context.Context, workloadID shared.ID) (WorkloadStageConfig, error) {
	return s.repo.GetWorkloadDefaultConfig(ctx, workloadID)
}

func (s *Service) ListWorkloadStageConfigs(ctx context.Context, workloadID shared.ID) ([]WorkloadStageConfig, error) {
	if _, err := s.repo.GetWorkload(ctx, workloadID); err != nil {
		return nil, err
	}
	return s.repo.ListWorkloadStageConfigs(ctx, workloadID)
}

func (s *Service) changeWorkloadStatus(ctx context.Context, input WorkloadStatusInput, status WorkloadStatus) (Workload, error) {
	workload, err := s.requireWorkloadInApplication(ctx, input.ApplicationID, input.WorkloadID)
	if err != nil {
		return Workload{}, err
	}
	if err := s.check(ctx, input.Actor, identityaccess.ResourceScope{Kind: identityaccess.ScopeApplication, TenantID: workload.TenantID, ProjectID: workload.ProjectID, ApplicationID: workload.ApplicationID}, "application:update"); err != nil {
		return Workload{}, err
	}
	if workload.Status == WorkloadStatusDeleted {
		return Workload{}, shared.NewError(shared.CodeFailedPrecondition, "deleted workload cannot change status")
	}
	workload.Status = status
	workload.UpdatedAt = s.clock.Now()
	if err := s.repo.UpdateWorkload(ctx, workload); err != nil {
		return Workload{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "workload.status_change", ResourceType: "workload", ResourceID: workload.ID, Result: "succeeded", Summary: "变更 Workload 状态", OccurredAt: workload.UpdatedAt})
	return workload, nil
}

func (s *Service) requireWorkloadInApplication(ctx context.Context, applicationID shared.ID, workloadID shared.ID) (Workload, error) {
	workload, err := s.repo.GetWorkload(ctx, workloadID)
	if err != nil {
		return Workload{}, err
	}
	if workload.ApplicationID != applicationID {
		return Workload{}, shared.NewError(shared.CodeNotFound, "workload not found")
	}
	return workload, nil
}

func (s *Service) GetApplicationStageState(ctx context.Context, applicationID shared.ID, stageKey string) (ApplicationStageState, error) {
	normalized, err := normalizeStageKey(stageKey)
	if err != nil {
		return ApplicationStageState{}, err
	}
	return s.repo.GetApplicationStageState(ctx, applicationID, normalized)
}

func (s *Service) ListApplicationStageStates(ctx context.Context, applicationID shared.ID) ([]ApplicationStageState, error) {
	if _, err := s.repo.GetApplication(ctx, applicationID); err != nil {
		return nil, err
	}
	return s.repo.ListApplicationStageStatesByApplication(ctx, applicationID)
}

func (s *Service) UpdateApplicationStageState(ctx context.Context, input UpdateApplicationStageStateInput) (ApplicationStageState, error) {
	app, err := s.repo.GetApplication(ctx, input.ApplicationID)
	if err != nil {
		return ApplicationStageState{}, err
	}
	stageKey, err := normalizeStageKey(input.StageKey)
	if err != nil {
		return ApplicationStageState{}, err
	}
	if err := shared.ValidateStatus(string(input.Status), AllowedApplicationStageStatuses); err != nil {
		return ApplicationStageState{}, err
	}
	now := s.clock.Now()
	state := ApplicationStageState{
		TenantID:       app.TenantID,
		ProjectID:      app.ProjectID,
		ApplicationID:  app.ID,
		StageKey:       stageKey,
		Status:         input.Status,
		Message:        strings.TrimSpace(input.Message),
		LastReportedAt: input.ReportedAt,
		UpdatedAt:      now,
	}
	if err := s.repo.SaveApplicationStageState(ctx, state); err != nil {
		return ApplicationStageState{}, err
	}
	_ = s.appendApplicationStageEvent(ctx, state, "application_stage_state.updated", now)
	return state, nil
}

func (s *Service) ListApplicationStageEvents(ctx context.Context, applicationID shared.ID, stageKey string, page shared.PageRequest) (shared.PageResult[ApplicationStageEvent], error) {
	normalized, err := normalizeStageKey(stageKey)
	if err != nil {
		return shared.PageResult[ApplicationStageEvent]{}, err
	}
	if _, err := s.repo.GetApplication(ctx, applicationID); err != nil {
		return shared.PageResult[ApplicationStageEvent]{}, err
	}
	return s.repo.ListApplicationStageEvents(ctx, applicationID, normalized, page)
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
		out = append(out, ApplicationRuntimeEnvironment{ID: environment.ID, Name: environment.Name, RuntimeBaseImage: environment.RuntimeBaseImage, ArtifactDeployPath: environment.ArtifactDeployPath, DockerfilePath: environment.DockerfilePath, SelectorLabels: cleanStringMap(environment.SelectorLabels)})
	}
	return out
}

func cleanStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
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

func firstSubject(subjects ...identityaccess.Subject) identityaccess.Subject {
	for _, subject := range subjects {
		if !subject.ID.IsZero() {
			return subject
		}
	}
	return identityaccess.Subject{}
}

func normalizeStageKey(stageKey string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(stageKey))
	if !sourceKeyPattern.MatchString(normalized) {
		return "", shared.NewError(shared.CodeInvalidArgument, "stage_key must start with a lowercase letter and contain lowercase letters, numbers or hyphens")
	}
	return normalized, nil
}

func (s *Service) appendApplicationStageEvent(ctx context.Context, state ApplicationStageState, eventType string, occurredAt time.Time) error {
	id, err := s.ids.NewID("stage_evt")
	if err != nil {
		return err
	}
	return s.repo.AppendApplicationStageEvent(ctx, ApplicationStageEvent{ID: id, TenantID: state.TenantID, ProjectID: state.ProjectID, ApplicationID: state.ApplicationID, StageKey: state.StageKey, Type: eventType, Status: state.Status, Message: strings.TrimSpace(state.Message), OccurredAt: occurredAt})
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
