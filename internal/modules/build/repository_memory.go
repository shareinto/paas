package build

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/shareinto/paas/internal/shared"
)

func appendUniqueID(items []shared.ID, id shared.ID) []shared.ID {
	for _, existing := range items {
		if existing == id {
			return items
		}
	}
	return append(items, id)
}

type MemoryRepository struct {
	mu                        sync.RWMutex
	buildEnvironments         map[shared.ID]BuildEnvironment
	runtimeEnvironments       map[shared.ID]RuntimeEnvironment
	buildTemplate             BuildTemplate
	templates                 map[shared.ID]JenkinsJobTemplate
	pipelines                 map[shared.ID]BuildPipeline
	pipelinesByApplication    map[shared.ID][]shared.ID
	pipelineByApplicationName map[shared.ID]map[string]shared.ID
	pipelineSourcesByPipeline map[shared.ID][]BuildPipelineSource
	runs                      map[shared.ID]BuildRun
	runsByApplication         map[shared.ID][]shared.ID
	runSourcesByRun           map[shared.ID][]BuildRunSource
	artifacts                 map[shared.ID]BuildArtifact
	artifactsByRun            map[shared.ID][]shared.ID
	logsByRun                 map[shared.ID][]string
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		buildEnvironments:         map[shared.ID]BuildEnvironment{},
		runtimeEnvironments:       map[shared.ID]RuntimeEnvironment{},
		templates:                 map[shared.ID]JenkinsJobTemplate{},
		pipelines:                 map[shared.ID]BuildPipeline{},
		pipelinesByApplication:    map[shared.ID][]shared.ID{},
		pipelineByApplicationName: map[shared.ID]map[string]shared.ID{},
		pipelineSourcesByPipeline: map[shared.ID][]BuildPipelineSource{},
		runs:                      map[shared.ID]BuildRun{},
		runsByApplication:         map[shared.ID][]shared.ID{},
		runSourcesByRun:           map[shared.ID][]BuildRunSource{},
		artifacts:                 map[shared.ID]BuildArtifact{},
		artifactsByRun:            map[shared.ID][]shared.ID{},
		logsByRun:                 map[shared.ID][]string{},
	}
}

func (r *MemoryRepository) CreateBuildEnvironment(_ context.Context, environment BuildEnvironment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.buildEnvironments[environment.ID]; exists {
		return shared.NewError(shared.CodeConflict, "build environment already exists")
	}
	if environment.IsDefault {
		for id, existing := range r.buildEnvironments {
			existing.IsDefault = false
			r.buildEnvironments[id] = existing
		}
	}
	r.buildEnvironments[environment.ID] = environment
	return nil
}

func (r *MemoryRepository) UpdateBuildEnvironment(_ context.Context, environment BuildEnvironment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.buildEnvironments[environment.ID]; !ok {
		return shared.NewError(shared.CodeNotFound, "build environment not found")
	}
	if environment.IsDefault {
		for id, existing := range r.buildEnvironments {
			if id == environment.ID {
				continue
			}
			existing.IsDefault = false
			r.buildEnvironments[id] = existing
		}
	}
	r.buildEnvironments[environment.ID] = environment
	return nil
}

func (r *MemoryRepository) DeleteBuildEnvironment(_ context.Context, id shared.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	environment, ok := r.buildEnvironments[id]
	if !ok || environment.Status == BuildEnvironmentDeleted {
		return shared.NewError(shared.CodeNotFound, "build environment not found")
	}
	environment.Status = BuildEnvironmentDeleted
	environment.IsDefault = false
	r.buildEnvironments[id] = environment
	return nil
}

func (r *MemoryRepository) GetBuildEnvironment(_ context.Context, id shared.ID) (BuildEnvironment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	environment, ok := r.buildEnvironments[id]
	if !ok {
		return BuildEnvironment{}, shared.NewError(shared.CodeNotFound, "build environment not found")
	}
	return environment, nil
}

