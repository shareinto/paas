package build

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

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
	mux.HandleFunc("GET /api/build-environments", h.handleListEnabledBuildEnvironments)
	mux.HandleFunc("GET /api/admin/build-environments", h.handleListAdminBuildEnvironments)
	mux.HandleFunc("POST /api/admin/build-environments", h.handleCreateBuildEnvironment)
	mux.HandleFunc("PATCH /api/admin/build-environments/{environmentId}", h.handleUpdateBuildEnvironment)
	mux.HandleFunc("DELETE /api/admin/build-environments/{environmentId}", h.handleDeleteBuildEnvironment)
	mux.HandleFunc("GET /api/runtime-environments", h.handleListEnabledRuntimeEnvironments)
	mux.HandleFunc("GET /api/admin/runtime-environments", h.handleListAdminRuntimeEnvironments)
	mux.HandleFunc("POST /api/admin/runtime-environments", h.handleCreateRuntimeEnvironment)
	mux.HandleFunc("PATCH /api/admin/runtime-environments/{environmentId}", h.handleUpdateRuntimeEnvironment)
	mux.HandleFunc("DELETE /api/admin/runtime-environments/{environmentId}", h.handleDeleteRuntimeEnvironment)
	mux.HandleFunc("GET /api/admin/build-template", h.handleGetBuildTemplate)
	mux.HandleFunc("PUT /api/admin/build-template", h.handleSaveBuildTemplate)
	mux.HandleFunc("POST /api/admin/build-template/revisions", h.handleSaveBuildTemplate)
	mux.HandleFunc("GET /api/jenkins-job-templates", h.handleListEnabledJenkinsJobTemplates)
	mux.HandleFunc("GET /api/admin/jenkins-job-templates", h.handleListAdminJenkinsJobTemplates)
	mux.HandleFunc("POST /api/admin/jenkins-job-templates", h.handleCreateJenkinsJobTemplate)
	mux.HandleFunc("GET /api/admin/jenkins-job-templates/{templateId}", h.handleGetJenkinsJobTemplate)
	mux.HandleFunc("PATCH /api/admin/jenkins-job-templates/{templateId}", h.handleUpdateJenkinsJobTemplate)
	mux.HandleFunc("DELETE /api/admin/jenkins-job-templates/{templateId}", h.handleDeleteJenkinsJobTemplate)
	mux.HandleFunc("POST /api/admin/jenkins-job-templates/{templateId}/revisions", h.handleUploadJenkinsJobTemplateRevision)
	mux.HandleFunc("GET /api/build-types", h.handleListEnabledJenkinsJobTemplates)
	mux.HandleFunc("GET /api/admin/build-types", h.handleListAdminJenkinsJobTemplates)
	mux.HandleFunc("POST /api/admin/build-types", h.handleCreateJenkinsJobTemplate)
	mux.HandleFunc("GET /api/admin/build-types/{templateId}", h.handleGetJenkinsJobTemplate)
	mux.HandleFunc("PATCH /api/admin/build-types/{templateId}", h.handleUpdateJenkinsJobTemplate)
	mux.HandleFunc("DELETE /api/admin/build-types/{templateId}", h.handleDeleteJenkinsJobTemplate)
	mux.HandleFunc("POST /api/admin/build-types/{templateId}/revisions", h.handleUploadJenkinsJobTemplateRevision)
	mux.HandleFunc("GET /api/apps/{appId}/build-pipelines", h.handleListBuildPipelines)
	mux.HandleFunc("POST /api/apps/{appId}/build-pipelines", h.handleCreateBuildPipeline)
	mux.HandleFunc("GET /api/build-pipelines/{pipelineId}", h.handleGetBuildPipeline)
	mux.HandleFunc("PATCH /api/build-pipelines/{pipelineId}", h.handleUpdateBuildPipeline)
	mux.HandleFunc("DELETE /api/build-pipelines/{pipelineId}", h.handleDeleteBuildPipeline)
	mux.HandleFunc("GET /api/build-pipelines/{pipelineId}/sources", h.handleListBuildPipelineSources)
	mux.HandleFunc("POST /api/build-pipelines/{pipelineId}/builds", h.handleTriggerPipelineBuild)
	mux.HandleFunc("POST /api/apps/{appId}/builds", h.handleTriggerBuild)
	mux.HandleFunc("GET /api/apps/{appId}/builds", h.handleListBuilds)
	mux.HandleFunc("GET /api/builds/{buildRunId}", h.handleGetBuild)
	mux.HandleFunc("GET /api/builds/{buildRunId}/artifacts", h.handleListArtifacts)
	mux.HandleFunc("GET /api/builds/{buildRunId}/logs/stream", h.handleLogStream)
	mux.HandleFunc("POST /api/builds/{buildRunId}/cancel", h.handleCancelBuild)
	mux.HandleFunc("POST /api/builds/{buildRunId}/callback", h.handleBuildCallback)
	mux.HandleFunc("POST /api/builds/{buildRunId}/queue-sync", h.handleQueueSync)
}

