package delivery

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

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
	mux.HandleFunc("GET /api/apps/{appId}/stages", h.handleListAppStages)
	mux.HandleFunc("GET /api/freights/{freightId}", h.handleGetFreight)
	mux.HandleFunc("DELETE /api/freights/{freightId}", h.handleArchiveFreight)
	mux.HandleFunc("POST /api/freights/{freightId}/approvals", h.handleCompleteFreightApproval)
	mux.HandleFunc("GET /api/tenants/{tenantId}/delivery-flow-template", h.handleGetDeliveryFlowTemplate)
	mux.HandleFunc("PUT /api/tenants/{tenantId}/delivery-flow-template/graph", h.handleReplaceDeliveryFlowTemplateGraph)
	mux.HandleFunc("POST /api/tenants/{tenantId}/delivery-flow-template/stages", h.handleSaveDeliveryFlowTemplateStage)
	mux.HandleFunc("PATCH /api/tenants/{tenantId}/delivery-flow-template/stages/{stageKey}", h.handleSaveDeliveryFlowTemplateStage)
	mux.HandleFunc("DELETE /api/tenants/{tenantId}/delivery-flow-template/stages/{stageKey}", h.handleDeleteDeliveryFlowTemplateStage)
	mux.HandleFunc("GET /api/tenants/{tenantId}/delivery-flow-template/stages/{stageKey}/cluster-bindings", h.handleListStageClusterBindings)
	mux.HandleFunc("PUT /api/tenants/{tenantId}/delivery-flow-template/stages/{stageKey}/cluster-bindings", h.handleReplaceStageClusterBindings)
	mux.HandleFunc("GET /api/apps/{appId}/delivery/stages/{stageId}/eligible-freights", h.handleEligibleFreights)
	mux.HandleFunc("POST /api/apps/{appId}/delivery/stages/{stageId}/promotions", h.handleCreateStagePromotion)
	mux.HandleFunc("POST /api/apps/{appId}/stages/{stageKey}/verification", h.handleCompleteStageVerification)
	mux.HandleFunc("POST /api/promotions", h.handleCreatePromotion)
	mux.HandleFunc("POST /api/promotions/rollback", h.handleRollbackPromotion)
	mux.HandleFunc("GET /api/promotions/{promotionId}", h.handleGetPromotion)
	mux.HandleFunc("GET /api/apps/{appId}/promotions", h.handleListPromotions)
	mux.HandleFunc("POST /api/promotions/{promotionId}/approve", h.handleApprove)
	mux.HandleFunc("POST /api/promotions/{promotionId}/reject", h.handleReject)
	mux.HandleFunc("POST /api/promotions/{promotionId}/publish", h.handlePublish)
	mux.HandleFunc("POST /api/promotions/{promotionId}/reject-publish", h.handleRejectPendingPublish)
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
func (h *Handler) handleListAppStages(w http.ResponseWriter, r *http.Request) {
	stages, err := h.service.ListAppStages(r.Context(), shared.ID(r.PathValue("appId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": stages})
}
func (h *Handler) handleGetDeliveryFlowTemplate(w http.ResponseWriter, r *http.Request) {
	template, err := h.service.GetDeliveryFlowTemplate(r.Context(), shared.ID(r.PathValue("tenantId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, template)
}
func (h *Handler) handleReplaceDeliveryFlowTemplateGraph(w http.ResponseWriter, r *http.Request) {
	var req ReplaceDeliveryFlowTemplateGraphInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.TenantID = shared.ID(r.PathValue("tenantId"))
	template, err := h.service.ReplaceDeliveryFlowTemplateGraph(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, template)
}
func (h *Handler) handleSaveDeliveryFlowTemplateStage(w http.ResponseWriter, r *http.Request) {
	var req SaveDeliveryFlowTemplateStageInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.TenantID = shared.ID(r.PathValue("tenantId"))
	if stageKey := r.PathValue("stageKey"); stageKey != "" {
		req.StageKey = stageKey
	}
	stage, err := h.service.SaveDeliveryFlowTemplateStage(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	status := http.StatusOK
	if r.Method == http.MethodPost {
		status = http.StatusCreated
	}
	writeJSON(w, status, stage)
}
func (h *Handler) handleDeleteDeliveryFlowTemplateStage(w http.ResponseWriter, r *http.Request) {
	var req StageTemplateActionInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.TenantID = shared.ID(r.PathValue("tenantId"))
	req.StageKey = r.PathValue("stageKey")
	stage, err := h.service.DeleteDeliveryFlowTemplateStage(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stage)
}
func (h *Handler) handleReplaceStageClusterBindings(w http.ResponseWriter, r *http.Request) {
	var req ReplaceStageClusterBindingsInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.TenantID = shared.ID(r.PathValue("tenantId"))
	req.StageKey = r.PathValue("stageKey")
	bindings, err := h.service.ReplaceStageClusterBindings(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": bindings})
}
func (h *Handler) handleListStageClusterBindings(w http.ResponseWriter, r *http.Request) {
	bindings, err := h.service.ListStageClusterBindings(r.Context(), shared.ID(r.PathValue("tenantId")), r.PathValue("stageKey"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": bindings})
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
func (h *Handler) handleArchiveFreight(w http.ResponseWriter, r *http.Request) {
	var req ArchiveFreightInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.FreightID = shared.ID(r.PathValue("freightId"))
	freight, err := h.service.ArchiveFreight(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, freight)
}
func (h *Handler) handleCompleteFreightApproval(w http.ResponseWriter, r *http.Request) {
	var req FreightApprovalInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.FreightID = shared.ID(r.PathValue("freightId"))
	approval, err := h.service.CompleteFreightApproval(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, approval)
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
func (h *Handler) handleCreateStagePromotion(w http.ResponseWriter, r *http.Request) {
	var req CreatePromotionInput
	if !decodeJSON(w, r, &req) {
		return
	}
	appID := shared.ID(r.PathValue("appId"))
	stage, err := h.service.deliveryStageByIDOrKey(r.Context(), appID, shared.ID(r.PathValue("stageId")))
	if err != nil {
		writeError(w, err)
		return
	}
	req.TargetStageKey = stage.Name
	promotion, err := h.service.CreatePromotion(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, promotion)
}
func (h *Handler) handleCompleteStageVerification(w http.ResponseWriter, r *http.Request) {
	var req StageVerificationInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.ApplicationID = shared.ID(r.PathValue("appId"))
	req.StageKey = r.PathValue("stageKey")
	verification, err := h.service.CompleteStageVerification(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, verification)
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
func (h *Handler) handlePublish(w http.ResponseWriter, r *http.Request) {
	var req ApprovalInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.PromotionID = shared.ID(r.PathValue("promotionId"))
	promotion, err := h.service.PublishPromotion(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, promotion)
}
func (h *Handler) handleRejectPendingPublish(w http.ResponseWriter, r *http.Request) {
	var req ApprovalInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.PromotionID = shared.ID(r.PathValue("promotionId"))
	promotion, err := h.service.RejectPendingPromotion(r.Context(), req)
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
	writeJSON(w, shared.HTTPStatusOf(err), map[string]any{"error": map[string]any{"code": shared.CodeOf(err), "message": readableErrorMessage(err)}})
}
func readableErrorMessage(err error) string {
	var appErr *shared.AppError
	if !errors.As(err, &appErr) {
		return "请求处理失败"
	}
	switch appErr.Code {
	case shared.CodeInvalidArgument:
		return invalidArgumentMessage(appErr.Message)
	case shared.CodeNotFound:
		return notFoundMessage(appErr.Message)
	case shared.CodeConflict:
		return conflictMessage(appErr.Message)
	case shared.CodeFailedPrecondition:
		return failedPreconditionMessage(appErr.Message)
	default:
		return "请求处理失败"
	}
}
func invalidArgumentMessage(message string) string {
	switch strings.TrimSpace(message) {
	case "stage can target only one bound cluster":
		return "一个 Stage 只能绑定一个目标集群"
	case "target cluster is not bound to stage":
		return "目标集群未绑定到该 Stage"
	case "target_cluster_ids is invalid":
		return "目标集群参数无效"
	case "stage does not belong to application":
		return "目标 Stage 不属于当前应用"
	default:
		return "请求参数无效"
	}
}
func notFoundMessage(message string) string {
	switch strings.TrimSpace(message) {
	case "stage template not found":
		return "目标 Stage 模板不存在"
	case "stage binding not found":
		return "目标 Stage 集群绑定不存在"
	case "delivery stage not found":
		return "目标 Stage 不存在"
	case "freight not found":
		return "Freight 不存在"
	case "cluster not found":
		return "目标集群不存在或已被删除，请重新绑定 Stage 集群"
	case "application not found":
		return "应用不存在或已被删除"
	case "workload not found":
		return "工作负载不存在或已被删除"
	case "deployment template not found", "application template not found", "platform template not found":
		return "部署模板不存在，请先初始化应用部署模板"
	default:
		return "请求资源不存在"
	}
}
func conflictMessage(message string) string {
	switch strings.TrimSpace(message) {
	case "stage has multiple bound clusters":
		return "该 Stage 绑定了多个集群，请明确选择目标集群"
	case "target cluster is duplicated":
		return "目标集群重复"
	default:
		return "请求存在冲突"
	}
}
func failedPreconditionMessage(message string) string {
	trimmed := strings.TrimSpace(message)
	switch trimmed {
	case "stage has no bound cluster":
		return "该 Stage 未绑定集群，请先在交付流模板中绑定集群"
	case "stage is disabled":
		return "该 Stage 已禁用，不能发布"
	case "freight must include every enabled workload":
		return "该 Freight 未覆盖全部启用 Workload，请重新创建完整 Freight"
	case "freight item workload is not enabled":
		return "该 Freight 包含非启用 Workload，不能发布"
	case "freight item workload is duplicated":
		return "该 Freight 中存在重复 Workload，不能发布"
	case "freight has not passed previous stage":
		return "该 Freight 尚未通过全部上游 Stage，不能发布到目标 Stage"
	case "freight has no items":
		return "该 Freight 没有可部署镜像，不能发布"
	case "freight is archived":
		return "该 Freight 已归档，不能继续发布或审批"
	case "freight is currently used by stage":
		return "该 Freight 正在被 Stage 使用，不能归档"
	case "freight has unfinished promotion":
		return "该 Freight 存在未完成的发布晋级，不能归档"
	case "stage has no deployment record for freight":
		return "该 Stage 尚无此 Freight 的部署记录，不能验证"
	case "gitops deployment command is required":
		return "GitOps 发布能力未配置，不能执行部署"
	}
	if strings.HasPrefix(trimmed, "image bundle for workload ") {
		return "当前版本不适用于目标环境，请联系平台管理员检查运行时镜像配置"
	}
	if safeChinesePreconditionMessage(trimmed) {
		return trimmed
	}
	return "发布前置条件不满足"
}

func safeChinesePreconditionMessage(message string) bool {
	if message == "" || !containsCJK(message) {
		return false
	}
	lower := strings.ToLower(message)
	for _, sensitive := range []string{"token", "secret", "password", "passwd", "credential", "kubeconfig", "client_secret", "access_key", "private_key"} {
		if strings.Contains(lower, sensitive) {
			return false
		}
	}
	return true
}

func containsCJK(message string) bool {
	for _, r := range message {
		if r >= '\u4e00' && r <= '\u9fff' {
			return true
		}
	}
	return false
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
