package appenv

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/shared"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/applications", h.handleCreateApplication)
	mux.HandleFunc("GET /api/applications/{applicationId}", h.handleGetApplication)
	mux.HandleFunc("PATCH /api/applications/{applicationId}", h.handleUpdateApplication)
	mux.HandleFunc("DELETE /api/applications/{applicationId}", h.handleDeleteApplication)
	mux.HandleFunc("GET /api/projects/{projectId}/applications", h.handleListApplications)
	mux.HandleFunc("GET /api/applications/{applicationId}/source", h.handleGetApplicationSource)
	mux.HandleFunc("GET /api/applications/{applicationId}/workloads", h.handleListWorkloads)
	mux.HandleFunc("POST /api/applications/{applicationId}/workloads", h.handleCreateWorkload)
	mux.HandleFunc("GET /api/applications/{applicationId}/workloads/{workloadId}", h.handleGetWorkload)
	mux.HandleFunc("PUT /api/applications/{applicationId}/workloads/{workloadId}", h.handleUpdateWorkload)
	mux.HandleFunc("DELETE /api/applications/{applicationId}/workloads/{workloadId}", h.handleDeleteWorkload)
	mux.HandleFunc("POST /api/applications/{applicationId}/workloads/{workloadAction}", h.handleWorkloadAction)
	mux.HandleFunc("GET /api/applications/{applicationId}/workloads/{workloadId}/default-config", h.handleGetWorkloadDefaultConfig)
	mux.HandleFunc("PUT /api/applications/{applicationId}/workloads/{workloadId}/default-config", h.handleSaveWorkloadDefaultConfig)
	mux.HandleFunc("GET /api/applications/{applicationId}/workloads/{workloadId}/environment-configs", h.handleListWorkloadEnvironmentConfigs)
	mux.HandleFunc("PUT /api/applications/{applicationId}/workloads/{workloadId}/environment-configs/{environmentId}", h.handleSaveWorkloadEnvironmentConfig)
	mux.HandleFunc("GET /api/applications/{applicationId}/environments", h.handleListEnvironments)
	mux.HandleFunc("GET /api/environments/{environmentId}", h.handleGetEnvironment)
	mux.HandleFunc("POST /api/environments/{environmentId}/cluster-binding", h.handleBindEnvironmentCluster)
	mux.HandleFunc("PUT /api/environments/{environmentId}/configs", h.handleSetEnvironmentConfig)
	mux.HandleFunc("PUT /api/environments/{environmentId}/secrets", h.handleSetEnvironmentSecret)
	mux.HandleFunc("GET /api/environments/{environmentId}/state", h.handleGetEnvironmentState)
	mux.HandleFunc("PUT /api/environments/{environmentId}/state", h.handleUpdateEnvironmentState)
	mux.HandleFunc("GET /api/environments/{environmentId}/events", h.handleListEnvironmentEvents)
}

func (h *Handler) handleCreateApplication(w http.ResponseWriter, r *http.Request) {
	var req CreateApplicationInput
	if !decodeJSON(w, r, &req) {
		return
	}
	app, err := h.service.CreateApplication(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, app)
}