func (h *Handler) handleListEnabledBuildEnvironments(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListBuildEnvironments(r.Context(), false, pageFromQuery(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleListAdminBuildEnvironments(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListBuildEnvironments(r.Context(), true, pageFromQuery(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleCreateBuildEnvironment(w http.ResponseWriter, r *http.Request) {
	var req CreateBuildEnvironmentInput
	if !decodeJSON(w, r, &req) {
		return
	}
	environment, err := h.service.CreateBuildEnvironment(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, environment)
}

func (h *Handler) handleUpdateBuildEnvironment(w http.ResponseWriter, r *http.Request) {
	var req UpdateBuildEnvironmentInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.EnvironmentID = shared.ID(r.PathValue("environmentId"))
	environment, err := h.service.UpdateBuildEnvironment(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, environment)
}

func (h *Handler) handleDeleteBuildEnvironment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Actor identityaccess.Subject `json:"actor"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.service.DeleteBuildEnvironment(r.Context(), req.Actor, shared.ID(r.PathValue("environmentId"))); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleListEnabledRuntimeEnvironments(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListRuntimeEnvironments(r.Context(), false, pageFromQuery(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapRuntimeEnvironmentPage(result, false))
}

func (h *Handler) handleListAdminRuntimeEnvironments(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListRuntimeEnvironments(r.Context(), true, pageFromQuery(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleCreateRuntimeEnvironment(w http.ResponseWriter, r *http.Request) {
	var req CreateRuntimeEnvironmentInput
	if !decodeJSON(w, r, &req) {
		return
	}
	environment, err := h.service.CreateRuntimeEnvironment(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, environment)
}

func (h *Handler) handleUpdateRuntimeEnvironment(w http.ResponseWriter, r *http.Request) {
	var req UpdateRuntimeEnvironmentInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.EnvironmentID = shared.ID(r.PathValue("environmentId"))
	environment, err := h.service.UpdateRuntimeEnvironment(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, environment)
}

func (h *Handler) handleDeleteRuntimeEnvironment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Actor identityaccess.Subject `json:"actor"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.service.DeleteRuntimeEnvironment(r.Context(), req.Actor, shared.ID(r.PathValue("environmentId"))); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleGetBuildTemplate(w http.ResponseWriter, r *http.Request) {
	template, err := h.service.GetBuildTemplate(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, template)
}

func (h *Handler) handleSaveBuildTemplate(w http.ResponseWriter, r *http.Request) {
	var req SaveBuildTemplateInput
	if !decodeJSON(w, r, &req) {
		return
	}
	template, err := h.service.SaveBuildTemplate(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, template)
}

func (h *Handler) handleListEnabledJenkinsJobTemplates(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListJenkinsJobTemplates(r.Context(), false, pageFromQuery(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapJenkinsTemplatePage(result, false))
}

func (h *Handler) handleListAdminJenkinsJobTemplates(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListJenkinsJobTemplates(r.Context(), true, pageFromQuery(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapJenkinsTemplatePage(result, false))
}

func (h *Handler) handleCreateJenkinsJobTemplate(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJenkinsTemplateUpload(w, r)
	if !ok {
		return
	}
	template, err := h.service.CreateJenkinsJobTemplate(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, mapJenkinsTemplate(template, true))
}

func (h *Handler) handleGetJenkinsJobTemplate(w http.ResponseWriter, r *http.Request) {
	template, err := h.service.GetJenkinsJobTemplate(r.Context(), shared.ID(r.PathValue("templateId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapJenkinsTemplate(template, true))
}

func (h *Handler) handleUpdateJenkinsJobTemplate(w http.ResponseWriter, r *http.Request) {
	var req UpdateJenkinsJobTemplateInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.TemplateID = shared.ID(r.PathValue("templateId"))
	template, err := h.service.UpdateJenkinsJobTemplate(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapJenkinsTemplate(template, true))
}

func (h *Handler) handleDeleteJenkinsJobTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Actor identityaccess.Subject `json:"actor"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.service.DeleteJenkinsJobTemplate(r.Context(), req.Actor, shared.ID(r.PathValue("templateId"))); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleUploadJenkinsJobTemplateRevision(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJenkinsTemplateRevisionUpload(w, r)
	if !ok {
		return
	}
	req.TemplateID = shared.ID(r.PathValue("templateId"))
	template, err := h.service.UploadJenkinsJobTemplateRevision(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapJenkinsTemplate(template, true))
}

func (h *Handler) handleTriggerBuild(w http.ResponseWriter, r *http.Request) {
	var req TriggerBuildInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.ApplicationID = shared.ID(r.PathValue("appId"))
	run, err := h.service.TriggerBuild(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, run)
}

func (h *Handler) handleTriggerPipelineBuild(w http.ResponseWriter, r *http.Request) {
	var req TriggerBuildInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.PipelineID = shared.ID(r.PathValue("pipelineId"))
	run, err := h.service.TriggerBuild(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, run)
}

func (h *Handler) handleCreateBuildPipeline(w http.ResponseWriter, r *http.Request) {
	var req CreateBuildPipelineInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.ApplicationID = shared.ID(r.PathValue("appId"))
	pipeline, err := h.service.CreateBuildPipeline(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, mapBuildPipeline(pipeline))
}

func (h *Handler) handleListBuildPipelines(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListBuildPipelines(r.Context(), shared.ID(r.PathValue("appId")), pageFromQuery(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapBuildPipelinePage(result))
}

func (h *Handler) handleGetBuildPipeline(w http.ResponseWriter, r *http.Request) {
	pipeline, err := h.service.GetBuildPipeline(r.Context(), shared.ID(r.PathValue("pipelineId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapBuildPipeline(pipeline))
}

func (h *Handler) handleListBuildPipelineSources(w http.ResponseWriter, r *http.Request) {
	sources, err := h.service.ListBuildPipelineSources(r.Context(), shared.ID(r.PathValue("pipelineId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": sources})
}

func (h *Handler) handleUpdateBuildPipeline(w http.ResponseWriter, r *http.Request) {
	var req UpdateBuildPipelineInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.PipelineID = shared.ID(r.PathValue("pipelineId"))
	pipeline, err := h.service.UpdateBuildPipeline(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, mapBuildPipeline(pipeline))
}

func (h *Handler) handleDeleteBuildPipeline(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Actor identityaccess.Subject `json:"actor"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.service.DeleteNamedBuildPipeline(r.Context(), req.Actor, shared.ID(r.PathValue("pipelineId"))); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleListBuilds(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListBuildRuns(r.Context(), shared.ID(r.PathValue("appId")), pageFromQuery(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleGetBuild(w http.ResponseWriter, r *http.Request) {
	run, err := h.service.GetBuildRun(r.Context(), shared.ID(r.PathValue("buildRunId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (h *Handler) handleListArtifacts(w http.ResponseWriter, r *http.Request) {
	artifacts, err := h.service.ListBuildArtifacts(r.Context(), shared.ID(r.PathValue("buildRunId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": artifacts})
}

func (h *Handler) handleLogStream(w http.ResponseWriter, r *http.Request) {
	buildRunID := shared.ID(r.PathValue("buildRunId"))
	if _, err := h.service.GetBuildRun(r.Context(), buildRunID); err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	events, err := h.service.BuildLogEvents(r.Context(), buildRunID)
	if err != nil {
		log.Printf("build log stream initial drain failed: build_run_id=%s code=%s error=%v", buildRunID, shared.CodeOf(err), err)
		writeSSE(w, LogEvent{Event: "error", Data: "请求处理失败"})
		return
	}
	if terminal := writeSSEEvents(w, flusher, events); terminal {
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			events, err := h.service.StreamBuildLogs(r.Context(), buildRunID)
			if err != nil {
				log.Printf("build log stream drain failed: build_run_id=%s code=%s error=%v", buildRunID, shared.CodeOf(err), err)
				writeSSE(w, LogEvent{Event: "error", Data: "请求处理失败"})
				if flusher != nil {
					flusher.Flush()
				}
				return
			}
			if terminal := writeSSEEvents(w, flusher, events); terminal {
				return
			}
		}
	}
}

func writeSSEEvents(w http.ResponseWriter, flusher http.Flusher, events []LogEvent) bool {
	terminal := false
	for _, event := range events {
		writeSSE(w, event)
		if event.Event == "status" && terminalBuildStatus(event.Data) {
			terminal = true
		}
	}
	if flusher != nil {
		flusher.Flush()
	}
	return terminal
}

func writeSSE(w http.ResponseWriter, event LogEvent) {
	_, _ = fmt.Fprintf(w, "event: %s\n", event.Event)
	for _, line := range strings.Split(strings.ReplaceAll(event.Data, "\r\n", "\n"), "\n") {
		_, _ = fmt.Fprintf(w, "data: %s\n", line)
	}
	_, _ = fmt.Fprint(w, "\n")
}

func terminalBuildStatus(status string) bool {
	switch BuildRunStatus(strings.TrimSpace(status)) {
	case BuildRunSucceeded, BuildRunFailed, BuildRunAborted, BuildRunUnstable:
		return true
	default:
		return false
	}
}

func (h *Handler) handleCancelBuild(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Actor identityaccess.Subject `json:"actor"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	run, err := h.service.CancelBuild(r.Context(), req.Actor, shared.ID(r.PathValue("buildRunId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (h *Handler) handleBuildCallback(w http.ResponseWriter, r *http.Request) {
	var req BuildCallbackInput
	if !decodeJSON(w, r, &req) {
		return
	}
	req.BuildRunID = shared.ID(r.PathValue("buildRunId"))
	run, err := h.service.HandleBuildCallback(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (h *Handler) handleQueueSync(w http.ResponseWriter, r *http.Request) {
	run, err := h.service.SyncQueueItem(r.Context(), shared.ID(r.PathValue("buildRunId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeError(w, shared.WrapError(shared.CodeInvalidArgument, "invalid json body", err))
		return false
	}
	return true
}

func decodeJenkinsTemplateUpload(w http.ResponseWriter, r *http.Request) (CreateJenkinsJobTemplateInput, bool) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(2 << 20); err != nil {
			writeError(w, shared.WrapError(shared.CodeInvalidArgument, "invalid multipart body", err))
			return CreateJenkinsJobTemplateInput{}, false
		}
		jenkinsfileContent, ok := readUploadedText(w, r, "jenkinsfile", "Jenkinsfile file is required", "failed to read Jenkinsfile")
		if !ok {
			return CreateJenkinsJobTemplateInput{}, false
		}
		return CreateJenkinsJobTemplateInput{
			Actor:              identityaccess.Subject{Type: identityaccess.SubjectUser, ID: shared.ID(r.FormValue("actor_id"))},
			Name:               r.FormValue("name"),
			JenkinsfileContent: jenkinsfileContent,
			IsDefault:          r.FormValue("is_default") == "true",
		}, true
	}
	var req CreateJenkinsJobTemplateInput
	if !decodeJSON(w, r, &req) {
		return CreateJenkinsJobTemplateInput{}, false
	}
	return req, true
}

func decodeJenkinsTemplateRevisionUpload(w http.ResponseWriter, r *http.Request) (UploadJenkinsJobTemplateRevisionInput, bool) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(2 << 20); err != nil {
			writeError(w, shared.WrapError(shared.CodeInvalidArgument, "invalid multipart body", err))
			return UploadJenkinsJobTemplateRevisionInput{}, false
		}
		jenkinsfileContent, ok := readUploadedText(w, r, "jenkinsfile", "Jenkinsfile file is required", "failed to read Jenkinsfile")
		if !ok {
			return UploadJenkinsJobTemplateRevisionInput{}, false
		}
		return UploadJenkinsJobTemplateRevisionInput{Actor: identityaccess.Subject{Type: identityaccess.SubjectUser, ID: shared.ID(r.FormValue("actor_id"))}, JenkinsfileContent: jenkinsfileContent}, true
	}
	var req UploadJenkinsJobTemplateRevisionInput
	if !decodeJSON(w, r, &req) {
		return UploadJenkinsJobTemplateRevisionInput{}, false
	}
	return req, true
}

func readUploadedText(w http.ResponseWriter, r *http.Request, field string, missingMessage string, readMessage string) (string, bool) {
	file, _, err := r.FormFile(field)
	if err != nil {
		writeError(w, shared.WrapError(shared.CodeInvalidArgument, missingMessage, err))
		return "", false
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 2<<20))
	if err != nil {
		writeError(w, shared.WrapError(shared.CodeInvalidArgument, readMessage, err))
		return "", false
	}
	return string(data), true
}

func mapJenkinsTemplatePage(result shared.PageResult[JenkinsJobTemplate], includeXML bool) shared.PageResult[map[string]any] {
	items := make([]map[string]any, 0, len(result.Items))
	for _, template := range result.Items {
		items = append(items, mapJenkinsTemplate(template, includeXML))
	}
	return shared.PageResult[map[string]any]{Items: items, Total: result.Total, Page: result.Page, PageSize: result.PageSize}
}

func mapRuntimeEnvironmentPage(result shared.PageResult[RuntimeEnvironment], includeImages bool) shared.PageResult[map[string]any] {
	items := make([]map[string]any, 0, len(result.Items))
	for _, environment := range result.Items {
		items = append(items, mapRuntimeEnvironment(environment, includeImages))
	}
	return shared.PageResult[map[string]any]{Items: items, Total: result.Total, Page: result.Page, PageSize: result.PageSize}
}

func mapRuntimeEnvironment(environment RuntimeEnvironment, includeImages bool) map[string]any {
	out := map[string]any{
		"id":          environment.ID,
		"name":        environment.Name,
		"description": environment.Description,
		"status":      environment.Status,
		"created_by":  environment.CreatedBy,
		"created_at":  environment.CreatedAt,
		"updated_at":  environment.UpdatedAt,
	}
	if includeImages {
		out["runtime_base_image"] = environment.RuntimeBaseImage
		out["artifact_deploy_path"] = environment.ArtifactDeployPath
		out["dockerfile_path"] = environment.DockerfilePath
		out["selector_labels"] = environment.SelectorLabels
		out["images"] = environment.Images
	}
	return out
}

func mapBuildPipelinePage(result shared.PageResult[BuildPipeline]) shared.PageResult[map[string]any] {
	items := make([]map[string]any, 0, len(result.Items))
	for _, pipeline := range result.Items {
		items = append(items, mapBuildPipeline(pipeline))
	}
	return shared.PageResult[map[string]any]{Items: items, Total: result.Total, Page: result.Page, PageSize: result.PageSize}
}

func mapBuildPipeline(pipeline BuildPipeline) map[string]any {
	return map[string]any{
		"id":                   pipeline.ID,
		"tenant_id":            pipeline.TenantID,
		"project_id":           pipeline.ProjectID,
		"application_id":       pipeline.ApplicationID,
		"name":                 pipeline.Name,
		"display_name":         pipeline.DisplayName,
		"description":          pipeline.Description,
		"provider":             pipeline.Provider,
		"external_job_name":    pipeline.ExternalJobName,
		"template_id":          pipeline.TemplateID,
		"config_hash":          pipeline.ConfigHash,
		"status":               pipeline.Status,
		"managed_by_platform":  pipeline.ManagedByPlatform,
		"runtime_environments": mapRuntimeEnvironmentRefs(pipeline.RuntimeEnvironments),
		"created_at":           pipeline.CreatedAt,
		"updated_at":           pipeline.UpdatedAt,
	}
}

func mapRuntimeEnvironmentRefs(runtimes []RuntimeEnvironmentRef) []map[string]any {
	items := make([]map[string]any, 0, len(runtimes))
	for _, runtime := range runtimes {
		items = append(items, map[string]any{
			"id":   runtime.ID,
			"name": runtime.Name,
		})
	}
	return items
}

func mapJenkinsTemplate(template JenkinsJobTemplate, includeXML bool) map[string]any {
	out := map[string]any{
		"id":         template.ID,
		"name":       template.Name,
		"version":    template.Version,
		"status":     template.Status,
		"is_default": template.IsDefault,
		"created_by": template.CreatedBy,
		"created_at": template.CreatedAt,
		"updated_at": template.UpdatedAt,
	}
	if includeXML {
		out["jenkinsfile_content"] = template.XMLContent
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	message := "请求处理失败"
	var appErr *shared.AppError
	if errors.As(err, &appErr) && appErr.Message != "" {
		message = appErr.Message
	}
	writeJSON(w, shared.HTTPStatusOf(err), map[string]any{"error": map[string]any{"code": shared.CodeOf(err), "message": message}})
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
