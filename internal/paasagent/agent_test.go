package paasagent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shareinto/paas/internal/modules/clusteragent"
	"github.com/shareinto/paas/internal/shared"
)

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func TestHeartbeatApplicationStatusMappingAndTaskExecution(t *testing.T) {
	control := &FakeControlPlaneClient{Tasks: []Task{{ID: "task_1", Type: "argocd_refresh", TargetRef: "order-dev"}, {ID: "task_2", Type: "argocd_sync", TargetRef: "order-dev"}}}
	reader := &FakeKubernetesReader{SnapshotValue: Snapshot{
		Applications: []ArgoApplication{{Name: "order-dev", ApplicationID: "app_1", StageKey: "dev", DeploymentID: "deployment_1", SyncStatus: "Synced", HealthStatus: "Healthy", OperationPhase: "Succeeded"}},
		Workloads:    []Workload{{Kind: "Deployment", Name: "order-api", Desired: 3, Ready: 2, Updated: 3, Available: 2}},
		Events:       []KubernetesEvent{{Type: "Warning", Resource: "pod/order", Message: "重启", OccurredAt: time.Date(2026, 5, 30, 14, 0, 0, 0, time.UTC)}},
	}}
	agent := New(Config{ClusterID: "cluster_1", ControlPlaneURL: "https://paas.example", AgentToken: "token"}, control, reader, fixedClock{now: time.Date(2026, 5, 30, 14, 0, 0, 0, time.UTC)})
	if err := agent.SendHeartbeat(context.Background()); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	report, err := agent.ReportSnapshot(context.Background())
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if report.ClusterID != "cluster_1" || report.Applications[0].OperationState != "succeeded" || len(control.EventReports) != 0 || len(report.Workloads) != 0 || len(report.RuntimeResources) != 0 || len(report.Events) != 0 {
		t.Fatalf("unexpected report: %#v", report)
	}
	if err := agent.RunTaskOnce(context.Background()); err != nil {
		t.Fatalf("tasks: %v", err)
	}
	if len(reader.Refreshed) != 1 || len(reader.Synced) != 1 || len(control.Results) != 2 {
		t.Fatalf("task execution not recorded")
	}
}

func TestUnsupportedTaskFailsWithoutApplyingResources(t *testing.T) {
	control := &FakeControlPlaneClient{Tasks: []Task{{ID: "task_1", Type: "kubectl_apply", TargetRef: "anything"}}}
	reader := &FakeKubernetesReader{}
	agent := New(Config{ClusterID: "cluster_1"}, control, reader, nil)
	if err := agent.RunTaskOnce(context.Background()); err != nil {
		t.Fatalf("tasks should report failure instead of returning: %v", err)
	}
	if control.Results[0]["status"] != "failed" || len(reader.Synced) != 0 || len(reader.Refreshed) != 0 {
		t.Fatalf("unsupported task should fail without resource mutation: %#v", control.Results)
	}
}

func TestRuntimeRestartTaskRestartsSupportedWorkload(t *testing.T) {
	control := &FakeControlPlaneClient{Tasks: []Task{{ID: "task_1", Type: "runtime_restart", Payload: map[string]string{"kind": "Deployment", "namespace": "order-dev", "name": "order-api"}}}}
	reader := &FakeKubernetesReader{}
	agent := New(Config{ClusterID: "cluster_1"}, control, reader, nil)
	if err := agent.RunTaskOnce(context.Background()); err != nil {
		t.Fatalf("runtime restart task: %v", err)
	}
	if len(reader.Restarted) != 1 || reader.Restarted[0] != "Deployment/order-dev/order-api" || control.Results[0]["status"] != "succeeded" {
		t.Fatalf("restart task should restart supported workload and report success, restarted=%+v results=%+v", reader.Restarted, control.Results)
	}
}

func TestWatchChangesDebouncesSnapshotReports(t *testing.T) {
	control := &FakeControlPlaneClient{}
	reader := &FakeKubernetesReader{SnapshotValue: Snapshot{Applications: []ArgoApplication{{Name: "order-dev", SyncStatus: "Synced", HealthStatus: "Healthy"}}}, WatchChanges: 3, WatchBlock: true}
	agent := New(Config{ClusterID: "cluster_1"}, control, reader, nil)
	agent.watchDebounce = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- agent.WatchChanges(ctx)
	}()
	for i := 0; i < 100; i++ {
		if len(control.Reports) == 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if len(control.Reports) != 1 {
		cancel()
		t.Fatalf("expected debounced report on watch changes, got %+v", control.Reports)
	}
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("watch changes error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("watch changes did not stop")
	}
}

