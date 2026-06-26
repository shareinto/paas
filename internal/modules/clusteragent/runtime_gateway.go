package clusteragent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shareinto/paas/internal/shared"
	"nhooyr.io/websocket"
)

type RuntimeWireMessage struct {
	ID            string                  `json:"id"`
	Type          string                  `json:"type"`
	ApplicationID shared.ID               `json:"application_id,omitempty"`
	StageKey      string                  `json:"stage_key,omitempty"`
	Target        RuntimeResourceTarget   `json:"target,omitempty"`
	LogOptions    RuntimeLogOptions       `json:"log_options,omitempty"`
	TermOptions   RuntimeTerminalOptions  `json:"terminal_options,omitempty"`
	Resources     []RuntimeResourceStatus `json:"resources,omitempty"`
	Data          string                  `json:"data,omitempty"`
	Error         string                  `json:"error,omitempty"`
	Done          bool                    `json:"done,omitempty"`
}

type WebSocketRuntimeGateway struct {
	mu     sync.RWMutex
	conns  map[shared.ID]*runtimeAgentConn
	subsMu sync.Mutex
	groups map[runtimeSubscriptionKey]*runtimeSubscriptionGroup
	seq    atomic.Uint64
	subSeq atomic.Uint64

	// OnStageChanged is called when an agent reports a stage_changed event.
	// Set by the server to trigger cache invalidation and WS broadcast.
	OnStageChanged func(applicationID string, stageKey string)
}

func NewWebSocketRuntimeGateway() *WebSocketRuntimeGateway {
	return &WebSocketRuntimeGateway{conns: map[shared.ID]*runtimeAgentConn{}, groups: map[runtimeSubscriptionKey]*runtimeSubscriptionGroup{}}
}

func (g *WebSocketRuntimeGateway) Connect(ctx context.Context, clusterID shared.ID, conn *websocket.Conn) error {
	agent := newRuntimeAgentConn(clusterID, conn)
	g.mu.Lock()
	if current := g.conns[clusterID]; current != nil {
		current.close()
	}
	g.conns[clusterID] = agent
	g.mu.Unlock()
	g.refreshClusterSubscriptions(clusterID, "connected")
	defer func() {
		g.mu.Lock()
		if g.conns[clusterID] == agent {
			delete(g.conns, clusterID)
		}
		g.mu.Unlock()
		agent.close()
		g.notifyClusterSubscriptions(clusterID, "agent_offline")
	}()
	return agent.readLoop(ctx, func(msg RuntimeWireMessage) {
		if msg.Type == "stage_changed" && msg.ApplicationID != "" && strings.TrimSpace(msg.StageKey) != "" {
			g.invalidateStage(clusterID, msg.ApplicationID, msg.StageKey)
		}
	})
}

func (g *WebSocketRuntimeGateway) ListRuntimeResources(ctx context.Context, clusterID shared.ID, applicationID shared.ID, stageKey string) ([]RuntimeResource, error) {
	resp, err := g.call(ctx, clusterID, RuntimeWireMessage{Type: "list_resources", ApplicationID: applicationID, StageKey: stageKey})
	if err != nil {
		return nil, err
	}
	return runtimeResourcesFromStatuses(clusterID, resp.Resources), nil
}

func (g *WebSocketRuntimeGateway) WatchRuntimeResources(ctx context.Context, clusterID shared.ID, applicationID shared.ID, stageKey string, onSnapshot func([]RuntimeResource) error, onStatus func(string) error) error {
	key := runtimeSubscriptionKey{ClusterID: clusterID, ApplicationID: applicationID, StageKey: strings.TrimSpace(stageKey)}
	group := g.subscriptionGroup(key)
	id := g.newSubscriptionID()
	ch := make(chan runtimeSubscriptionEvent, 2)
	group.add(id, ch)
	defer func() {
		group.remove(id)
		group.stopWatchIfEmpty()
		g.removeGroupIfEmpty(group)
	}()
	if err := group.sendStatus(id, g.connectionStatus(clusterID)); err != nil {
		return err
	}
	g.ensureWatch(group)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-ch:
			if !ok {
				return nil
			}
			switch event.kind {
			case "snapshot":
				if onSnapshot != nil {
					if err := onSnapshot(event.resources); err != nil {
						return err
					}
				}
			case "status":
				if onStatus != nil {
					if err := onStatus(event.status); err != nil {
						return err
					}
				}
			}
		}
	}
}

