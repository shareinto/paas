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

	SaveWorkloadEnvironmentConfig(ctx context.Context, config WorkloadEnvironmentConfig) error
	GetWorkloadEnvironmentConfig(ctx context.Context, workloadID shared.ID, environmentID shared.ID) (WorkloadEnvironmentConfig, error)
	ListWorkloadEnvironmentConfigs(ctx context.Context, workloadID shared.ID) ([]WorkloadEnvironmentConfig, error)

	CreateEnvironment(ctx context.Context, environment Environment) error
	UpdateEnvironment(ctx context.Context, environment Environment) error
	GetEnvironment(ctx context.Context, id shared.ID) (Environment, error)
	ListEnvironmentsByApplication(ctx context.Context, applicationID shared.ID) ([]Environment, error)

	CreateEnvironmentConfig(ctx context.Context, config EnvironmentConfig) error
	CreateEnvironmentSecret(ctx context.Context, secret EnvironmentSecret) error
	CreateEnvironmentRoute(ctx context.Context, route EnvironmentRoute) error

	CreateEnvironmentClusterBinding(ctx context.Context, binding EnvironmentClusterBinding) error
	GetEnvironmentClusterBinding(ctx context.Context, environmentID shared.ID) (EnvironmentClusterBinding, error)
	ListEnvironmentClusterBindingsByApplication(ctx context.Context, applicationID shared.ID) ([]EnvironmentClusterBinding, error)

	SaveEnvironmentState(ctx context.Context, state EnvironmentState) error
	GetEnvironmentState(ctx context.Context, environmentID shared.ID) (EnvironmentState, error)
	ListEnvironmentStatesByApplication(ctx context.Context, applicationID shared.ID) ([]EnvironmentState, error)

	AppendEnvironmentEvent(ctx context.Context, event EnvironmentEvent) error
	ListEnvironmentEvents(ctx context.Context, environmentID shared.ID, page shared.PageRequest) (shared.PageResult[EnvironmentEvent], error)
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

type ClusterCandidate struct {
	ClusterID   shared.ID
	ClusterName string
	Namespace   string
}

type ClusterPlacementQuery interface {
	SelectCluster(ctx context.Context, environment Environment) (ClusterCandidate, bool, error)
}

type ClusterRef struct {
	ID       shared.ID
	TenantID shared.ID
	Name     string
	Status   string
}

type ClusterQuery interface {
	GetCluster(ctx context.Context, id shared.ID) (ClusterRef, error)
}

type GitOpsEnvironmentProvisioner interface {
	ProvisionEnvironment(ctx context.Context, spec GitOpsEnvironmentSpec) error
}

type GitOpsEnvironmentSpec struct {
	TenantID           shared.ID
	ProjectID          shared.ID
	ApplicationID      shared.ID
	EnvironmentID      shared.ID
	ApplicationName    string
	EnvironmentName    string
	SourceRepositoryID shared.ID
	SourcePath         string
	ClusterID          shared.ID
	Namespace          string
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
	SaveWorkloadEnvironmentConfig(ctx context.Context, input SaveWorkloadEnvironmentConfigInput) (WorkloadEnvironmentConfig, error)
}

type WorkloadQuery interface {
	GetWorkload(ctx context.Context, applicationID shared.ID, workloadID shared.ID) (Workload, error)
	ListWorkloads(ctx context.Context, applicationID shared.ID) ([]Workload, error)
	ListEnabledWorkloads(ctx context.Context, applicationID shared.ID) ([]Workload, error)
	GetWorkloadEnvironmentConfig(ctx context.Context, workloadID shared.ID, environmentID shared.ID) (WorkloadEnvironmentConfig, error)
	ListWorkloadEnvironmentConfigs(ctx context.Context, workloadID shared.ID) ([]WorkloadEnvironmentConfig, error)
	ListWorkloadEnvironmentConfigsForApplication(ctx context.Context, applicationID shared.ID, workloadID shared.ID) ([]WorkloadEnvironmentConfig, error)
}

type EnvironmentCommand interface {
	UpdateEnvironmentState(ctx context.Context, input UpdateEnvironmentStateInput) (EnvironmentState, error)
	SetEnvironmentConfig(ctx context.Context, input SetEnvironmentConfigInput) (EnvironmentConfig, error)
	SetEnvironmentSecret(ctx context.Context, input SetEnvironmentSecretInput) (EnvironmentSecret, error)
}

type EnvironmentQuery interface {
	ListEnvironments(ctx context.Context, applicationID shared.ID) ([]Environment, error)
	GetEnvironment(ctx context.Context, id shared.ID) (Environment, error)
	ListEnvironmentEvents(ctx context.Context, environmentID shared.ID, page shared.PageRequest) (shared.PageResult[EnvironmentEvent], error)
}

type EnvironmentBindingCommand interface {
	BindEnvironmentCluster(ctx context.Context, input BindEnvironmentClusterInput) (EnvironmentClusterBinding, error)
}

type EnvironmentStateQuery interface {
	GetEnvironmentState(ctx context.Context, environmentID shared.ID) (EnvironmentState, error)
	ListEnvironmentStates(ctx context.Context, applicationID shared.ID) ([]EnvironmentState, error)
}
