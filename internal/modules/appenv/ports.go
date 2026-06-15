package appenv

import (
	"context"
	"time"

	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/modules/tenantproject"
	"github.com/shareinto/paas/internal/shared"
)

type Repository interface {
	CreateApplication(ctx context.Context, application Application) error
	UpdateApplication(ctx context.Context, application Application) error
	DeleteApplicationData(ctx context.Context, applicationID shared.ID) error
	GetApplication(ctx context.Context, id shared.ID) (Application, error)
	FindApplicationByProjectAndName(ctx context.Context, projectID shared.ID, name string) (Application, error)
	ListApplicationsByTenant(ctx context.Context, tenantID shared.ID, page shared.PageRequest) (shared.PageResult[Application], error)
	ListApplicationsByProject(ctx context.Context, projectID shared.ID, page shared.PageRequest) (shared.PageResult[Application], error)
	ListApplicationsByRuntimeEnvironment(ctx context.Context, runtimeEnvironmentID shared.ID, page shared.PageRequest) (shared.PageResult[Application], error)

	CreateApplicationSource(ctx context.Context, source ApplicationSource) error
	UpdateApplicationSource(ctx context.Context, source ApplicationSource) error
	ReplaceApplicationSources(ctx context.Context, applicationID shared.ID, sources []ApplicationSource) error
	GetApplicationSource(ctx context.Context, applicationID shared.ID) (ApplicationSource, error)
	ListApplicationSources(ctx context.Context, applicationID shared.ID) ([]ApplicationSource, error)

	CreateWorkload(ctx context.Context, workload Workload) error
	UpdateWorkload(ctx context.Context, workload Workload) error
	GetWorkload(ctx context.Context, id shared.ID) (Workload, error)
	FindWorkloadByApplicationAndName(ctx context.Context, applicationID shared.ID, name string) (Workload, error)
	ListWorkloadsByApplication(ctx context.Context, applicationID shared.ID) ([]Workload, error)
	ListEnabledWorkloadsByApplication(ctx context.Context, applicationID shared.ID) ([]Workload, error)

	SaveWorkloadStageConfig(ctx context.Context, config WorkloadStageConfig) error
	GetWorkloadStageConfig(ctx context.Context, workloadID shared.ID, stageKey string) (WorkloadStageConfig, error)
	ListWorkloadStageConfigs(ctx context.Context, workloadID shared.ID) ([]WorkloadStageConfig, error)
	SaveWorkloadDefaultConfig(ctx context.Context, config WorkloadStageConfig) error
	GetWorkloadDefaultConfig(ctx context.Context, workloadID shared.ID) (WorkloadStageConfig, error)

	SaveApplicationStageState(ctx context.Context, state ApplicationStageState) error
	GetApplicationStageState(ctx context.Context, applicationID shared.ID, stageKey string) (ApplicationStageState, error)
	ListApplicationStageStatesByApplication(ctx context.Context, applicationID shared.ID) ([]ApplicationStageState, error)

	AppendApplicationStageEvent(ctx context.Context, event ApplicationStageEvent) error
	ListApplicationStageEvents(ctx context.Context, applicationID shared.ID, stageKey string, page shared.PageRequest) (shared.PageResult[ApplicationStageEvent], error)
}

type SourceRepositoryRef struct {
	ID            shared.ID
	TenantID      shared.ID
	ProjectID     shared.ID
	DefaultBranch string
	Status        string
}

type SourceRepositoryQuery interface {
	GetSourceRepository(ctx context.Context, id shared.ID) (SourceRepositoryRef, error)
}

type JenkinsTemplateRef struct {
	ID     shared.ID
	Status string
}

type JenkinsTemplateQuery interface {
	GetJenkinsJobTemplate(ctx context.Context, id shared.ID) (JenkinsTemplateRef, error)
	FindDefaultJenkinsJobTemplate(ctx context.Context) (JenkinsTemplateRef, error)
}

type BuildEnvironmentRef struct {
	ID     shared.ID
	Status string
}

type BuildEnvironmentQuery interface {
	GetBuildEnvironment(ctx context.Context, id shared.ID) (BuildEnvironmentRef, error)
	FindDefaultBuildEnvironment(ctx context.Context) (BuildEnvironmentRef, error)
}

type RuntimeEnvironmentRef struct {
	ID                 shared.ID
	Name               string
	Status             string
	RuntimeBaseImage   string
	ArtifactDeployPath string
	DockerfilePath     string
	SelectorLabels     map[string]string
}

