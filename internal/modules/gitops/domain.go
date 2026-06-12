package gitops

import (
	"fmt"
	"strings"
	"time"

	"github.com/shareinto/paas/internal/shared"
)

type TemplateScope string

const (
	TemplateScopePlatform    TemplateScope = "platform"
	TemplateScopeApplication TemplateScope = "application"
)

type DeploymentStatus string

const (
	DeploymentPending     DeploymentStatus = "pending"
	DeploymentSyncing     DeploymentStatus = "syncing"
	DeploymentProgressing DeploymentStatus = "progressing"
	DeploymentSucceeded   DeploymentStatus = "succeeded"
	DeploymentFailed      DeploymentStatus = "failed"
	DeploymentDegraded    DeploymentStatus = "degraded"
	DeploymentUnknown     DeploymentStatus = "unknown"
)

type DeploymentTemplate struct {
	ID             shared.ID     `json:"id"`
	TenantID       shared.ID     `json:"tenant_id"`
	ProjectID      shared.ID     `json:"project_id"`
	ApplicationID  shared.ID     `json:"application_id"`
	Name           string        `json:"name"`
	Scope          TemplateScope `json:"scope"`
	Content        string        `json:"content"`
	CurrentVersion int           `json:"current_version"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

type DeploymentTemplateRevision struct {
	ID         shared.ID `json:"id"`
	TemplateID shared.ID `json:"template_id"`
	Version    int       `json:"version"`
	Content    string    `json:"content"`
	CreatedBy  shared.ID `json:"created_by"`
	CreatedAt  time.Time `json:"created_at"`
}

type ManifestRevision struct {
	ID                 shared.ID `json:"id"`
	DeploymentID       shared.ID `json:"deployment_id"`
	PromotionID        shared.ID `json:"promotion_id"`
	ApplicationID      shared.ID `json:"application_id"`
	EnvironmentID      shared.ID `json:"environment_id"`
	TemplateRevisionID shared.ID `json:"template_revision_id"`
	Path               string    `json:"path"`
	CommitSHA          string    `json:"commit_sha"`
	MergeRequestID     string    `json:"merge_request_id"`
	ChangeType         string    `json:"change_type"`
	CreatedAt          time.Time `json:"created_at"`
}

type Deployment struct {
	ID                 shared.ID        `json:"id"`
	TenantID           shared.ID        `json:"tenant_id"`
	ProjectID          shared.ID        `json:"project_id"`
	ApplicationID      shared.ID        `json:"application_id"`
	EnvironmentID      shared.ID        `json:"environment_id"`
	ClusterBindingID   shared.ID        `json:"cluster_binding_id"`
	PromotionID        shared.ID        `json:"promotion_id"`
	FreightID          shared.ID        `json:"freight_id"`
	ManifestRevisionID shared.ID        `json:"manifest_revision_id"`
	ImageRepository    string           `json:"image_repository"`
	ImageTag           string           `json:"image_tag"`
	ImageDigest        string           `json:"image_digest"`
	WorkloadSummary    string           `json:"workload_summary"`
	Status             DeploymentStatus `json:"status"`
	Message            string           `json:"message"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
	CompletedAt        *time.Time       `json:"completed_at,omitempty"`
}

type DeploymentEvent struct {
	ID           shared.ID        `json:"id"`
	DeploymentID shared.ID        `json:"deployment_id"`
	Status       DeploymentStatus `json:"status"`
	Message      string           `json:"message"`
	OccurredAt   time.Time        `json:"occurred_at"`
}

type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

type EnvironmentRef struct {
	ID            shared.ID
	TenantID      shared.ID
	ProjectID     shared.ID
	ApplicationID shared.ID
	Name          string
}

type ClusterBindingRef struct {
	ID            shared.ID
	EnvironmentID shared.ID
	ClusterID     shared.ID
	ClusterName   string
	Namespace     string
	Active        bool
}

type WorkloadRef struct {
	ID            shared.ID
	TenantID      shared.ID
	ProjectID     shared.ID
	ApplicationID shared.ID
	Name          string
	DisplayName   string
	WorkloadType  string
}

type WorkloadResourceListRef struct {
	CPU    string `yaml:"cpu,omitempty"`
	Memory string `yaml:"memory,omitempty"`
}

