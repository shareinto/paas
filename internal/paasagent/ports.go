package paasagent

import (
	"context"

	"github.com/shareinto/paas/internal/modules/clusteragent"
)

type ControlPlaneClient interface {
	Heartbeat(ctx context.Context, heartbeat clusteragent.ClusterHeartbeat) error
	ReportStatus(ctx context.Context, report clusteragent.StatusReport) error
	ReportEvents(ctx context.Context, report clusteragent.StatusReport) error
	PullTasks(ctx context.Context) ([]Task, error)
	ReportTaskResult(ctx context.Context, taskID string, status string, message string) error
}

type KubernetesReader interface {
	Snapshot(ctx context.Context, namespaces []string) (Snapshot, error)
	Watch(ctx context.Context, namespaces []string, onChange func()) error
	RefreshArgoApplication(ctx context.Context, name string) error
	SyncArgoApplication(ctx context.Context, name string) error
}