func (g *WebSocketRuntimeGateway) RestartRuntimeResource(ctx context.Context, target RuntimeResourceTarget) error {
	_, err := g.call(ctx, target.ClusterID, RuntimeWireMessage{Type: "restart_resource", Target: target})
	return err
}

func (g *WebSocketRuntimeGateway) StreamPodLogs(ctx context.Context, target RuntimeResourceTarget, opts RuntimeLogOptions, writer io.Writer) error {
	return g.stream(ctx, target.ClusterID, RuntimeWireMessage{Type: "pod_logs", Target: target, LogOptions: opts}, func(msg RuntimeWireMessage) error {
		if msg.Data == "" {
			return nil
		}
		data, err := base64.StdEncoding.DecodeString(msg.Data)
		if err != nil {
			return err
		}
		_, err = writer.Write(data)
		return err
	})
}

func (g *WebSocketRuntimeGateway) Terminal(ctx context.Context, target RuntimeResourceTarget, opts RuntimeTerminalOptions, input <-chan []byte, output chan<- []byte) error {
	agent, err := g.agent(target.ClusterID)
	if err != nil {
		return err
	}
	id := g.newRuntimeRequestID()
	ch := make(chan RuntimeWireMessage, 32)
	agent.addPending(id, ch)
	defer agent.removePending(id)
	if err := agent.write(ctx, RuntimeWireMessage{ID: id, Type: "terminal_open", Target: target, TermOptions: opts}); err != nil {
		return err
	}
	errCh := make(chan error, 1)
	go func() {
		for data := range input {
			if len(data) == 0 {
				continue
			}
			if err := agent.write(ctx, RuntimeWireMessage{ID: id, Type: "terminal_input", Data: base64.StdEncoding.EncodeToString(data)}); err != nil {
				errCh <- err
				return
			}
		}
		_ = agent.write(ctx, RuntimeWireMessage{ID: id, Type: "terminal_close"})
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			return err
		case msg, ok := <-ch:
			if !ok {
				return shared.NewError(shared.CodeUnavailable, "agent_offline")
			}
			if msg.Error != "" {
				return shared.NewError(shared.CodeUnavailable, msg.Error)
			}
			if msg.Done {
				return nil
			}
			if msg.Data != "" {
				data, err := base64.StdEncoding.DecodeString(msg.Data)
				if err != nil {
					return err
				}
				select {
				case output <- data:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		}
	}
}

func runtimeResourcesFromStatuses(clusterID shared.ID, statuses []RuntimeResourceStatus) []RuntimeResource {
	out := make([]RuntimeResource, 0, len(statuses))
	for _, status := range statuses {
		status = normalizeRuntimeResourceStatus(status)
		out = append(out, RuntimeResource{
			ID:              stableRuntimeResourceID(clusterID, status),
			ClusterID:       clusterID,
			ApplicationID:   status.ApplicationID,
			StageKey:        status.StageKey,
			Group:           status.Group,
			Version:         status.Version,
			Kind:            status.Kind,
			Namespace:       status.Namespace,
			Name:            status.Name,
			ParentKind:      status.ParentKind,
			ParentNamespace: status.ParentNamespace,
			ParentName:      status.ParentName,
			Status:          status.Status,
			HealthStatus:    status.HealthStatus,
			Message:         status.Message,
			Desired:         status.Desired,
			Ready:           status.Ready,
			Containers:      status.Containers,
			Events:          status.Events,
		})
	}
	return out
}

func (g *WebSocketRuntimeGateway) call(ctx context.Context, clusterID shared.ID, req RuntimeWireMessage) (RuntimeWireMessage, error) {
	var out RuntimeWireMessage
	err := g.stream(ctx, clusterID, req, func(msg RuntimeWireMessage) error {
		out = msg
		return nil
	})
	return out, err
}

func (g *WebSocketRuntimeGateway) stream(ctx context.Context, clusterID shared.ID, req RuntimeWireMessage, onMessage func(RuntimeWireMessage) error) error {
	agent, err := g.agent(clusterID)
	if err != nil {
		return err
	}
	id := g.newRuntimeRequestID()
	req.ID = id
	ch := make(chan RuntimeWireMessage, 32)
	agent.addPending(id, ch)
	defer func() {
		if req.Type == "watch_resources" {
			cancelCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = agent.write(cancelCtx, RuntimeWireMessage{ID: id, Type: "cancel_request"})
			cancel()
		}
		agent.removePending(id)
	}()
	if err := agent.write(ctx, req); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return shared.NewError(shared.CodeUnavailable, "agent_offline")
			}
			if msg.Error != "" {
				return shared.NewError(shared.CodeUnavailable, msg.Error)
			}
			if err := onMessage(msg); err != nil {
				return err
			}
			if msg.Done || req.Type == "list_resources" || req.Type == "restart_resource" {
				return nil
			}
		}
	}
}

