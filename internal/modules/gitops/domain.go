package gitops

import (
	"fmt"
	"strings"
	"time"

	"github.com/shareinto/paas/internal/shared"
)

type TemplateScope string

const (
	TemplateScopePlatform TemplateScope = "platform"
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
	ID             shared.ID `json:"id"`
	TenantID       shared.ID `json:"tenant_id"`
	Name           string    `json:"name"`
	Content        string    `json:"content"`
	CurrentVersion int       `json:"current_version"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
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
	StageKey           string    `json:"stage_key"`
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
	StageKey           string           `json:"stage_key"`
	ClusterBindingID   shared.ID        `json:"cluster_binding_id"`
	PromotionID        shared.ID        `json:"promotion_id"`
	FreightID          shared.ID        `json:"freight_id"`
	ManifestRevisionID shared.ID        `json:"manifest_revision_id"`
	ImageRepository    string           `json:"image_repository"`
	ImageTag           string           `json:"image_tag"`
	ImageDigest        string           `json:"image_digest"`
	WorkloadSummary    string           `json:"workload_summary"`
	ConfigHash         string           `json:"config_hash"`
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

type ClusterBindingRef struct {
	ID          shared.ID
	StageKey    string
	ClusterID   shared.ID
	ClusterName string
	Namespace   string
	Labels      map[string]string
	Active      bool
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
	CPU    string `json:"cpu,omitempty" yaml:"cpu,omitempty"`
	Memory string `json:"memory,omitempty" yaml:"memory,omitempty"`
}

type WorkloadServicePortRef struct {
	Name       string `json:"name" yaml:"name"`
	Port       int    `json:"port" yaml:"port"`
	TargetPort int    `json:"target_port" yaml:"targetPort"`
	Protocol   string `json:"protocol,omitempty" yaml:"protocol,omitempty"`
}

type WorkloadEnvVarRef struct {
	Name  string `json:"name" yaml:"name"`
	Value string `json:"value" yaml:"value"`
}

type WorkloadProbeRef struct {
	Name                string   `json:"name" yaml:"name"`
	Type                string   `json:"type" yaml:"type"`
	Path                string   `json:"path,omitempty" yaml:"path,omitempty"`
	Port                int      `json:"port,omitempty" yaml:"port,omitempty"`
	Command             []string `json:"command,omitempty" yaml:"command,omitempty"`
	InitialDelaySeconds int      `json:"initial_delay_seconds,omitempty" yaml:"initialDelaySeconds,omitempty"`
	PeriodSeconds       int      `json:"period_seconds,omitempty" yaml:"periodSeconds,omitempty"`
}

type WorkloadIngressHostRef struct {
	Host        string `yaml:"host"`
	Path        string `yaml:"path"`
	ServerName  string `yaml:"serverName,omitempty"`
	ServicePort string `yaml:"servicePort,omitempty"`
	PathType    string `yaml:"pathType,omitempty"`
	TLS         bool   `yaml:"tls,omitempty"`
	TLSRedirect bool   `yaml:"tlsRedirect,omitempty"`
	Rewrite     bool   `yaml:"rewrite,omitempty"`
	RewritePath string `yaml:"rewritePath,omitempty"`
}

type WorkloadSecretRef struct {
	Name      string `json:"name" yaml:"name"`
	SecretRef string `json:"secret_ref" yaml:"secretRef"`
}

type WorkloadConfigFileRef struct {
	MountPath     string `json:"mount_path" yaml:"mountPath"`
	Content       string `json:"content" yaml:"content"`
	Base64Encoded bool   `json:"base64_encoded,omitempty" yaml:"base64Encoded,omitempty"`
}

type WorkloadWritableDirRef struct {
	MountPath  string `json:"mount_path" yaml:"mountPath"`
	SizeLimit  string `json:"size_limit,omitempty" yaml:"sizeLimit,omitempty"`
	OwnerGroup string `json:"owner_group,omitempty" yaml:"ownerGroup,omitempty"`
	Mode       string `json:"mode,omitempty" yaml:"mode,omitempty"`
}

type WorkloadVolumeMountRef struct {
	Name      string `json:"name" yaml:"name"`
	MountPath string `json:"mount_path" yaml:"mountPath"`
}

type WorkloadInitContainerRef struct {
	Name    string   `json:"name" yaml:"name"`
	Image   string   `json:"image" yaml:"image"`
	Command []string `json:"command,omitempty" yaml:"command,omitempty"`
}

type WorkloadStageConfigRef struct {
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

func manifestPath(appName, stageKey string) string {
	return fmt.Sprintf("apps/%s/%s/manifests.yaml", appName, stageKey)
}

func manifestPathForBinding(appName, stageKey string, binding ClusterBindingRef) string {
	return manifestPath(appName, stageKey)
}

func argoApplicationPath(appName, stageKey string) string {
	return fmt.Sprintf("argocd/apps/%s/%s-%s.yaml", stageKey, appName, stageKey)
}

func argoApplicationStageKeepPath(stageKey string) string {
	return fmt.Sprintf("argocd/apps/%s/.gitkeep", stageKey)
}

func argoApplicationPathForBinding(appName, stageKey string, binding ClusterBindingRef) string {
	return argoApplicationPath(appName, stageKey)
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
