package clusteragent

import (
	"context"
	"time"

	"github.com/shareinto/paas/internal/shared"
)

type Repository interface {
	CreateCluster(ctx context.Context, cluster Cluster) error
	UpdateCluster(ctx context.Context, cluster Cluster) error
	GetCluster(ctx context.Context, id shared.ID) (Cluster, error)
	ListClusters(ctx context.Context, page shared.PageRequest) (shared.PageResult[Cluster], error)

	CreateHeartbeat(ctx context.Context, heartbeat ClusterHeartbeat) error
	CreateSnapshot(ctx context.Context, snapshot ClusterResourceSnapshot) error

	CreateTask(ctx context.Context, task ClusterTask) error
	UpdateTask(ctx context.Context, task ClusterTask) error
	GetTask(ctx context.Context, id shared.ID) (ClusterTask, error)
	ListPendingTasks(ctx context.Context, clusterID shared.ID, limit int) ([]ClusterTask, error)
	CreateTaskResult(ctx context.Context, result ClusterTaskResult) error
}

type EnvironmentStateUpdater interface {
	UpdateFromAgent(ctx context.Context, report StatusReport) error
}

type DeploymentStatusUpdater interface {
	UpdateFromAgent(ctx context.Context, report StatusReport) error
}

type AuditLogger interface {
	Log(ctx context.Context, event AuditEvent) error
}

type AuditEvent struct {
	ActorID      shared.ID
	TenantID     shared.ID
	Action       string
	ResourceType string
	ResourceID   shared.ID
	Result       string
	Summary      string
	OccurredAt   time.Time
}
