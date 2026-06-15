package clusteragent

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/shared"
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
	mux.HandleFunc("GET /api/apps/{appId}/stages/{stageKey}/runtime/resources/{resourceId}", h.handleGetRuntimeResource)
	mux.HandleFunc("POST /api/apps/{appId}/stages/{stageKey}/runtime/resources/{resourceId}/restart", h.handleRestartRuntimeResource)
	mux.HandleFunc("GET /api/apps/{appId}/stages/{stageKey}/runtime/resources/{resourceId}/logs", h.handleGetPodLogs)
	mux.HandleFunc("POST /api/apps/{appId}/stages/{stageKey}/runtime/resources/{resourceId}/terminal", h.handleOpenTerminal)
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
