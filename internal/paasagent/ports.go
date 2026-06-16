package paasagent

import (
	"context"
	"io"

	"github.com/shareinto/paas/internal/modules/clusteragent"
	"github.com/shareinto/paas/internal/shared"
)

type ControlPlaneClient interface {
	Heartbeat(ctx context.Context, heartbeat clusteragent.ClusterHeartbeat) error
	ReportStatus(ctx context.Context, report clusteragent.StatusReport) error
	ReportEvents(ctx context.Context, report clusteragent.StatusReport) error
	ConnectRuntime(ctx context.Context, handler RuntimeRequestHandler) error
	PullTasks(ctx context.Context) ([]Task, error)
	ReportTaskResult(ctx context.Context, taskID string, status string, message string) error
}

type KubernetesReader interface {
	Snapshot(ctx context.Context, namespaces []string) (Snapshot, error)
	Watch(ctx context.Context, namespaces []string, onChange func()) error
	RunRuntimeCache(ctx context.Context, namespaces []string, onInvalidation func(RuntimeInvalidation)) error
	ListRuntimeResources(ctx context.Context, namespaces []string, applicationID shared.ID, stageKey string) ([]RuntimeResource, error)
	WatchRuntimeResources(ctx context.Context, namespaces []string, applicationID shared.ID, stageKey string, onChange func([]RuntimeResource)) error
	RefreshArgoApplication(ctx context.Context, name string) error
	SyncArgoApplication(ctx context.Context, name string) error
	RestartRuntimeResource(ctx context.Context, kind string, namespace string, name string) error
	StreamPodLogs(ctx context.Context, namespace string, name string, container string, tailLines int64, writer io.Writer) error
	Terminal(ctx context.Context, namespace string, name string, container string, command string, input <-chan []byte, output chan<- []byte) error
}

type RuntimeRequestHandler interface {
	HandleRuntimeRequest(ctx context.Context, msg clusteragent.RuntimeWireMessage, sender RuntimeMessageSender) error
}

type RuntimeMessageSender interface {
	SendRuntimeMessage(ctx context.Context, msg clusteragent.RuntimeWireMessage) error
}

type RuntimeInvalidation struct {
	ApplicationID shared.ID
	StageKey      string
}
