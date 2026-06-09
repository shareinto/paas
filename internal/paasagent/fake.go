package paasagent

import (
	"context"

	"github.com/shareinto/paas/internal/modules/clusteragent"
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
func (c *FakeControlPlaneClient) PullTasks(context.Context) ([]Task, error) { return c.Tasks, nil }
func (c *FakeControlPlaneClient) ReportTaskResult(_ context.Context, taskID string, status string, message string) error {
	c.Results = append(c.Results, map[string]string{"task_id": taskID, "status": status, "message": message})
	return nil
}

type FakeKubernetesReader struct {
	SnapshotValue Snapshot
	Refreshed     []string
	Synced        []string
}

func (r *FakeKubernetesReader) Snapshot(context.Context, []string) (Snapshot, error) {
	return r.SnapshotValue, nil
}
func (r *FakeKubernetesReader) Watch(_ context.Context, _ []string, onChange func()) error {
	if onChange != nil {
		onChange()
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