func (g *WebSocketRuntimeGateway) agent(clusterID shared.ID) (*runtimeAgentConn, error) {
	g.mu.RLock()
	agent := g.conns[clusterID]
	g.mu.RUnlock()
	if agent == nil {
		return nil, shared.NewError(shared.CodeUnavailable, "agent_offline")
	}
	return agent, nil
}

type runtimeAgentConn struct {
	clusterID shared.ID
	conn      *websocket.Conn
	mu        sync.Mutex
	pendingMu sync.Mutex
	pending   map[string]chan RuntimeWireMessage
}

func newRuntimeAgentConn(clusterID shared.ID, conn *websocket.Conn) *runtimeAgentConn {
	return &runtimeAgentConn{clusterID: clusterID, conn: conn, pending: map[string]chan RuntimeWireMessage{}}
}

func (c *runtimeAgentConn) readLoop(ctx context.Context, onUnsolicited func(RuntimeWireMessage)) error {
	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			return err
		}
		var msg RuntimeWireMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		if msg.ID == "" {
			if onUnsolicited != nil {
				onUnsolicited(msg)
			}
			continue
		}
		c.pendingMu.Lock()
		ch := c.pending[msg.ID]
		c.pendingMu.Unlock()
		if ch != nil {
			ch <- msg
		}
	}
}

func (c *runtimeAgentConn) addPending(id string, ch chan RuntimeWireMessage) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	c.pending[id] = ch
}

func (c *runtimeAgentConn) removePending(id string) {
	c.pendingMu.Lock()
	ch := c.pending[id]
	delete(c.pending, id)
	c.pendingMu.Unlock()
	if ch != nil {
		close(ch)
	}
}

func (c *runtimeAgentConn) write(ctx context.Context, msg RuntimeWireMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn.Write(ctx, websocket.MessageText, data)
}

func (c *runtimeAgentConn) close() {
	_ = c.conn.Close(websocket.StatusNormalClosure, "")
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	for id, ch := range c.pending {
		delete(c.pending, id)
		close(ch)
	}
}

func (g *WebSocketRuntimeGateway) newRuntimeRequestID() string {
	return time.Now().UTC().Format("20060102150405.000000000") + "-" + stringID(g.seq.Add(1))
}

func (g *WebSocketRuntimeGateway) newSubscriptionID() string {
	return "sub_" + stringID(g.subSeq.Add(1))
}

type runtimeSubscriptionKey struct {
	ClusterID     shared.ID
	ApplicationID shared.ID
	StageKey      string
}

