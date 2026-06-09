package build

import (
	"context"
	"database/sql"
	"time"

	"github.com/shareinto/paas/internal/platform/database"
	"github.com/shareinto/paas/internal/shared"
)

type MySQLRepository struct {
	*MemoryRepository
	db    *sql.DB
	store *database.SnapshotStore
}

const maxBuildLogChunkBytes = 60 * 1024

type buildSnapshot struct {
	BuildEnvironments   []BuildEnvironment
	RuntimeEnvironments []RuntimeEnvironment
	BuildTemplate       BuildTemplate
	Templates           []JenkinsJobTemplate
	Pipelines           []BuildPipeline
	PipelineSources     []BuildPipelineSource
	Runs                []BuildRun
	RunSources          []BuildRunSource
	Artifacts           []BuildArtifact
}

func NewMySQLRepository(ctx context.Context, db *sql.DB) (*MySQLRepository, error) {
	repo := &MySQLRepository{MemoryRepository: NewMemoryRepository(), db: db, store: database.NewSnapshotStore(db, "build")}
	var snapshot buildSnapshot
	if err := repo.store.Load(ctx, &snapshot); err != nil {
		return nil, err
	}
	for _, environment := range snapshot.BuildEnvironments {
		repo.buildEnvironments[environment.ID] = environment
	}
	for _, environment := range snapshot.RuntimeEnvironments {
		repo.runtimeEnvironments[environment.ID] = environment
	}
	repo.buildTemplate = snapshot.BuildTemplate
	for _, template := range snapshot.Templates {
		repo.templates[template.ID] = template
	}
	for _, pipeline := range snapshot.Pipelines {
		repo.pipelines[pipeline.ID] = pipeline
		repo.pipelinesByApplication[pipeline.ApplicationID] = appendUniqueID(repo.pipelinesByApplication[pipeline.ApplicationID], pipeline.ID)
		if pipeline.Status == BuildPipelineStatusActive && pipeline.Name != "" {
			if repo.pipelineByApplicationName[pipeline.ApplicationID] == nil {
				repo.pipelineByApplicationName[pipeline.ApplicationID] = map[string]shared.ID{}
			}
			repo.pipelineByApplicationName[pipeline.ApplicationID][pipeline.Name] = pipeline.ID
		}
	}
	for _, source := range snapshot.PipelineSources {
		repo.pipelineSourcesByPipeline[source.PipelineID] = append(repo.pipelineSourcesByPipeline[source.PipelineID], source)
	}
	for _, run := range snapshot.Runs {
		repo.runs[run.ID] = run
		repo.runsByApplication[run.ApplicationID] = append(repo.runsByApplication[run.ApplicationID], run.ID)
	}
	for _, source := range snapshot.RunSources {
		repo.runSourcesByRun[source.BuildRunID] = append(repo.runSourcesByRun[source.BuildRunID], source)
	}
	for _, artifact := range snapshot.Artifacts {
		repo.artifacts[artifact.ID] = artifact
		repo.artifactsByRun[artifact.BuildRunID] = append(repo.artifactsByRun[artifact.BuildRunID], artifact.ID)
	}
	return repo, nil
}

func (r *MySQLRepository) snapshot() buildSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := buildSnapshot{BuildEnvironments: make([]BuildEnvironment, 0, len(r.buildEnvironments)), RuntimeEnvironments: make([]RuntimeEnvironment, 0, len(r.runtimeEnvironments)), BuildTemplate: r.buildTemplate, Templates: make([]JenkinsJobTemplate, 0, len(r.templates)), Pipelines: make([]BuildPipeline, 0, len(r.pipelines)), PipelineSources: []BuildPipelineSource{}, Runs: make([]BuildRun, 0, len(r.runs)), RunSources: []BuildRunSource{}, Artifacts: make([]BuildArtifact, 0, len(r.artifacts))}
	for _, v := range r.buildEnvironments {
		out.BuildEnvironments = append(out.BuildEnvironments, v)
	}
	for _, v := range r.runtimeEnvironments {
		out.RuntimeEnvironments = append(out.RuntimeEnvironments, v)
	}
	for _, v := range r.templates {
		out.Templates = append(out.Templates, v)
	}
	for _, v := range r.pipelines {
		out.Pipelines = append(out.Pipelines, v)
	}
	for _, sources := range r.pipelineSourcesByPipeline {
		out.PipelineSources = append(out.PipelineSources, sources...)
	}
	for _, v := range r.runs {
		out.Runs = append(out.Runs, v)
	}
	for _, sources := range r.runSourcesByRun {
		out.RunSources = append(out.RunSources, sources...)
	}
	for _, v := range r.artifacts {
		out.Artifacts = append(out.Artifacts, v)
	}
	return out
}

