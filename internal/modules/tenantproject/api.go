package tenantproject

import (
	"encoding/json"
	"net/http"

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
	mux.HandleFunc("GET /api/tenants", h.handleListTenants)
	mux.HandleFunc("POST /api/tenants", h.handleCreateTenant)
	mux.HandleFunc("GET /api/projects", h.handleListProjects)
	mux.HandleFunc("POST /api/projects", h.handleCreateProject)
	mux.HandleFunc("GET /api/projects/{projectId}", h.handleGetProject)
	mux.HandleFunc("PATCH /api/projects/{projectId}", h.handleUpdateProject)
	mux.HandleFunc("DELETE /api/projects/{projectId}", h.handleDeleteProject)
}

type tenantRow struct {
	ID          shared.ID `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"displayName"`
	Description string    `json:"description"`
	UpdatedAt   string    `json:"updatedAt"`
}

type projectRow struct {
	ID          shared.ID `json:"id"`
	TenantID    shared.ID `json:"tenantId"`
	Name        string    `json:"name"`
	DisplayName string    `json:"displayName"`
	Description string    `json:"description"`
	Tenant      string    `json:"tenant"`
	Owner       string    `json:"owner"`
	UpdatedAt   string    `json:"updatedAt"`
}

func (h *Handler) handleListTenants(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListTenants(r.Context(), pageFromQuery(r))
	if err != nil {
		writeError(w, err)
		return
	}
	rows := make([]tenantRow, 0, len(result.Items))
	for _, tenant := range result.Items {
		rows = append(rows, tenantRow{ID: tenant.ID, Name: tenant.Name, DisplayName: tenant.DisplayName, Description: tenant.Description, UpdatedAt: formatAPITime(tenant.UpdatedAt)})
	}
	writeJSON(w, http.StatusOK, shared.NewPageResult(rows, result.Total, pageFromQuery(r)))
}

func (h *Handler) handleCreateTenant(w http.ResponseWriter, r *http.Request) {
	var req CreateTenantInput
	if !decodeJSON(w, r, &req) {
		return
	}
	tenant, err := h.service.CreateTenant(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, tenant)
}

func (h *Handler) handleListProjects(w http.ResponseWriter, r *http.Request) {
	tenants, err := h.service.ListTenants(r.Context(), shared.PageRequest{Page: 1, PageSize: 100})
	if err != nil {
		writeError(w, err)
		return
	}
	rows := make([]projectRow, 0)
	for _, tenant := range tenants.Items {
		if tenantID := shared.ID(r.URL.Query().Get("tenant_id")); !tenantID.IsZero() && tenantID != tenant.ID {
			continue
		}
		projects, err := h.service.ListProjectsByTenant(r.Context(), tenant.ID, shared.PageRequest{Page: 1, PageSize: 100})
		if err != nil {
			writeError(w, err)
			return
		}
		for _, project := range projects.Items {
			rows = append(rows, projectRow{ID: project.ID, TenantID: project.TenantID, Name: project.Name, DisplayName: project.DisplayName, Description: project.Description, Tenant: tenant.DisplayName, Owner: "平台管理员", UpdatedAt: formatAPITime(project.UpdatedAt)})
		}
	}
	writeJSON(w, http.StatusOK, shared.NewPageResult(rows, int64(len(rows)), pageFromQuery(r)))
}

func (h *Handler) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var req CreateProjectInput
	if !decodeJSON(w, r, &req) {
		return
	}
	project, err := h.service.CreateProject(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, project)
}

func (h *Handler) handleGetProject(w http.ResponseWriter, r *http.Request) {
	project, err := h.service.GetProject(r.Context(), shared.ID(r.PathValue("projectId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (h *Handler) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	var req UpdateProjectInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.ProjectID = shared.ID(r.PathValue("projectId"))
	project, err := h.service.UpdateProject(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (h *Handler) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Actor identityaccess.Subject `json:"actor"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.service.DeleteProject(r.Context(), DeleteProjectInput{Actor: req.Actor, ProjectID: shared.ID(r.PathValue("projectId"))}); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	writeJSON(w, shared.HTTPStatusOf(err), map[string]any{"error": map[string]any{"code": shared.CodeOf(err), "message": "请求处理失败"}})
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

func formatAPITime(value interface{ Format(string) string }) string {
	return value.Format("2006-01-02 15:04")
}
