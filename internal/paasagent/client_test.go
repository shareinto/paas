package paasagent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/shareinto/paas/internal/modules/clusteragent"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
)

func TestHTTPControlPlaneClientSendsAuthenticatedRequests(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" || r.Header.Get("X-PaaS-Cluster-ID") != "cluster_1" {
			t.Fatalf("missing auth headers: %#v", r.Header)
		}
		mu.Lock()
		seen[r.Method+" "+r.URL.Path]++
		mu.Unlock()
		if r.URL.Path == "/api/agent/v1/tasks" {
			_ = json.NewEncoder(w).Encode(map[string]any{"items": []Task{{ID: "task_1", Type: "argocd_refresh", TargetRef: "order-dev"}}})
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTPControlPlaneClient(Config{ClusterID: "cluster_1", ControlPlaneURL: server.URL + "/", AgentToken: "token"})
	if err := client.Heartbeat(context.Background(), clusteragent.ClusterHeartbeat{AgentVersion: "v1"}); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	if err := client.ReportStatus(context.Background(), clusteragent.StatusReport{ClusterID: "cluster_1"}); err != nil {
		t.Fatalf("status: %v", err)
	}
	if err := client.ReportEvents(context.Background(), clusteragent.StatusReport{}); err != nil {
		t.Fatalf("events: %v", err)
	}
	tasks, err := client.PullTasks(context.Background())
	if err != nil {
		t.Fatalf("pull tasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].ID != "task_1" {
		t.Fatalf("unexpected tasks: %#v", tasks)
	}
	if err := client.ReportTaskResult(context.Background(), "task_1", "succeeded", "ok"); err != nil {
		t.Fatalf("task result: %v", err)
	}
	for _, key := range []string{
		"POST /api/agent/v1/heartbeat",
		"POST /api/agent/v1/status/report",
		"POST /api/agent/v1/events/report",
		"GET /api/agent/v1/tasks",
		"POST /api/agent/v1/tasks/result",
	} {
		if seen[key] != 1 {
			t.Fatalf("expected one %s request, got %#v", key, seen)
		}
	}
}

func TestHTTPControlPlaneClientMapsServerErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	client := NewHTTPControlPlaneClient(Config{ClusterID: "cluster_1", ControlPlaneURL: server.URL, AgentToken: "token"})
	if err := client.Heartbeat(context.Background(), clusteragent.ClusterHeartbeat{}); err == nil {
		t.Fatalf("expected server error")
	}
}

func TestNotifyOnWatchAndStopWatchers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fakeWatch := watch.NewFake()
	called := make(chan struct{}, 1)
	go notifyOnWatch(ctx, fakeWatch.ResultChan(), func() { called <- struct{}{} })
	fakeWatch.Add(&corev1.Pod{})
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatalf("watch event did not trigger callback")
	}
	stopWatchers([]interface{ Stop() }{fakeWatch})
}
