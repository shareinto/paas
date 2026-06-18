package paasagent

import (
	"context"
	"io"

	"github.com/shareinto/paas/internal/modules/clusteragent"
	"github.com/shareinto/paas/internal/shared"
)

type FakeControlPlaneClient struct {
	Heartbeats   []clusteragent.ClusterHeartbeat
	Reports      []clusteragent.StatusReport
	EventReports []clusteragent.StatusReport
	Tasks        []Task
	Results      []map[string]string
}

func (c *FakeControlPlaneClient) Heartbeat(_ context.Context, heartbeat clusteragent.ClusterHeartbeat) error {
	c.Heartbeats = append(c.Heartbeats, heartbeat)
	return nil
}
func (c *FakeControlPlaneClient) ReportStatus(_ context.Context, report clusteragent.StatusReport) error {
	c.Reports = append(c.Reports, report)
	return nil
}
func (c *FakeControlPlaneClient) ReportEvents(_ context.Context, report clusteragent.StatusReport) error {
	c.EventReports = append(c.EventReports, report)
	return nil
}
func (c *FakeControlPlaneClient) ConnectRuntime(context.Context, RuntimeRequestHandler) error {
	return nil
}
func (c *FakeControlPlaneClient) PullTasks(context.Context) ([]Task, error) { return c.Tasks, nil }
func (c *FakeControlPlaneClient) ReportTaskResult(_ context.Context, taskID string, status string, message string) error {
	c.Results = append(c.Results, map[string]string{"task_id": taskID, "status": status, "message": message})
	return nil
}

type FakeKubernetesReader struct {
	SnapshotValue        Snapshot
	ApplicationSnapshots []Snapshot
	Refreshed            []string
	Synced               []string
	Restarted            []string
	Logs                 string
	WatchChanges         int
	WatchBlock           bool
	Invalidations        []RuntimeInvalidation
}

func (r *FakeKubernetesReader) Snapshot(context.Context, []string) (Snapshot, error) {
	return r.SnapshotValue, nil
}
func (r *FakeKubernetesReader) ApplicationStatusSnapshot(context.Context) (Snapshot, error) {
	return applicationsOnlySnapshot(r.SnapshotValue), nil
}
func (r *FakeKubernetesReader) RunApplicationStatusCache(ctx context.Context, onChange func(Snapshot)) error {
	snapshots := r.ApplicationSnapshots
	if len(snapshots) == 0 {
		snapshots = []Snapshot{applicationsOnlySnapshot(r.SnapshotValue)}
	}
	for _, snapshot := range snapshots {
		if onChange != nil {
			onChange(applicationsOnlySnapshot(snapshot))
		}
	}
	if r.WatchBlock {
		<-ctx.Done()
		return ctx.Err()
	}
	return nil
}
func (r *FakeKubernetesReader) Watch(ctx context.Context, _ []string, onChange func()) error {
	changes := r.WatchChanges
	if changes <= 0 {
		changes = 1
	}
	for i := 0; i < changes; i++ {
		if onChange != nil {
			onChange()
		}
	}
	if r.WatchBlock {
		<-ctx.Done()
		return ctx.Err()
	}
	return nil
}
func (r *FakeKubernetesReader) RunRuntimeCache(ctx context.Context, _ []string, onInvalidation func(RuntimeInvalidation)) error {
	invalidations := r.Invalidations
	if len(invalidations) == 0 {
		invalidations = runtimeInvalidationsFromResources(r.SnapshotValue.RuntimeResources)
	}
	if len(invalidations) == 0 && r.WatchChanges > 0 {
		invalidations = []RuntimeInvalidation{{ApplicationID: "app_1", StageKey: "dev"}}
	}
	for _, invalidation := range invalidations {
		if onInvalidation != nil {
			onInvalidation(invalidation)
		}
	}
	if r.WatchBlock {
		<-ctx.Done()
		return ctx.Err()
	}
	return nil
}
func (r *FakeKubernetesReader) ListRuntimeResources(_ context.Context, _ []string, _ shared.ID, _ string) ([]RuntimeResource, error) {
	return r.SnapshotValue.RuntimeResources, nil
}
func (r *FakeKubernetesReader) WatchRuntimeResources(_ context.Context, _ []string, _ shared.ID, _ string, onChange func([]RuntimeResource)) error {
	if onChange != nil {
		onChange(r.SnapshotValue.RuntimeResources)
	}
	return nil
}
func (r *FakeKubernetesReader) RefreshArgoApplication(_ context.Context, name string) error {
	r.Refreshed = append(r.Refreshed, name)
	return nil
}
func (r *FakeKubernetesReader) SyncArgoApplication(_ context.Context, name string) error {
	r.Synced = append(r.Synced, name)
	return nil
}
func (r *FakeKubernetesReader) RestartRuntimeResource(_ context.Context, kind string, namespace string, name string) error {
	r.Restarted = append(r.Restarted, kind+"/"+namespace+"/"+name)
	return nil
}
func (r *FakeKubernetesReader) StreamPodLogs(_ context.Context, _ string, _ string, _ string, _ int64, writer io.Writer) error {
	_, err := writer.Write([]byte(r.Logs))
	return err
}
func (r *FakeKubernetesReader) Terminal(ctx context.Context, _ string, _ string, _ string, _ string, input <-chan []byte, output chan<- []byte) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case data, ok := <-input:
			if !ok {
				return nil
			}
			output <- data
		}
	}
}

func runtimeInvalidationsFromResources(resources []RuntimeResource) []RuntimeInvalidation {
	seen := map[RuntimeInvalidation]struct{}{}
	out := []RuntimeInvalidation{}
	for _, resource := range resources {
		if resource.ApplicationID.IsZero() || resource.StageKey == "" {
			continue
		}
		invalidation := RuntimeInvalidation{ApplicationID: resource.ApplicationID, StageKey: resource.StageKey}
		if _, ok := seen[invalidation]; ok {
			continue
		}
		seen[invalidation] = struct{}{}
		out = append(out, invalidation)
	}
	return out
}

func applicationsOnlySnapshot(snapshot Snapshot) Snapshot {
	return Snapshot{Applications: append([]ArgoApplication(nil), snapshot.Applications...)}
}
