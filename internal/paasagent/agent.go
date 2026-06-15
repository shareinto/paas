package paasagent

import (
	"context"
	"strings"

	"github.com/shareinto/paas/internal/modules/clusteragent"
	"github.com/shareinto/paas/internal/shared"
)

type Agent struct {
	config Config
	client ControlPlaneClient
	reader KubernetesReader
	clock  shared.Clock
}

func New(config Config, client ControlPlaneClient, reader KubernetesReader, clock shared.Clock) *Agent {
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Agent{config: config.Normalize(), client: client, reader: reader, clock: clock}
}

func (a *Agent) SendHeartbeat(ctx context.Context) error {
	return a.client.Heartbeat(ctx, clusteragent.ClusterHeartbeat{ClusterID: a.config.ClusterID, ObservedAt: a.clock.Now(), ControlPlaneURL: a.config.ControlPlaneURL})
}

func (a *Agent) ReportSnapshot(ctx context.Context) (clusteragent.StatusReport, error) {
	snapshot, err := a.reader.Snapshot(ctx, a.config.Namespaces)
	if err != nil {
		return clusteragent.StatusReport{}, err
	}
	report := ToStatusReport(a.config.ClusterID, snapshot, a.clock.Now())
	if err := a.client.ReportStatus(ctx, report); err != nil {
		return clusteragent.StatusReport{}, err
	}
	if len(report.Events) > 0 {
		if err := a.client.ReportEvents(ctx, report); err != nil {
			return clusteragent.StatusReport{}, err
		}
	}
	return report, nil
}

func (a *Agent) RunTaskOnce(ctx context.Context) error {
	tasks, err := a.client.PullTasks(ctx)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		status, message := "succeeded", "任务执行成功"
		if err := a.executeTask(ctx, task); err != nil {
			status = "failed"
			message = err.Error()
		}
		if err := a.client.ReportTaskResult(ctx, task.ID.String(), status, message); err != nil {
			return err
		}
	}
	return nil
}

func (a *Agent) WatchChanges(ctx context.Context) error {
	return a.reader.Watch(ctx, a.config.Namespaces, func() {
		_, _ = a.ReportSnapshot(ctx)
	})
}

func (a *Agent) executeTask(ctx context.Context, task Task) error {
	target := strings.TrimSpace(task.TargetRef)
	if target == "" {
		target = strings.TrimSpace(task.Payload["argo_application"])
	}
	switch task.Type {
	case "argocd_refresh":
		if target == "" {
			return shared.NewError(shared.CodeInvalidArgument, "argo application target is required")
		}
		return a.reader.RefreshArgoApplication(ctx, target)
	case "argocd_sync":
		if target == "" {
			return shared.NewError(shared.CodeInvalidArgument, "argo application target is required")
		}
		return a.reader.SyncArgoApplication(ctx, target)
	case "runtime_restart":
		kind := strings.TrimSpace(task.Payload["kind"])
		namespace := strings.TrimSpace(task.Payload["namespace"])
		name := strings.TrimSpace(task.Payload["name"])
		if kind == "" || namespace == "" || name == "" {
			return shared.NewError(shared.CodeInvalidArgument, "runtime restart target is required")
		}
		return a.reader.RestartRuntimeResource(ctx, kind, namespace, name)
	default:
		return shared.NewError(shared.CodeInvalidArgument, "unsupported agent task")
	}
}
