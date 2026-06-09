package notification

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/shareinto/paas/internal/shared"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/notifications", h.handleList)
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListNotifications(r.Context(), pageFromRequest(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
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
