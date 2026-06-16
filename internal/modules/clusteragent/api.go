package clusteragent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/shared"
	"nhooyr.io/websocket"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/clusters", h.handleRegisterCluster)
	mux.HandleFunc("GET /api/clusters", h.handleListClusters)
	mux.HandleFunc("POST /api/clusters/{clusterId}/disable", h.handleDisableCluster)
	mux.HandleFunc("POST /api/clusters/{clusterId}/drain", h.handleDrainCluster)
	mux.HandleFunc("POST /api/clusters/{clusterId}/rotate-token", h.handleRotateAgentToken)
	mux.HandleFunc("GET /api/apps/{appId}/stages/{stageKey}/runtime/resources", h.handleListRuntimeResources)
	mux.HandleFunc("GET /api/apps/{appId}/stages/{stageKey}/runtime/resources/stream", h.handleWatchRuntimeResources)
	mux.HandleFunc("GET /api/apps/{appId}/stages/{stageKey}/runtime/resources/{resourceId}", h.handleGetRuntimeResource)
	mux.HandleFunc("POST /api/apps/{appId}/stages/{stageKey}/runtime/resources/{resourceId}/restart", h.handleRestartRuntimeResource)
	mux.HandleFunc("GET /api/apps/{appId}/stages/{stageKey}/runtime/resources/{resourceId}/logs", h.handleGetPodLogs)
	mux.HandleFunc("POST /api/apps/{appId}/stages/{stageKey}/runtime/resources/{resourceId}/terminal", h.handleOpenTerminal)
	mux.HandleFunc("GET /api/apps/{appId}/stages/{stageKey}/runtime/pods/{namespace}/{pod}/logs/stream", h.handlePodLogsStream)
	mux.HandleFunc("GET /api/apps/{appId}/stages/{stageKey}/runtime/pods/{namespace}/{pod}/terminal", h.handlePodTerminal)
	mux.HandleFunc("GET /api/agent/v1/runtime/connect", h.handleRuntimeConnect)
	mux.HandleFunc("POST /api/agent/v1/heartbeat", h.handleHeartbeat)
	mux.HandleFunc("POST /api/agent/v1/status/report", h.handleStatusReport)
	mux.HandleFunc("POST /api/agent/v1/events/report", h.handleEventsReport)
	mux.HandleFunc("GET /api/agent/v1/tasks", h.handlePullTasks)
	mux.HandleFunc("POST /api/agent/v1/tasks/result", h.handleTaskResult)
}

