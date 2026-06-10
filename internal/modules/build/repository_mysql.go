package build

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/shareinto/paas/internal/platform/database"
	"github.com/shareinto/paas/internal/shared"
)

type MySQLRepository struct {
	db *sql.DB
}

const maxBuildLogChunkBytes = 60 * 1024

func NewMySQLRepository(_ context.Context, db *sql.DB) (*MySQLRepository, error) {
	return &MySQLRepository{db: db}, nil
}

func (r *MySQLRepository) CreateBuildEnvironment(ctx context.Context, environment BuildEnvironment) error {
	if environment.IsDefault {
		if _, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, "UPDATE build_environments SET is_default = 0 WHERE status <> ?", BuildEnvironmentDeleted); err != nil {
			return database.WrapUnavailable(err, "unset default build environment failed")
		}
	}
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO build_environments (id, name, description, build_image, status, is_default, created_by, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		environment.ID, environment.Name, environment.Description, environment.BuildImage, environment.Status,
		environment.IsDefault, environment.CreatedBy, mysqlTime(environment.CreatedAt), mysqlTime(environment.UpdatedAt))
	return database.ConflictOrUnavailable(err, "build environment already exists", "create build environment failed")
}

func (r *MySQLRepository) UpdateBuildEnvironment(ctx context.Context, environment BuildEnvironment) error {
	if _, err := r.GetBuildEnvironment(ctx, environment.ID); err != nil {
		return err
	}
	if environment.IsDefault {
		if _, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, "UPDATE build_environments SET is_default = 0 WHERE id <> ? AND status <> ?", environment.ID, BuildEnvironmentDeleted); err != nil {
			return database.WrapUnavailable(err, "unset default build environment failed")
		}
	}
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
UPDATE build_environments
SET name = ?, description = ?, build_image = ?, status = ?, is_default = ?, created_by = ?, created_at = ?, updated_at = ?
WHERE id = ?`,
		environment.Name, environment.Description, environment.BuildImage, environment.Status, environment.IsDefault,
		environment.CreatedBy, mysqlTime(environment.CreatedAt), mysqlTime(environment.UpdatedAt), environment.ID)
	if err != nil {
		return database.ConflictOrUnavailable(err, "build environment already exists", "update build environment failed")
	}
	return nil
}

func (r *MySQLRepository) DeleteBuildEnvironment(ctx context.Context, id shared.ID) error {
	result, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
UPDATE build_environments SET status = ?, is_default = 0 WHERE id = ? AND status <> ?`,
		BuildEnvironmentDeleted, id, BuildEnvironmentDeleted)
	if err != nil {
		return database.WrapUnavailable(err, "delete build environment failed")
	}
	return database.RequireAffected(result, "build environment not found")
}

func (r *MySQLRepository) GetBuildEnvironment(ctx context.Context, id shared.ID) (BuildEnvironment, error) {
	environment, err := scanBuildEnvironment(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, buildEnvironmentSelect()+" WHERE id = ?", id))
	if err != nil {
		return BuildEnvironment{}, database.NotFound(err, "build environment not found")
	}
	return environment, nil
}

func (r *MySQLRepository) FindDefaultBuildEnvironment(ctx context.Context) (BuildEnvironment, error) {
	environment, err := scanBuildEnvironment(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, buildEnvironmentSelect()+`
 WHERE status = ? ORDER BY is_default DESC, created_at ASC LIMIT 1`, BuildEnvironmentEnabled))
	if err != nil {
		return BuildEnvironment{}, database.NotFound(err, "enabled build environment not found")
	}
	return environment, nil
}

func (r *MySQLRepository) ListBuildEnvironments(ctx context.Context, includeDisabled bool, page shared.PageRequest) (shared.PageResult[BuildEnvironment], error) {
	where := "status <> ?"
	args := []any{BuildEnvironmentDeleted}
	if !includeDisabled {
		where = "status = ?"
		args = []any{BuildEnvironmentEnabled}
	}
	return listPage(ctx, r.db, page, "build_environments", buildEnvironmentSelect(), where, args, "is_default DESC, created_at ASC", scanBuildEnvironment)
}

func (r *MySQLRepository) CreateRuntimeEnvironment(ctx context.Context, environment RuntimeEnvironment) error {
	if environment.IsDefault {
		if _, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, "UPDATE runtime_environments SET is_default = 0 WHERE status <> ?", RuntimeEnvironmentDeleted); err != nil {
			return database.WrapUnavailable(err, "unset default runtime environment failed")
		}
	}
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO runtime_environments (id, name, description, runtime_base_image, artifact_deploy_path, dockerfile_path, status, is_default, created_by, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		environment.ID, environment.Name, environment.Description, environment.RuntimeBaseImage, environment.ArtifactDeployPath,
		environment.DockerfilePath, environment.Status, environment.IsDefault, environment.CreatedBy, mysqlTime(environment.CreatedAt), mysqlTime(environment.UpdatedAt))
	return database.ConflictOrUnavailable(err, "runtime environment already exists", "create runtime environment failed")
}

