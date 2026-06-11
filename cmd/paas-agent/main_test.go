package main

import (
	"context"
	"errors"
	"log"
	"testing"
	"time"

	"github.com/shareinto/paas/internal/modules/clusteragent"
)

func TestRuntimeConfigFromEnvDefaultsArgoCDNamespace(t *testing.T) {
	t.Setenv("PAAS_CLUSTER_ID", "cluster_1")
	t.Setenv("PAAS_CONTROL_PLANE_URL", "https://paas.example")
	t.Setenv("PAAS_AGENT_TOKEN", "token")
	t.Setenv("PAAS_AGENT_NAMESPACES", " apps, argocd ,,")
	t.Setenv("PAAS_HEARTBEAT_INTERVAL", "5s")
	t.Setenv("PAAS_SNAPSHOT_INTERVAL", "15s")

	config, argoNamespace := runtimeConfigFromEnv()

	if argoNamespace != "argocd" {
		t.Fatalf("argo namespace = %q, want argocd", argoNamespace)
	}
	if config.ClusterID.String() != "cluster_1" || config.ControlPlaneURL != "https://paas.example" || config.AgentToken != "token" {
		t.Fatalf("unexpected config: %#v", config)
	}
	if len(config.Namespaces) != 2 || config.Namespaces[0] != "apps" || config.Namespaces[1] != "argocd" {
		t.Fatalf("unexpected namespaces: %#v", config.Namespaces)
	}
	if config.HeartbeatInterval != 5*time.Second || config.SnapshotInterval != 15*time.Second {
		t.Fatalf("unexpected intervals: heartbeat=%s snapshot=%s", config.HeartbeatInterval, config.SnapshotInterval)
	}
}

func TestRuntimeConfigFromEnvReadsArgoCDNamespace(t *testing.T) {
	t.Setenv("PAAS_CLUSTER_ID", "cluster_1")
	t.Setenv("PAAS_CONTROL_PLANE_URL", "https://paas.example")
	t.Setenv("PAAS_AGENT_TOKEN", "token")
	t.Setenv("PAAS_ARGOCD_NAMESPACE", " platform-argocd ")

	_, argoNamespace := runtimeConfigFromEnv()

	if argoNamespace != "platform-argocd" {
		t.Fatalf("argo namespace = %q, want platform-argocd", argoNamespace)
	}
}

func TestRunAgentSendsInitialReportsAndStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	agent := &fakeRuntimeAgent{cancel: cancel}

	err := runAgent(ctx, agent, 20*time.Millisecond, 20*time.Millisecond, log.New(testWriter{t: t}, "", 0))

	if err != nil {
		t.Fatalf("runAgent() error = %v", err)
	}
	if agent.heartbeats != 1 {
		t.Fatalf("heartbeats = %d, want 1", agent.heartbeats)
	}
	if agent.snapshots != 1 {
		t.Fatalf("snapshots = %d, want 1", agent.snapshots)
	}
	if agent.tasks != 1 {
		t.Fatalf("tasks = %d, want 1", agent.tasks)
	}
	if agent.watches != 1 {
		t.Fatalf("watches = %d, want 1", agent.watches)
	}
}

func TestRunAgentReturnsWatchError(t *testing.T) {
	errWatch := errors.New("watch unavailable")
	agent := &fakeRuntimeAgent{watchErr: errWatch}

	err := runAgent(context.Background(), agent, time.Hour, time.Hour, log.New(testWriter{t: t}, "", 0))

	if !errors.Is(err, errWatch) {
		t.Fatalf("runAgent() error = %v, want %v", err, errWatch)
	}
}

func TestSplitCSVTrimsAndSkipsBlankParts(t *testing.T) {
	values := splitCSV(" dev, test ,, prod ")
	if len(values) != 3 || values[0] != "dev" || values[2] != "prod" {
		t.Fatalf("unexpected values: %#v", values)
	}
	if values := splitCSV("  "); values != nil {
		t.Fatalf("blank input should return nil: %#v", values)
	}
}

type fakeRuntimeAgent struct {
	cancel     context.CancelFunc
	heartbeats int
	snapshots  int
	tasks      int
	watches    int
	watchErr   error
}

func (a *fakeRuntimeAgent) SendHeartbeat(context.Context) error {
	a.heartbeats++
	return nil
}

func (a *fakeRuntimeAgent) ReportSnapshot(context.Context) (clusteragent.StatusReport, error) {
	a.snapshots++
	return clusteragent.StatusReport{}, nil
}

func (a *fakeRuntimeAgent) RunTaskOnce(context.Context) error {
	a.tasks++
	if a.cancel != nil {
		a.cancel()
	}
	return nil
}

func (a *fakeRuntimeAgent) WatchChanges(ctx context.Context) error {
	a.watches++
	if a.watchErr != nil {
		return a.watchErr
	}
	<-ctx.Done()
	return ctx.Err()
}

type testWriter struct {
	t *testing.T
}

func (w testWriter) Write(data []byte) (int, error) {
	w.t.Log(string(data))
	return len(data), nil
}