type runtimeSubscriptionEvent struct {
	kind      string
	resources []RuntimeResource
	status    string
}

type runtimeSubscriptionGroup struct {
	key         runtimeSubscriptionKey
	mu          sync.Mutex
	subscribers map[string]chan runtimeSubscriptionEvent
	last        []RuntimeResource
	watching    bool
	watchCancel context.CancelFunc
}

func (g *WebSocketRuntimeGateway) subscriptionGroup(key runtimeSubscriptionKey) *runtimeSubscriptionGroup {
	g.subsMu.Lock()
	defer g.subsMu.Unlock()
	group := g.groups[key]
	if group == nil {
		group = &runtimeSubscriptionGroup{key: key, subscribers: map[string]chan runtimeSubscriptionEvent{}}
		g.groups[key] = group
	}
	return group
}

func (g *WebSocketRuntimeGateway) groupIfActive(key runtimeSubscriptionKey) *runtimeSubscriptionGroup {
	g.subsMu.Lock()
	group := g.groups[key]
	g.subsMu.Unlock()
	if group == nil || !group.hasSubscribers() {
		return nil
	}
	return group
}

func (g *WebSocketRuntimeGateway) invalidateStage(clusterID shared.ID, applicationID shared.ID, stageKey string) {
	sk := strings.TrimSpace(stageKey)
	if g.OnStageChanged != nil {
		g.OnStageChanged(string(applicationID), sk)
	}
	key := runtimeSubscriptionKey{ClusterID: clusterID, ApplicationID: applicationID, StageKey: sk}
	group := g.groupIfActive(key)
	if group == nil {
		return
	}
	g.ensureWatch(group)
}

func (g *WebSocketRuntimeGateway) ensureWatch(group *runtimeSubscriptionGroup) {
	watchCtx, cancel := context.WithCancel(context.Background())
	if !group.startWatch(cancel) {
		cancel()
		return
	}
	go func() {
		defer group.finishWatch()
		group.broadcastStatus("connected")
		err := g.stream(watchCtx, group.key.ClusterID, RuntimeWireMessage{Type: "watch_resources", ApplicationID: group.key.ApplicationID, StageKey: group.key.StageKey}, func(msg RuntimeWireMessage) error {
			if msg.Type == "snapshot" || len(msg.Resources) > 0 {
				group.broadcastStatus("connected")
				group.broadcastSnapshot(runtimeResourcesFromStatuses(group.key.ClusterID, msg.Resources))
			}
			return nil
		})
		if err != nil && group.hasSubscribers() {
			if shared.CodeOf(err) == shared.CodeUnavailable {
				group.broadcastStatus("agent_offline")
			} else {
				group.broadcastStatus("error")
			}
		}
	}()
}

func (g *WebSocketRuntimeGateway) refreshClusterSubscriptions(clusterID shared.ID, status string) {
	for _, group := range g.groupsForCluster(clusterID) {
		group.broadcastStatus(status)
		if group.hasSubscribers() {
			g.ensureWatch(group)
		}
	}
}

func (g *WebSocketRuntimeGateway) notifyClusterSubscriptions(clusterID shared.ID, status string) {
	for _, group := range g.groupsForCluster(clusterID) {
		group.broadcastStatus(status)
	}
}

func (g *WebSocketRuntimeGateway) groupsForCluster(clusterID shared.ID) []*runtimeSubscriptionGroup {
	g.subsMu.Lock()
	defer g.subsMu.Unlock()
	out := make([]*runtimeSubscriptionGroup, 0)
	for key, group := range g.groups {
		if key.ClusterID == clusterID && group.hasSubscribers() {
			out = append(out, group)
		}
	}
	return out
}

func (g *WebSocketRuntimeGateway) connectionStatus(clusterID shared.ID) string {
	if _, err := g.agent(clusterID); err != nil {
		return "agent_offline"
	}
	return "connected"
}

