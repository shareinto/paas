package appenv

import (
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/shareinto/paas/internal/shared"
)

var applicationNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{1,62}$`)
var sourceKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

type ApplicationStatus string

const (
	ApplicationStatusActive   ApplicationStatus = "active"
	ApplicationStatusDisabled ApplicationStatus = "disabled"
)

type Application struct {
	ID                   shared.ID                       `json:"id"`
	TenantID             shared.ID                       `json:"tenant_id"`
	ProjectID            shared.ID                       `json:"project_id"`
	Name                 string                          `json:"name"`
	DisplayName          string                          `json:"display_name"`
	Description          string                          `json:"description"`
	RuntimeEnvironmentID shared.ID                       `json:"runtime_environment_id"`
	RuntimeEnvironments  []ApplicationRuntimeEnvironment `json:"runtime_environments"`
	Status               ApplicationStatus               `json:"status"`
	CreatedAt            time.Time                       `json:"created_at"`
	UpdatedAt            time.Time                       `json:"updated_at"`
}

type ApplicationRuntimeEnvironment struct {
	ID                 shared.ID `json:"id"`
	Name               string    `json:"name"`
	RuntimeBaseImage   string    `json:"runtime_base_image"`
	ArtifactDeployPath string    `json:"artifact_deploy_path"`
	DockerfilePath     string    `json:"dockerfile_path"`
}

type ApplicationSource struct {
	ID                 shared.ID `json:"id"`
	TenantID           shared.ID `json:"tenant_id"`
	ProjectID          shared.ID `json:"project_id"`
	ApplicationID      shared.ID `json:"application_id"`
	Key                string    `json:"key"`
	DisplayName        string    `json:"display_name"`
	SourceRepositoryID shared.ID `json:"source_repository_id"`
	JenkinsTemplateID  shared.ID `json:"jenkins_template_id"`
	BuildEnvironmentID shared.ID `json:"build_environment_id"`
	SourcePath         string    `json:"source_path"`
	BuildSpec          BuildSpec `json:"build_spec"`
	IsPrimary          bool      `json:"is_primary"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type BuildSpec struct {
	SourcePath          string `json:"source_path"`
	BuildCommand        string `json:"build_command"`
	ArtifactCopyCommand string `json:"artifact_copy_command"`
	RuntimeBaseImage    string `json:"runtime_base_image"`
	ArtifactDeployPath  string `json:"artifact_deploy_path"`
	DefaultRef          string `json:"default_ref"`
}

