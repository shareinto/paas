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
var workloadNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

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

type WorkloadType string

const (
	WorkloadTypeDeployment  WorkloadType = "Deployment"
	WorkloadTypeStatefulSet WorkloadType = "StatefulSet"
)

var AllowedWorkloadTypes = []string{string(WorkloadTypeDeployment), string(WorkloadTypeStatefulSet)}
var allowedWorkloadImageSourceModes = map[string]struct{}{
	"pipeline_artifact": {},
	"custom_image":      {},
	"mixed":             {},
	"none":              {},
}

type WorkloadStatus string

const (
	WorkloadStatusEnabled  WorkloadStatus = "enabled"
	WorkloadStatusDisabled WorkloadStatus = "disabled"
	WorkloadStatusDeleted  WorkloadStatus = "deleted"
)

type Workload struct {
	ID              shared.ID      `json:"id"`
	TenantID        shared.ID      `json:"tenant_id"`
	ProjectID       shared.ID      `json:"project_id"`
	ApplicationID   shared.ID      `json:"application_id"`
	Name            string         `json:"name"`
	DisplayName     string         `json:"display_name"`
	WorkloadType    WorkloadType   `json:"workload_type"`
	Description     string         `json:"description"`
	Status          WorkloadStatus `json:"status"`
	ImageSourceMode string         `json:"image_source_mode"`
	CreatedBy       shared.ID      `json:"created_by"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type WorkloadResourceList struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

type WorkloadServicePort struct {
	Name       string `json:"name"`
	Port       int    `json:"port"`
	TargetPort int    `json:"target_port"`
	Protocol   string `json:"protocol,omitempty"`
}

type WorkloadProbe struct {
	Name                string   `json:"name"`
	Type                string   `json:"type"`
	Path                string   `json:"path,omitempty"`
	Port                int      `json:"port,omitempty"`
	Command             []string `json:"command,omitempty"`
	InitialDelaySeconds int      `json:"initial_delay_seconds,omitempty"`
	PeriodSeconds       int      `json:"period_seconds,omitempty"`
}

type WorkloadIngressHost struct {
	Host string `json:"host"`
	Path string `json:"path"`
}

type WorkloadEnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type WorkloadSecretRef struct {
	Name      string `json:"name"`
	SecretRef string `json:"secret_ref"`
}

type WorkloadConfigFile struct {
	MountPath string `json:"mount_path"`
	Content   string `json:"content"`
}

type WorkloadWritableDir struct {
	MountPath string `json:"mount_path"`
	SizeLimit string `json:"size_limit,omitempty"`
}

type WorkloadVolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mount_path"`
}

type WorkloadInitContainer struct {
	Name    string   `json:"name"`
	Image   string   `json:"image"`
	Command []string `json:"command,omitempty"`
}

type WorkloadEnvironmentConfig struct {
	ID               shared.ID               `json:"id"`
	TenantID         shared.ID               `json:"tenant_id"`
	ProjectID        shared.ID               `json:"project_id"`
	ApplicationID    shared.ID               `json:"application_id"`
	WorkloadID       shared.ID               `json:"workload_id"`
	EnvironmentID    shared.ID               `json:"environment_id"`
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
	CreatedAt        time.Time               `json:"created_at"`
	UpdatedAt        time.Time               `json:"updated_at"`
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

func normalizeWorkloadName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func validateWorkloadName(name string) error {
	if !workloadNamePattern.MatchString(name) {
		return shared.NewError(shared.CodeInvalidArgument, "workload name must start with a lowercase letter and contain lowercase letters, numbers or hyphens")
	}
	return nil
}

func normalizeWorkloadType(workloadType WorkloadType) WorkloadType {
	switch strings.ToLower(strings.TrimSpace(string(workloadType))) {
	case "deployment":
		return WorkloadTypeDeployment
	case "statefulset":
		return WorkloadTypeStatefulSet
	default:
		return WorkloadType(strings.TrimSpace(string(workloadType)))
	}
}

func validateWorkloadType(workloadType WorkloadType) error {
	return shared.ValidateStatus(string(workloadType), AllowedWorkloadTypes)
}

func normalizeWorkloadImageSourceMode(mode string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	if normalized == "" {
		normalized = "pipeline_artifact"
	}
	if _, ok := allowedWorkloadImageSourceModes[normalized]; !ok {
		return "", shared.NewError(shared.CodeInvalidArgument, "unsupported workload image_source_mode")
	}
	return normalized, nil
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