type WorkloadServicePortRef struct {
	Name       string `yaml:"name"`
	Port       int    `yaml:"port"`
	TargetPort int    `yaml:"targetPort"`
	Protocol   string `yaml:"protocol,omitempty"`
}

type WorkloadEnvVarRef struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type WorkloadProbeRef struct {
	Name                string   `yaml:"name"`
	Type                string   `yaml:"type"`
	Path                string   `yaml:"path,omitempty"`
	Port                int      `yaml:"port,omitempty"`
	Command             []string `yaml:"command,omitempty"`
	InitialDelaySeconds int      `yaml:"initialDelaySeconds,omitempty"`
	PeriodSeconds       int      `yaml:"periodSeconds,omitempty"`
}

type WorkloadIngressHostRef struct {
	Host string `yaml:"host"`
	Path string `yaml:"path"`
}

type WorkloadSecretRef struct {
	Name      string `yaml:"name"`
	SecretRef string `yaml:"secretRef"`
}

type WorkloadConfigFileRef struct {
	MountPath string `yaml:"mountPath"`
	Content   string `yaml:"content"`
}

type WorkloadWritableDirRef struct {
	MountPath string `yaml:"mountPath"`
	SizeLimit string `yaml:"sizeLimit,omitempty"`
}

type WorkloadVolumeMountRef struct {
	Name      string `yaml:"name"`
	MountPath string `yaml:"mountPath"`
}

type WorkloadInitContainerRef struct {
	Name    string   `yaml:"name"`
	Image   string   `yaml:"image"`
	Command []string `yaml:"command,omitempty"`
}

type WorkloadEnvironmentConfigRef struct {
	Replicas         int
	ServicePorts     []WorkloadServicePortRef
	ResourceRequests WorkloadResourceListRef
	ResourceLimits   WorkloadResourceListRef
	Probes           []WorkloadProbeRef
	EnvVars          []WorkloadEnvVarRef
	IngressHosts     []WorkloadIngressHostRef
	SecretRefs       []WorkloadSecretRef
	ConfigFiles      []WorkloadConfigFileRef
	WritableDirs     []WorkloadWritableDirRef
	VolumeMounts     []WorkloadVolumeMountRef
	InitContainers   []WorkloadInitContainerRef
	ValuesOverride   map[string]any
}

func manifestPath(appName, envName string) string {
	return fmt.Sprintf("apps/%s/%s/values.yaml", appName, envName)
}

func manifestPathForBinding(appName, envName string, binding ClusterBindingRef) string {
	if binding.ClusterID.IsZero() {
		return manifestPath(appName, envName)
	}
	return fmt.Sprintf("apps/%s/%s/%s/values.yaml", appName, envName, binding.ClusterID)
}

func argoApplicationPath(appName, envName string) string {
	return fmt.Sprintf("argocd/apps/%s-%s.yaml", appName, envName)
}

func argoApplicationPathForBinding(appName, envName string, binding ClusterBindingRef) string {
	if binding.ClusterID.IsZero() {
		return argoApplicationPath(appName, envName)
	}
	return fmt.Sprintf("argocd/apps/%s-%s-%s.yaml", appName, envName, binding.ClusterID)
}

func commitDirectly(envName string) bool {
	return envName == "dev" || envName == "test"
}

func splitImage(uri string) (repository string, tag string) {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return "", ""
	}
	lastSlash := strings.LastIndex(uri, "/")
	lastColon := strings.LastIndex(uri, ":")
	if lastColon > lastSlash {
		return uri[:lastColon], uri[lastColon+1:]
	}
	return uri, ""
}

func validateTemplate(content string) ValidationResult {
	content = strings.TrimSpace(content)
	result := ValidationResult{Valid: true}
	if content == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "模板内容不能为空")
	}
	if strings.Count(content, "{{") != strings.Count(content, "}}") {
		result.Valid = false
		result.Errors = append(result.Errors, "模板变量语法不完整")
	}
	lower := strings.ToLower(content)
	if strings.Contains(lower, "hostpath:") || strings.Contains(lower, "privileged: true") {
		result.Valid = false
		result.Errors = append(result.Errors, "模板包含不允许的特权配置")
	}
	if strings.Contains(content, "initContainers") || strings.Contains(content, "volumeMounts") || strings.Contains(content, "securityContext") {
		result.Warnings = append(result.Warnings, "模板包含高级运行时配置，发布前请确认权限和目录策略")
	}
	return result
}