type RuntimeEnvironmentSnapshotInput struct {
	Actor       identityaccess.Subject
	Environment RuntimeEnvironmentRef
}

type RuntimeEnvironmentQuery interface {
	GetRuntimeEnvironment(ctx context.Context, id shared.ID) (RuntimeEnvironmentRef, error)
	FindDefaultRuntimeEnvironment(ctx context.Context) (RuntimeEnvironmentRef, error)
}

type BuildPipelineProvisioner interface {
	EnsureBuildPipeline(ctx context.Context, applicationID shared.ID) error
	DeleteBuildPipeline(ctx context.Context, applicationID shared.ID) error
}

type BuildPipelineRef struct {
	ID            shared.ID
	ApplicationID shared.ID
	Name          string
	DisplayName   string
	Status        string
}

type BuildPipelineQuery interface {
	GetBuildPipeline(ctx context.Context, id shared.ID) (BuildPipelineRef, error)
}

type PermissionChecker interface {
	Check(ctx context.Context, subject identityaccess.Subject, resource identityaccess.ResourceScope, action identityaccess.Permission) error
}

type ProjectQuery interface {
	GetProject(ctx context.Context, id shared.ID) (tenantproject.Project, error)
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
	OccurredAt   time.Time
}

type EventPublisher interface {
	Publish(ctx context.Context, event shared.DomainEvent) error
}

type ApplicationCommand interface {
	CreateApplication(ctx context.Context, input CreateApplicationInput) (Application, error)
	UpdateApplication(ctx context.Context, input UpdateApplicationInput) (Application, error)
	DeleteApplication(ctx context.Context, actor identityaccess.Subject, applicationID shared.ID) error
}

type ApplicationQuery interface {
	GetApplication(ctx context.Context, id shared.ID) (Application, error)
	ListApplicationsByProject(ctx context.Context, projectID shared.ID, page shared.PageRequest) (shared.PageResult[Application], error)
	GetApplicationSource(ctx context.Context, applicationID shared.ID) (ApplicationSource, error)
	ListApplicationSources(ctx context.Context, applicationID shared.ID) ([]ApplicationSource, error)
}

type WorkloadCommand interface {
	CreateWorkload(ctx context.Context, input CreateWorkloadInput) (Workload, error)
	UpdateWorkload(ctx context.Context, input UpdateWorkloadInput) (Workload, error)
	EnableWorkload(ctx context.Context, input WorkloadStatusInput) (Workload, error)
	DisableWorkload(ctx context.Context, input WorkloadStatusInput) (Workload, error)
	DeleteWorkload(ctx context.Context, input WorkloadStatusInput) (Workload, error)
	SaveWorkloadStageConfig(ctx context.Context, input SaveWorkloadStageConfigInput) (WorkloadStageConfig, error)
	SaveWorkloadDefaultConfig(ctx context.Context, input SaveWorkloadDefaultConfigInput) (WorkloadStageConfig, error)
}

type WorkloadQuery interface {
	GetWorkload(ctx context.Context, applicationID shared.ID, workloadID shared.ID) (Workload, error)
	ListWorkloads(ctx context.Context, applicationID shared.ID) ([]Workload, error)
	ListEnabledWorkloads(ctx context.Context, applicationID shared.ID) ([]Workload, error)
	GetWorkloadStageConfig(ctx context.Context, workloadID shared.ID, stageKey string) (WorkloadStageConfig, error)
	ListWorkloadStageConfigs(ctx context.Context, workloadID shared.ID) ([]WorkloadStageConfig, error)
	GetWorkloadDefaultConfig(ctx context.Context, workloadID shared.ID) (WorkloadStageConfig, error)
}

type ApplicationStageCommand interface {
	UpdateApplicationStageState(ctx context.Context, input UpdateApplicationStageStateInput) (ApplicationStageState, error)
}

type ApplicationStageQuery interface {
	GetApplicationStageState(ctx context.Context, applicationID shared.ID, stageKey string) (ApplicationStageState, error)
	ListApplicationStageStates(ctx context.Context, applicationID shared.ID) ([]ApplicationStageState, error)
	ListApplicationStageEvents(ctx context.Context, applicationID shared.ID, stageKey string, page shared.PageRequest) (shared.PageResult[ApplicationStageEvent], error)
}