func (r *MySQLRepository) persist(ctx context.Context) error { return r.store.Save(ctx, r.snapshot()) }
func (r *MySQLRepository) CreateBuildEnvironment(ctx context.Context, environment BuildEnvironment) error {
	if err := r.MemoryRepository.CreateBuildEnvironment(ctx, environment); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) UpdateBuildEnvironment(ctx context.Context, environment BuildEnvironment) error {
	if err := r.MemoryRepository.UpdateBuildEnvironment(ctx, environment); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) DeleteBuildEnvironment(ctx context.Context, id shared.ID) error {
	if err := r.MemoryRepository.DeleteBuildEnvironment(ctx, id); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreateRuntimeEnvironment(ctx context.Context, environment RuntimeEnvironment) error {
	if err := r.MemoryRepository.CreateRuntimeEnvironment(ctx, environment); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) UpdateRuntimeEnvironment(ctx context.Context, environment RuntimeEnvironment) error {
	if err := r.MemoryRepository.UpdateRuntimeEnvironment(ctx, environment); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) DeleteRuntimeEnvironment(ctx context.Context, id shared.ID) error {
	if err := r.MemoryRepository.DeleteRuntimeEnvironment(ctx, id); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) SaveBuildTemplate(ctx context.Context, template BuildTemplate) error {
	if err := r.MemoryRepository.SaveBuildTemplate(ctx, template); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreateJenkinsJobTemplate(ctx context.Context, template JenkinsJobTemplate) error {
	if err := r.MemoryRepository.CreateJenkinsJobTemplate(ctx, template); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) UpdateJenkinsJobTemplate(ctx context.Context, template JenkinsJobTemplate) error {
	if err := r.MemoryRepository.UpdateJenkinsJobTemplate(ctx, template); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) DeleteJenkinsJobTemplate(ctx context.Context, id shared.ID) error {
	if err := r.MemoryRepository.DeleteJenkinsJobTemplate(ctx, id); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreatePipeline(ctx context.Context, pipeline BuildPipeline) error {
	if err := r.MemoryRepository.CreatePipeline(ctx, pipeline); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) UpdatePipeline(ctx context.Context, pipeline BuildPipeline) error {
	if err := r.MemoryRepository.UpdatePipeline(ctx, pipeline); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) ReplacePipelineSources(ctx context.Context, pipelineID shared.ID, sources []BuildPipelineSource) error {
	if err := r.MemoryRepository.ReplacePipelineSources(ctx, pipelineID, sources); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreateRun(ctx context.Context, run BuildRun) error {
	if err := r.MemoryRepository.CreateRun(ctx, run); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreateRunSource(ctx context.Context, source BuildRunSource) error {
	if err := r.MemoryRepository.CreateRunSource(ctx, source); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) UpdateRun(ctx context.Context, run BuildRun) error {
	if err := r.MemoryRepository.UpdateRun(ctx, run); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreateArtifact(ctx context.Context, artifact BuildArtifact) error {
	if err := r.MemoryRepository.CreateArtifact(ctx, artifact); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) AppendBuildLog(ctx context.Context, buildRunID shared.ID, text string) error {
	r.mu.RLock()
	_, ok := r.runs[buildRunID]
	r.mu.RUnlock()
	if !ok {
		return shared.NewError(shared.CodeNotFound, "build run not found")
	}
	if text == "" {
		return nil
	}
	for _, chunk := range splitBuildLogText(text, maxBuildLogChunkBytes) {
		_, err := r.db.ExecContext(ctx, "INSERT INTO build_logs(build_run_id, log_text, created_at) VALUES (?, ?, ?)", buildRunID, chunk, time.Now().UTC())
		if err != nil {
			return shared.WrapError(shared.CodeUnavailable, "append build log failed", err)
		}
	}
	return nil
}

func splitBuildLogText(text string, maxBytes int) []string {
	if text == "" {
		return nil
	}
	if maxBytes <= 0 || len(text) <= maxBytes {
		return []string{text}
	}
	chunks := make([]string, 0, len(text)/maxBytes+1)
	start := 0
	size := 0
	for offset, r := range text {
		runeSize := len(string(r))
		if size > 0 && size+runeSize > maxBytes {
			chunks = append(chunks, text[start:offset])
			start = offset
			size = 0
		}
		size += runeSize
	}
	if start < len(text) {
		chunks = append(chunks, text[start:])
	}
	return chunks
}

func (r *MySQLRepository) ListBuildLogs(ctx context.Context, buildRunID shared.ID) ([]string, error) {
	r.mu.RLock()
	_, ok := r.runs[buildRunID]
	r.mu.RUnlock()
	if !ok {
		return nil, shared.NewError(shared.CodeNotFound, "build run not found")
	}
	rows, err := r.db.QueryContext(ctx, "SELECT log_text FROM build_logs WHERE build_run_id = ? ORDER BY id", buildRunID)
	if err != nil {
		return nil, shared.WrapError(shared.CodeUnavailable, "list build logs failed", err)
	}
	defer rows.Close()
	logs := []string{}
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			return nil, shared.WrapError(shared.CodeUnavailable, "scan build log failed", err)
		}
		logs = append(logs, text)
	}
	if err := rows.Err(); err != nil {
		return nil, shared.WrapError(shared.CodeUnavailable, "list build logs failed", err)
	}
	return logs, nil
}
