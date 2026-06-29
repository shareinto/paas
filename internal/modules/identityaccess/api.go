package identityaccess

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/shareinto/paas/internal/shared"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/auth/login", h.handleLogin)
	mux.HandleFunc("POST /api/auth/register", h.handleRegister)
	mux.HandleFunc("POST /api/auth/logout", h.handleLogout)
	mux.HandleFunc("POST /api/auth/refresh", h.handleRefresh)
	mux.HandleFunc("GET /api/auth/me", h.handleMe)
	mux.HandleFunc("POST /api/auth/oidc/providers", h.handleCreateOIDCProvider)
	mux.HandleFunc("GET /api/auth/oidc/providers", h.handleOIDCProviders)
	mux.HandleFunc("GET /api/auth/oidc/{providerId}/start", h.handleOIDCStart)
	mux.HandleFunc("GET /api/auth/oidc/{providerId}/callback", h.handleOIDCCallback)
	mux.HandleFunc("POST /api/users", h.handleCreateUser)
	mux.HandleFunc("GET /api/users", h.handleListUsers)
	mux.HandleFunc("GET /api/users/{userId}", h.handleGetUser)
	mux.HandleFunc("POST /api/users/{userId}/reset-password", h.handleResetPassword)
	mux.HandleFunc("GET /api/roles", h.handleRoles)
	mux.HandleFunc("POST /api/roles", h.handleCreateRole)
	mux.HandleFunc("PUT /api/roles/{roleId}", h.handleUpdateRole)
	mux.HandleFunc("DELETE /api/roles/{roleId}", h.handleDeleteRole)
	mux.HandleFunc("PATCH /api/roles/{roleId}/permissions", h.handleUpdateRolePermissions)
	mux.HandleFunc("GET /api/permissions", h.handlePermissions)
	mux.HandleFunc("GET /api/role-bindings", h.handleRoleBindings)
	mux.HandleFunc("POST /api/role-bindings", h.handleRoleBinding)
	mux.HandleFunc("PUT /api/role-bindings", h.handleReplaceRoleBinding)
	mux.HandleFunc("DELETE /api/role-bindings", h.handleDeleteRoleBinding)
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	pair, user, err := h.service.LoginLocal(r.Context(), req.Username, req.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": pair, "user": user})
}

func (h *Handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterLocalUserInput
	if !decodeJSON(w, r, &req) {
		return
	}
	pair, user, err := h.service.RegisterLocal(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"token": pair, "user": user})
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r)
	if token == "" {
		writeError(w, shared.NewError(shared.CodeUnauthenticated, "missing bearer token"))
		return
	}
	if err := h.service.Logout(r.Context(), token); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	pair, err := h.service.RefreshAccessToken(r.Context(), req.RefreshToken)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": pair})
}

func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	user, err := h.service.AuthenticateAccessToken(r.Context(), bearerToken(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ToUserDTO(user))
}

func (h *Handler) handleOIDCProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := h.service.ListOIDCProviders(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": providers})
}