func (r *MySQLRepository) UpdateRuntimeEnvironment(ctx context.Context, environment RuntimeEnvironment) error {
	if _, err := r.GetRuntimeEnvironment(ctx, environment.ID); err != nil {
		return err
	}
	if environment.IsDefault {
		if _, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, "UPDATE runtime_environments SET is_default = 0 WHERE id <> ? AND status <> ?", environment.ID, RuntimeEnvironmentDeleted); err != nil {
			return database.WrapUnavailable(err, "unset default runtime environment failed")
		}
	}
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
UPDATE runtime_environments
SET name = ?, description = ?, runtime_base_image = ?, artifact_deploy_path = ?, dockerfile_path = ?,
    status = ?, is_default = ?, created_by = ?, created_at = ?, updated_at = ?
WHERE id = ?`,
		environment.Name, environment.Description, environment.RuntimeBaseImage, environment.ArtifactDeployPath,
		environment.DockerfilePath, environment.Status, environment.IsDefault, environment.CreatedBy,
		mysqlTime(environment.CreatedAt), mysqlTime(environment.UpdatedAt), environment.ID)
	if err != nil {
		return database.ConflictOrUnavailable(err, "runtime environment already exists", "update runtime environment failed")
	}
	return nil
}

func (r *MySQLRepository) DeleteRuntimeEnvironment(ctx context.Context, id shared.ID) error {
	result, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
UPDATE runtime_environments SET status = ?, is_default = 0 WHERE id = ? AND status <> ?`,
		RuntimeEnvironmentDeleted, id, RuntimeEnvironmentDeleted)
	if err != nil {
		return database.WrapUnavailable(err, "delete runtime environment failed")
	}
	return database.RequireAffected(result, "runtime environment not found")
}

func (r *MySQLRepository) GetRuntimeEnvironment(ctx context.Context, id shared.ID) (RuntimeEnvironment, error) {
	environment, err := scanRuntimeEnvironment(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, runtimeEnvironmentSelect()+" WHERE id = ?", id))
	if err != nil {
		return RuntimeEnvironment{}, database.NotFound(err, "runtime environment not found")
	}
	return environment, nil
}

func (r *MySQLRepository) FindDefaultRuntimeEnvironment(ctx context.Context) (RuntimeEnvironment, error) {
	environment, err := scanRuntimeEnvironment(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, runtimeEnvironmentSelect()+`
 WHERE status = ? ORDER BY is_default DESC, created_at ASC LIMIT 1`, RuntimeEnvironmentEnabled))
	if err != nil {
		return RuntimeEnvironment{}, database.NotFound(err, "enabled runtime environment not found")
	}
	return environment, nil
}

func (r *MySQLRepository) ListRuntimeEnvironments(ctx context.Context, includeDisabled bool, page shared.PageRequest) (shared.PageResult[RuntimeEnvironment], error) {
	where := "status <> ?"
	args := []any{RuntimeEnvironmentDeleted}
	if !includeDisabled {
		where = "status = ?"
		args = []any{RuntimeEnvironmentEnabled}
	}
	return listPage(ctx, r.db, page, "runtime_environments", runtimeEnvironmentSelect(), where, args, "is_default DESC, created_at ASC", scanRuntimeEnvironment)
}

func (r *MySQLRepository) GetBuildTemplate(ctx context.Context) (BuildTemplate, error) {
	template, err := scanBuildTemplate(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, buildTemplateSelect()+" ORDER BY updated_at DESC, created_at DESC LIMIT 1"))
	if err != nil {
		return BuildTemplate{}, database.NotFound(err, "build template not found")
	}
	return template, nil
}

func (r *MySQLRepository) SaveBuildTemplate(ctx context.Context, template BuildTemplate) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO build_templates (id, name, version, content, created_by, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE name = VALUES(name), version = VALUES(version), content = VALUES(content),
 created_by = VALUES(created_by), created_at = VALUES(created_at), updated_at = VALUES(updated_at)`,
		template.ID, template.Name, template.Version, template.Content, template.CreatedBy, mysqlTime(template.CreatedAt), mysqlTime(template.UpdatedAt))
	return database.WrapUnavailable(err, "save build template failed")
}

func (r *MySQLRepository) CreateJenkinsJobTemplate(ctx context.Context, template JenkinsJobTemplate) error {
	if template.IsDefault {
		if _, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, "UPDATE jenkins_job_templates SET is_default = 0"); err != nil {
			return database.WrapUnavailable(err, "unset default jenkins job template failed")
		}
	}
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO jenkins_job_templates (id, name, display_name, description, runtime_base_image, version, xml_content, status, is_default, created_by, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		template.ID, template.Name, template.DisplayName, template.Description, template.RuntimeBaseImage,
		template.Version, template.XMLContent, template.Status, template.IsDefault, template.CreatedBy, mysqlTime(template.CreatedAt), mysqlTime(template.UpdatedAt))
	return database.ConflictOrUnavailable(err, "jenkins job template already exists", "create jenkins job template failed")
}

func (r *MySQLRepository) UpdateJenkinsJobTemplate(ctx context.Context, template JenkinsJobTemplate) error {
	if _, err := r.GetJenkinsJobTemplate(ctx, template.ID); err != nil {
		return err
	}
	if template.IsDefault {
		if _, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, "UPDATE jenkins_job_templates SET is_default = 0 WHERE id <> ?", template.ID); err != nil {
			return database.WrapUnavailable(err, "unset default jenkins job template failed")
		}
	}
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
UPDATE jenkins_job_templates
SET name = ?, display_name = ?, description = ?, runtime_base_image = ?, version = ?, xml_content = ?,
    status = ?, is_default = ?, created_by = ?, created_at = ?, updated_at = ?
WHERE id = ?`,
		template.Name, template.DisplayName, template.Description, template.RuntimeBaseImage, template.Version,
		template.XMLContent, template.Status, template.IsDefault, template.CreatedBy, mysqlTime(template.CreatedAt), mysqlTime(template.UpdatedAt), template.ID)
	if err != nil {
		return database.ConflictOrUnavailable(err, "jenkins job template already exists", "update jenkins job template failed")
	}
	return nil
}

