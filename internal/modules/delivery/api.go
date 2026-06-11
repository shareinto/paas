package delivery

import (
	"encoding/json"
	"net/http"

	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/shared"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/delivery/build-succeeded", h.handleBuildSucceeded)
	mux.HandleFunc("GET /api/apps/{appId}/freights", h.handleListFreights)
	mux.HandleFunc("POST /api/apps/{appId}/freights", h.handleCreateFreight)
	mux.HandleFunc("GET /api/apps/{appId}/freights/creation-context", h.handleFreightCreationContext)
	mux.HandleFunc("GET /api/freights/{freightId}", h.handleGetFreight)
	mux.HandleFunc("GET /api/apps/{appId}/delivery/stages/{stageId}/eligible-freights", h.handleEligibleFreights)
	mux.HandleFunc("POST /api/promotions", h.handleCreatePromotion)
	mux.HandleFunc("POST /api/promotions/rollback", h.handleRollbackPromotion)
	mux.HandleFunc("GET /api/promotions/{promotionId}", h.handleGetPromotion)
	mux.HandleFunc("GET /api/apps/{appId}/promotions", h.handleListPromotions)
	mux.HandleFunc("POST /api/promotions/{promotionId}/approve", h.handleApprove)
	mux.HandleFunc("POST /api/promotions/{promotionId}/reject", h.handleReject)
	mux.HandleFunc("POST /api/promotions/{promotionId}/abort", h.handleAbort)
}

func (h *Handler) handleBuildSucceeded(w http.ResponseWriter, r *http.Request) {
	var req BuildSucceededPayload
	if !decodeJSON(w, r, &req) {
		return
	}
	release, err := h.service.HandleBuildSucceeded(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"release": release})
}
func (h *Handler) handleListFreights(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListFreights(r.Context(), shared.ID(r.PathValue("appId")), pageFromQuery(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *Handler) handleCreateFreight(w http.ResponseWriter, r *http.Request) {
	var req CreateFreightInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.ApplicationID = shared.ID(r.PathValue("appId"))
	freight, err := h.service.CreateFreight(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, freight)
}
func (h *Handler) handleFreightCreationContext(w http.ResponseWriter, r *http.Request) {
	context, err := h.service.GetFreightCreationContext(r.Context(), shared.ID(r.PathValue("appId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, context)
}
func (h *Handler) handleEligibleFreights(w http.ResponseWriter, r *http.Request) {
	freights, err := h.service.ListEligibleFreights(r.Context(), shared.ID(r.PathValue("appId")), shared.ID(r.PathValue("stageId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, freights)
}
func (h *Handler) handleGetFreight(w http.ResponseWriter, r *http.Request) {
	freight, err := h.service.GetFreightDetail(r.Context(), shared.ID(r.PathValue("freightId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, freight)
}
func (h *Handler) handleCreatePromotion(w http.ResponseWriter, r *http.Request) {
	var req CreatePromotionInput
	if !decodeJSON(w, r, &req) {
		return
	}
	promotion, err := h.service.CreatePromotion(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, promotion)
}
func (h *Handler) handleRollbackPromotion(w http.ResponseWriter, r *http.Request) {
	var req CreateRollbackPromotionInput
	if !decodeJSON(w, r, &req) {
		return
	}
	promotion, err := h.service.CreateRollbackPromotion(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, promotion)
}
func (h *Handler) handleGetPromotion(w http.ResponseWriter, r *http.Request) {
	promotion, err := h.service.GetPromotion(r.Context(), shared.ID(r.PathValue("promotionId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, promotion)
}
func (h *Handler) handleListPromotions(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListPromotions(r.Context(), shared.ID(r.PathValue("appId")), pageFromQuery(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}
func (h *Handler) handleApprove(w http.ResponseWriter, r *http.Request) {
	var req ApprovalInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.PromotionID = shared.ID(r.PathValue("promotionId"))
	promotion, err := h.service.ApprovePromotion(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, promotion)
}
func (h *Handler) handleReject(w http.ResponseWriter, r *http.Request) {
	var req ApprovalInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.PromotionID = shared.ID(r.PathValue("promotionId"))
	promotion, err := h.service.RejectPromotion(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, promotion)
}
func (h *Handler) handleAbort(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Actor identityaccess.Subject `json:"actor"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	promotion, err := h.service.AbortPromotion(r.Context(), req.Actor, shared.ID(r.PathValue("promotionId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, promotion)
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
	q := r.URL.Query()
	return shared.PageRequest{Page: parsePositiveInt(q.Get("page")), PageSize: parsePositiveInt(q.Get("page_size"))}
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