func (h *Handler) handleCreateOIDCProvider(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID              shared.ID `json:"id"`
		Name            string    `json:"name"`
		Issuer          string    `json:"issuer"`
		ClientID        string    `json:"client_id"`
		ClientSecretRef string    `json:"client_secret_ref"`
		Scopes          []string  `json:"scopes"`
		RedirectURI     string    `json:"redirect_uri"`
		Enabled         bool      `json:"enabled"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	provider, err := h.service.CreateOIDCProvider(r.Context(), OIDCProvider{
		ID:              req.ID,
		Name:            req.Name,
		Issuer:          req.Issuer,
		ClientID:        req.ClientID,
		ClientSecretRef: req.ClientSecretRef,
		Scopes:          req.Scopes,
		RedirectURI:     req.RedirectURI,
		Enabled:         req.Enabled,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, provider)
}

func (h *Handler) handleOIDCStart(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.StartOIDC(r.Context(), shared.ID(r.PathValue("providerId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	pair, user, err := h.service.CallbackOIDC(r.Context(), shared.ID(r.PathValue("providerId")), r.URL.Query().Get("state"), r.URL.Query().Get("code"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": pair, "user": user})
}

func (h *Handler) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req CreateLocalUserInput
	if !decodeJSON(w, r, &req) {
		return
	}
	user, err := h.service.CreateLocalUser(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, user)
}

func (h *Handler) handleListUsers(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListUsers(r.Context(), pageFromQuery(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleGetUser(w http.ResponseWriter, r *http.Request) {
	user, err := h.service.GetUser(r.Context(), shared.ID(r.PathValue("userId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ToUserDTO(user))
}

func (h *Handler) handleResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ActorID  shared.ID `json:"actor_id"`
		Password string    `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.service.ResetPassword(r.Context(), req.ActorID, shared.ID(r.PathValue("userId")), req.Password); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.service.ListRoles(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": roles})
}

func (h *Handler) handleCreateRole(w http.ResponseWriter, r *http.Request) {
	var req CreateRoleInput
	if !decodeJSON(w, r, &req) {
		return
	}
	role, err := h.service.CreateRole(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, role)
}

func (h *Handler) handleUpdateRole(w http.ResponseWriter, r *http.Request) {
	var req UpdateRoleInput
	if !decodeJSON(w, r, &req) {
		return
	}
	role, err := h.service.UpdateRole(r.Context(), RoleID(r.PathValue("roleId")), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, role)
}

func (h *Handler) handleDeleteRole(w http.ResponseWriter, r *http.Request) {
	actorID := shared.ID(r.URL.Query().Get("actor_id"))
	if actorID.IsZero() {
		actorID = shared.ID(r.URL.Query().Get("actor"))
	}
	if err := h.service.DeleteRole(r.Context(), RoleID(r.PathValue("roleId")), Subject{Type: SubjectUser, ID: actorID}); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleUpdateRolePermissions(w http.ResponseWriter, r *http.Request) {
	var req UpdateRolePermissionsInput
	if !decodeJSON(w, r, &req) {
		return
	}
	role, err := h.service.UpdateRolePermissions(r.Context(), RoleID(r.PathValue("roleId")), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, role)
}

func (h *Handler) handlePermissions(w http.ResponseWriter, r *http.Request) {
	permissions, err := h.service.ListPermissions(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": permissions})
}

func (h *Handler) handleRoleBinding(w http.ResponseWriter, r *http.Request) {
	var req RoleBinding
	if !decodeJSON(w, r, &req) {
		return
	}
	binding, err := h.service.CreateRoleBinding(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, binding)
}

func (h *Handler) handleRoleBindings(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	subjectType := SubjectType(query.Get("subject_type"))
	subjectID := shared.ID(query.Get("subject_id"))
	scopeKind := ScopeKind(query.Get("scope_kind"))
	scopeID := shared.ID(query.Get("scope_id"))

	var (
		items []RoleBinding
		err   error
	)
	if subjectType != "" || !subjectID.IsZero() {
		items, err = h.service.ListRoleBindingsForSubject(r.Context(), Subject{Type: subjectType, ID: subjectID})
		if err == nil && scopeKind != "" {
			filtered := make([]RoleBinding, 0, len(items))
			for _, item := range items {
				if item.ScopeKind == scopeKind && item.ScopeID == scopeID {
					filtered = append(filtered, item)
				}
			}
			items = filtered
		}
	} else if scopeKind != "" {
		items, err = h.service.ListRoleBindingsByScope(r.Context(), scopeKind, scopeID)
	} else {
		err = shared.NewError(shared.CodeInvalidArgument, "subject or scope filter is required")
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (h *Handler) handleReplaceRoleBinding(w http.ResponseWriter, r *http.Request) {
	var req RoleBinding
	if !decodeJSON(w, r, &req) {
		return
	}
	binding, err := h.service.ReplaceRoleBindingForSubjectScope(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, binding)
}

func (h *Handler) handleDeleteRoleBinding(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	err := h.service.DeleteRoleBindingsForSubjectScope(
		r.Context(),
		Subject{Type: SubjectType(query.Get("subject_type")), ID: shared.ID(query.Get("subject_id"))},
		ScopeKind(query.Get("scope_kind")),
		shared.ID(query.Get("scope_id")),
	)
	if err != nil {
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
	writeJSON(w, shared.HTTPStatusOf(err), map[string]any{"error": map[string]any{"code": shared.CodeOf(err), "message": errorMessage(err)}})
}

func errorMessage(err error) string {
	var appErr *shared.AppError
	if errors.As(err, &appErr) && strings.TrimSpace(appErr.Message) != "" {
		return appErr.Message
	}
	return "请求处理失败"
}

func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
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