func (r *MySQLRepository) DeleteJenkinsJobTemplate(ctx context.Context, id shared.ID) error {
	if _, err := r.GetJenkinsJobTemplate(ctx, id); err != nil {
		return err
	}
	var count int64
	if err := database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, "SELECT COUNT(*) FROM build_pipelines WHERE template_id = ?", id.String()).Scan(&count); err != nil {
		return database.WrapUnavailable(err, "check jenkins job template usage failed")
	}
	if count > 0 {
		return shared.NewError(shared.CodeFailedPrecondition, "build type is used by application")
	}
	result, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, "DELETE FROM jenkins_job_templates WHERE id = ?", id)
	if err != nil {
		return database.WrapUnavailable(err, "delete jenkins job template failed")
	}
	return database.RequireAffected(result, "jenkins job template not found")
}

func (r *MySQLRepository) GetJenkinsJobTemplate(ctx context.Context, id shared.ID) (JenkinsJobTemplate, error) {
	template, err := scanJenkinsJobTemplate(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, jenkinsJobTemplateSelect()+" WHERE id = ?", id))
	if err != nil {
		return JenkinsJobTemplate{}, database.NotFound(err, "jenkins job template not found")
	}
	return template, nil
}

func (r *MySQLRepository) FindDefaultJenkinsJobTemplate(ctx context.Context) (JenkinsJobTemplate, error) {
	template, err := scanJenkinsJobTemplate(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, jenkinsJobTemplateSelect()+`
 WHERE status = ? ORDER BY is_default DESC, created_at ASC LIMIT 1`, JenkinsJobTemplateEnabled))
	if err != nil {
		return JenkinsJobTemplate{}, database.NotFound(err, "enabled jenkins job template not found")
	}
	return template, nil
}

func (r *MySQLRepository) ListJenkinsJobTemplates(ctx context.Context, includeDisabled bool, page shared.PageRequest) (shared.PageResult[JenkinsJobTemplate], error) {
	where := "1 = 1"
	args := []any{}
	if !includeDisabled {
		where = "status = ?"
		args = []any{JenkinsJobTemplateEnabled}
	}
	return listPage(ctx, r.db, page, "jenkins_job_templates", jenkinsJobTemplateSelect(), where, args, "created_at DESC", scanJenkinsJobTemplate)
}

func (r *MySQLRepository) CreatePipeline(ctx context.Context, pipeline BuildPipeline) error {
	if pipeline.Status == "" {
		pipeline.Status = BuildPipelineStatusActive
	}
	if err := r.ensurePipelineNameAvailable(ctx, pipeline.ID, pipeline.ApplicationID, pipeline.Name); err != nil {
		return err
	}
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO build_pipelines (id, tenant_id, project_id, application_id, name, display_name, description, provider, external_job_name, template_id, config_hash, status, managed_by_platform, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		pipeline.ID, pipeline.TenantID, pipeline.ProjectID, pipeline.ApplicationID, strings.TrimSpace(pipeline.Name),
		pipeline.DisplayName, pipeline.Description, pipeline.Provider, pipeline.ExternalJobName, pipeline.TemplateID,
		pipeline.ConfigHash, pipeline.Status, pipeline.ManagedByPlatform, mysqlTime(pipeline.CreatedAt), mysqlTime(pipeline.UpdatedAt))
	return database.ConflictOrUnavailable(err, "build pipeline already exists", "create build pipeline failed")
}

