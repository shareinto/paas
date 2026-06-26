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
	ID                 shared.ID         `json:"id"`
	Name               string            `json:"name"`
	RuntimeBaseImage   string            `json:"runtime_base_image"`
	ArtifactDeployPath string            `json:"artifact_deploy_path"`
	DockerfilePath     string            `json:"dockerfile_path"`
	SelectorLabels     map[string]string `json:"selector_labels"`
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
	PipelineID      shared.ID      `json:"pipeline_id"`
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
	Host        string `json:"host"`
	Path        string `json:"path"`
	ServerName  string `json:"server_name,omitempty"`
	ServicePort string `json:"service_port,omitempty"`
	PathType    string `json:"path_type,omitempty"`
	TLS         bool   `json:"tls,omitempty"`
	TLSRedirect bool   `json:"tls_redirect,omitempty"`
	Rewrite     bool   `json:"rewrite,omitempty"`
	RewritePath string `json:"rewrite_path,omitempty"`
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
	MountPath     string `json:"mount_path"`
	Content       string `json:"content"`
	Base64Encoded bool   `json:"base64_encoded,omitempty"`
}

type WorkloadWritableDir struct {
	MountPath  string `json:"mount_path"`
	SizeLimit  string `json:"size_limit,omitempty"`
	OwnerGroup string `json:"owner_group,omitempty"`
	Mode       string `json:"mode,omitempty"`
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

type WorkloadStageConfig struct {
	ID               shared.ID               `json:"id"`
	TenantID         shared.ID               `json:"tenant_id"`
	ProjectID        shared.ID               `json:"project_id"`
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

type ApplicationStageStatus string

const (
	ApplicationStageStatusDraft                 ApplicationStageStatus = "draft"
	ApplicationStageStatusPendingClusterBinding ApplicationStageStatus = "pending_cluster_binding"
	ApplicationStageStatusClusterBound          ApplicationStageStatus = "cluster_bound"
	ApplicationStageStatusDeployable            ApplicationStageStatus = "deployable"
	ApplicationStageStatusDeploying             ApplicationStageStatus = "deploying"
	ApplicationStageStatusRunning               ApplicationStageStatus = "running"
	ApplicationStageStatusDegraded              ApplicationStageStatus = "degraded"
	ApplicationStageStatusDisabled              ApplicationStageStatus = "disabled"
)

var AllowedApplicationStageStatuses = []string{
	string(ApplicationStageStatusDraft),
	string(ApplicationStageStatusPendingClusterBinding),
	string(ApplicationStageStatusClusterBound),
	string(ApplicationStageStatusDeployable),
	string(ApplicationStageStatusDeploying),
	string(ApplicationStageStatusRunning),
	string(ApplicationStageStatusDegraded),
	string(ApplicationStageStatusDisabled),
}

type ApplicationStageState struct {
	TenantID       shared.ID              `json:"tenant_id"`
	ProjectID      shared.ID              `json:"project_id"`
	ApplicationID  shared.ID              `json:"application_id"`
	StageKey       string                 `json:"stage_key"`
	Status         ApplicationStageStatus `json:"status"`
	Message        string                 `json:"message"`
	LastReportedAt *time.Time             `json:"last_reported_at,omitempty"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

type ApplicationStageEvent struct {
	ID            shared.ID              `json:"id"`
	TenantID      shared.ID              `json:"tenant_id"`
	ProjectID     shared.ID              `json:"project_id"`
	ApplicationID shared.ID              `json:"application_id"`
	StageKey      string                 `json:"stage_key"`
	Type          string                 `json:"type"`
	Status        ApplicationStageStatus `json:"status"`
	Message       string                 `json:"message"`
	OccurredAt    time.Time              `json:"occurred_at"`
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
	return strings.TrimSpace(name)
}

func validateWorkloadName(name string) error {
	if strings.TrimSpace(name) == "" {
		return shared.NewError(shared.CodeInvalidArgument, "workload name is required")
	}
	if len([]rune(name)) > 64 {
		return shared.NewError(shared.CodeInvalidArgument, "workload name must be 64 characters or fewer")
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
