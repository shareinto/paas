package clusteragent

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"time"

	"github.com/shareinto/paas/internal/shared"
)

type ClusterStatus string

const (
	ClusterReady       ClusterStatus = "ready"
	ClusterDegraded    ClusterStatus = "degraded"
	ClusterUnreachable ClusterStatus = "unreachable"
	ClusterDraining    ClusterStatus = "draining"
	ClusterDisabled    ClusterStatus = "disabled"
)

type Cluster struct {
	ID              shared.ID         `json:"id"`
	TenantID        shared.ID         `json:"tenant_id"`
	Name            string            `json:"name"`
	Region          string            `json:"region"`
	Labels          map[string]string `json:"labels"`
	ServerVersion   string            `json:"server_version"`
	Status          ClusterStatus     `json:"status"`
	AgentTokenHash  string            `json:"-"`
	LastHeartbeatAt *time.Time        `json:"last_heartbeat_at,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

type ClusterHeartbeat struct {
	ID              shared.ID `json:"id"`
	ClusterID       shared.ID `json:"cluster_id"`
	AgentVersion    string    `json:"agent_version"`
	ObservedAt      time.Time `json:"observed_at"`
	Message         string    `json:"message"`
	ControlPlaneURL string    `json:"control_plane_url"`
}

type ClusterResourceSnapshot struct {
	ID          shared.ID         `json:"id"`
	ClusterID   shared.ID         `json:"cluster_id"`
	TenantID    shared.ID         `json:"tenant_id"`
	Payload     StatusReport      `json:"payload"`
	ReportedAt  time.Time         `json:"reported_at"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type ClusterTaskStatus string

const (
	ClusterTaskPending   ClusterTaskStatus = "pending"
	ClusterTaskRunning   ClusterTaskStatus = "running"
	ClusterTaskSucceeded ClusterTaskStatus = "succeeded"
	ClusterTaskFailed    ClusterTaskStatus = "failed"
	ClusterTaskCanceled  ClusterTaskStatus = "canceled"
)

type ClusterTask struct {
	ID            shared.ID         `json:"id"`
	ClusterID     shared.ID         `json:"cluster_id"`
	Type          string            `json:"type"`
	TargetRef     string            `json:"target_ref"`
	Payload       map[string]string `json:"payload,omitempty"`
	Status        ClusterTaskStatus `json:"status"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	LeasedAt      *time.Time        `json:"leased_at,omitempty"`
	CompletedAt   *time.Time        `json:"completed_at,omitempty"`
	ResultMessage string            `json:"result_message"`
}

type ClusterTaskResult struct {
	ID         shared.ID         `json:"id"`
	ClusterID  shared.ID         `json:"cluster_id"`
	TaskID     shared.ID         `json:"task_id"`
	Status     ClusterTaskStatus `json:"status"`
	Message    string            `json:"message"`
	ReportedAt time.Time         `json:"reported_at"`
}

type StatusReport struct {
	ClusterID    shared.ID              `json:"cluster_id"`
	Applications []ApplicationStatus    `json:"applications"`
	Workloads    []WorkloadStatus       `json:"workloads"`
	Events       []ClusterReportedEvent `json:"events,omitempty"`
	ReportedAt   time.Time              `json:"reported_at"`
}

type ApplicationStatus struct {
	ApplicationID       shared.ID `json:"application_id"`
	EnvironmentID       shared.ID `json:"environment_id"`
	DeploymentID        shared.ID `json:"deployment_id"`
	ArgoApplicationName string    `json:"argo_application_name"`
	SyncStatus          string    `json:"sync_status"`
	HealthStatus        string    `json:"health_status"`
	OperationState      string    `json:"operation_state"`
	Message             string    `json:"message"`
}

type WorkloadStatus struct {
	ApplicationID shared.ID `json:"application_id"`
	EnvironmentID shared.ID `json:"environment_id"`
	Kind          string    `json:"kind"`
	Name          string    `json:"name"`
	Desired       int       `json:"desired"`
	Ready         int       `json:"ready"`
	Updated       int       `json:"updated"`
	Available     int       `json:"available"`
}

type ClusterReportedEvent struct {
	Type       string    `json:"type"`
	Resource   string    `json:"resource"`
	Message    string    `json:"message"`
	OccurredAt time.Time `json:"occurred_at"`
}

func normalizeCluster(cluster Cluster) (Cluster, error) {
	cluster.Name = strings.TrimSpace(cluster.Name)
	cluster.Region = strings.TrimSpace(cluster.Region)
	cluster.Labels = normalizeLabels(cluster.Labels)
	if cluster.Name == "" {
		return Cluster{}, shared.NewError(shared.CodeInvalidArgument, "cluster name is required")
	}
	if cluster.Status == "" {
		cluster.Status = ClusterReady
	}
	return cluster, nil
}

func normalizeLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return nil
	}
	out := map[string]string{}
	for key, value := range labels {
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

func newAgentToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "paas_agent_" + base64.RawURLEncoding.EncodeToString(raw[:]), nil
}