func (r *MySQLRepository) UpdatePipeline(ctx context.Context, pipeline BuildPipeline) error {
	previous, err := r.GetPipeline(ctx, pipeline.ID)
	if err != nil {
		return err
	}
	if previous.ApplicationID != pipeline.ApplicationID || previous.TenantID != pipeline.TenantID || previous.ProjectID != pipeline.ProjectID {
		return shared.NewError(shared.CodeInvalidArgument, "build pipeline ownership cannot be changed")
	}
	if strings.TrimSpace(previous.Name) != "" && strings.TrimSpace(previous.Name) != strings.TrimSpace(pipeline.Name) {
		return shared.NewError(shared.CodeInvalidArgument, "build pipeline name cannot be changed")
	}
	if err := r.ensurePipelineNameAvailable(ctx, pipeline.ID, pipeline.ApplicationID, pipeline.Name); err != nil {
		return err
	}
	_, err = database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
UPDATE build_pipelines
SET name = ?, display_name = ?, description = ?, provider = ?, external_job_name = ?, template_id = ?,
    config_hash = ?, status = ?, managed_by_platform = ?, created_at = ?, updated_at = ?
WHERE id = ?`,
		strings.TrimSpace(pipeline.Name), pipeline.DisplayName, pipeline.Description, pipeline.Provider,
		pipeline.ExternalJobName, pipeline.TemplateID, pipeline.ConfigHash, pipeline.Status, pipeline.ManagedByPlatform,
		mysqlTime(pipeline.CreatedAt), mysqlTime(pipeline.UpdatedAt), pipeline.ID)
	if err != nil {
		return database.WrapUnavailable(err, "update build pipeline failed")
	}
	return nil
}

func (r *MySQLRepository) GetPipeline(ctx context.Context, id shared.ID) (BuildPipeline, error) {
	pipeline, err := scanBuildPipeline(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, buildPipelineSelect()+" WHERE id = ?", id))
	if err != nil {
		return BuildPipeline{}, database.NotFound(err, "build pipeline not found")
	}
	pipeline.RuntimeEnvironments, _ = r.listPipelineRuntimeEnvironments(ctx, pipeline.ID)
	return pipeline, nil
}

func (r *MySQLRepository) FindPipelineByApplication(ctx context.Context, applicationID shared.ID) (BuildPipeline, error) {
	pipeline, err := scanBuildPipeline(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, buildPipelineSelect()+`
 WHERE application_id = ? AND status = ? ORDER BY created_at ASC, id ASC LIMIT 1`, applicationID, BuildPipelineStatusActive))
	if err != nil {
		return BuildPipeline{}, database.NotFound(err, "build pipeline not found")
	}
	pipeline.RuntimeEnvironments, _ = r.listPipelineRuntimeEnvironments(ctx, pipeline.ID)
	return pipeline, nil
}

func (r *MySQLRepository) FindPipelineByApplicationAndName(ctx context.Context, applicationID shared.ID, name string) (BuildPipeline, error) {
	pipeline, err := scanBuildPipeline(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, buildPipelineSelect()+`
 WHERE application_id = ? AND name = ? AND status = ? LIMIT 1`, applicationID, strings.TrimSpace(name), BuildPipelineStatusActive))
	if err != nil {
		return BuildPipeline{}, database.NotFound(err, "build pipeline not found")
	}
	pipeline.RuntimeEnvironments, _ = r.listPipelineRuntimeEnvironments(ctx, pipeline.ID)
	return pipeline, nil
}

func (r *MySQLRepository) ListPipelinesByApplication(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[BuildPipeline], error) {
	result, err := listPage(ctx, r.db, page, "build_pipelines", buildPipelineSelect(), "application_id = ? AND status = ?", []any{applicationID, BuildPipelineStatusActive}, "created_at DESC, id DESC", scanBuildPipeline)
	if err != nil {
		return result, err
	}
	for i := range result.Items {
		result.Items[i].RuntimeEnvironments, _ = r.listPipelineRuntimeEnvironments(ctx, result.Items[i].ID)
	}
	return result, nil
}

func (r *MySQLRepository) ReplacePipelineRuntimeEnvironments(ctx context.Context, pipelineID shared.ID, runtimes []RuntimeEnvironmentRef) error {
	if _, err := r.GetPipeline(ctx, pipelineID); err != nil {
		return err
	}
	if len(runtimes) == 0 {
		return shared.NewError(shared.CodeInvalidArgument, "runtime_environment_ids is required")
	}
	return r.withTx(ctx, func(txCtx context.Context) error {
		exec := database.ExecutorFromContext(txCtx, r.db)
		if _, err := exec.ExecContext(txCtx, "DELETE FROM build_pipeline_runtime_environments WHERE pipeline_id = ?", pipelineID); err != nil {
			return database.WrapUnavailable(err, "replace build pipeline runtime environments failed")
		}
		for i, runtime := range runtimes {
			if runtime.ID.IsZero() {
				return shared.NewError(shared.CodeInvalidArgument, "runtime_environment_id is required")
			}
			if _, err := exec.ExecContext(txCtx, `
INSERT INTO build_pipeline_runtime_environments (pipeline_id, runtime_environment_id, name, runtime_base_image, artifact_deploy_path, dockerfile_path, position)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
				pipelineID, runtime.ID, runtime.Name, runtime.RuntimeBaseImage, runtime.ArtifactDeployPath, runtime.DockerfilePath, i); err != nil {
				return database.WrapUnavailable(err, "replace build pipeline runtime environments failed")
			}
		}
		return nil
	})
}

func (r *MySQLRepository) ListPipelineRuntimeEnvironments(ctx context.Context, pipelineID shared.ID) ([]RuntimeEnvironmentRef, error) {
	if _, err := r.GetPipeline(ctx, pipelineID); err != nil {
		return nil, err
	}
	return r.listPipelineRuntimeEnvironments(ctx, pipelineID)
}

func (r *MySQLRepository) listPipelineRuntimeEnvironments(ctx context.Context, pipelineID shared.ID) ([]RuntimeEnvironmentRef, error) {
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, `
SELECT runtime_environment_id, name, runtime_base_image, artifact_deploy_path, dockerfile_path
FROM build_pipeline_runtime_environments WHERE pipeline_id = ? ORDER BY position ASC, runtime_environment_id ASC`, pipelineID)
	if err != nil {
		return nil, database.WrapUnavailable(err, "list build pipeline runtime environments failed")
	}
	defer rows.Close()
	items := []RuntimeEnvironmentRef{}
	for rows.Next() {
		var runtime RuntimeEnvironmentRef
		if err := rows.Scan(&runtime.ID, &runtime.Name, &runtime.RuntimeBaseImage, &runtime.ArtifactDeployPath, &runtime.DockerfilePath); err != nil {
			return nil, database.WrapUnavailable(err, "scan build pipeline runtime environment failed")
		}
		items = append(items, runtime)
	}
	return items, rows.Err()
}

