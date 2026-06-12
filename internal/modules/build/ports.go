package build

import (
	"context"
	"time"

	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/shared"
)

type Repository interface {
	CreateBuildEnvironment(ctx context.Context, environment BuildEnvironment) error
	UpdateBuildEnvironment(ctx context.Context, environment BuildEnvironment) error
	DeleteBuildEnvironment(ctx context.Context, id shared.ID) error
	GetBuildEnvironment(ctx context.Context, id shared.ID) (BuildEnvironment, error)
	FindDefaultBuildEnvironment(ctx context.Context) (BuildEnvironment, error)
	ListBuildEnvironments(ctx context.Context, includeDisabled bool, page shared.PageRequest) (shared.PageResult[BuildEnvironment], error)

	CreateRuntimeEnvironment(ctx context.Context, environment RuntimeEnvironment) error
	UpdateRuntimeEnvironment(ctx context.Context, environment RuntimeEnvironment) error
	DeleteRuntimeEnvironment(ctx context.Context, id shared.ID) error
	GetRuntimeEnvironment(ctx context.Context, id shared.ID) (RuntimeEnvironment, error)
	FindDefaultRuntimeEnvironment(ctx context.Context) (RuntimeEnvironment, error)
	ListRuntimeEnvironments(ctx context.Context, includeDisabled bool, page shared.PageRequest) (shared.PageResult[RuntimeEnvironment], error)

	GetBuildTemplate(ctx context.Context) (BuildTemplate, error)
	SaveBuildTemplate(ctx context.Context, template BuildTemplate) error

	CreateJenkinsJobTemplate(ctx context.Context, template JenkinsJobTemplate) error
	UpdateJenkinsJobTemplate(ctx context.Context, template JenkinsJobTemplate) error
	DeleteJenkinsJobTemplate(ctx context.Context, id shared.ID) error
	GetJenkinsJobTemplate(ctx context.Context, id shared.ID) (JenkinsJobTemplate, error)
	FindDefaultJenkinsJobTemplate(ctx context.Context) (JenkinsJobTemplate, error)
	ListJenkinsJobTemplates(ctx context.Context, includeDisabled bool, page shared.PageRequest) (shared.PageResult[JenkinsJobTemplate], error)

	CreatePipeline(ctx context.Context, pipeline BuildPipeline) error
	UpdatePipeline(ctx context.Context, pipeline BuildPipeline) error
	GetPipeline(ctx context.Context, id shared.ID) (BuildPipeline, error)
	FindPipelineByApplication(ctx context.Context, applicationID shared.ID) (BuildPipeline, error)
	FindPipelineByApplicationAndName(ctx context.Context, applicationID shared.ID, name string) (BuildPipeline, error)
	ListPipelinesByApplication(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[BuildPipeline], error)
	ReplacePipelineRuntimeEnvironments(ctx context.Context, pipelineID shared.ID, runtimes []RuntimeEnvironmentRef) error
	ListPipelineRuntimeEnvironments(ctx context.Context, pipelineID shared.ID) ([]RuntimeEnvironmentRef, error)
	ReplacePipelineSources(ctx context.Context, pipelineID shared.ID, sources []BuildPipelineSource) error
	ListPipelineSources(ctx context.Context, pipelineID shared.ID) ([]BuildPipelineSource, error)
	HasActiveRunsByPipeline(ctx context.Context, pipelineID shared.ID) (bool, error)
	ListActiveRunsByPipeline(ctx context.Context, pipelineID shared.ID) ([]BuildRun, error)

	CreateRun(ctx context.Context, run BuildRun) error
	CreateRunWithSources(ctx context.Context, run BuildRun, sources []BuildRunSource) error
	UpdateRun(ctx context.Context, run BuildRun) error
	GetRun(ctx context.Context, id shared.ID) (BuildRun, error)
	ListRunsByApplication(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[BuildRun], error)
	CreateRunSource(ctx context.Context, source BuildRunSource) error
	ListRunSources(ctx context.Context, buildRunID shared.ID) ([]BuildRunSource, error)

	CreateArtifact(ctx context.Context, artifact BuildArtifact) error
	GetArtifact(ctx context.Context, id shared.ID) (BuildArtifact, error)
	ListArtifactsByRun(ctx context.Context, buildRunID shared.ID) ([]BuildArtifact, error)

	AppendBuildLog(ctx context.Context, buildRunID shared.ID, text string) error
	ListBuildLogs(ctx context.Context, buildRunID shared.ID) ([]string, error)
}

type ApplicationRef struct {
	ID                  shared.ID
	TenantID            shared.ID
	ProjectID           shared.ID
	TenantName          string
	ProjectName         string
	Name                string
	RuntimeEnvironments []RuntimeEnvironmentRef
}

