package tenantproject

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/shared"
)

type Handler struct {
	service  *Service
	subjects SubjectQuery
}

type HandlerOptions struct {
	SubjectQuery SubjectQuery
}

func NewHandler(service *Service, opts ...HandlerOptions) *Handler {
	handler := &Handler{service: service}
	if len(opts) > 0 {
		handler.subjects = opts[0].SubjectQuery
	}
	return handler
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/tenants", h.handleListTenants)
	mux.HandleFunc("POST /api/tenants", h.handleCreateTenant)
	mux.HandleFunc("PATCH /api/tenants/{tenantId}", h.handleUpdateTenant)
	mux.HandleFunc("GET /api/tenants/{tenantId}/members", h.handleListTenantMembers)
	mux.HandleFunc("PUT /api/tenants/{tenantId}/members/{userId}", h.handleUpsertTenantMember)
	mux.HandleFunc("DELETE /api/tenants/{tenantId}/members/{userId}", h.handleRemoveTenantMember)
	mux.HandleFunc("GET /api/projects", h.handleListProjects)
	mux.HandleFunc("POST /api/projects", h.handleCreateProject)
	mux.HandleFunc("GET /api/projects/{projectId}", h.handleGetProject)
	mux.HandleFunc("PATCH /api/projects/{projectId}", h.handleUpdateProject)
	mux.HandleFunc("DELETE /api/projects/{projectId}", h.handleDeleteProject)
	mux.HandleFunc("GET /api/projects/{projectId}/members", h.handleListProjectMembers)
	mux.HandleFunc("PUT /api/projects/{projectId}/members/{userId}", h.handleUpsertProjectMember)
	mux.HandleFunc("DELETE /api/projects/{projectId}/members/{userId}", h.handleRemoveProjectMember)
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

type memberRow struct {
	UserID      shared.ID             `json:"userId"`
	Username    string                `json:"username"`
	DisplayName string                `json:"displayName"`
	Email       string                `json:"email"`
	Disabled    bool                  `json:"disabled"`
	RoleID      identityaccess.RoleID `json:"roleId"`
	CreatedAt   string                `json:"createdAt"`
	UpdatedAt   string                `json:"updatedAt"`
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

func (h *Handler) handleUpdateTenant(w http.ResponseWriter, r *http.Request) {
	var req UpdateTenantInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.TenantID = shared.ID(r.PathValue("tenantId"))
	tenant, err := h.service.UpdateTenant(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tenant)
}

func (h *Handler) handleListTenantMembers(w http.ResponseWriter, r *http.Request) {
	members, err := h.service.ListTenantMembers(r.Context(), shared.ID(r.PathValue("tenantId")))
	if err != nil {
		writeError(w, err)
		return
	}
	rows := make([]memberRow, 0, len(members))
	for _, member := range members {
		rows = append(rows, h.memberRow(r, member.UserID, member.RoleID, member.CreatedAt, member.UpdatedAt))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": rows})
}

func (h *Handler) handleUpsertTenantMember(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Actor  identityaccess.Subject `json:"actor"`
		RoleID identityaccess.RoleID  `json:"role_id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	member, err := h.service.AddTenantMember(r.Context(), AddTenantMemberInput{Actor: req.Actor, TenantID: shared.ID(r.PathValue("tenantId")), UserID: shared.ID(r.PathValue("userId")), RoleID: req.RoleID})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.memberRow(r, member.UserID, member.RoleID, member.CreatedAt, member.UpdatedAt))
}

func (h *Handler) handleRemoveTenantMember(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Actor identityaccess.Subject `json:"actor"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.service.RemoveTenantMember(r.Context(), RemoveTenantMemberInput{Actor: req.Actor, TenantID: shared.ID(r.PathValue("tenantId")), UserID: shared.ID(r.PathValue("userId"))}); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

func (h *Handler) handleListProjectMembers(w http.ResponseWriter, r *http.Request) {
	members, err := h.service.ListProjectMembers(r.Context(), shared.ID(r.PathValue("projectId")))
	if err != nil {
		writeError(w, err)
		return
	}
	rows := make([]memberRow, 0, len(members))
	for _, member := range members {
		rows = append(rows, h.memberRow(r, member.UserID, member.RoleID, member.CreatedAt, member.UpdatedAt))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": rows})
}

func (h *Handler) handleUpsertProjectMember(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Actor  identityaccess.Subject `json:"actor"`
		RoleID identityaccess.RoleID  `json:"role_id"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	member, err := h.service.UpsertProjectMember(r.Context(), UpsertProjectMemberInput{Actor: req.Actor, ProjectID: shared.ID(r.PathValue("projectId")), UserID: shared.ID(r.PathValue("userId")), RoleID: req.RoleID})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, h.memberRow(r, member.UserID, member.RoleID, member.CreatedAt, member.UpdatedAt))
}

func (h *Handler) handleRemoveProjectMember(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Actor identityaccess.Subject `json:"actor"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.service.RemoveProjectMember(r.Context(), RemoveProjectMemberInput{Actor: req.Actor, ProjectID: shared.ID(r.PathValue("projectId")), UserID: shared.ID(r.PathValue("userId"))}); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

func (h *Handler) memberRow(r *http.Request, userID shared.ID, roleID identityaccess.RoleID, createdAt time.Time, updatedAt time.Time) memberRow {
	row := memberRow{UserID: userID, Username: userID.String(), DisplayName: userID.String(), RoleID: roleID, CreatedAt: formatAPITime(createdAt), UpdatedAt: formatAPITime(updatedAt)}
	if h.subjects == nil {
		return row
	}
	user, err := h.subjects.GetUser(r.Context(), userID)
	if err != nil {
		return row
	}
	row.Username = user.Username
	row.DisplayName = normalizeDisplayName(user.DisplayName, user.Username)
	row.Email = user.Email
	row.Disabled = user.Disabled
	return row
}
