package main

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/shareinto/paas/internal/modules/appenv"
	"github.com/shareinto/paas/internal/modules/build"
	"github.com/shareinto/paas/internal/modules/clusteragent"
	"github.com/shareinto/paas/internal/modules/delivery"
	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/shared"
)

type consoleV2Handler struct {
	apps     *appenv.Service
	builds   *build.Service
	delivery *delivery.Service
	runtime  *clusteragent.Service
}

type consoleV2Error struct {
	Scope   string `json:"scope"`
	Key     string `json:"key,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type deploymentWorkspaceResponse struct {
	Stages                  []delivery.AppStage                         `json:"stages"`
	Freights                shared.PageResult[delivery.Freight]         `json:"freights"`
	FreightDetails          map[shared.ID]delivery.FreightDetail        `json:"freight_details"`
	EligibleFreightsByStage map[string][]delivery.Freight               `json:"eligible_freights_by_stage"`
	RuntimeResourcesByStage map[string][]clusteragent.RuntimeResource   `json:"runtime_resources_by_stage"`
	ApprovalGates           []consoleV2ApprovalGate                     `json:"approval_gates"`
	PublishGates            []consoleV2PublishGate                      `json:"publish_gates"`
	BuildPipelines          shared.PageResult[build.BuildPipeline]      `json:"build_pipelines"`
	BuildRuns               shared.PageResult[build.BuildRun]           `json:"build_runs"`
	PipelineSources         map[shared.ID][]build.BuildPipelineSource   `json:"pipeline_sources"`
	RuntimeEnvironments     shared.PageResult[build.RuntimeEnvironment] `json:"runtime_environments"`
	BuildEnvironments       shared.PageResult[build.BuildEnvironment]   `json:"build_environments"`
	Workloads               []appenv.Workload                           `json:"workloads"`
	WorkloadDefaultConfigs  map[shared.ID]appenv.WorkloadStageConfig    `json:"workload_default_configs"`
	Errors                  []consoleV2Error                            `json:"errors,omitempty"`
}

type consoleV2ApprovalGate struct {
	TargetStageKey         string `json:"target_stage_key"`
	TargetStageName        string `json:"target_stage_name"`
	PendingCount           int    `json:"pending_count"`
	CanReview              bool   `json:"can_review"`
	RequiredPermissionCode string `json:"required_permission_code"`
}

type consoleV2PublishGate struct {
	TargetStageKey  string `json:"target_stage_key"`
	TargetStageName string `json:"target_stage_name"`
	PendingCount    int    `json:"pending_count"`
	CanPublish      bool   `json:"can_publish"`
}

type consoleV2ApprovalTask struct {
	ID              shared.ID `json:"id"`
	PromotionID     shared.ID `json:"promotion_id"`
	FreightID       shared.ID `json:"freight_id"`
	FreightName     string    `json:"freight_name"`
	TargetStageKey  string    `json:"target_stage_key"`
	TargetStageName string    `json:"target_stage_name"`
	RequestedBy     shared.ID `json:"requested_by"`
	RequestedAt     string    `json:"requested_at"`
	Message         string    `json:"message"`
	DiffType        string    `json:"diff_type"`
}

type consoleV2ApprovalTaskListResponse struct {
	Items []consoleV2ApprovalTask `json:"items"`
}

type consoleV2ApprovalDetail struct {
	Task           consoleV2ApprovalTask         `json:"task"`
	DiffType       string                        `json:"diff_type"`
	CurrentFreight *consoleV2FreightSummary      `json:"current_freight,omitempty"`
	PendingFreight consoleV2FreightSummary       `json:"pending_freight"`
	ImageChanges   []consoleV2ImageChange        `json:"image_changes"`
	ConfigDiff     string                        `json:"config_diff"`
	DeployItems    []consoleV2DeploySnapshotItem `json:"deploy_items"`
}

type consoleV2FreightSummary struct {
	ID        shared.ID `json:"id"`
	Name      string    `json:"name"`
	CreatedAt string    `json:"created_at"`
}

type consoleV2ImageChange struct {
	WorkloadID     shared.ID `json:"workload_id"`
	WorkloadName   string    `json:"workload_name"`
	ContainerName  string    `json:"container_name"`
	CurrentImage   string    `json:"current_image"`
	PendingImage   string    `json:"pending_image"`
	CurrentVersion string    `json:"current_version"`
	PendingVersion string    `json:"pending_version"`
}

type consoleV2DeploySnapshotItem struct {
	WorkloadID    shared.ID `json:"workload_id"`
	WorkloadName  string    `json:"workload_name"`
	ContainerName string    `json:"container_name"`
	Version       string    `json:"version"`
	Image         string    `json:"image"`
}

type consoleV2ApprovalDecisionInput struct {
	Actor   identityaccess.Subject `json:"actor"`
	Comment string                 `json:"comment"`
}

func newConsoleV2Handler(apps *appenv.Service, builds *build.Service, delivery *delivery.Service, runtime *clusteragent.Service) *consoleV2Handler {
	return &consoleV2Handler{apps: apps, builds: builds, delivery: delivery, runtime: runtime}
}

func (h *consoleV2Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/console-v2/apps/{appId}/deployment-workspace", h.handleDeploymentWorkspace)
	mux.HandleFunc("GET /api/console-v2/apps/{appId}/stages/{stageKey}/approval-tasks", h.handleListApprovalTasks)
	mux.HandleFunc("GET /api/console-v2/apps/{appId}/stages/{stageKey}/publish-tasks", h.handleListPublishTasks)
	mux.HandleFunc("GET /api/console-v2/approval-tasks/{taskId}", h.handleGetApprovalTask)
	mux.HandleFunc("GET /api/console-v2/publish-tasks/{taskId}", h.handleGetPublishTask)
	mux.HandleFunc("POST /api/console-v2/approval-tasks/{taskId}/approve", h.handleApproveApprovalTask)
	mux.HandleFunc("POST /api/console-v2/approval-tasks/{taskId}/reject", h.handleRejectApprovalTask)
	mux.HandleFunc("POST /api/console-v2/publish-tasks/{taskId}/publish", h.handlePublishTask)
	mux.HandleFunc("POST /api/console-v2/publish-tasks/{taskId}/reject", h.handleRejectPublishTask)
}

func (h *consoleV2Handler) handleDeploymentWorkspace(w http.ResponseWriter, r *http.Request) {
	appID := shared.ID(r.PathValue("appId"))
	page := shared.PageRequest{Page: 1, PageSize: 100}.Normalize()
	ctx := r.Context()

	stages, err := h.delivery.ListAppStages(ctx, appID)
	if err != nil {
		writeDevelopmentError(w, err)
		return
	}
	freights, err := h.delivery.ListFreights(ctx, appID, page)
	if err != nil {
		writeDevelopmentError(w, err)
		return
	}
	pipelines, err := h.builds.ListBuildPipelines(ctx, appID, page)
	if err != nil {
		writeDevelopmentError(w, err)
		return
	}
	buildRuns, err := h.builds.ListBuildRuns(ctx, appID, page)
	if err != nil {
		writeDevelopmentError(w, err)
		return
	}
	runtimeEnvironments, err := h.builds.ListRuntimeEnvironments(ctx, false, page)
	if err != nil {
		writeDevelopmentError(w, err)
		return
	}
	buildEnvironments, err := h.builds.ListBuildEnvironments(ctx, false, page)
	if err != nil {
		writeDevelopmentError(w, err)
		return
	}
	workloads, err := h.apps.ListEnabledWorkloads(ctx, appID)
	if err != nil {
		writeDevelopmentError(w, err)
		return
	}

	out := deploymentWorkspaceResponse{
		Stages:                  stages,
		Freights:                freights,
		FreightDetails:          map[shared.ID]delivery.FreightDetail{},
		EligibleFreightsByStage: map[string][]delivery.Freight{},
		RuntimeResourcesByStage: map[string][]clusteragent.RuntimeResource{},
		BuildPipelines:          pipelines,
		BuildRuns:               buildRuns,
		PipelineSources:         map[shared.ID][]build.BuildPipelineSource{},
		RuntimeEnvironments:     runtimeEnvironments,
		BuildEnvironments:       buildEnvironments,
		Workloads:               workloads,
		WorkloadDefaultConfigs:  map[shared.ID]appenv.WorkloadStageConfig{},
	}

	out.ApprovalGates = h.approvalGates(ctx, appID, stages)
	out.PublishGates = h.publishGates(ctx, appID, stages)

	for _, freight := range freights.Items {
		detail, err := h.delivery.GetFreightDetail(ctx, freight.ID)
		if err != nil {
			out.Errors = append(out.Errors, consoleV2Err("freight_detail", freight.ID.String(), err))
			continue
		}
		out.FreightDetails[freight.ID] = detail
	}

	actor := identityaccess.Subject{Type: identityaccess.SubjectUser, ID: shared.ID(firstNonEmpty(r.URL.Query().Get("actor_id"), "usr_admin"))}
	for _, stage := range stages {
		stageKey := stageKeyForConsoleV2(stage)
		eligible, err := h.delivery.ListEligibleFreights(ctx, appID, shared.ID(stageKey))
		if err != nil {
			out.Errors = append(out.Errors, consoleV2Err("eligible_freights", stageKey, err))
		} else {
			out.EligibleFreightsByStage[stageKey] = eligible
		}
		if stage.BoundClusterID.IsZero() {
			out.RuntimeResourcesByStage[stageKey] = []clusteragent.RuntimeResource{}
			continue
		}
		resources, err := h.runtime.ListRuntimeResources(ctx, clusteragent.RuntimeResourceQuery{Actor: actor, ApplicationID: appID, StageKey: stageKey})
		if err != nil {
			out.Errors = append(out.Errors, consoleV2Err("runtime_resources", stageKey, err))
		} else {
			out.RuntimeResourcesByStage[stageKey] = resources
		}
	}

	for _, pipeline := range pipelines.Items {
		sources, err := h.builds.ListBuildPipelineSources(ctx, pipeline.ID)
		if err != nil {
			out.Errors = append(out.Errors, consoleV2Err("pipeline_sources", pipeline.ID.String(), err))
			continue
		}
		out.PipelineSources[pipeline.ID] = sources
	}

	for _, workload := range workloads {
		config, err := h.apps.GetWorkloadDefaultConfig(ctx, workload.ID)
		if err != nil {
			if shared.CodeOf(err) == shared.CodeNotFound {
				continue
			}
			out.Errors = append(out.Errors, consoleV2Err("workload_default_config", workload.ID.String(), err))
			continue
		}
		out.WorkloadDefaultConfigs[workload.ID] = config
	}

	writeDevelopmentJSON(w, http.StatusOK, out)
}

func (h *consoleV2Handler) handleListApprovalTasks(w http.ResponseWriter, r *http.Request) {
	appID := shared.ID(r.PathValue("appId"))
	stageKey := r.PathValue("stageKey")
	items, err := h.approvalTasks(r.Context(), appID, stageKey)
	if err != nil {
		writeDevelopmentError(w, err)
		return
	}
	writeDevelopmentJSON(w, http.StatusOK, consoleV2ApprovalTaskListResponse{Items: items})
}

func (h *consoleV2Handler) handleListPublishTasks(w http.ResponseWriter, r *http.Request) {
	appID := shared.ID(r.PathValue("appId"))
	stageKey := r.PathValue("stageKey")
	items, err := h.publishTasks(r.Context(), appID, stageKey)
	if err != nil {
		writeDevelopmentError(w, err)
		return
	}
	writeDevelopmentJSON(w, http.StatusOK, consoleV2ApprovalTaskListResponse{Items: items})
}

func (h *consoleV2Handler) handleGetApprovalTask(w http.ResponseWriter, r *http.Request) {
	detail, err := h.approvalTaskDetail(r.Context(), shared.ID(r.PathValue("taskId")))
	if err != nil {
		writeDevelopmentError(w, err)
		return
	}
	writeDevelopmentJSON(w, http.StatusOK, detail)
}

func (h *consoleV2Handler) handleGetPublishTask(w http.ResponseWriter, r *http.Request) {
	detail, err := h.approvalTaskDetail(r.Context(), shared.ID(r.PathValue("taskId")))
	if err != nil {
		writeDevelopmentError(w, err)
		return
	}
	writeDevelopmentJSON(w, http.StatusOK, detail)
}

func (h *consoleV2Handler) handleApproveApprovalTask(w http.ResponseWriter, r *http.Request) {
	var input consoleV2ApprovalDecisionInput
	if !decodeDevelopmentJSON(w, r, &input) {
		return
	}
	if input.Actor.ID.IsZero() {
		input.Actor = identityaccess.Subject{Type: identityaccess.SubjectUser, ID: shared.ID(firstNonEmpty(r.URL.Query().Get("actor_id"), "usr_admin"))}
	}
	promotion, err := h.delivery.ApprovePromotion(r.Context(), delivery.ApprovalInput{Actor: input.Actor, PromotionID: shared.ID(r.PathValue("taskId")), Comment: input.Comment})
	if err != nil {
		writeDevelopmentError(w, err)
		return
	}
	writeDevelopmentJSON(w, http.StatusOK, promotion)
}

func (h *consoleV2Handler) handleRejectApprovalTask(w http.ResponseWriter, r *http.Request) {
	var input consoleV2ApprovalDecisionInput
	if !decodeDevelopmentJSON(w, r, &input) {
		return
	}
	if input.Actor.ID.IsZero() {
		input.Actor = identityaccess.Subject{Type: identityaccess.SubjectUser, ID: shared.ID(firstNonEmpty(r.URL.Query().Get("actor_id"), "usr_admin"))}
	}
	promotion, err := h.delivery.RejectPromotion(r.Context(), delivery.ApprovalInput{Actor: input.Actor, PromotionID: shared.ID(r.PathValue("taskId")), Comment: input.Comment})
	if err != nil {
		writeDevelopmentError(w, err)
		return
	}
	writeDevelopmentJSON(w, http.StatusOK, promotion)
}

func (h *consoleV2Handler) handlePublishTask(w http.ResponseWriter, r *http.Request) {
	var input consoleV2ApprovalDecisionInput
	if !decodeDevelopmentJSON(w, r, &input) {
		return
	}
	if input.Actor.ID.IsZero() {
		input.Actor = identityaccess.Subject{Type: identityaccess.SubjectUser, ID: shared.ID(firstNonEmpty(r.URL.Query().Get("actor_id"), "usr_admin"))}
	}
	promotion, err := h.delivery.PublishPromotion(r.Context(), delivery.ApprovalInput{Actor: input.Actor, PromotionID: shared.ID(r.PathValue("taskId")), Comment: input.Comment})
	if err != nil {
		writeDevelopmentError(w, err)
		return
	}
	writeDevelopmentJSON(w, http.StatusOK, promotion)
}

func (h *consoleV2Handler) handleRejectPublishTask(w http.ResponseWriter, r *http.Request) {
	var input consoleV2ApprovalDecisionInput
	if !decodeDevelopmentJSON(w, r, &input) {
		return
	}
	if input.Actor.ID.IsZero() {
		input.Actor = identityaccess.Subject{Type: identityaccess.SubjectUser, ID: shared.ID(firstNonEmpty(r.URL.Query().Get("actor_id"), "usr_admin"))}
	}
	promotion, err := h.delivery.RejectPendingPromotion(r.Context(), delivery.ApprovalInput{Actor: input.Actor, PromotionID: shared.ID(r.PathValue("taskId")), Comment: input.Comment})
	if err != nil {
		writeDevelopmentError(w, err)
		return
	}
	writeDevelopmentJSON(w, http.StatusOK, promotion)
}

func stageKeyForConsoleV2(stage delivery.AppStage) string {
	if stage.StageKey != "" {
		return stage.StageKey
	}
	return stage.DeliveryStageID.String()
}

func consoleV2Err(scope string, key string, err error) consoleV2Error {
	return consoleV2Error{Scope: scope, Key: key, Code: string(shared.CodeOf(err)), Message: developmentErrorMessage(err)}
}

func (h *consoleV2Handler) approvalGates(ctx context.Context, appID shared.ID, stages []delivery.AppStage) []consoleV2ApprovalGate {
	pending := h.pendingPromotionCountByStage(ctx, appID)
	out := make([]consoleV2ApprovalGate, 0, len(stages))
	for _, stage := range stages {
		stageKey := stageKeyForConsoleV2(stage)
		if stageKey == "" || !stage.RequiresApproval {
			continue
		}
		out = append(out, consoleV2ApprovalGate{
			TargetStageKey:         stageKey,
			TargetStageName:        firstNonEmpty(stage.DisplayName, stageKey),
			PendingCount:           pending[stageKey],
			CanReview:              true,
			RequiredPermissionCode: "deployment:approve",
		})
	}
	return out
}

func (h *consoleV2Handler) pendingPromotionCountByStage(ctx context.Context, appID shared.ID) map[string]int {
	out := map[string]int{}
	promotions, err := h.delivery.ListPromotions(ctx, appID, shared.PageRequest{Page: 1, PageSize: 1000})
	if err != nil {
		return out
	}
	for _, promotion := range promotions.Items {
		if promotion.Status != delivery.PromotionPendingApproval {
			continue
		}
		stageKey := strings.TrimSpace(promotion.TargetStageKey)
		if stageKey == "" {
			continue
		}
		out[stageKey]++
	}
	return out
}

func (h *consoleV2Handler) publishGates(ctx context.Context, appID shared.ID, stages []delivery.AppStage) []consoleV2PublishGate {
	pending := h.pendingPublishCountByStage(ctx, appID)
	out := make([]consoleV2PublishGate, 0, len(stages))
	for _, stage := range stages {
		stageKey := stageKeyForConsoleV2(stage)
		if stageKey == "" {
			continue
		}
		out = append(out, consoleV2PublishGate{
			TargetStageKey:  stageKey,
			TargetStageName: firstNonEmpty(stage.DisplayName, stageKey),
			PendingCount:    pending[stageKey],
			CanPublish:      true,
		})
	}
	return out
}

func (h *consoleV2Handler) pendingPublishCountByStage(ctx context.Context, appID shared.ID) map[string]int {
	out := map[string]int{}
	promotions, err := h.delivery.ListPromotions(ctx, appID, shared.PageRequest{Page: 1, PageSize: 1000})
	if err != nil {
		return out
	}
	for _, promotion := range promotions.Items {
		if !pendingPublishPromotion(promotion) {
			continue
		}
		stageKey := strings.TrimSpace(promotion.TargetStageKey)
		if stageKey == "" {
			continue
		}
		out[stageKey]++
	}
	return out
}

func pendingPublishPromotion(promotion delivery.Promotion) bool {
	if promotion.AutoPublish {
		return false
	}
	return promotion.Status == delivery.PromotionCreated || promotion.Status == delivery.PromotionApproved
}

func (h *consoleV2Handler) approvalTasks(ctx context.Context, appID shared.ID, stageKey string) ([]consoleV2ApprovalTask, error) {
	stages, err := h.delivery.ListAppStages(ctx, appID)
	if err != nil {
		return nil, err
	}
	stageName := stageDisplayName(stages, stageKey)
	currentFreightID := currentFreightForStage(stages, stageKey)
	promotions, err := h.delivery.ListPromotions(ctx, appID, shared.PageRequest{Page: 1, PageSize: 1000})
	if err != nil {
		return nil, err
	}
	out := make([]consoleV2ApprovalTask, 0)
	for _, promotion := range promotions.Items {
		if promotion.Status != delivery.PromotionPendingApproval || strings.TrimSpace(promotion.TargetStageKey) != strings.TrimSpace(stageKey) {
			continue
		}
		freight, err := h.delivery.GetFreight(ctx, promotion.FreightID)
		if err != nil {
			return nil, err
		}
		out = append(out, consoleV2ApprovalTask{
			ID:              promotion.ID,
			PromotionID:     promotion.ID,
			FreightID:       promotion.FreightID,
			FreightName:     firstNonEmpty(freight.Name, promotion.FreightID.String()),
			TargetStageKey:  strings.TrimSpace(stageKey),
			TargetStageName: stageName,
			RequestedBy:     promotion.CreatedBy,
			RequestedAt:     formatConsoleTime(promotion.CreatedAt),
			Message:         promotion.Message,
			DiffType:        diffTypeFromCurrentFreight(currentFreightID),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].RequestedAt > out[j].RequestedAt })
	return out, nil
}

func (h *consoleV2Handler) publishTasks(ctx context.Context, appID shared.ID, stageKey string) ([]consoleV2ApprovalTask, error) {
	stages, err := h.delivery.ListAppStages(ctx, appID)
	if err != nil {
		return nil, err
	}
	stageName := stageDisplayName(stages, stageKey)
	currentFreightID := currentFreightForStage(stages, stageKey)
	promotions, err := h.delivery.ListPromotions(ctx, appID, shared.PageRequest{Page: 1, PageSize: 1000})
	if err != nil {
		return nil, err
	}
	out := make([]consoleV2ApprovalTask, 0)
	for _, promotion := range promotions.Items {
		if !pendingPublishPromotion(promotion) || strings.TrimSpace(promotion.TargetStageKey) != strings.TrimSpace(stageKey) {
			continue
		}
		freight, err := h.delivery.GetFreight(ctx, promotion.FreightID)
		if err != nil {
			return nil, err
		}
		out = append(out, consoleV2ApprovalTask{
			ID:              promotion.ID,
			PromotionID:     promotion.ID,
			FreightID:       promotion.FreightID,
			FreightName:     firstNonEmpty(freight.Name, promotion.FreightID.String()),
			TargetStageKey:  strings.TrimSpace(stageKey),
			TargetStageName: stageName,
			RequestedBy:     promotion.CreatedBy,
			RequestedAt:     formatConsoleTime(promotion.CreatedAt),
			Message:         promotion.Message,
			DiffType:        diffTypeFromCurrentFreight(currentFreightID),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].RequestedAt > out[j].RequestedAt })
	return out, nil
}

func (h *consoleV2Handler) approvalTaskDetail(ctx context.Context, promotionID shared.ID) (consoleV2ApprovalDetail, error) {
	promotion, err := h.delivery.GetPromotion(ctx, promotionID)
	if err != nil {
		return consoleV2ApprovalDetail{}, err
	}
	stages, err := h.delivery.ListAppStages(ctx, promotion.ApplicationID)
	if err != nil {
		return consoleV2ApprovalDetail{}, err
	}
	stageName := stageDisplayName(stages, promotion.TargetStageKey)
	currentFreightID := currentFreightForStage(stages, promotion.TargetStageKey)
	pending, err := h.delivery.GetFreightDetail(ctx, promotion.FreightID)
	if err != nil {
		return consoleV2ApprovalDetail{}, err
	}
	task := consoleV2ApprovalTask{
		ID:              promotion.ID,
		PromotionID:     promotion.ID,
		FreightID:       promotion.FreightID,
		FreightName:     firstNonEmpty(pending.Freight.Name, promotion.FreightID.String()),
		TargetStageKey:  promotion.TargetStageKey,
		TargetStageName: stageName,
		RequestedBy:     promotion.CreatedBy,
		RequestedAt:     formatConsoleTime(promotion.CreatedAt),
		Message:         promotion.Message,
		DiffType:        diffTypeFromCurrentFreight(currentFreightID),
	}
	detail := consoleV2ApprovalDetail{
		Task:           task,
		DiffType:       task.DiffType,
		PendingFreight: freightSummary(pending.Freight),
		DeployItems:    deploySnapshotItems(pending.Items),
	}
	if currentFreightID.IsZero() {
		return detail, nil
	}
	current, err := h.delivery.GetFreightDetail(ctx, currentFreightID)
	if err != nil {
		return consoleV2ApprovalDetail{}, err
	}
	currentSummary := freightSummary(current.Freight)
	detail.CurrentFreight = &currentSummary
	detail.ImageChanges = imageChanges(current.Items, pending.Items)
	detail.ConfigDiff = configDiff(current.Freight.SourceFingerprint, pending.Freight.SourceFingerprint)
	return detail, nil
}

func stageDisplayName(stages []delivery.AppStage, stageKey string) string {
	for _, stage := range stages {
		if stageKeyForConsoleV2(stage) == stageKey {
			return firstNonEmpty(stage.DisplayName, stageKey)
		}
	}
	return stageKey
}

func currentFreightForStage(stages []delivery.AppStage, stageKey string) shared.ID {
	for _, stage := range stages {
		if stageKeyForConsoleV2(stage) == stageKey {
			return stage.CurrentFreightID
		}
	}
	return ""
}

func diffTypeFromCurrentFreight(id shared.ID) string {
	if id.IsZero() {
		return "first_deploy"
	}
	return "compare"
}

func freightSummary(freight delivery.Freight) consoleV2FreightSummary {
	return consoleV2FreightSummary{ID: freight.ID, Name: firstNonEmpty(freight.Name, freight.ID.String()), CreatedAt: formatConsoleTime(freight.CreatedAt)}
}

func deploySnapshotItems(items []delivery.FreightItem) []consoleV2DeploySnapshotItem {
	out := make([]consoleV2DeploySnapshotItem, 0, len(items))
	for _, item := range items {
		out = append(out, consoleV2DeploySnapshotItem{
			WorkloadID:    item.WorkloadID,
			WorkloadName:  firstNonEmpty(item.Name, item.WorkloadID.String()),
			ContainerName: firstNonEmpty(item.ContainerName, "app"),
			Version:       freightItemVersion(item),
			Image:         freightItemImage(item),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].WorkloadName == out[j].WorkloadName {
			return out[i].ContainerName < out[j].ContainerName
		}
		return out[i].WorkloadName < out[j].WorkloadName
	})
	return out
}

func imageChanges(current []delivery.FreightItem, pending []delivery.FreightItem) []consoleV2ImageChange {
	currentByKey := map[string]delivery.FreightItem{}
	for _, item := range current {
		currentByKey[freightItemKey(item)] = item
	}
	out := make([]consoleV2ImageChange, 0, len(pending))
	for _, next := range pending {
		prev, ok := currentByKey[freightItemKey(next)]
		change := consoleV2ImageChange{
			WorkloadID:     next.WorkloadID,
			WorkloadName:   firstNonEmpty(next.Name, next.WorkloadID.String()),
			ContainerName:  firstNonEmpty(next.ContainerName, "app"),
			CurrentImage:   "-",
			PendingImage:   freightItemImage(next),
			CurrentVersion: "-",
			PendingVersion: freightItemVersion(next),
		}
		if ok {
			change.CurrentImage = freightItemImage(prev)
			change.CurrentVersion = freightItemVersion(prev)
		}
		if change.CurrentImage != change.PendingImage || change.CurrentVersion != change.PendingVersion {
			out = append(out, change)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].WorkloadName == out[j].WorkloadName {
			return out[i].ContainerName < out[j].ContainerName
		}
		return out[i].WorkloadName < out[j].WorkloadName
	})
	return out
}

func freightItemKey(item delivery.FreightItem) string {
	return item.WorkloadID.String() + "/" + firstNonEmpty(item.ContainerName, "app")
}

func freightItemVersion(item delivery.FreightItem) string {
	if item.SourceType == delivery.FreightItemCustomImage || item.Type == delivery.FreightItemCustomImage {
		return "自定义"
	}
	return firstNonEmpty(item.ImageTag, item.URI, item.ImageRef, "-")
}

func freightItemImage(item delivery.FreightItem) string {
	if item.ImageRef != "" {
		return item.ImageRef
	}
	if item.ImageRepository != "" && item.ImageTag != "" {
		return item.ImageRepository + ":" + item.ImageTag
	}
	if item.URI != "" {
		return item.URI
	}
	return "-"
}

func configDiff(current string, pending string) string {
	current = strings.TrimSpace(current)
	pending = strings.TrimSpace(pending)
	if current == "" && pending == "" || current == pending {
		return ""
	}
	return strings.Join([]string{
		"diff --git a/workload-config.snapshot b/workload-config.snapshot",
		"--- a/workload-config.snapshot",
		"+++ b/workload-config.snapshot",
		"@@",
		"- source_fingerprint: " + firstNonEmpty(current, "<none>"),
		"+ source_fingerprint: " + firstNonEmpty(pending, "<none>"),
	}, "\n")
}

func formatConsoleTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}
