package paasagent

import (
	"strings"
	"time"

	"github.com/shareinto/paas/internal/modules/clusteragent"
	"github.com/shareinto/paas/internal/shared"
)

type ArgoApplication struct {
	Name           string
	ApplicationID  shared.ID
	EnvironmentID  shared.ID
	DeploymentID   shared.ID
	SyncStatus     string
	HealthStatus   string
	OperationPhase string
	Message        string
}

type Workload struct {
	Kind          string
	Name          string
	ApplicationID shared.ID
	EnvironmentID shared.ID
	Desired       int
	Ready         int
	Updated       int
	Available     int
}

type KubernetesEvent struct {
	Type       string
	Resource   string
	Message    string
	OccurredAt time.Time
}

type Snapshot struct {
	Applications []ArgoApplication
	Workloads    []Workload
	Events       []KubernetesEvent
}

type Task struct {
	ID        shared.ID         `json:"id"`
	Type      string            `json:"type"`
	TargetRef string            `json:"target_ref"`
	Payload   map[string]string `json:"payload,omitempty"`
}

func ToStatusReport(clusterID shared.ID, snapshot Snapshot, reportedAt time.Time) clusteragent.StatusReport {
	report := clusteragent.StatusReport{ClusterID: clusterID, ReportedAt: reportedAt}
	for _, app := range snapshot.Applications {
		report.Applications = append(report.Applications, clusteragent.ApplicationStatus{
			ApplicationID: app.ApplicationID, EnvironmentID: app.EnvironmentID, DeploymentID: app.DeploymentID,
			ArgoApplicationName: app.Name, SyncStatus: app.SyncStatus, HealthStatus: app.HealthStatus, OperationState: mapOperation(app.OperationPhase), Message: app.Message,
		})
	}
	for _, workload := range snapshot.Workloads {
		report.Workloads = append(report.Workloads, clusteragent.WorkloadStatus{ApplicationID: workload.ApplicationID, EnvironmentID: workload.EnvironmentID, Kind: workload.Kind, Name: workload.Name, Desired: workload.Desired, Ready: workload.Ready, Updated: workload.Updated, Available: workload.Available})
	}
	for _, event := range snapshot.Events {
		report.Events = append(report.Events, clusteragent.ClusterReportedEvent{Type: event.Type, Resource: event.Resource, Message: event.Message, OccurredAt: event.OccurredAt})
	}
	return report
}

func mapOperation(phase string) string {
	switch strings.ToLower(strings.TrimSpace(phase)) {
	case "running":
		return "running"
	case "succeeded":
		return "succeeded"
	case "failed", "error":
		return "failed"
	default:
		return "unknown"
	}
}