func (h *Handler) handleRegisterCluster(w http.ResponseWriter, r *http.Request) {
	var req RegisterClusterInput
	if !decodeJSON(w, r, &req) {
		return
	}
	result, err := h.service.RegisterCluster(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) handleListClusters(w http.ResponseWriter, r *http.Request) {
	actor := identityaccess.Subject{Type: identityaccess.SubjectUser, ID: shared.ID(r.URL.Query().Get("actor_id"))}
	result, err := h.service.ListClusters(r.Context(), actor, shared.ID(r.URL.Query().Get("tenant_id")), pageFromRequest(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleDisableCluster(w http.ResponseWriter, r *http.Request) {
	actor, ok := decodeActor(w, r)
	if !ok {
		return
	}
	cluster, err := h.service.UpdateClusterStatus(r.Context(), actor, shared.ID(r.PathValue("clusterId")), ClusterDisabled)
	if err != nil {
		writeError(w, err)
		return
	}
	cluster.AgentTokenHash = ""
	writeJSON(w, http.StatusOK, cluster)
}

func (h *Handler) handleDrainCluster(w http.ResponseWriter, r *http.Request) {
	actor, ok := decodeActor(w, r)
	if !ok {
		return
	}
	cluster, err := h.service.UpdateClusterStatus(r.Context(), actor, shared.ID(r.PathValue("clusterId")), ClusterDraining)
	if err != nil {
		writeError(w, err)
		return
	}
	cluster.AgentTokenHash = ""
	writeJSON(w, http.StatusOK, cluster)
}

func (h *Handler) handleRotateAgentToken(w http.ResponseWriter, r *http.Request) {
	actor, ok := decodeActor(w, r)
	if !ok {
		return
	}
	token, err := h.service.RotateAgentToken(r.Context(), actor, shared.ID(r.PathValue("clusterId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"agent_token": token})
}

func (h *Handler) handleListRuntimeResources(w http.ResponseWriter, r *http.Request) {
	query := RuntimeResourceQuery{
		Actor:         actorFromQuery(r),
		ApplicationID: shared.ID(r.PathValue("appId")),
		StageKey:      r.PathValue("stageKey"),
	}
	resources, err := h.service.ListRuntimeResources(r.Context(), query)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": resources})
}

func (h *Handler) handleGetRuntimeResource(w http.ResponseWriter, r *http.Request) {
	query := RuntimeResourceQuery{
		Actor:         actorFromQuery(r),
		ApplicationID: shared.ID(r.PathValue("appId")),
		StageKey:      r.PathValue("stageKey"),
		ResourceID:    shared.ID(r.PathValue("resourceId")),
	}
	resource, err := h.service.GetRuntimeResource(r.Context(), query)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resource)
}

func (h *Handler) handleWatchRuntimeResources(w http.ResponseWriter, r *http.Request) {
	query := RuntimeResourceQuery{Actor: actorFromQuery(r), ApplicationID: shared.ID(r.PathValue("appId")), StageKey: r.PathValue("stageKey")}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	writer := runtimeSSEWriter{w: w, flusher: flusher}
	if err := h.service.WatchRuntimeResources(r.Context(), query, writer); err != nil {
		writeSSE(w, "error", "请求处理失败")
		if flusher != nil {
			flusher.Flush()
		}
	}
}

func (h *Handler) handleRestartRuntimeResource(w http.ResponseWriter, r *http.Request) {
	input := runtimeActionInputFromRequest(r)
	task, err := h.service.RestartRuntimeResource(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, task)
}

func (h *Handler) handleGetPodLogs(w http.ResponseWriter, r *http.Request) {
	input := runtimeActionInputFromRequest(r)
	input.Container = r.URL.Query().Get("container")
	result, err := h.service.GetPodLogs(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	status := http.StatusOK
	if !result.Supported {
		status = http.StatusNotImplemented
	}
	writeJSON(w, status, result)
}

func (h *Handler) handleOpenTerminal(w http.ResponseWriter, r *http.Request) {
	input := runtimeActionInputFromRequest(r)
	var req struct {
		Actor     identityaccess.Subject `json:"actor"`
		Container string                 `json:"container"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Actor.ID != "" {
		input.Actor = req.Actor
	}
	input.Container = req.Container
	result, err := h.service.OpenTerminal(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	status := http.StatusOK
	if !result.Supported {
		status = http.StatusNotImplemented
	}
	writeJSON(w, status, result)
}

func (h *Handler) handlePodLogsStream(w http.ResponseWriter, r *http.Request) {
	input := podActionInputFromRequest(r)
	input.Container = r.URL.Query().Get("container")
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	writer := runtimeLogSSEWriter{w: w, flusher: flusher}
	if err := h.service.StreamPodLogs(r.Context(), input, writer); err != nil {
		writeSSE(w, "error", "请求处理失败")
		if flusher != nil {
			flusher.Flush()
		}
	}
}

func (h *Handler) handlePodTerminal(w http.ResponseWriter, r *http.Request) {
	input := podActionInputFromRequest(r)
	input.Container = r.URL.Query().Get("container")
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	in := make(chan []byte, 16)
	out := make(chan []byte, 16)
	errCh := make(chan error, 2)
	go func() {
		defer close(in)
		for {
			_, data, err := conn.Read(r.Context())
			if err != nil {
				errCh <- err
				return
			}
			in <- data
		}
	}()
	go func() {
		errCh <- h.service.Terminal(r.Context(), input, in, out)
	}()
	for {
		select {
		case <-r.Context().Done():
			return
		case err := <-errCh:
			if err != nil {
				_ = conn.Write(r.Context(), websocket.MessageText, []byte("终端连接已断开"))
			}
			return
		case data, ok := <-out:
			if !ok {
				return
			}
			if err := conn.Write(r.Context(), websocket.MessageText, data); err != nil {
				return
			}
		}
	}
}

func (h *Handler) handleRuntimeConnect(w http.ResponseWriter, r *http.Request) {
	clusterID, token := agentAuth(r)
	if _, err := h.service.Authenticate(r.Context(), clusterID, token); err != nil {
		writeError(w, err)
		return
	}
	gateway, ok := h.service.runtime.(*WebSocketRuntimeGateway)
	if !ok || gateway == nil {
		writeError(w, shared.NewError(shared.CodeUnavailable, "runtime gateway is disabled"))
		return
	}
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return
	}
	_ = gateway.Connect(r.Context(), clusterID, conn)
}

func (h *Handler) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	clusterID, token := agentAuth(r)
	var req ClusterHeartbeat
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.service.Heartbeat(r.Context(), clusterID, token, req); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleStatusReport(w http.ResponseWriter, r *http.Request) {
	clusterID, token := agentAuth(r)
	var req StatusReport
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.service.ReportStatus(r.Context(), clusterID, token, req); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleEventsReport(w http.ResponseWriter, r *http.Request) {
	clusterID, token := agentAuth(r)
	var req StatusReport
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.service.ReportStatus(r.Context(), clusterID, token, req); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handlePullTasks(w http.ResponseWriter, r *http.Request) {
	clusterID, token := agentAuth(r)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	tasks, err := h.service.PullTasks(r.Context(), clusterID, token, limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": tasks})
}

func (h *Handler) handleTaskResult(w http.ResponseWriter, r *http.Request) {
	clusterID, token := agentAuth(r)
	var req ClusterTaskResult
	if !decodeJSON(w, r, &req) {
		return
	}
	task, err := h.service.CompleteTask(r.Context(), clusterID, token, req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func agentAuth(r *http.Request) (shared.ID, string) {
	clusterID := shared.ID(r.Header.Get("X-PaaS-Cluster-ID"))
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return clusterID, token
}

func decodeActor(w http.ResponseWriter, r *http.Request) (identityaccess.Subject, bool) {
	var req struct {
		Actor identityaccess.Subject `json:"actor"`
	}
	if !decodeJSON(w, r, &req) {
		return identityaccess.Subject{}, false
	}
	return req.Actor, true
}

func actorFromQuery(r *http.Request) identityaccess.Subject {
	return identityaccess.Subject{Type: identityaccess.SubjectUser, ID: shared.ID(r.URL.Query().Get("actor_id"))}
}

func runtimeActionInputFromRequest(r *http.Request) RuntimeResourceActionInput {
	return RuntimeResourceActionInput{
		Actor:         actorFromQuery(r),
		ApplicationID: shared.ID(r.PathValue("appId")),
		StageKey:      r.PathValue("stageKey"),
		ResourceID:    shared.ID(r.PathValue("resourceId")),
	}
}

func podActionInputFromRequest(r *http.Request) RuntimeResourceActionInput {
	return RuntimeResourceActionInput{
		Actor:         actorFromQuery(r),
		ApplicationID: shared.ID(r.PathValue("appId")),
		StageKey:      r.PathValue("stageKey"),
		Namespace:     r.PathValue("namespace"),
		Name:          r.PathValue("pod"),
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeError(w, shared.WrapError(shared.CodeInvalidArgument, "invalid json body", err))
		return false
	}
	return true
}

func pageFromRequest(r *http.Request) shared.PageRequest {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	return shared.PageRequest{Page: page, PageSize: pageSize}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	writeJSON(w, shared.HTTPStatusOf(err), map[string]any{"error": map[string]any{"code": shared.CodeOf(err), "message": "请求处理失败"}})
}

type runtimeSSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func (w runtimeSSEWriter) WriteRuntimeSnapshot(resources []RuntimeResource) error {
	payload, err := json.Marshal(map[string]any{"items": resources})
	if err != nil {
		return err
	}
	writeSSE(w.w, "snapshot", string(payload))
	if w.flusher != nil {
		w.flusher.Flush()
	}
	return nil
}

func (w runtimeSSEWriter) WriteRuntimeStatus(status string) error {
	writeSSE(w.w, "status", status)
	if w.flusher != nil {
		w.flusher.Flush()
	}
	return nil
}

type runtimeLogSSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

func (w runtimeLogSSEWriter) Write(data []byte) (int, error) {
	writeSSE(w.w, "log", string(data))
	if w.flusher != nil {
		w.flusher.Flush()
	}
	return len(data), nil
}

func writeSSE(w http.ResponseWriter, event string, data string) {
	_, _ = fmt.Fprintf(w, "event: %s\n", event)
	for _, line := range strings.Split(strings.ReplaceAll(data, "\r\n", "\n"), "\n") {
		_, _ = fmt.Fprintf(w, "data: %s\n", line)
	}
	_, _ = fmt.Fprint(w, "\n")
}