func (r *MemoryRepository) FindDefaultBuildEnvironment(_ context.Context) (BuildEnvironment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, environment := range r.buildEnvironments {
		if environment.IsDefault && environment.Status == BuildEnvironmentEnabled {
			return environment, nil
		}
	}
	var fallback BuildEnvironment
	for _, environment := range r.buildEnvironments {
		if environment.Status != BuildEnvironmentEnabled {
			continue
		}
		if fallback.ID.IsZero() || environment.CreatedAt.Before(fallback.CreatedAt) {
			fallback = environment
		}
	}
	if fallback.ID.IsZero() {
		return BuildEnvironment{}, shared.NewError(shared.CodeNotFound, "enabled build environment not found")
	}
	return fallback, nil
}

func (r *MemoryRepository) ListBuildEnvironments(_ context.Context, includeDisabled bool, page shared.PageRequest) (shared.PageResult[BuildEnvironment], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	page = page.Normalize()
	items := make([]BuildEnvironment, 0, len(r.buildEnvironments))
	for _, environment := range r.buildEnvironments {
		if environment.Status == BuildEnvironmentDeleted {
			continue
		}
		if includeDisabled || environment.Status == BuildEnvironmentEnabled {
			items = append(items, environment)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDefault != items[j].IsDefault {
			return items[i].IsDefault
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	start := page.Offset()
	if start > len(items) {
		start = len(items)
	}
	end := start + page.PageSize
	if end > len(items) {
		end = len(items)
	}
	return shared.NewPageResult(items[start:end], int64(len(items)), page), nil
}

func (r *MemoryRepository) CreateRuntimeEnvironment(_ context.Context, environment RuntimeEnvironment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.runtimeEnvironments[environment.ID]; exists {
		return shared.NewError(shared.CodeConflict, "runtime environment already exists")
	}
	if environment.IsDefault {
		for id, existing := range r.runtimeEnvironments {
			existing.IsDefault = false
			r.runtimeEnvironments[id] = existing
		}
	}
	r.runtimeEnvironments[environment.ID] = environment
	return nil
}

func (r *MemoryRepository) UpdateRuntimeEnvironment(_ context.Context, environment RuntimeEnvironment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.runtimeEnvironments[environment.ID]; !ok {
		return shared.NewError(shared.CodeNotFound, "runtime environment not found")
	}
	if environment.IsDefault {
		for id, existing := range r.runtimeEnvironments {
			if id == environment.ID {
				continue
			}
			existing.IsDefault = false
			r.runtimeEnvironments[id] = existing
		}
	}
	r.runtimeEnvironments[environment.ID] = environment
	return nil
}

func (r *MemoryRepository) DeleteRuntimeEnvironment(_ context.Context, id shared.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	environment, ok := r.runtimeEnvironments[id]
	if !ok || environment.Status == RuntimeEnvironmentDeleted {
		return shared.NewError(shared.CodeNotFound, "runtime environment not found")
	}
	environment.Status = RuntimeEnvironmentDeleted
	environment.IsDefault = false
	r.runtimeEnvironments[id] = environment
	return nil
}

func (r *MemoryRepository) GetRuntimeEnvironment(_ context.Context, id shared.ID) (RuntimeEnvironment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	environment, ok := r.runtimeEnvironments[id]
	if !ok {
		return RuntimeEnvironment{}, shared.NewError(shared.CodeNotFound, "runtime environment not found")
	}
	return environment, nil
}

func (r *MemoryRepository) FindDefaultRuntimeEnvironment(_ context.Context) (RuntimeEnvironment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, environment := range r.runtimeEnvironments {
		if environment.IsDefault && environment.Status == RuntimeEnvironmentEnabled {
			return environment, nil
		}
	}
	var fallback RuntimeEnvironment
	for _, environment := range r.runtimeEnvironments {
		if environment.Status != RuntimeEnvironmentEnabled {
			continue
		}
		if fallback.ID.IsZero() || environment.CreatedAt.Before(fallback.CreatedAt) {
			fallback = environment
		}
	}
	if fallback.ID.IsZero() {
		return RuntimeEnvironment{}, shared.NewError(shared.CodeNotFound, "enabled runtime environment not found")
	}
	return fallback, nil
}

func (r *MemoryRepository) ListRuntimeEnvironments(_ context.Context, includeDisabled bool, page shared.PageRequest) (shared.PageResult[RuntimeEnvironment], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	page = page.Normalize()
	items := make([]RuntimeEnvironment, 0, len(r.runtimeEnvironments))
	for _, environment := range r.runtimeEnvironments {
		if environment.Status == RuntimeEnvironmentDeleted {
			continue
		}
		if includeDisabled || environment.Status == RuntimeEnvironmentEnabled {
			items = append(items, environment)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDefault != items[j].IsDefault {
			return items[i].IsDefault
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	start := page.Offset()
	if start > len(items) {
		start = len(items)
	}
	end := start + page.PageSize
	if end > len(items) {
		end = len(items)
	}
	return shared.NewPageResult(items[start:end], int64(len(items)), page), nil
}

func (r *MemoryRepository) GetBuildTemplate(_ context.Context) (BuildTemplate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.buildTemplate.ID.IsZero() {
		return BuildTemplate{}, shared.NewError(shared.CodeNotFound, "build template not found")
	}
	return r.buildTemplate, nil
}

func (r *MemoryRepository) SaveBuildTemplate(_ context.Context, template BuildTemplate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.buildTemplate = template
	return nil
}

func (r *MemoryRepository) CreateJenkinsJobTemplate(_ context.Context, template JenkinsJobTemplate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.templates[template.ID]; exists {
		return shared.NewError(shared.CodeConflict, "jenkins job template already exists")
	}
	if template.IsDefault {
		for id, existing := range r.templates {
			existing.IsDefault = false
			r.templates[id] = existing
		}
	}
	r.templates[template.ID] = template
	return nil
}

func (r *MemoryRepository) UpdateJenkinsJobTemplate(_ context.Context, template JenkinsJobTemplate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.templates[template.ID]; !ok {
		return shared.NewError(shared.CodeNotFound, "jenkins job template not found")
	}
	if template.IsDefault {
		for id, existing := range r.templates {
			if id == template.ID {
				continue
			}
			existing.IsDefault = false
			r.templates[id] = existing
		}
	}
	r.templates[template.ID] = template
	return nil
}

func (r *MemoryRepository) DeleteJenkinsJobTemplate(_ context.Context, id shared.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.templates[id]; !ok {
		return shared.NewError(shared.CodeNotFound, "jenkins job template not found")
	}
	for _, pipeline := range r.pipelines {
		if pipeline.TemplateID == id.String() {
			return shared.NewError(shared.CodeFailedPrecondition, "build type is used by application")
		}
	}
	delete(r.templates, id)
	return nil
}

func (r *MemoryRepository) GetJenkinsJobTemplate(_ context.Context, id shared.ID) (JenkinsJobTemplate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	template, ok := r.templates[id]
	if !ok {
		return JenkinsJobTemplate{}, shared.NewError(shared.CodeNotFound, "jenkins job template not found")
	}
	return template, nil
}

func (r *MemoryRepository) FindDefaultJenkinsJobTemplate(_ context.Context) (JenkinsJobTemplate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, template := range r.templates {
		if template.IsDefault && template.Status == JenkinsJobTemplateEnabled {
			return template, nil
		}
	}
	var fallback JenkinsJobTemplate
	for _, template := range r.templates {
		if template.Status != JenkinsJobTemplateEnabled {
			continue
		}
		if fallback.ID.IsZero() || template.CreatedAt.Before(fallback.CreatedAt) {
			fallback = template
		}
	}
	if fallback.ID.IsZero() {
		return JenkinsJobTemplate{}, shared.NewError(shared.CodeNotFound, "enabled jenkins job template not found")
	}
	return fallback, nil
}

func (r *MemoryRepository) ListJenkinsJobTemplates(_ context.Context, includeDisabled bool, page shared.PageRequest) (shared.PageResult[JenkinsJobTemplate], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	page = page.Normalize()
	items := make([]JenkinsJobTemplate, 0, len(r.templates))
	for _, template := range r.templates {
		if includeDisabled || template.Status == JenkinsJobTemplateEnabled {
			items = append(items, template)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	start := page.Offset()
	if start > len(items) {
		start = len(items)
	}
	end := start + page.PageSize
	if end > len(items) {
		end = len(items)
	}
	return shared.NewPageResult(items[start:end], int64(len(items)), page), nil
}

func (r *MemoryRepository) CreatePipeline(_ context.Context, pipeline BuildPipeline) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.pipelines[pipeline.ID]; exists {
		return shared.NewError(shared.CodeConflict, "build pipeline already exists")
	}
	if pipeline.Status == "" {
		pipeline.Status = BuildPipelineStatusActive
	}
	name := strings.TrimSpace(pipeline.Name)
	if name != "" {
		if r.pipelineByApplicationName[pipeline.ApplicationID] == nil {
			r.pipelineByApplicationName[pipeline.ApplicationID] = map[string]shared.ID{}
		}
		if existingID, exists := r.pipelineByApplicationName[pipeline.ApplicationID][name]; exists && existingID != pipeline.ID {
			existing := r.pipelines[existingID]
			if existing.Status == BuildPipelineStatusActive {
				return shared.NewError(shared.CodeConflict, "build pipeline name already exists")
			}
		}
	}
	r.pipelines[pipeline.ID] = pipeline
	r.pipelinesByApplication[pipeline.ApplicationID] = appendUniqueID(r.pipelinesByApplication[pipeline.ApplicationID], pipeline.ID)
	if name != "" && pipeline.Status == BuildPipelineStatusActive {
		r.pipelineByApplicationName[pipeline.ApplicationID][name] = pipeline.ID
	}
	return nil
}

func (r *MemoryRepository) UpdatePipeline(_ context.Context, pipeline BuildPipeline) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	previous, ok := r.pipelines[pipeline.ID]
	if !ok {
		return shared.NewError(shared.CodeNotFound, "build pipeline not found")
	}
	if previous.ApplicationID != pipeline.ApplicationID || previous.TenantID != pipeline.TenantID || previous.ProjectID != pipeline.ProjectID {
		return shared.NewError(shared.CodeInvalidArgument, "build pipeline ownership cannot be changed")
	}
	previousName := strings.TrimSpace(previous.Name)
	name := strings.TrimSpace(pipeline.Name)
	if previousName != "" && previousName != name {
		return shared.NewError(shared.CodeInvalidArgument, "build pipeline name cannot be changed")
	}
	if name != "" {
		if r.pipelineByApplicationName[pipeline.ApplicationID] == nil {
			r.pipelineByApplicationName[pipeline.ApplicationID] = map[string]shared.ID{}
		}
		if existingID, exists := r.pipelineByApplicationName[pipeline.ApplicationID][name]; exists && existingID != pipeline.ID {
			existing := r.pipelines[existingID]
			if existing.Status == BuildPipelineStatusActive {
				return shared.NewError(shared.CodeConflict, "build pipeline name already exists")
			}
		}
	}
	if name != "" && pipeline.Status == BuildPipelineStatusActive {
		r.pipelineByApplicationName[pipeline.ApplicationID][name] = pipeline.ID
	} else if name != "" {
		if existingID := r.pipelineByApplicationName[pipeline.ApplicationID][name]; existingID == pipeline.ID {
			delete(r.pipelineByApplicationName[pipeline.ApplicationID], name)
		}
	}
	r.pipelines[pipeline.ID] = pipeline
	return nil
}

func (r *MemoryRepository) GetPipeline(_ context.Context, id shared.ID) (BuildPipeline, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	pipeline, ok := r.pipelines[id]
	if !ok {
		return BuildPipeline{}, shared.NewError(shared.CodeNotFound, "build pipeline not found")
	}
	return pipeline, nil
}

func (r *MemoryRepository) FindPipelineByApplication(_ context.Context, applicationID shared.ID) (BuildPipeline, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, id := range r.pipelinesByApplication[applicationID] {
		pipeline := r.pipelines[id]
		if pipeline.Status == BuildPipelineStatusActive {
			return pipeline, nil
		}
	}
	return BuildPipeline{}, shared.NewError(shared.CodeNotFound, "build pipeline not found")
}

func (r *MemoryRepository) FindPipelineByApplicationAndName(_ context.Context, applicationID shared.ID, name string) (BuildPipeline, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.pipelineByApplicationName[applicationID][strings.TrimSpace(name)]
	if !ok {
		return BuildPipeline{}, shared.NewError(shared.CodeNotFound, "build pipeline not found")
	}
	pipeline := r.pipelines[id]
	if pipeline.Status != BuildPipelineStatusActive {
		return BuildPipeline{}, shared.NewError(shared.CodeNotFound, "build pipeline not found")
	}
	return pipeline, nil
}

func (r *MemoryRepository) ListPipelinesByApplication(_ context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[BuildPipeline], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	page = page.Normalize()
	items := make([]BuildPipeline, 0, len(r.pipelinesByApplication[applicationID]))
	for _, id := range r.pipelinesByApplication[applicationID] {
		pipeline := r.pipelines[id]
		if pipeline.Status == BuildPipelineStatusActive {
			items = append(items, pipeline)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	start := page.Offset()
	if start > len(items) {
		start = len(items)
	}
	end := start + page.PageSize
	if end > len(items) {
		end = len(items)
	}
	return shared.NewPageResult(items[start:end], int64(len(items)), page), nil
}

func (r *MemoryRepository) ReplacePipelineSources(_ context.Context, pipelineID shared.ID, sources []BuildPipelineSource) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	pipeline, ok := r.pipelines[pipelineID]
	if !ok {
		return shared.NewError(shared.CodeNotFound, "build pipeline not found")
	}
	seen := map[string]struct{}{}
	for _, source := range sources {
		if source.PipelineID != pipelineID || source.ApplicationID != pipeline.ApplicationID || source.TenantID != pipeline.TenantID || source.ProjectID != pipeline.ProjectID {
			return shared.NewError(shared.CodeInvalidArgument, "build pipeline source ownership cannot be changed")
		}
		key := strings.TrimSpace(source.Key)
		if key == "" {
			return shared.NewError(shared.CodeInvalidArgument, "source key is required")
		}
		if _, ok := seen[key]; ok {
			return shared.NewError(shared.CodeConflict, "build pipeline source already exists")
		}
		seen[key] = struct{}{}
	}
	if len(sources) == 0 {
		return shared.NewError(shared.CodeInvalidArgument, "pipeline sources are required")
	}
	r.pipelineSourcesByPipeline[pipelineID] = append([]BuildPipelineSource(nil), sources...)
	return nil
}

func (r *MemoryRepository) ListPipelineSources(_ context.Context, pipelineID shared.ID) ([]BuildPipelineSource, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.pipelines[pipelineID]; !ok {
		return nil, shared.NewError(shared.CodeNotFound, "build pipeline not found")
	}
	items := append([]BuildPipelineSource(nil), r.pipelineSourcesByPipeline[pipelineID]...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsPrimary != items[j].IsPrimary {
			return items[i].IsPrimary
		}
		return items[i].Key < items[j].Key
	})
	return items, nil
}

func (r *MemoryRepository) HasActiveRunsByPipeline(_ context.Context, pipelineID shared.ID) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.pipelines[pipelineID]; !ok {
		return false, shared.NewError(shared.CodeNotFound, "build pipeline not found")
	}
	for _, run := range r.runs {
		if run.PipelineID == pipelineID && (run.Status == BuildRunQueued || run.Status == BuildRunRunning) {
			return true, nil
		}
	}
	return false, nil
}

func (r *MemoryRepository) ListActiveRunsByPipeline(_ context.Context, pipelineID shared.ID) ([]BuildRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.pipelines[pipelineID]; !ok {
		return nil, shared.NewError(shared.CodeNotFound, "build pipeline not found")
	}
	items := make([]BuildRun, 0)
	for _, run := range r.runs {
		if run.PipelineID == pipelineID && (run.Status == BuildRunQueued || run.Status == BuildRunRunning) {
			items = append(items, run)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items, nil
}

func (r *MemoryRepository) CreateRun(_ context.Context, run BuildRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.pipelines[run.PipelineID]; !ok {
		return shared.NewError(shared.CodeNotFound, "build pipeline not found")
	}
	if _, exists := r.runs[run.ID]; exists {
		return shared.NewError(shared.CodeConflict, "build run already exists")
	}
	r.runs[run.ID] = run
	r.runsByApplication[run.ApplicationID] = append(r.runsByApplication[run.ApplicationID], run.ID)
	return nil
}

func (r *MemoryRepository) UpdateRun(_ context.Context, run BuildRun) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	previous, ok := r.runs[run.ID]
	if !ok {
		return shared.NewError(shared.CodeNotFound, "build run not found")
	}
	if previous.ApplicationID != run.ApplicationID || previous.PipelineID != run.PipelineID || previous.TenantID != run.TenantID || previous.ProjectID != run.ProjectID {
		return shared.NewError(shared.CodeInvalidArgument, "build run ownership cannot be changed")
	}
	r.runs[run.ID] = run
	return nil
}

func (r *MemoryRepository) GetRun(_ context.Context, id shared.ID) (BuildRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	run, ok := r.runs[id]
	if !ok {
		return BuildRun{}, shared.NewError(shared.CodeNotFound, "build run not found")
	}
	return run, nil
}

func (r *MemoryRepository) ListRunsByApplication(_ context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[BuildRun], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	page = page.Normalize()
	items := make([]BuildRun, 0, len(r.runsByApplication[applicationID]))
	for _, id := range r.runsByApplication[applicationID] {
		items = append(items, r.runs[id])
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	start := page.Offset()
	if start > len(items) {
		start = len(items)
	}
	end := start + page.PageSize
	if end > len(items) {
		end = len(items)
	}
	return shared.NewPageResult(items[start:end], int64(len(items)), page), nil
}

func (r *MemoryRepository) CreateRunSource(_ context.Context, source BuildRunSource) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.runs[source.BuildRunID]; !ok {
		return shared.NewError(shared.CodeNotFound, "build run not found")
	}
	for _, existing := range r.runSourcesByRun[source.BuildRunID] {
		if existing.SourceKey == source.SourceKey {
			return shared.NewError(shared.CodeConflict, "build run source already exists")
		}
	}
	r.runSourcesByRun[source.BuildRunID] = append(r.runSourcesByRun[source.BuildRunID], source)
	return nil
}

func (r *MemoryRepository) ListRunSources(_ context.Context, buildRunID shared.ID) ([]BuildRunSource, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.runs[buildRunID]; !ok {
		return nil, shared.NewError(shared.CodeNotFound, "build run not found")
	}
	items := append([]BuildRunSource(nil), r.runSourcesByRun[buildRunID]...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsPrimary != items[j].IsPrimary {
			return items[i].IsPrimary
		}
		return items[i].SourceKey < items[j].SourceKey
	})
	return items, nil
}

func (r *MemoryRepository) CreateArtifact(_ context.Context, artifact BuildArtifact) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.runs[artifact.BuildRunID]; !ok {
		return shared.NewError(shared.CodeNotFound, "build run not found")
	}
	if _, exists := r.artifacts[artifact.ID]; exists {
		return shared.NewError(shared.CodeConflict, "build artifact already exists")
	}
	r.artifacts[artifact.ID] = artifact
	r.artifactsByRun[artifact.BuildRunID] = append(r.artifactsByRun[artifact.BuildRunID], artifact.ID)
	return nil
}

func (r *MemoryRepository) GetArtifact(_ context.Context, id shared.ID) (BuildArtifact, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	artifact, ok := r.artifacts[id]
	if !ok {
		return BuildArtifact{}, shared.NewError(shared.CodeNotFound, "build artifact not found")
	}
	return artifact, nil
}

func (r *MemoryRepository) ListArtifactsByRun(_ context.Context, buildRunID shared.ID) ([]BuildArtifact, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.runs[buildRunID]; !ok {
		return nil, shared.NewError(shared.CodeNotFound, "build run not found")
	}
	items := make([]BuildArtifact, 0, len(r.artifactsByRun[buildRunID]))
	for _, id := range r.artifactsByRun[buildRunID] {
		items = append(items, r.artifacts[id])
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return items, nil
}

func (r *MemoryRepository) AppendBuildLog(_ context.Context, buildRunID shared.ID, text string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.runs[buildRunID]; !ok {
		return shared.NewError(shared.CodeNotFound, "build run not found")
	}
	if text != "" {
		r.logsByRun[buildRunID] = append(r.logsByRun[buildRunID], text)
	}
	return nil
}

func (r *MemoryRepository) ListBuildLogs(_ context.Context, buildRunID shared.ID) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.runs[buildRunID]; !ok {
		return nil, shared.NewError(shared.CodeNotFound, "build run not found")
	}
	return append([]string(nil), r.logsByRun[buildRunID]...), nil
}