func (r *MySQLRepository) ReplacePipelineSources(ctx context.Context, pipelineID shared.ID, sources []BuildPipelineSource) error {
	pipeline, err := r.GetPipeline(ctx, pipelineID)
	if err != nil {
		return err
	}
	if len(sources) == 0 {
		return shared.NewError(shared.CodeInvalidArgument, "pipeline sources are required")
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
	return r.withTx(ctx, func(txCtx context.Context) error {
		exec := database.ExecutorFromContext(txCtx, r.db)
		if _, err := exec.ExecContext(txCtx, "DELETE FROM build_pipeline_sources WHERE pipeline_id = ?", pipelineID); err != nil {
			return database.WrapUnavailable(err, "replace build pipeline sources failed")
		}
		for _, source := range sources {
			if err := r.insertPipelineSource(txCtx, source); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *MySQLRepository) ListPipelineSources(ctx context.Context, pipelineID shared.ID) ([]BuildPipelineSource, error) {
	if _, err := r.GetPipeline(ctx, pipelineID); err != nil {
		return nil, err
	}
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, buildPipelineSourceSelect()+`
 WHERE pipeline_id = ? ORDER BY is_primary DESC, source_key ASC`, pipelineID)
	if err != nil {
		return nil, database.WrapUnavailable(err, "list build pipeline sources failed")
	}
	defer rows.Close()
	return scanRows(rows, scanBuildPipelineSource, "list build pipeline sources failed")
}

func (r *MySQLRepository) HasActiveRunsByPipeline(ctx context.Context, pipelineID shared.ID) (bool, error) {
	if _, err := r.GetPipeline(ctx, pipelineID); err != nil {
		return false, err
	}
	var count int64
	if err := database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT COUNT(*) FROM build_runs WHERE pipeline_id = ? AND status IN (?, ?)`,
		pipelineID, BuildRunQueued, BuildRunRunning).Scan(&count); err != nil {
		return false, database.WrapUnavailable(err, "count active build runs failed")
	}
	return count > 0, nil
}

func (r *MySQLRepository) ListActiveRunsByPipeline(ctx context.Context, pipelineID shared.ID) ([]BuildRun, error) {
	if _, err := r.GetPipeline(ctx, pipelineID); err != nil {
		return nil, err
	}
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, buildRunSelect()+`
 WHERE pipeline_id = ? AND status IN (?, ?) ORDER BY created_at ASC`,
		pipelineID, BuildRunQueued, BuildRunRunning)
	if err != nil {
		return nil, database.WrapUnavailable(err, "list active build runs failed")
	}
	defer rows.Close()
	return scanRows(rows, scanBuildRun, "list active build runs failed")
}

func (r *MySQLRepository) CreateRun(ctx context.Context, run BuildRun) error {
	if _, err := r.GetPipeline(ctx, run.PipelineID); err != nil {
		return err
	}
	return r.insertRun(ctx, run)
}

func (r *MySQLRepository) CreateRunWithSources(ctx context.Context, run BuildRun, sources []BuildRunSource) error {
	if _, err := r.GetPipeline(ctx, run.PipelineID); err != nil {
		return err
	}
	return r.withTx(ctx, func(txCtx context.Context) error {
		if err := r.insertRun(txCtx, run); err != nil {
			return err
		}
		for _, source := range sources {
			if source.BuildRunID != run.ID || source.ApplicationID != run.ApplicationID || source.TenantID != run.TenantID || source.ProjectID != run.ProjectID {
				return shared.NewError(shared.CodeInvalidArgument, "build run source ownership cannot be changed")
			}
			if err := r.insertRunSource(txCtx, source); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *MySQLRepository) insertRun(ctx context.Context, run BuildRun) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO build_runs (id, tenant_id, project_id, pipeline_id, pipeline_name, pipeline_display_name, application_id, source_repository_id, git_ref, commit_sha, status, jenkins_queue_id, jenkins_build_number, primary_artifact_id, log_offset, error_message, requested_by, created_at, updated_at, started_at, finished_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.TenantID, run.ProjectID, run.PipelineID, run.PipelineName, run.PipelineDisplayName,
		run.ApplicationID, run.SourceRepositoryID, run.GitRef, run.CommitSHA, run.Status, run.JenkinsQueueID,
		run.JenkinsBuildNumber, run.PrimaryArtifactID, run.LogOffset, run.ErrorMessage, run.RequestedBy,
		mysqlTime(run.CreatedAt), mysqlTime(run.UpdatedAt), mysqlTimePtr(run.StartedAt), mysqlTimePtr(run.FinishedAt))
	return database.ConflictOrUnavailable(err, "build run already exists", "create build run failed")
}

func (r *MySQLRepository) UpdateRun(ctx context.Context, run BuildRun) error {
	previous, err := r.GetRun(ctx, run.ID)
	if err != nil {
		return err
	}
	if previous.ApplicationID != run.ApplicationID || previous.PipelineID != run.PipelineID || previous.TenantID != run.TenantID || previous.ProjectID != run.ProjectID {
		return shared.NewError(shared.CodeInvalidArgument, "build run ownership cannot be changed")
	}
	_, err = database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
UPDATE build_runs
SET pipeline_name = ?, pipeline_display_name = ?, source_repository_id = ?, git_ref = ?, commit_sha = ?,
    status = ?, jenkins_queue_id = ?, jenkins_build_number = ?, primary_artifact_id = ?, log_offset = ?,
    error_message = ?, requested_by = ?, created_at = ?, updated_at = ?, started_at = ?, finished_at = ?
WHERE id = ?`,
		run.PipelineName, run.PipelineDisplayName, run.SourceRepositoryID, run.GitRef, run.CommitSHA,
		run.Status, run.JenkinsQueueID, run.JenkinsBuildNumber, run.PrimaryArtifactID, run.LogOffset,
		run.ErrorMessage, run.RequestedBy, mysqlTime(run.CreatedAt), mysqlTime(run.UpdatedAt), mysqlTimePtr(run.StartedAt), mysqlTimePtr(run.FinishedAt), run.ID)
	if err != nil {
		return database.WrapUnavailable(err, "update build run failed")
	}
	return nil
}

func (r *MySQLRepository) GetRun(ctx context.Context, id shared.ID) (BuildRun, error) {
	run, err := scanBuildRun(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, buildRunSelect()+" WHERE id = ?", id))
	if err != nil {
		return BuildRun{}, database.NotFound(err, "build run not found")
	}
	return run, nil
}

func (r *MySQLRepository) ListRunsByApplication(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[BuildRun], error) {
	return listPage(ctx, r.db, page, "build_runs", buildRunSelect(), "application_id = ?", []any{applicationID}, "created_at DESC", scanBuildRun)
}

func (r *MySQLRepository) CreateRunSource(ctx context.Context, source BuildRunSource) error {
	if _, err := r.GetRun(ctx, source.BuildRunID); err != nil {
		return err
	}
	return r.insertRunSource(ctx, source)
}

func (r *MySQLRepository) insertRunSource(ctx context.Context, source BuildRunSource) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO build_run_sources (id, tenant_id, project_id, build_run_id, application_id, source_key, source_repository_id, git_ref, commit_sha, source_path, is_primary, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		source.ID, source.TenantID, source.ProjectID, source.BuildRunID, source.ApplicationID, source.SourceKey,
		source.SourceRepositoryID, source.GitRef, source.CommitSHA, source.SourcePath, source.IsPrimary, mysqlTime(source.CreatedAt))
	return database.ConflictOrUnavailable(err, "build run source already exists", "create build run source failed")
}

