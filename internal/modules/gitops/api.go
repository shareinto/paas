package gitops

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/shareinto/paas/internal/shared"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/apps/{appId}/deployments", h.handleListDeployments)
	mux.HandleFunc("GET /api/deployments/{deploymentId}", h.handleGetDeployment)
	mux.HandleFunc("POST /api/deployment-templates/platform", h.handleCreatePlatformTemplate)
	mux.HandleFunc("GET /api/apps/{appId}/deployment-template", h.handleGetApplicationTemplate)
	mux.HandleFunc("POST /api/apps/{appId}/deployment-template", h.handleCreateApplicationTemplate)
	mux.HandleFunc("POST /api/apps/{appId}/deployment-template/validate", h.handleValidateTemplate)
}

func (h *Handler) handleListDeployments(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListDeployments(r.Context(), shared.ID(r.PathValue("appId")), pageFromRequest(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleGetDeployment(w http.ResponseWriter, r *http.Request) {
	deployment, err := h.service.GetDeployment(r.Context(), shared.ID(r.PathValue("deploymentId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, deployment)
}

func (h *Handler) handleValidateTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string `json:"content"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	writeJSON(w, http.StatusOK, h.service.ValidateTemplate(r.Context(), req.Content))
}

func (h *Handler) handleCreatePlatformTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	template, err := h.service.EnsurePlatformTemplate(r.Context(), req.Name, req.Content)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, template)
}

func (h *Handler) handleGetApplicationTemplate(w http.ResponseWriter, r *http.Request) {
	template, revision, err := h.service.GetPlatformTemplateRevision(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"template": template, "current_revision": revision})
}

func (h *Handler) handleCreateApplicationTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BaseTemplateName string    `json:"base_template_name"`
		ActorID          shared.ID `json:"actor_id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	template, revision, err := h.service.GetPlatformTemplateRevision(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"template": template, "current_revision": revision})
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
