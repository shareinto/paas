package sourcerepository

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
	mux.HandleFunc("POST /api/source-repositories", h.handleCreateRepository)
	mux.HandleFunc("DELETE /api/source-repositories/{repositoryId}", h.handleDeleteRepository)
	mux.HandleFunc("GET /api/source-repositories/{repositoryId}", h.handleGetRepository)
	mux.HandleFunc("GET /api/projects/{projectId}/source-repositories", h.handleListRepositories)
	mux.HandleFunc("GET /api/source-repositories/{repositoryId}/applications", h.handleAssociatedApplications)
	mux.HandleFunc("GET /api/source-repositories/{repositoryId}/branches", h.handleRepositoryBranches)
	mux.HandleFunc("GET /api/source-repositories/{repositoryId}/tree", h.handleRepositoryTree)
	mux.HandleFunc("POST /api/source-repositories/{repositoryId}/scan/java", h.handleScanJava)
	mux.HandleFunc("POST /api/source-repositories/{repositoryId}/permission-sync", h.handlePermissionSync)
	mux.HandleFunc("POST /api/repository-migrations", h.handleCreateMigration)
	mux.HandleFunc("GET /api/repository-migrations/{migrationId}", h.handleGetMigration)
	mux.HandleFunc("POST /api/repository-migrations/{migrationId}/retry", h.handleRetryMigration)
	mux.HandleFunc("POST /api/repository-migrations/{migrationId}/cancel", h.handleCancelMigration)
	mux.HandleFunc("POST /api/repository-migrations/{migrationId}/process", h.handleProcessMigration)
}

func (h *Handler) handleCreateRepository(w http.ResponseWriter, r *http.Request) {
	var req CreateSourceRepositoryInput
	if !decodeJSON(w, r, &req) {
		return
	}
	repository, err := h.service.CreateSourceRepository(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, repository)
}

func (h *Handler) handleGetRepository(w http.ResponseWriter, r *http.Request) {
	repository, err := h.service.GetSourceRepository(r.Context(), shared.ID(r.PathValue("repositoryId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, repository)
}

func (h *Handler) handleDeleteRepository(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Actor identityaccess.Subject `json:"actor"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.service.DeleteSourceRepository(r.Context(), DeleteSourceRepositoryInput{Actor: req.Actor, SourceRepositoryID: shared.ID(r.PathValue("repositoryId"))}); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleListRepositories(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ListSourceRepositoriesByProject(r.Context(), shared.ID(r.PathValue("projectId")), pageFromQuery(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) handleAssociatedApplications(w http.ResponseWriter, r *http.Request) {
	applications, err := h.service.ListAssociatedApplications(r.Context(), shared.ID(r.PathValue("repositoryId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": applications})
}

func (h *Handler) handleRepositoryTree(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListRepositoryTree(r.Context(), shared.ID(r.PathValue("repositoryId")), r.URL.Query().Get("ref"), r.URL.Query().Get("path"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) handleRepositoryBranches(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListRepositoryBranches(r.Context(), shared.ID(r.PathValue("repositoryId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) handleScanJava(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Ref string `json:"ref"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	suggestions, err := h.service.GenerateBuildSpecSuggestions(r.Context(), shared.ID(r.PathValue("repositoryId")), req.Ref)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": suggestions})
}

func (h *Handler) handlePermissionSync(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Actor identityaccess.Subject `json:"actor"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	job, err := h.service.SyncRepositoryPermissions(r.Context(), req.Actor, shared.ID(r.PathValue("repositoryId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, job)
}

func (h *Handler) handleCreateMigration(w http.ResponseWriter, r *http.Request) {
	var req CreateRepositoryMigrationInput
	if !decodeJSON(w, r, &req) {
		return
	}
	migration, err := h.service.CreateRepositoryMigration(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, migration)
}

func (h *Handler) handleGetMigration(w http.ResponseWriter, r *http.Request) {
	migration, err := h.service.GetRepositoryMigration(r.Context(), shared.ID(r.PathValue("migrationId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, migration)
}

func (h *Handler) handleRetryMigration(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Actor identityaccess.Subject `json:"actor"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	migration, err := h.service.RetryRepositoryMigration(r.Context(), req.Actor, shared.ID(r.PathValue("migrationId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, migration)
}

func (h *Handler) handleCancelMigration(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Actor identityaccess.Subject `json:"actor"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	migration, err := h.service.CancelRepositoryMigration(r.Context(), req.Actor, shared.ID(r.PathValue("migrationId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, migration)
}

func (h *Handler) handleProcessMigration(w http.ResponseWriter, r *http.Request) {
	migration, suggestions, err := h.service.ProcessRepositoryMigration(r.Context(), shared.ID(r.PathValue("migrationId")))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"migration": migration, "build_spec_suggestions": suggestions})
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
	status := shared.HTTPStatusOf(err)
	fmt.Fprintf(os.Stdout, "time=%s level=error module=source-repository status=%d code=%s error=%q\n", time.Now().Format(time.RFC3339), status, shared.CodeOf(err), err.Error())
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": shared.CodeOf(err), "message": "请求处理失败"}})
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