func (r *MySQLRepository) ListRunSources(ctx context.Context, buildRunID shared.ID) ([]BuildRunSource, error) {
	if _, err := r.GetRun(ctx, buildRunID); err != nil {
		return nil, err
	}
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, buildRunSourceSelect()+`
 WHERE build_run_id = ? ORDER BY is_primary DESC, source_key ASC`, buildRunID)
	if err != nil {
		return nil, database.WrapUnavailable(err, "list build run sources failed")
	}
	defer rows.Close()
	return scanRows(rows, scanBuildRunSource, "list build run sources failed")
}

func (r *MySQLRepository) CreateArtifact(ctx context.Context, artifact BuildArtifact) error {
	if _, err := r.GetRun(ctx, artifact.BuildRunID); err != nil {
		return err
	}
	metadata, err := database.MarshalJSON(artifact.Metadata)
	if err != nil {
		return err
	}
	_, err = database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO build_artifacts (id, tenant_id, project_id, build_run_id, application_id, source_key, type, name, uri, digest, is_primary, metadata, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		artifact.ID, artifact.TenantID, artifact.ProjectID, artifact.BuildRunID, artifact.ApplicationID,
		artifact.SourceKey, artifact.Type, artifact.Name, artifact.URI, artifact.Digest, artifact.IsPrimary, string(metadata), mysqlTime(artifact.CreatedAt))
	return database.ConflictOrUnavailable(err, "build artifact already exists", "create build artifact failed")
}

func (r *MySQLRepository) GetArtifact(ctx context.Context, id shared.ID) (BuildArtifact, error) {
	artifact, err := scanBuildArtifact(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, buildArtifactSelect()+" WHERE id = ?", id))
	if err != nil {
		return BuildArtifact{}, database.NotFound(err, "build artifact not found")
	}
	return artifact, nil
}

func (r *MySQLRepository) ListArtifactsByRun(ctx context.Context, buildRunID shared.ID) ([]BuildArtifact, error) {
	if _, err := r.GetRun(ctx, buildRunID); err != nil {
		return nil, err
	}
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, buildArtifactSelect()+`
 WHERE build_run_id = ? ORDER BY created_at ASC`, buildRunID)
	if err != nil {
		return nil, database.WrapUnavailable(err, "list build artifacts failed")
	}
	defer rows.Close()
	return scanRows(rows, scanBuildArtifact, "list build artifacts failed")
}

