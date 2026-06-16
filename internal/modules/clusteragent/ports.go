package clusteragent

import (
	"context"
	"io"
	"time"

	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/shared"
)

type Repository interface {
	CreateCluster(ctx context.Context, cluster Cluster) error
	UpdateCluster(ctx context.Context, cluster Cluster) error
	GetCluster(ctx context.Context, id shared.ID) (Cluster, error)
	ListClusters(ctx context.Context, page shared.PageRequest) (shared.PageResult[Cluster], error)
	ListClustersByTenant(ctx context.Context, tenantID shared.ID, page shared.PageRequest) (shared.PageResult[Cluster], error)

	CreateHeartbeat(ctx context.Context, heartbeat ClusterHeartbeat) error
	CreateSnapshot(ctx context.Context, snapshot ClusterResourceSnapshot) error
	ReplaceRuntimeResources(ctx context.Context, clusterID shared.ID, tenantID shared.ID, reportedAt time.Time, resources []RuntimeResourceStatus) error
	ListRuntimeResources(ctx context.Context, filter RuntimeResourceFilter) ([]RuntimeResource, error)
	GetRuntimeResource(ctx context.Context, id shared.ID) (RuntimeResource, error)

	CreateTask(ctx context.Context, task ClusterTask) error
	UpdateTask(ctx context.Context, task ClusterTask) error
	GetTask(ctx context.Context, id shared.ID) (ClusterTask, error)
	ListPendingTasks(ctx context.Context, clusterID shared.ID, limit int) ([]ClusterTask, error)
	CreateTaskResult(ctx context.Context, result ClusterTaskResult) error
}

type RuntimeGateway interface {
	ListRuntimeResources(ctx context.Context, clusterID shared.ID, applicationID shared.ID, stageKey string) ([]RuntimeResource, error)
	WatchRuntimeResources(ctx context.Context, clusterID shared.ID, applicationID shared.ID, stageKey string, onSnapshot func([]RuntimeResource) error, onStatus func(string) error) error
	RestartRuntimeResource(ctx context.Context, target RuntimeResourceTarget) error
	StreamPodLogs(ctx context.Context, target RuntimeResourceTarget, opts RuntimeLogOptions, writer io.Writer) error
	Terminal(ctx context.Context, target RuntimeResourceTarget, opts RuntimeTerminalOptions, input <-chan []byte, output chan<- []byte) error
}

type StageClusterResolver interface {
	ResolveStageCluster(ctx context.Context, applicationID shared.ID, stageKey string) (StageClusterRef, error)
}

type StageClusterRef struct {
	ClusterID shared.ID
	TenantID  shared.ID
}

type StageStateUpdater interface {
	UpdateFromAgent(ctx context.Context, report StatusReport) error
}

type DeploymentStatusUpdater interface {
	UpdateFromAgent(ctx context.Context, report StatusReport) error
}

type AuditLogger interface {
	Log(ctx context.Context, event AuditEvent) error
}

type PermissionChecker interface {
	Check(ctx context.Context, subject identityaccess.Subject, resource identityaccess.ResourceScope, action identityaccess.Permission) error
}

type TenantRef struct {
	ID shared.ID
}

type TenantQuery interface {
	GetTenant(ctx context.Context, id shared.ID) (TenantRef, error)
}

type RuntimeResourceFilter struct {
	ApplicationID shared.ID
	StageKey      string
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