type Environment struct {
	ID            shared.ID `json:"id"`
	TenantID      shared.ID `json:"tenant_id"`
	ProjectID     shared.ID `json:"project_id"`
	ApplicationID shared.ID `json:"application_id"`
	Name          string    `json:"name"`
	DisplayName   string    `json:"display_name"`
	Description   string    `json:"description"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type EnvironmentConfig struct {
	ID            shared.ID `json:"id"`
	TenantID      shared.ID `json:"tenant_id"`
	ProjectID     shared.ID `json:"project_id"`
	ApplicationID shared.ID `json:"application_id"`
	EnvironmentID shared.ID `json:"environment_id"`
	Key           string    `json:"key"`
	Value         string    `json:"value"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type EnvironmentSecret struct {
	ID            shared.ID `json:"id"`
	TenantID      shared.ID `json:"tenant_id"`
	ProjectID     shared.ID `json:"project_id"`
	ApplicationID shared.ID `json:"application_id"`
	EnvironmentID shared.ID `json:"environment_id"`
	Key           string    `json:"key"`
	SecretRef     string    `json:"secret_ref"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type EnvironmentRoute struct {
	ID            shared.ID `json:"id"`
	TenantID      shared.ID `json:"tenant_id"`
	ProjectID     shared.ID `json:"project_id"`
	ApplicationID shared.ID `json:"application_id"`
	EnvironmentID shared.ID `json:"environment_id"`
	Host          string    `json:"host"`
	Path          string    `json:"path"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type EnvironmentClusterBindingStatus string

const (
	EnvironmentClusterBindingActive   EnvironmentClusterBindingStatus = "active"
	EnvironmentClusterBindingDisabled EnvironmentClusterBindingStatus = "disabled"
)

type EnvironmentClusterBinding struct {
	ID            shared.ID                       `json:"id"`
	TenantID      shared.ID                       `json:"tenant_id"`
	ProjectID     shared.ID                       `json:"project_id"`
	ApplicationID shared.ID                       `json:"application_id"`
	EnvironmentID shared.ID                       `json:"environment_id"`
	ClusterID     shared.ID                       `json:"cluster_id"`
	ClusterName   string                          `json:"cluster_name"`
	Namespace     string                          `json:"namespace"`
	Status        EnvironmentClusterBindingStatus `json:"status"`
	CreatedAt     time.Time                       `json:"created_at"`
	UpdatedAt     time.Time                       `json:"updated_at"`
}

type EnvironmentStatus string

const (
	EnvironmentStatusDraft                 EnvironmentStatus = "draft"
	EnvironmentStatusPendingClusterBinding EnvironmentStatus = "pending_cluster_binding"
	EnvironmentStatusClusterBound          EnvironmentStatus = "cluster_bound"
	EnvironmentStatusDeployable            EnvironmentStatus = "deployable"
	EnvironmentStatusDeploying             EnvironmentStatus = "deploying"
	EnvironmentStatusRunning               EnvironmentStatus = "running"
	EnvironmentStatusDegraded              EnvironmentStatus = "degraded"
	EnvironmentStatusDisabled              EnvironmentStatus = "disabled"
)

var AllowedEnvironmentStatuses = []string{
	string(EnvironmentStatusDraft),
	string(EnvironmentStatusPendingClusterBinding),
	string(EnvironmentStatusClusterBound),
	string(EnvironmentStatusDeployable),
	string(EnvironmentStatusDeploying),
	string(EnvironmentStatusRunning),
	string(EnvironmentStatusDegraded),
	string(EnvironmentStatusDisabled),
}

type EnvironmentState struct {
	TenantID       shared.ID         `json:"tenant_id"`
	ProjectID      shared.ID         `json:"project_id"`
	ApplicationID  shared.ID         `json:"application_id"`
	EnvironmentID  shared.ID         `json:"environment_id"`
	Status         EnvironmentStatus `json:"status"`
	Message        string            `json:"message"`
	LastReportedAt *time.Time        `json:"last_reported_at,omitempty"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

type EnvironmentEvent struct {
	ID            shared.ID         `json:"id"`
	TenantID      shared.ID         `json:"tenant_id"`
	ProjectID     shared.ID         `json:"project_id"`
	ApplicationID shared.ID         `json:"application_id"`
	EnvironmentID shared.ID         `json:"environment_id"`
	Type          string            `json:"type"`
	Status        EnvironmentStatus `json:"status"`
	Message       string            `json:"message"`
	OccurredAt    time.Time         `json:"occurred_at"`
}

type ApplicationCreatedPayload struct {
	ApplicationID      shared.ID `json:"application_id"`
	TenantID           shared.ID `json:"tenant_id"`
	ProjectID          shared.ID `json:"project_id"`
	SourceRepositoryID shared.ID `json:"source_repository_id"`
	SourceKeys         []string  `json:"source_keys"`
	Name               string    `json:"name"`
}

func normalizeApplicationName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func validateApplicationName(name string) error {
	if !applicationNamePattern.MatchString(name) {
		return shared.NewError(shared.CodeInvalidArgument, "application name must start with a lowercase letter and contain lowercase letters, numbers or hyphens")
	}
	return nil
}

func normalizeSourceKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, "_", "-")
	return key
}

func validateSourceKey(key string) error {
	if !sourceKeyPattern.MatchString(key) {
		return shared.NewError(shared.CodeInvalidArgument, "source key must start with a lowercase letter and contain lowercase letters, numbers or hyphens")
	}
	return nil
}

func normalizeDisplayName(name string, fallback string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return fallback
	}
	return name
}

func normalizeRelativePath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", shared.NewError(shared.CodeInvalidArgument, "path is required")
	}
	if strings.HasPrefix(value, "/") || path.IsAbs(value) {
		return "", shared.NewError(shared.CodeInvalidArgument, "path must be relative")
	}
	cleaned := path.Clean(strings.ReplaceAll(value, "\\", "/"))
	if cleaned == "." {
		return ".", nil
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
		return "", shared.NewError(shared.CodeInvalidArgument, "path cannot contain ..")
	}
	return cleaned, nil
}
