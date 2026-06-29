package build

import (
	"bytes"
	"encoding/xml"
	"path"
	"strings"
	"time"

	"github.com/shareinto/paas/internal/shared"
)

type BuildSpec struct {
	SourcePath          string `json:"source_path"`
	BuildCommand        string `json:"build_command"`
	ArtifactCopyCommand string `json:"artifact_copy_command"`
	RuntimeBaseImage    string `json:"runtime_base_image"`
	ArtifactDeployPath  string `json:"artifact_deploy_path"`
	DefaultRef          string `json:"default_ref"`
}

type BuildEnvironmentStatus string

const (
	BuildEnvironmentEnabled  BuildEnvironmentStatus = "enabled"
	BuildEnvironmentDisabled BuildEnvironmentStatus = "disabled"
	BuildEnvironmentDeleted  BuildEnvironmentStatus = "deleted"
)

type BuildEnvironment struct {
	ID          shared.ID              `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	BuildImage  string                 `json:"build_image"`
	Status      BuildEnvironmentStatus `json:"status"`
	IsDefault   bool                   `json:"is_default"`
	CreatedBy   shared.ID              `json:"created_by"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type RuntimeEnvironmentStatus string

const (
	RuntimeEnvironmentEnabled  RuntimeEnvironmentStatus = "enabled"
	RuntimeEnvironmentDisabled RuntimeEnvironmentStatus = "disabled"
	RuntimeEnvironmentDeleted  RuntimeEnvironmentStatus = "deleted"
)

type RuntimeEnvironment struct {
	ID                 shared.ID                 `json:"id"`
	Name               string                    `json:"name"`
	Description        string                    `json:"description"`
	RuntimeBaseImage   string                    `json:"runtime_base_image"`
	ArtifactDeployPath string                    `json:"artifact_deploy_path"`
	DockerfilePath     string                    `json:"dockerfile_path"`
	SelectorLabels     map[string]string         `json:"selector_labels"`
	Images             []RuntimeEnvironmentImage `json:"images"`
	Status             RuntimeEnvironmentStatus  `json:"status"`
	CreatedBy          shared.ID                 `json:"created_by"`
	CreatedAt          time.Time                 `json:"created_at"`
	UpdatedAt          time.Time                 `json:"updated_at"`
}

type RuntimeEnvironmentImage struct {
	ID                 shared.ID         `json:"id"`
	Name               string            `json:"name"`
	DisplayName        string            `json:"display_name"`
	RuntimeBaseImage   string            `json:"runtime_base_image"`
	ArtifactDeployPath string            `json:"artifact_deploy_path"`
	DockerfilePath     string            `json:"dockerfile_path"`
	SelectorLabels     map[string]string `json:"selector_labels"`
	Status             string            `json:"status"`
}

type BuildTemplate struct {
	ID        shared.ID `json:"id"`
	Name      string    `json:"name"`
	Version   int       `json:"version"`
	Content   string    `json:"content"`
	CreatedBy shared.ID `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type JenkinsJobTemplateStatus string

const (
	JenkinsJobTemplateEnabled  JenkinsJobTemplateStatus = "enabled"
	JenkinsJobTemplateDisabled JenkinsJobTemplateStatus = "disabled"
)

type JenkinsJobTemplate struct {
	ID               shared.ID                `json:"id"`
	Name             string                   `json:"name"`
	DisplayName      string                   `json:"display_name"`
	Description      string                   `json:"description"`
	RuntimeBaseImage string                   `json:"runtime_base_image"`
	Version          int                      `json:"version"`
	XMLContent       string                   `json:"xml_content,omitempty"`
	Status           JenkinsJobTemplateStatus `json:"status"`
	IsDefault        bool                     `json:"is_default"`
	CreatedBy        shared.ID                `json:"created_by"`
	CreatedAt        time.Time                `json:"created_at"`
	UpdatedAt        time.Time                `json:"updated_at"`
}

type BuildPipelineStatus string

const (
	BuildPipelineStatusActive   BuildPipelineStatus = "active"
	BuildPipelineStatusDisabled BuildPipelineStatus = "disabled"
)

type SourceType string

const (
	SourceTypeGit SourceType = "git"
	SourceTypeSVN SourceType = "svn"
)

type SVNCheckoutPath struct {
	Local string `json:"local"`
	Path  string `json:"path"`
	Depth string `json:"depth"`
}

type BuildPipeline struct {
	ID                  shared.ID               `json:"id"`
	TenantID            shared.ID               `json:"tenant_id"`
	ProjectID           shared.ID               `json:"project_id"`
	ApplicationID       shared.ID               `json:"application_id"`
	Name                string                  `json:"name"`
	DisplayName         string                  `json:"display_name"`
	Description         string                  `json:"description"`
	Provider            string                  `json:"provider"`
	ExternalJobName     string                  `json:"external_job_name"`
	TemplateID          string                  `json:"template_id"`
	ConfigHash          string                  `json:"config_hash"`
	ImageRepository     string                  `json:"image_repository,omitempty"`
	Status              BuildPipelineStatus     `json:"status"`
	ManagedByPlatform   bool                    `json:"managed_by_platform"`
	RuntimeEnvironments []RuntimeEnvironmentRef `json:"runtime_environments"`
	CreatedAt           time.Time               `json:"created_at"`
	UpdatedAt           time.Time               `json:"updated_at"`
}

type BuildPipelineSource struct {
	ID                 shared.ID         `json:"id"`
	TenantID           shared.ID         `json:"tenant_id"`
	ProjectID          shared.ID         `json:"project_id"`
	ApplicationID      shared.ID         `json:"application_id"`
	PipelineID         shared.ID         `json:"pipeline_id"`
	Key                string            `json:"key"`
	DisplayName        string            `json:"display_name"`
	SourceType         SourceType        `json:"source_type"`
	SourceURL          string            `json:"source_url"`
	SourceRef          string            `json:"source_ref"`
	SVNRevision        string            `json:"svn_revision,omitempty"`
	SVNCheckoutPaths   []SVNCheckoutPath `json:"svn_checkout_paths,omitempty"`
	BuildEnvironmentID shared.ID         `json:"build_environment_id"`
	SourcePath         string            `json:"source_path"`
	BuildSpec          BuildSpec         `json:"build_spec"`
	IsPrimary          bool              `json:"is_primary"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

type BuildRunStatus string

const (
	BuildRunQueued    BuildRunStatus = "queued"
	BuildRunRunning   BuildRunStatus = "running"
	BuildRunSucceeded BuildRunStatus = "succeeded"
	BuildRunFailed    BuildRunStatus = "failed"
	BuildRunAborted   BuildRunStatus = "aborted"
	BuildRunUnstable  BuildRunStatus = "unstable"
	BuildRunUnknown   BuildRunStatus = "unknown"
)

var AllowedBuildRunStatuses = []string{
	string(BuildRunQueued),
	string(BuildRunRunning),
	string(BuildRunSucceeded),
	string(BuildRunFailed),
	string(BuildRunAborted),
	string(BuildRunUnstable),
	string(BuildRunUnknown),
}

type BuildRun struct {
	ID                  shared.ID      `json:"id"`
	TenantID            shared.ID      `json:"tenant_id"`
	ProjectID           shared.ID      `json:"project_id"`
	PipelineID          shared.ID      `json:"pipeline_id"`
	PipelineName        string         `json:"pipeline_name"`
	PipelineDisplayName string         `json:"pipeline_display_name"`
	ApplicationID       shared.ID      `json:"application_id"`
	WorkloadID          shared.ID      `json:"workload_id"`
	SourceType          SourceType     `json:"source_type"`
	SourceURL           string         `json:"source_url"`
	SourceRef           string         `json:"source_ref"`
	CommitSHA           string         `json:"commit_sha"`
	Version             string         `json:"version"`
	Status              BuildRunStatus `json:"status"`
	JenkinsQueueID      string         `json:"jenkins_queue_id"`
	JenkinsBuildNumber  int64          `json:"jenkins_build_number"`
	PrimaryArtifactID   shared.ID      `json:"primary_artifact_id"`
	LogOffset           int64          `json:"log_offset"`
	ErrorMessage        string         `json:"error_message"`
	RequestedBy         shared.ID      `json:"requested_by"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	StartedAt           *time.Time     `json:"started_at,omitempty"`
	FinishedAt          *time.Time     `json:"finished_at,omitempty"`
}

type BuildRunSource struct {
	ID               shared.ID         `json:"id"`
	TenantID         shared.ID         `json:"tenant_id"`
	ProjectID        shared.ID         `json:"project_id"`
	BuildRunID       shared.ID         `json:"build_run_id"`
	ApplicationID    shared.ID         `json:"application_id"`
	SourceKey        string            `json:"source_key"`
	SourceType       SourceType        `json:"source_type"`
	SourceURL        string            `json:"source_url"`
	SourceRef        string            `json:"source_ref"`
	SVNCheckoutPaths []SVNCheckoutPath `json:"svn_checkout_paths,omitempty"`
	CommitSHA        string            `json:"commit_sha"`
	SourcePath       string            `json:"source_path"`
	IsPrimary        bool              `json:"is_primary"`
	CreatedAt        time.Time         `json:"created_at"`
}

type BuildArtifactType string

const (
	BuildArtifactImage   BuildArtifactType = "image"
	BuildArtifactSBOM    BuildArtifactType = "sbom"
	BuildArtifactReport  BuildArtifactType = "report"
	BuildArtifactArchive BuildArtifactType = "archive"
)

type BuildArtifact struct {
	ID             shared.ID         `json:"id"`
	TenantID       shared.ID         `json:"tenant_id"`
	ProjectID      shared.ID         `json:"project_id"`
	BuildRunID     shared.ID         `json:"build_run_id"`
	ApplicationID  shared.ID         `json:"application_id"`
	WorkloadID     shared.ID         `json:"workload_id"`
	ContainerName  string            `json:"container_name"`
	SourceKey      string            `json:"source_key"`
	Type           BuildArtifactType `json:"type"`
	Name           string            `json:"name"`
	URI            string            `json:"uri"`
	Digest         string            `json:"digest"`
	IsPrimary      bool              `json:"is_primary"`
	SelectorLabels map[string]string `json:"selector_labels"`
	Metadata       map[string]string `json:"metadata"`
	CreatedAt      time.Time         `json:"created_at"`
}

type BuildStartedPayload struct {
	BuildRunID    shared.ID `json:"build_run_id"`
	ApplicationID shared.ID `json:"application_id"`
	ProjectID     shared.ID `json:"project_id"`
}

type BuildSucceededPayload struct {
	BuildRunID          shared.ID        `json:"build_run_id"`
	ApplicationID       shared.ID        `json:"application_id"`
	WorkloadID          shared.ID        `json:"workload_id"`
	WorkloadIDs         []shared.ID      `json:"workload_ids"`
	WorkloadTargets     []WorkloadTarget `json:"workload_targets"`
	ContainerName       string           `json:"container_name"`
	PipelineID          shared.ID        `json:"pipeline_id"`
	PipelineName        string           `json:"pipeline_name"`
	PipelineDisplayName string           `json:"pipeline_display_name"`
	BuildArtifactID     shared.ID        `json:"build_artifact_id"`
	BuildArtifactIDs    []shared.ID      `json:"build_artifact_ids"`
	CommitSHA           string           `json:"commit_sha"`
}

type WorkloadTarget struct {
	WorkloadID    shared.ID `json:"workload_id"`
	ContainerName string    `json:"container_name"`
}

type BuildFailedPayload struct {
	BuildRunID    shared.ID `json:"build_run_id"`
	ApplicationID shared.ID `json:"application_id"`
	Status        string    `json:"status"`
	Message       string    `json:"message"`
}

func terminalStatus(status BuildRunStatus) bool {
	return status == BuildRunSucceeded || status == BuildRunFailed || status == BuildRunAborted || status == BuildRunUnstable
}

func validateBuildSpec(spec BuildSpec) error {
	sourcePath, err := normalizeRelativePath(spec.SourcePath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(spec.BuildCommand) == "" {
		return shared.NewError(shared.CodeInvalidArgument, "build_command is required")
	}
	if strings.TrimSpace(spec.ArtifactCopyCommand) == "" {
		return shared.NewError(shared.CodeInvalidArgument, "artifact_copy_command is required")
	}
	if strings.TrimSpace(spec.RuntimeBaseImage) == "" {
		return shared.NewError(shared.CodeInvalidArgument, "runtime_base_image is required")
	}
	artifactDeployPath := strings.TrimSpace(spec.ArtifactDeployPath)
	if artifactDeployPath != "" && (!strings.HasPrefix(artifactDeployPath, "/") || strings.Contains(artifactDeployPath, "..")) {
		return shared.NewError(shared.CodeInvalidArgument, "artifact_deploy_path must be absolute and stay under runtime root")
	}
	_ = sourcePath
	return nil
}

func validateJenkinsfile(content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return shared.NewError(shared.CodeInvalidArgument, "Jenkinsfile is required")
	}
	for _, line := range strings.Split(content, "\n") {
		lower := strings.ToLower(strings.TrimSpace(line))
		if strings.Contains(lower, "password=") || strings.Contains(lower, "token=") || strings.Contains(lower, "secret=") {
			return shared.NewError(shared.CodeInvalidArgument, "Jenkinsfile must not contain plaintext secrets")
		}
	}
	return nil
}

func jenkinsPipelineJobXML(jenkinsfile string) string {
	var script bytes.Buffer
	_ = xml.EscapeText(&script, []byte(strings.TrimSpace(jenkinsfile)))
	return `<flow-definition plugin="workflow-job">
  <description>Managed by PaaS</description>
  <keepDependencies>false</keepDependencies>
  <properties/>
  <definition class="org.jenkinsci.plugins.workflow.cps.CpsFlowDefinition" plugin="workflow-cps">
    <script>` + script.String() + `</script>
    <sandbox>true</sandbox>
  </definition>
  <triggers/>
  <disabled>false</disabled>
</flow-definition>`
}

func normalizeRelativePath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", shared.NewError(shared.CodeInvalidArgument, "path is required")
	}
	value = strings.ReplaceAll(value, "\\", "/")
	if strings.HasPrefix(value, "/") || path.IsAbs(value) {
		return "", shared.NewError(shared.CodeInvalidArgument, "path must be relative")
	}
	cleaned := path.Clean(value)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
		return "", shared.NewError(shared.CodeInvalidArgument, "path cannot contain ..")
	}
	return cleaned, nil
}
