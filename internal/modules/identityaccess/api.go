package identityaccess

import (
	"encoding/json"
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
	mux.HandleFunc("POST /api/auth/logout", h.handleLogout)
	mux.HandleFunc("POST /api/auth/refresh", h.handleRefresh)
	mux.HandleFunc("GET /api/auth/me", h.handleMe)
	mux.HandleFunc("POST /api/auth/oidc/providers", h.handleCreateOIDCProvider)
	mux.HandleFunc("GET /api/auth/oidc/providers", h.handleOIDCProviders)
	mux.HandleFunc("GET /api/auth/oidc/{providerId}/start", h.handleOIDCStart)
	mux.HandleFunc("GET /api/auth/oidc/{providerId}/callback", h.handleOIDCCallback)
	mux.HandleFunc("POST /api/users", h.handleCreateUser)
	mux.HandleFunc("GET /api/users/{userId}", h.handleGetUser)
	mux.HandleFunc("POST /api/users/{userId}/reset-password", h.handleResetPassword)
	mux.HandleFunc("GET /api/roles", h.handleRoles)
	mux.HandleFunc("POST /api/role-bindings", h.handleRoleBinding)
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

func (h *Handler) handleRoles(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": h.service.ListRoles()})
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

func bearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
}