func (h *Handler) handleGetApplication(w http.ResponseWriter, r *http.Request) {
	app, err := h.service.GetApplication(r.Context(), shared.ID(r.PathValue("applicationId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (h *Handler) handleUpdateApplication(w http.ResponseWriter, r *http.Request) {
	var req UpdateApplicationInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.ApplicationID = shared.ID(r.PathValue("applicationId"))
	app, err := h.service.UpdateApplication(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (h *Handler) handleDeleteApplication(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Actor identityaccess.Subject `json:"actor"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.service.DeleteApplication(r.Context(), req.Actor, shared.ID(r.PathValue("applicationId"))); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleListApplications(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListApplicationsByProject(r.Context(), shared.ID(r.PathValue("projectId")), pageFromQuery(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleGetApplicationSource(w http.ResponseWriter, r *http.Request) {
	sources, err := h.service.ListApplicationSources(r.Context(), shared.ID(r.PathValue("applicationId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": sources})
}

func (h *Handler) handleCreateWorkload(w http.ResponseWriter, r *http.Request) {
	var req CreateWorkloadInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.ApplicationID = shared.ID(r.PathValue("applicationId"))
	workload, err := h.service.CreateWorkload(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, workload)
}

func (h *Handler) handleUpdateWorkload(w http.ResponseWriter, r *http.Request) {
	var req UpdateWorkloadInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.ApplicationID = shared.ID(r.PathValue("applicationId"))
	req.WorkloadID = shared.ID(r.PathValue("workloadId"))
	workload, err := h.service.UpdateWorkload(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, workload)
}

func (h *Handler) handleWorkloadAction(w http.ResponseWriter, r *http.Request) {
	var req WorkloadStatusInput
	if !decodeJSON(w, r, &req) {
		return
	}
	actionValue := r.PathValue("workloadAction")
	workloadID, action, ok := strings.Cut(actionValue, ":")
	if !ok || workloadID == "" {
		writeError(w, shared.NewError(shared.CodeNotFound, "workload action not found"))
		return
	}
	req.ApplicationID = shared.ID(r.PathValue("applicationId"))
	req.WorkloadID = shared.ID(workloadID)
	var (
		workload Workload
		err      error
	)
	switch action {
	case "enable":
		workload, err = h.service.EnableWorkload(r.Context(), req)
	case "disable":
		workload, err = h.service.DisableWorkload(r.Context(), req)
	default:
		err = shared.NewError(shared.CodeNotFound, "workload action not found")
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, workload)
}

func (h *Handler) handleDeleteWorkload(w http.ResponseWriter, r *http.Request) {
	var req WorkloadStatusInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.ApplicationID = shared.ID(r.PathValue("applicationId"))
	req.WorkloadID = shared.ID(r.PathValue("workloadId"))
	workload, err := h.service.DeleteWorkload(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, workload)
}

func (h *Handler) handleGetWorkload(w http.ResponseWriter, r *http.Request) {
	workload, err := h.service.GetWorkload(r.Context(), shared.ID(r.PathValue("applicationId")), shared.ID(r.PathValue("workloadId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, workload)
}

func (h *Handler) handleListWorkloads(w http.ResponseWriter, r *http.Request) {
	applicationID := shared.ID(r.PathValue("applicationId"))
	var (
		workloads []Workload
		err       error
	)
	if r.URL.Query().Get("enabled") == "true" {
		workloads, err = h.service.ListEnabledWorkloads(r.Context(), applicationID)
	} else {
		workloads, err = h.service.ListWorkloads(r.Context(), applicationID)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": workloads})
}

func (h *Handler) handleSaveWorkloadEnvironmentConfig(w http.ResponseWriter, r *http.Request) {
	var req SaveWorkloadEnvironmentConfigInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.ApplicationID = shared.ID(r.PathValue("applicationId"))
	req.WorkloadID = shared.ID(r.PathValue("workloadId"))
	req.EnvironmentID = shared.ID(r.PathValue("environmentId"))
	config, err := h.service.SaveWorkloadEnvironmentConfig(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (h *Handler) handleSaveWorkloadDefaultConfig(w http.ResponseWriter, r *http.Request) {
	var req SaveWorkloadDefaultConfigInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.ApplicationID = shared.ID(r.PathValue("applicationId"))
	req.WorkloadID = shared.ID(r.PathValue("workloadId"))
	config, err := h.service.SaveWorkloadDefaultConfig(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (h *Handler) handleGetWorkloadDefaultConfig(w http.ResponseWriter, r *http.Request) {
	workload, err := h.service.GetWorkload(r.Context(), shared.ID(r.PathValue("applicationId")), shared.ID(r.PathValue("workloadId")))
	if err != nil {
		writeError(w, err)
		return
	}
	config, err := h.service.GetWorkloadDefaultConfig(r.Context(), workload.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (h *Handler) handleListWorkloadEnvironmentConfigs(w http.ResponseWriter, r *http.Request) {
	configs, err := h.service.ListWorkloadEnvironmentConfigsForApplication(r.Context(), shared.ID(r.PathValue("applicationId")), shared.ID(r.PathValue("workloadId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": configs})
}

func (h *Handler) handleListEnvironments(w http.ResponseWriter, r *http.Request) {
	environments, err := h.service.ListEnvironments(r.Context(), shared.ID(r.PathValue("applicationId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": environments})
}

func (h *Handler) handleGetEnvironment(w http.ResponseWriter, r *http.Request) {
	environment, err := h.service.GetEnvironment(r.Context(), shared.ID(r.PathValue("environmentId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, environment)
}

func (h *Handler) handleBindEnvironmentCluster(w http.ResponseWriter, r *http.Request) {
	var req BindEnvironmentClusterInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.EnvironmentID = shared.ID(r.PathValue("environmentId"))
	binding, err := h.service.BindEnvironmentCluster(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, binding)
}

func (h *Handler) handleSetEnvironmentConfig(w http.ResponseWriter, r *http.Request) {
	var req SetEnvironmentConfigInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.EnvironmentID = shared.ID(r.PathValue("environmentId"))
	config, err := h.service.SetEnvironmentConfig(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (h *Handler) handleSetEnvironmentSecret(w http.ResponseWriter, r *http.Request) {
	var req SetEnvironmentSecretInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.EnvironmentID = shared.ID(r.PathValue("environmentId"))
	secret, err := h.service.SetEnvironmentSecret(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, secret)
}

func (h *Handler) handleGetEnvironmentState(w http.ResponseWriter, r *http.Request) {
	state, err := h.service.GetEnvironmentState(r.Context(), shared.ID(r.PathValue("environmentId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (h *Handler) handleUpdateEnvironmentState(w http.ResponseWriter, r *http.Request) {
	var req UpdateEnvironmentStateInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.EnvironmentID = shared.ID(r.PathValue("environmentId"))
	state, err := h.service.UpdateEnvironmentState(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (h *Handler) handleListEnvironmentEvents(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListEnvironmentEvents(r.Context(), shared.ID(r.PathValue("environmentId")), pageFromQuery(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeError(w, shared.WrapError(shared.CodeInvalidArgument, "invalid json body", err))
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	message := "请求处理失败"
	var appErr *shared.AppError
	if errors.As(err, &appErr) && appErr.Message != "" {
		message = appErr.Message
	}
	writeJSON(w, shared.HTTPStatusOf(err), map[string]any{"error": map[string]any{"code": shared.CodeOf(err), "message": message}})
}

func pageFromQuery(r *http.Request) shared.PageRequest {
	query := r.URL.Query()
	return shared.PageRequest{Page: parsePositiveInt(query.Get("page")), PageSize: parsePositiveInt(query.Get("page_size"))}
}

func parsePositiveInt(value string) int {
	var result int
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0
		}
		result = result*10 + int(r-'0')
	}
	return result
}