func (r *MySQLRepository) AppendBuildLog(ctx context.Context, buildRunID shared.ID, text string) error {
	if _, err := r.GetRun(ctx, buildRunID); err != nil {
		return err
	}
	if text == "" {
		return nil
	}
	for _, chunk := range splitBuildLogText(text, maxBuildLogChunkBytes) {
		_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, "INSERT INTO build_logs(build_run_id, log_text, created_at) VALUES (?, ?, ?)", buildRunID, chunk, time.Now().UTC())
		if err != nil {
			return database.WrapUnavailable(err, "append build log failed")
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
	if _, err := r.GetRun(ctx, buildRunID); err != nil {
		return nil, err
	}
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, "SELECT log_text FROM build_logs WHERE build_run_id = ? ORDER BY id", buildRunID)
	if err != nil {
		return nil, database.WrapUnavailable(err, "list build logs failed")
	}
	defer rows.Close()
	logs := []string{}
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			return nil, database.WrapUnavailable(err, "scan build log failed")
		}
		logs = append(logs, text)
	}
	if err := rows.Err(); err != nil {
		return nil, database.WrapUnavailable(err, "list build logs failed")
	}
	return logs, nil
}

func (r *MySQLRepository) ensurePipelineNameAvailable(ctx context.Context, id shared.ID, applicationID shared.ID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	var existingID shared.ID
	err := database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id FROM build_pipelines WHERE application_id = ? AND name = ? AND status = ? LIMIT 1`,
		applicationID, name, BuildPipelineStatusActive).Scan(&existingID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return database.WrapUnavailable(err, "check build pipeline name failed")
	}
	if existingID != id {
		return shared.NewError(shared.CodeConflict, "build pipeline name already exists")
	}
	return nil
}

func (r *MySQLRepository) insertPipelineSource(ctx context.Context, source BuildPipelineSource) error {
	buildSpec, err := database.MarshalJSON(source.BuildSpec)
	if err != nil {
		return err
	}
	_, err = database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO build_pipeline_sources (id, tenant_id, project_id, application_id, pipeline_id, source_key, display_name, source_repository_id, build_environment_id, source_path, build_spec, is_primary, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		source.ID, source.TenantID, source.ProjectID, source.ApplicationID, source.PipelineID, strings.TrimSpace(source.Key),
		source.DisplayName, source.SourceRepositoryID, source.BuildEnvironmentID, source.SourcePath, string(buildSpec),
		source.IsPrimary, mysqlTime(source.CreatedAt), mysqlTime(source.UpdatedAt))
	return database.ConflictOrUnavailable(err, "build pipeline source already exists", "create build pipeline source failed")
}

func mysqlTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value
}

func mysqlTimePtr(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return *value
}

func (r *MySQLRepository) withTx(ctx context.Context, fn func(context.Context) error) error {
	if database.TxFromContext(ctx) != nil {
		return fn(ctx)
	}
	return database.NewTransactor(r.db).WithinTx(ctx, fn)
}

type buildScanner interface {
	Scan(dest ...any) error
}

func buildEnvironmentSelect() string {
	return "SELECT id, name, description, build_image, status, is_default, created_by, created_at, updated_at FROM build_environments"
}

func scanBuildEnvironment(scanner buildScanner) (BuildEnvironment, error) {
	var environment BuildEnvironment
	err := scanner.Scan(&environment.ID, &environment.Name, &environment.Description, &environment.BuildImage, &environment.Status, &environment.IsDefault, &environment.CreatedBy, &environment.CreatedAt, &environment.UpdatedAt)
	return environment, err
}

func runtimeEnvironmentSelect() string {
	return "SELECT id, name, description, runtime_base_image, artifact_deploy_path, dockerfile_path, status, is_default, created_by, created_at, updated_at FROM runtime_environments"
}

func scanRuntimeEnvironment(scanner buildScanner) (RuntimeEnvironment, error) {
	var environment RuntimeEnvironment
	err := scanner.Scan(&environment.ID, &environment.Name, &environment.Description, &environment.RuntimeBaseImage, &environment.ArtifactDeployPath, &environment.DockerfilePath, &environment.Status, &environment.IsDefault, &environment.CreatedBy, &environment.CreatedAt, &environment.UpdatedAt)
	return environment, err
}

func buildTemplateSelect() string {
	return "SELECT id, name, version, content, created_by, created_at, updated_at FROM build_templates"
}

func scanBuildTemplate(scanner buildScanner) (BuildTemplate, error) {
	var template BuildTemplate
	err := scanner.Scan(&template.ID, &template.Name, &template.Version, &template.Content, &template.CreatedBy, &template.CreatedAt, &template.UpdatedAt)
	return template, err
}

func jenkinsJobTemplateSelect() string {
	return "SELECT id, name, display_name, description, runtime_base_image, version, xml_content, status, is_default, created_by, created_at, updated_at FROM jenkins_job_templates"
}

func scanJenkinsJobTemplate(scanner buildScanner) (JenkinsJobTemplate, error) {
	var template JenkinsJobTemplate
	err := scanner.Scan(&template.ID, &template.Name, &template.DisplayName, &template.Description, &template.RuntimeBaseImage, &template.Version, &template.XMLContent, &template.Status, &template.IsDefault, &template.CreatedBy, &template.CreatedAt, &template.UpdatedAt)
	return template, err
}

func buildPipelineSelect() string {
	return "SELECT id, tenant_id, project_id, application_id, name, display_name, description, provider, external_job_name, template_id, config_hash, status, managed_by_platform, created_at, updated_at FROM build_pipelines"
}

func scanBuildPipeline(scanner buildScanner) (BuildPipeline, error) {
	var pipeline BuildPipeline
	err := scanner.Scan(&pipeline.ID, &pipeline.TenantID, &pipeline.ProjectID, &pipeline.ApplicationID, &pipeline.Name, &pipeline.DisplayName, &pipeline.Description, &pipeline.Provider, &pipeline.ExternalJobName, &pipeline.TemplateID, &pipeline.ConfigHash, &pipeline.Status, &pipeline.ManagedByPlatform, &pipeline.CreatedAt, &pipeline.UpdatedAt)
	return pipeline, err
}

func buildPipelineSourceSelect() string {
	return "SELECT id, tenant_id, project_id, application_id, pipeline_id, source_key, display_name, source_repository_id, build_environment_id, source_path, build_spec, is_primary, created_at, updated_at FROM build_pipeline_sources"
}

func scanBuildPipelineSource(scanner buildScanner) (BuildPipelineSource, error) {
	var source BuildPipelineSource
	var buildSpec []byte
	if err := scanner.Scan(&source.ID, &source.TenantID, &source.ProjectID, &source.ApplicationID, &source.PipelineID, &source.Key, &source.DisplayName, &source.SourceRepositoryID, &source.BuildEnvironmentID, &source.SourcePath, &buildSpec, &source.IsPrimary, &source.CreatedAt, &source.UpdatedAt); err != nil {
		return BuildPipelineSource{}, err
	}
	if err := database.UnmarshalJSON(buildSpec, &source.BuildSpec); err != nil {
		return BuildPipelineSource{}, err
	}
	return source, nil
}

func buildRunSelect() string {
	return "SELECT id, tenant_id, project_id, pipeline_id, pipeline_name, pipeline_display_name, application_id, source_repository_id, git_ref, commit_sha, status, jenkins_queue_id, jenkins_build_number, primary_artifact_id, log_offset, error_message, requested_by, created_at, updated_at, started_at, finished_at FROM build_runs"
}

func scanBuildRun(scanner buildScanner) (BuildRun, error) {
	var run BuildRun
	err := scanner.Scan(&run.ID, &run.TenantID, &run.ProjectID, &run.PipelineID, &run.PipelineName, &run.PipelineDisplayName, &run.ApplicationID, &run.SourceRepositoryID, &run.GitRef, &run.CommitSHA, &run.Status, &run.JenkinsQueueID, &run.JenkinsBuildNumber, &run.PrimaryArtifactID, &run.LogOffset, &run.ErrorMessage, &run.RequestedBy, &run.CreatedAt, &run.UpdatedAt, &run.StartedAt, &run.FinishedAt)
	return run, err
}

func buildRunSourceSelect() string {
	return "SELECT id, tenant_id, project_id, build_run_id, application_id, source_key, source_repository_id, git_ref, commit_sha, source_path, is_primary, created_at FROM build_run_sources"
}

func scanBuildRunSource(scanner buildScanner) (BuildRunSource, error) {
	var source BuildRunSource
	err := scanner.Scan(&source.ID, &source.TenantID, &source.ProjectID, &source.BuildRunID, &source.ApplicationID, &source.SourceKey, &source.SourceRepositoryID, &source.GitRef, &source.CommitSHA, &source.SourcePath, &source.IsPrimary, &source.CreatedAt)
	return source, err
}

func buildArtifactSelect() string {
	return "SELECT id, tenant_id, project_id, build_run_id, application_id, source_key, type, name, uri, digest, is_primary, metadata, created_at FROM build_artifacts"
}

func scanBuildArtifact(scanner buildScanner) (BuildArtifact, error) {
	var artifact BuildArtifact
	var metadata []byte
	if err := scanner.Scan(&artifact.ID, &artifact.TenantID, &artifact.ProjectID, &artifact.BuildRunID, &artifact.ApplicationID, &artifact.SourceKey, &artifact.Type, &artifact.Name, &artifact.URI, &artifact.Digest, &artifact.IsPrimary, &metadata, &artifact.CreatedAt); err != nil {
		return BuildArtifact{}, err
	}
	if err := database.UnmarshalJSON(metadata, &artifact.Metadata); err != nil {
		return BuildArtifact{}, err
	}
	if artifact.Metadata == nil {
		artifact.Metadata = map[string]string{}
	}
	return artifact, nil
}

func listPage[T any](ctx context.Context, db *sql.DB, page shared.PageRequest, table string, selectSQL string, where string, args []any, orderBy string, scan func(buildScanner) (T, error)) (shared.PageResult[T], error) {
	var total int64
	countArgs := append([]any{}, args...)
	if err := database.ExecutorFromContext(ctx, db).QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table+" WHERE "+where, countArgs...).Scan(&total); err != nil {
		return shared.PageResult[T]{}, database.WrapUnavailable(err, "count "+table+" failed")
	}
	page, limit, offset := database.LimitOffset(page)
	queryArgs := append(append([]any{}, args...), limit, offset)
	rows, err := database.ExecutorFromContext(ctx, db).QueryContext(ctx, selectSQL+" WHERE "+where+" ORDER BY "+orderBy+" LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		return shared.PageResult[T]{}, database.WrapUnavailable(err, "list "+table+" failed")
	}
	defer rows.Close()
	items, err := scanRows(rows, scan, "list "+table+" failed")
	if err != nil {
		return shared.PageResult[T]{}, err
	}
	return shared.NewPageResult(items, total, page), nil
}

func scanRows[T any](rows *sql.Rows, scan func(buildScanner) (T, error), message string) ([]T, error) {
	items := []T{}
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, database.WrapUnavailable(err, message)
	}
	return items, nil
}
