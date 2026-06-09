package audit

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/shareinto/paas/internal/shared"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/audit/logs", h.handleList)
	mux.HandleFunc("GET /api/audit/logs/{logId}", h.handleGet)
}

func (h *Handler) handleList(w http.ResponseWriter, r *http.Request) {
	query, err := queryFromRequest(r)
	if err != nil {
		writeError(w, err)
		return
	}
	result, err := h.service.List(r.Context(), query, pageFromRequest(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request) {
	log, err := h.service.Get(r.Context(), shared.ID(r.PathValue("logId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, log)
}

func queryFromRequest(r *http.Request) (Query, error) {
	values := r.URL.Query()
	query := Query{
		TenantID:     shared.ID(values.Get("tenant_id")),
		ProjectID:    shared.ID(values.Get("project_id")),
		ActorID:      shared.ID(values.Get("actor_id")),
		ResourceType: values.Get("resource_type"),
		ResourceID:   shared.ID(values.Get("resource_id")),
		Action:       values.Get("action"),
	}
	if values.Get("from") != "" {
		from, err := time.Parse(time.RFC3339, values.Get("from"))
		if err != nil {
			return Query{}, shared.WrapError(shared.CodeInvalidArgument, "invalid from time", err)
		}
		query.From = &from
	}
	if values.Get("to") != "" {
		to, err := time.Parse(time.RFC3339, values.Get("to"))
		if err != nil {
			return Query{}, shared.WrapError(shared.CodeInvalidArgument, "invalid to time", err)
		}
		query.To = &to
	}
	return query, nil
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
