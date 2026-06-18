package clusteragent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

func TestWebSocketRuntimeGatewayRefreshesOnlySubscribedStages(t *testing.T) {
	gateway := NewWebSocketRuntimeGateway()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Errorf("accept websocket: %v", err)
			return
		}
		_ = gateway.Connect(r.Context(), "cluster_1", conn)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	agentConn, _, err := websocket.Dial(ctx, "ws"+server.URL[len("http"):], nil)
	if err != nil {
		t.Fatalf("dial agent websocket: %v", err)
	}
	defer agentConn.Close(websocket.StatusNormalClosure, "")

	agent := &runtimeGatewayTestAgent{conn: agentConn, resources: []RuntimeResourceStatus{{ApplicationID: "app_1", StageKey: "dev", Kind: "Pod", Namespace: "order-dev", Name: "order-api-abc", Status: "Running"}}}
	go agent.serve(ctx)

	agent.send(t, RuntimeWireMessage{Type: "stage_changed", ApplicationID: "app_1", StageKey: "dev"})
	time.Sleep(100 * time.Millisecond)
	if got := agent.listCalls.Load(); got != 0 {
		t.Fatalf("stage_changed without subscribers should not list resources, got %d", got)
	}

	watchCtx, stopWatch := context.WithCancel(ctx)
	defer stopWatch()
	snapshots := make(chan []RuntimeResource, 4)
	done := make(chan error, 1)
	go func() {
		done <- gateway.WatchRuntimeResources(watchCtx, "cluster_1", "app_1", "dev", func(resources []RuntimeResource) error {
			snapshots <- resources
			return nil
		}, func(status string) error {
			return nil
		})
	}()
	waitRuntimeSnapshot(t, snapshots)
	if got := agent.watchCalls.Load(); got != 1 {
		t.Fatalf("first subscriber should trigger one watch_resources, got %d", got)
	}

	agent.send(t, RuntimeWireMessage{Type: "stage_changed", ApplicationID: "app_2", StageKey: "dev"})
	time.Sleep(100 * time.Millisecond)
	if got := agent.watchCalls.Load(); got != 1 {
		t.Fatalf("unsubscribed stage_changed should not start another watch, got %d", got)
	}

	agent.send(t, RuntimeWireMessage{Type: "stage_changed", ApplicationID: "app_1", StageKey: "dev"})
	time.Sleep(100 * time.Millisecond)
	if got := agent.watchCalls.Load(); got != 1 {
		t.Fatalf("subscribed stage_changed should reuse active watch, got %d", got)
	}
	stopWatch()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("watch error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("watch did not stop")
	}
	for i := 0; i < 20; i++ {
		if agent.cancelCalls.Load() == 1 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	if got := agent.cancelCalls.Load(); got != 1 {
		t.Fatalf("closing last subscriber should cancel agent watch, got %d", got)
	}
}

func waitRuntimeSnapshot(t *testing.T, snapshots <-chan []RuntimeResource) []RuntimeResource {
	t.Helper()
	select {
	case resources := <-snapshots:
		if len(resources) == 0 {
			t.Fatalf("snapshot should contain resources")
		}
		return resources
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for runtime snapshot")
		return nil
	}
}

type runtimeGatewayTestAgent struct {
	conn        *websocket.Conn
	resources   []RuntimeResourceStatus
	listCalls   atomic.Int64
	watchCalls  atomic.Int64
	cancelCalls atomic.Int64
	mu          sync.Mutex
}

func (a *runtimeGatewayTestAgent) serve(ctx context.Context) {
	for {
		_, data, err := a.conn.Read(ctx)
		if err != nil {
			return
		}
		var msg RuntimeWireMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		switch msg.Type {
		case "list_resources":
			a.listCalls.Add(1)
			a.sendMessage(ctx, RuntimeWireMessage{ID: msg.ID, Resources: a.resources, Done: true})
		case "watch_resources":
			a.watchCalls.Add(1)
			a.sendMessage(ctx, RuntimeWireMessage{ID: msg.ID, Type: "snapshot", Resources: a.resources})
		case "cancel_request":
			a.cancelCalls.Add(1)
		}
	}
}

func (a *runtimeGatewayTestAgent) send(t *testing.T, msg RuntimeWireMessage) {
	t.Helper()
	if err := a.sendMessage(context.Background(), msg); err != nil {
		t.Fatalf("send runtime message: %v", err)
	}
}

func (a *runtimeGatewayTestAgent) sendMessage(ctx context.Context, msg RuntimeWireMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.conn.Write(ctx, websocket.MessageText, data)
}