type RuntimeEnvironmentRef struct {
	ID                 shared.ID `json:"id"`
	Name               string    `json:"name"`
	RuntimeBaseImage   string    `json:"runtime_base_image"`
	ArtifactDeployPath string    `json:"artifact_deploy_path"`
	DockerfilePath     string    `json:"dockerfile_path"`
}

type ApplicationSourceRef struct {
	ApplicationID      shared.ID
	WorkloadID         shared.ID
	Key                string
	DisplayName        string
	SourceRepositoryID shared.ID
	JenkinsTemplateID  shared.ID
	BuildEnvironmentID shared.ID
	SourcePath         string
	BuildSpec          BuildSpec
	IsPrimary          bool
}

type SourceRepositoryRef struct {
	ID      shared.ID
	HTTPURL string
	SSHURL  string
}

type ApplicationQuery interface {
	GetApplication(ctx context.Context, id shared.ID) (ApplicationRef, error)
	GetApplicationSource(ctx context.Context, applicationID shared.ID) (ApplicationSourceRef, error)
	ListApplicationSources(ctx context.Context, applicationID shared.ID) ([]ApplicationSourceRef, error)
}

type WorkloadRef struct {
	ID            shared.ID
	TenantID      shared.ID
	ProjectID     shared.ID
	ApplicationID shared.ID
	Name          string
	DisplayName   string
	Status        string
}

type WorkloadQuery interface {
	GetWorkload(ctx context.Context, applicationID shared.ID, workloadID shared.ID) (WorkloadRef, error)
	ListEnabledWorkloads(ctx context.Context, applicationID shared.ID) ([]WorkloadRef, error)
}

type SourceRepositoryQuery interface {
	GetSourceRepository(ctx context.Context, id shared.ID) (SourceRepositoryRef, error)
}

type BuildRunnerPort interface {
	EnsureJob(ctx context.Context, spec BuildJobSpec) error
	DeleteJob(ctx context.Context, jobName string) error
	TriggerBuild(ctx context.Context, jobName string, parameters map[string]string) (BuildQueueItem, error)
	GetQueueItem(ctx context.Context, queueID string) (BuildQueueItem, error)
	GetBuildStatus(ctx context.Context, jobName string, buildNumber int64) (BuildStatus, error)
	ProgressiveText(ctx context.Context, jobName string, buildNumber int64, offset int64) (ProgressiveText, error)
	CancelBuild(ctx context.Context, jobName string, buildNumber int64) error
	CancelQueueItem(ctx context.Context, queueID string) error
}

type BuildJobSpec struct {
	JobName     string
	TemplateID  string
	TemplateXML string
}

type BuildQueueItem struct {
	QueueID     string
	BuildNumber int64
	Started     bool
	Canceled    bool
}

type BuildStatus struct {
	BuildNumber int64
	Building    bool
	Status      BuildRunStatus
}

type ProgressiveText struct {
	Text       string
	NextOffset int64
	MoreData   bool
}

type PermissionChecker interface {
	Check(ctx context.Context, subject identityaccess.Subject, resource identityaccess.ResourceScope, action identityaccess.Permission) error
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
	Details      map[string]string
	OccurredAt   time.Time
}

type EventPublisher interface {
	Publish(ctx context.Context, event shared.DomainEvent) error
}

type RuntimeEnvironmentSyncer interface {
	SyncRuntimeEnvironment(ctx context.Context, actor identityaccess.Subject, environment RuntimeEnvironment) error
}

type BuildCommand interface {
	EnsureBuildPipeline(ctx context.Context, applicationID shared.ID) error
	DeleteBuildPipeline(ctx context.Context, applicationID shared.ID) error
	TriggerBuild(ctx context.Context, input TriggerBuildInput) (BuildRun, error)
	CancelBuild(ctx context.Context, actor identityaccess.Subject, buildRunID shared.ID) (BuildRun, error)
}

type BuildQuery interface {
	GetBuildRun(ctx context.Context, id shared.ID) (BuildRun, error)
	ListBuildRuns(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[BuildRun], error)
	ListBuildArtifacts(ctx context.Context, buildRunID shared.ID) ([]BuildArtifact, error)
	ListBuildRunSources(ctx context.Context, buildRunID shared.ID) ([]BuildRunSource, error)
}

type BuildLogStream interface {
	StreamBuildLogs(ctx context.Context, buildRunID shared.ID) ([]LogEvent, error)
}

type BuildCallbackHandler interface {
	HandleBuildCallback(ctx context.Context, input BuildCallbackInput) (BuildRun, error)
}