func (g *WebSocketRuntimeGateway) removeGroupIfEmpty(group *runtimeSubscriptionGroup) {
	if group.hasSubscribers() {
		return
	}
	g.subsMu.Lock()
	if current := g.groups[group.key]; current == group && !group.hasSubscribers() {
		delete(g.groups, group.key)
	}
	g.subsMu.Unlock()
}

func (group *runtimeSubscriptionGroup) add(id string, ch chan runtimeSubscriptionEvent) {
	group.mu.Lock()
	defer group.mu.Unlock()
	group.subscribers[id] = ch
	if group.last != nil {
		sendLatest(ch, runtimeSubscriptionEvent{kind: "snapshot", resources: cloneRuntimeResources(group.last)})
	}
}

func (group *runtimeSubscriptionGroup) remove(id string) {
	group.mu.Lock()
	ch := group.subscribers[id]
	delete(group.subscribers, id)
	group.mu.Unlock()
	if ch != nil {
		close(ch)
	}
}

func (group *runtimeSubscriptionGroup) startWatch(cancel context.CancelFunc) bool {
	group.mu.Lock()
	defer group.mu.Unlock()
	if group.watching {
		return false
	}
	group.watching = true
	group.watchCancel = cancel
	return true
}

func (group *runtimeSubscriptionGroup) finishWatch() {
	group.mu.Lock()
	group.watching = false
	group.watchCancel = nil
	group.mu.Unlock()
}

func (group *runtimeSubscriptionGroup) stopWatchIfEmpty() {
	group.mu.Lock()
	if len(group.subscribers) > 0 {
		group.mu.Unlock()
		return
	}
	cancel := group.watchCancel
	group.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (group *runtimeSubscriptionGroup) hasSubscribers() bool {
	group.mu.Lock()
	defer group.mu.Unlock()
	return len(group.subscribers) > 0
}

func (group *runtimeSubscriptionGroup) sendStatus(id string, status string) error {
	group.mu.Lock()
	ch := group.subscribers[id]
	group.mu.Unlock()
	if ch == nil || status == "" {
		return nil
	}
	sendLatest(ch, runtimeSubscriptionEvent{kind: "status", status: status})
	return nil
}

func (group *runtimeSubscriptionGroup) broadcastStatus(status string) {
	if status == "" {
		return
	}
	group.broadcast(runtimeSubscriptionEvent{kind: "status", status: status})
}

func (group *runtimeSubscriptionGroup) broadcastSnapshot(resources []RuntimeResource) {
	group.mu.Lock()
	group.last = cloneRuntimeResources(resources)
	subscribers := make([]chan runtimeSubscriptionEvent, 0, len(group.subscribers))
	for _, ch := range group.subscribers {
		subscribers = append(subscribers, ch)
	}
	group.mu.Unlock()
	event := runtimeSubscriptionEvent{kind: "snapshot", resources: cloneRuntimeResources(resources)}
	for _, ch := range subscribers {
		sendLatest(ch, event)
	}
}

func (group *runtimeSubscriptionGroup) broadcast(event runtimeSubscriptionEvent) {
	group.mu.Lock()
	subscribers := make([]chan runtimeSubscriptionEvent, 0, len(group.subscribers))
	for _, ch := range group.subscribers {
		subscribers = append(subscribers, ch)
	}
	group.mu.Unlock()
	for _, ch := range subscribers {
		sendLatest(ch, event)
	}
}

func sendLatest(ch chan runtimeSubscriptionEvent, event runtimeSubscriptionEvent) {
	select {
	case ch <- event:
		return
	default:
	}
	select {
	case <-ch:
	default:
	}
	select {
	case ch <- event:
	default:
	}
}

func cloneRuntimeResources(resources []RuntimeResource) []RuntimeResource {
	if resources == nil {
		return nil
	}
	out := make([]RuntimeResource, len(resources))
	copy(out, resources)
	return out
}

func stringID(v uint64) string {
	return fmt.Sprintf("runtime_%d", v)
}