func TestAgentSendsRuntimeStageChangedInvalidation(t *testing.T) {
	control := &FakeControlPlaneClient{}
	reader := &FakeKubernetesReader{SnapshotValue: Snapshot{Applications: []ArgoApplication{{Name: "order-dev", ApplicationID: "app_1", StageKey: "dev", DeploymentID: "deployment_1", SyncStatus: "Synced", HealthStatus: "Healthy"}}}, WatchBlock: true}
	agent := New(Config{ClusterID: "cluster_1"}, control, reader, nil)
	sender := &recordingRuntimeSender{messages: make(chan clusteragent.RuntimeWireMessage, 1)}
	agent.SetRuntimeSender(sender)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- agent.WatchChanges(ctx)
	}()
	var msg clusteragent.RuntimeWireMessage
	select {
	case msg = <-sender.messages:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for stage_changed")
	}
	cancel()
	<-done
	if msg.ID != "" || msg.Type != "stage_changed" || msg.ApplicationID != "app_1" || msg.StageKey != "dev" {
		t.Fatalf("unexpected stage_changed message: %+v", msg)
	}
}

func TestConfigDefaults(t *testing.T) {
	cfg := Config{ClusterID: shared.ID("cluster_1")}.Normalize()
	if cfg.HeartbeatInterval != 10*time.Second || cfg.SnapshotInterval != 30*time.Second {
		t.Fatalf("unexpected defaults: %#v", cfg)
	}
}

type recordingRuntimeSender struct {
	messages chan clusteragent.RuntimeWireMessage
}

func (s *recordingRuntimeSender) SendRuntimeMessage(_ context.Context, msg clusteragent.RuntimeWireMessage) error {
	s.messages <- msg
	return nil
}

func TestConfigValidateRequiresControlPlaneIdentityAndToken(t *testing.T) {
	valid := Config{ClusterID: "cluster_1", ControlPlaneURL: "https://paas.example", AgentToken: "token"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid config should pass: %v", err)
	}
	for _, cfg := range []Config{
		{ControlPlaneURL: "https://paas.example", AgentToken: "token"},
		{ClusterID: "cluster_1", AgentToken: "token"},
		{ClusterID: "cluster_1", ControlPlaneURL: "https://paas.example"},
	} {
		if err := cfg.Validate(); shared.CodeOf(err) != shared.CodeInvalidArgument {
			t.Fatalf("invalid config should return invalid_argument, got %v", err)
		}
	}
}

type failingControlPlaneClient struct {
	FakeControlPlaneClient
	err error
}

func (c *failingControlPlaneClient) ReportStatus(context.Context, clusteragent.StatusReport) error {
	return c.err
}

func (c *failingControlPlaneClient) PullTasks(context.Context) ([]Task, error) {
	return nil, c.err
}

type failingKubernetesReader struct {
	FakeKubernetesReader
	err error
}

func (r *failingKubernetesReader) Snapshot(context.Context, []string) (Snapshot, error) {
	return Snapshot{}, r.err
}

func (r *failingKubernetesReader) ApplicationStatusSnapshot(context.Context) (Snapshot, error) {
	return Snapshot{}, r.err
}

func (r *failingKubernetesReader) RefreshArgoApplication(context.Context, string) error {
	return r.err
}

func TestAgentErrorBranchesAndOperationMapping(t *testing.T) {
	errBoom := errors.New("boom")
	agent := New(Config{ClusterID: "cluster_1"}, &FakeControlPlaneClient{}, &failingKubernetesReader{err: errBoom}, nil)
	if _, err := agent.ReportSnapshot(context.Background()); !errors.Is(err, errBoom) {
		t.Fatalf("snapshot error = %v", err)
	}
	agent = New(Config{ClusterID: "cluster_1"}, &failingControlPlaneClient{err: errBoom}, &FakeKubernetesReader{}, nil)
	if _, err := agent.ReportSnapshot(context.Background()); !errors.Is(err, errBoom) {
		t.Fatalf("report status error = %v", err)
	}
	if err := agent.RunTaskOnce(context.Background()); !errors.Is(err, errBoom) {
		t.Fatalf("pull task error = %v", err)
	}
	control := &FakeControlPlaneClient{Tasks: []Task{{ID: "task_1", Type: "argocd_refresh", Payload: map[string]string{"argo_application": "order-dev"}}}}
	agent = New(Config{ClusterID: "cluster_1"}, control, &failingKubernetesReader{err: errBoom}, nil)
	if err := agent.RunTaskOnce(context.Background()); err != nil {
		t.Fatalf("task failure should be reported, not returned: %v", err)
	}
	if control.Results[0]["status"] != "failed" {
		t.Fatalf("expected failed result, got %#v", control.Results)
	}
	for phase, want := range map[string]string{"Running": "running", "Succeeded": "succeeded", "Error": "failed", "": "unknown"} {
		if got := mapOperation(phase); got != want {
			t.Fatalf("mapOperation(%q)=%q want %q", phase, got, want)
		}
	}
}
