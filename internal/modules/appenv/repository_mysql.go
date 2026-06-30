package appenv

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/shareinto/paas/internal/platform/database"
	"github.com/shareinto/paas/internal/shared"
)

type MySQLRepository struct{ db *sql.DB }

func NewMySQLRepository(_ context.Context, db *sql.DB) (*MySQLRepository, error) {
	return &MySQLRepository{db: db}, nil
}

func (r *MySQLRepository) CreateApplication(ctx context.Context, app Application) error {
	if err := r.insertApplication(ctx, app); err != nil {
		return err
	}
	return r.replaceApplicationRuntimeEnvironments(ctx, app.ID, app.RuntimeEnvironments)
}

func (r *MySQLRepository) UpdateApplication(ctx context.Context, app Application) error {
	prev, err := r.GetApplication(ctx, app.ID)
	if err != nil {
		return err
	}
	if prev.TenantID != app.TenantID || prev.ProjectID != app.ProjectID {
		return shared.NewError(shared.CodeInvalidArgument, "application ownership cannot be changed")
	}
	_, err = database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
UPDATE applications SET runtime_environment_id = ?, name = ?, display_name = ?, description = ?, status = ?, updated_at = ? WHERE id = ?`,
		app.RuntimeEnvironmentID, app.Name, app.DisplayName, app.Description, app.Status, mysqlTime(app.UpdatedAt), app.ID)
	if err != nil {
		return database.ConflictOrUnavailable(err, "application name already exists in project", "update application failed")
	}
	return r.replaceApplicationRuntimeEnvironments(ctx, app.ID, app.RuntimeEnvironments)
}

func (r *MySQLRepository) DeleteApplicationData(ctx context.Context, applicationID shared.ID) error {
	if _, err := r.GetApplication(ctx, applicationID); err != nil {
		return err
	}
	exec := database.ExecutorFromContext(ctx, r.db)
	for _, stmt := range []string{
		"DELETE FROM workload_stage_configs WHERE application_id = ?",
		"DELETE FROM workload_default_configs WHERE application_id = ?",
		"DELETE FROM workloads WHERE application_id = ?",
		"DELETE FROM application_stage_events WHERE application_id = ?",
		"DELETE FROM application_stage_states WHERE application_id = ?",
		"DELETE FROM application_sources WHERE application_id = ?",
		"DELETE FROM application_runtime_environments WHERE application_id = ?",
		"DELETE FROM applications WHERE id = ?",
	} {
		if _, err := exec.ExecContext(ctx, stmt, applicationID); err != nil {
			return database.WrapUnavailable(err, "delete application data failed")
		}
	}
	return nil
}

func (r *MySQLRepository) GetApplication(ctx context.Context, id shared.ID) (Application, error) {
	app, err := scanApplication(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, applicationSelect()+" WHERE id = ?", id))
	if err != nil {
		return Application{}, database.NotFound(err, "application not found")
	}
	app.RuntimeEnvironments, _ = r.listApplicationRuntimeEnvironments(ctx, app.ID)
	return app, nil
}

func (r *MySQLRepository) FindApplicationByProjectAndName(ctx context.Context, projectID shared.ID, name string) (Application, error) {
	app, err := scanApplication(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, applicationSelect()+" WHERE project_id = ? AND name = ?", projectID, name))
	if err != nil {
		return Application{}, database.NotFound(err, "application not found")
	}
	app.RuntimeEnvironments, _ = r.listApplicationRuntimeEnvironments(ctx, app.ID)
	return app, nil
}

func (r *MySQLRepository) ListApplicationsByProject(ctx context.Context, projectID shared.ID, page shared.PageRequest) (shared.PageResult[Application], error) {
	return r.listApplications(ctx, "project_id = ?", []any{projectID}, page)
}

func (r *MySQLRepository) ListApplicationsByTenant(ctx context.Context, tenantID shared.ID, page shared.PageRequest) (shared.PageResult[Application], error) {
	return r.listApplications(ctx, "tenant_id = ?", []any{tenantID}, page)
}

func (r *MySQLRepository) ListApplicationsByRuntimeEnvironment(ctx context.Context, runtimeEnvironmentID shared.ID, page shared.PageRequest) (shared.PageResult[Application], error) {
	return r.listApplications(ctx, "id IN (SELECT application_id FROM application_runtime_environments WHERE runtime_environment_id = ?)", []any{runtimeEnvironmentID}, page)
}

func (r *MySQLRepository) CreateApplicationSource(ctx context.Context, source ApplicationSource) error {
	if _, err := r.GetApplication(ctx, source.ApplicationID); err != nil {
		return err
	}
	return r.insertApplicationSource(ctx, source)
}

func (r *MySQLRepository) UpdateApplicationSource(ctx context.Context, source ApplicationSource) error {
	result, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
UPDATE application_sources
SET source_key = ?, display_name = ?, source_type = ?, source_url = ?, source_ref = ?, svn_revision = ?, svn_checkout_paths = ?, jenkins_template_id = ?, build_environment_id = ?,
    source_path = ?, build_command = ?, artifact_copy_command = ?, runtime_base_image = ?, artifact_deploy_path = ?,
    default_ref = ?, is_primary = ?, updated_at = ?
WHERE id = ?`,
		normalizedSourceKey(source.Key), source.DisplayName, source.SourceType, source.SourceURL, source.SourceRef, source.SVNRevision, applicationSourceCheckoutPathsValue(source), source.JenkinsTemplateID, source.BuildEnvironmentID,
		source.SourcePath, source.BuildSpec.BuildCommand, source.BuildSpec.ArtifactCopyCommand, source.BuildSpec.RuntimeBaseImage,
		source.BuildSpec.ArtifactDeployPath, source.BuildSpec.DefaultRef, source.IsPrimary, mysqlTime(source.UpdatedAt), source.ID)
	if err != nil {
		return database.ConflictOrUnavailable(err, "application source already exists", "update application source failed")
	}
	return database.RequireAffected(result, "application source not found")
}

func (r *MySQLRepository) ReplaceApplicationSources(ctx context.Context, applicationID shared.ID, sources []ApplicationSource) error {
	if _, err := r.GetApplication(ctx, applicationID); err != nil {
		return err
	}
	if len(sources) == 0 {
		return shared.NewError(shared.CodeInvalidArgument, "sources is required")
	}
	exec := database.ExecutorFromContext(ctx, r.db)
	if _, err := exec.ExecContext(ctx, "DELETE FROM application_sources WHERE application_id = ?", applicationID); err != nil {
		return database.WrapUnavailable(err, "replace application sources failed")
	}
	seen := map[string]struct{}{}
	for _, source := range sources {
		if source.ApplicationID != applicationID {
			return shared.NewError(shared.CodeInvalidArgument, "application source ownership cannot be changed")
		}
		key := normalizedSourceKey(source.Key)
		if _, ok := seen[key]; ok {
			return shared.NewError(shared.CodeConflict, "application source already exists")
		}
		seen[key] = struct{}{}
		if err := r.insertApplicationSource(ctx, source); err != nil {
			return err
		}
	}
	return nil
}

func (r *MySQLRepository) GetApplicationSource(ctx context.Context, applicationID shared.ID) (ApplicationSource, error) {
	source, err := scanApplicationSource(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, applicationSourceSelect()+`
 WHERE application_id = ? ORDER BY is_primary DESC, source_key ASC LIMIT 1`, applicationID))
	if err != nil {
		return ApplicationSource{}, database.NotFound(err, "application source not found")
	}
	return source, nil
}

func (r *MySQLRepository) ListApplicationSources(ctx context.Context, applicationID shared.ID) ([]ApplicationSource, error) {
	if _, err := r.GetApplication(ctx, applicationID); err != nil {
		return nil, err
	}
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, applicationSourceSelect()+`
 WHERE application_id = ? ORDER BY is_primary DESC, source_key ASC`, applicationID)
	if err != nil {
		return nil, database.WrapUnavailable(err, "list application sources failed")
	}
	defer rows.Close()
	items := []ApplicationSource{}
	for rows.Next() {
		source, err := scanApplicationSource(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, source)
	}
	if err := rows.Err(); err != nil {
		return nil, database.WrapUnavailable(err, "list application sources failed")
	}
	if len(items) == 0 {
		return nil, shared.NewError(shared.CodeNotFound, "application source not found")
	}
	return items, nil
}

func (r *MySQLRepository) CreateWorkload(ctx context.Context, workload Workload) error {
	if _, err := r.GetApplication(ctx, workload.ApplicationID); err != nil {
		return err
	}
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO workloads (id, tenant_id, project_id, application_id, name, display_name, workload_type, description, status, image_source_mode, pipeline_id, created_by, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		workload.ID, workload.TenantID, workload.ProjectID, workload.ApplicationID, workload.Name, workload.DisplayName, workload.WorkloadType, workload.Description, workload.Status, workload.ImageSourceMode, workload.PipelineID, workload.CreatedBy, mysqlTime(workload.CreatedAt), mysqlTime(workload.UpdatedAt))
	return database.ConflictOrUnavailable(err, "workload name already exists in application", "create workload failed")
}

func (r *MySQLRepository) UpdateWorkload(ctx context.Context, workload Workload) error {
	prev, err := r.GetWorkload(ctx, workload.ID)
	if err != nil {
		return err
	}
	if prev.TenantID != workload.TenantID || prev.ProjectID != workload.ProjectID || prev.ApplicationID != workload.ApplicationID {
		return shared.NewError(shared.CodeInvalidArgument, "workload ownership cannot be changed")
	}
	result, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
UPDATE workloads
SET name = ?, display_name = ?, workload_type = ?, description = ?, status = ?, image_source_mode = ?, pipeline_id = ?, updated_at = ?
WHERE id = ?`,
		workload.Name, workload.DisplayName, workload.WorkloadType, workload.Description, workload.Status, workload.ImageSourceMode, workload.PipelineID, mysqlTime(workload.UpdatedAt), workload.ID)
	if err != nil {
		return database.ConflictOrUnavailable(err, "workload name already exists in application", "update workload failed")
	}
	return database.RequireAffected(result, "workload not found")
}

func (r *MySQLRepository) GetWorkload(ctx context.Context, id shared.ID) (Workload, error) {
	workload, err := scanWorkload(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, workloadSelect()+" WHERE id = ?", id))
	if err != nil {
		return Workload{}, database.NotFound(err, "workload not found")
	}
	return workload, nil
}

func (r *MySQLRepository) FindWorkloadByApplicationAndName(ctx context.Context, applicationID shared.ID, name string) (Workload, error) {
	workload, err := scanWorkload(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, workloadSelect()+" WHERE application_id = ? AND name = ? AND status <> ?", applicationID, name, WorkloadStatusDeleted))
	if err != nil {
		return Workload{}, database.NotFound(err, "workload not found")
	}
	return workload, nil
}

func (r *MySQLRepository) ListWorkloadsByApplication(ctx context.Context, applicationID shared.ID) ([]Workload, error) {
	if _, err := r.GetApplication(ctx, applicationID); err != nil {
		return nil, err
	}
	return r.listWorkloads(ctx, "application_id = ? AND status <> ?", applicationID, WorkloadStatusDeleted)
}

func (r *MySQLRepository) ListEnabledWorkloadsByApplication(ctx context.Context, applicationID shared.ID) ([]Workload, error) {
	if _, err := r.GetApplication(ctx, applicationID); err != nil {
		return nil, err
	}
	return r.listWorkloads(ctx, "application_id = ? AND status = ?", applicationID, WorkloadStatusEnabled)
}

type encodedWorkloadConfig struct {
	servicePorts     string
	resourceRequests string
	resourceLimits   string
	probes           string
	ingressHosts     string
	envVars          string
	secretRefs       string
	configFiles      string
	writableDirs     string
	volumeMounts     string
	initContainers   string
	valuesOverride   string
}

func encodeWorkloadConfigJSON(config WorkloadStageConfig) (encodedWorkloadConfig, error) {
	servicePorts, err := jsonText(config.ServicePorts)
	if err != nil {
		return encodedWorkloadConfig{}, err
	}
	resourceRequests, err := jsonText(config.ResourceRequests)
	if err != nil {
		return encodedWorkloadConfig{}, err
	}
	resourceLimits, err := jsonText(config.ResourceLimits)
	if err != nil {
		return encodedWorkloadConfig{}, err
	}
	probes, err := jsonText(config.Probes)
	if err != nil {
		return encodedWorkloadConfig{}, err
	}
	ingressHosts, err := jsonText(config.IngressHosts)
	if err != nil {
		return encodedWorkloadConfig{}, err
	}
	envVars, err := jsonText(config.EnvVars)
	if err != nil {
		return encodedWorkloadConfig{}, err
	}
	secretRefs, err := jsonText(config.SecretRefs)
	if err != nil {
		return encodedWorkloadConfig{}, err
	}
	configFiles, err := jsonText(config.ConfigFiles)
	if err != nil {
		return encodedWorkloadConfig{}, err
	}
	writableDirs, err := jsonText(config.WritableDirs)
	if err != nil {
		return encodedWorkloadConfig{}, err
	}
	volumeMounts, err := jsonText(config.VolumeMounts)
	if err != nil {
		return encodedWorkloadConfig{}, err
	}
	initContainers, err := jsonText(config.InitContainers)
	if err != nil {
		return encodedWorkloadConfig{}, err
	}
	valuesOverride, err := jsonText(config.ValuesOverride)
	if err != nil {
		return encodedWorkloadConfig{}, err
	}
	return encodedWorkloadConfig{servicePorts: servicePorts, resourceRequests: resourceRequests, resourceLimits: resourceLimits, probes: probes, ingressHosts: ingressHosts, envVars: envVars, secretRefs: secretRefs, configFiles: configFiles, writableDirs: writableDirs, volumeMounts: volumeMounts, initContainers: initContainers, valuesOverride: valuesOverride}, nil
}

func (r *MySQLRepository) SaveWorkloadStageConfig(ctx context.Context, config WorkloadStageConfig) error {
	if _, err := r.GetWorkload(ctx, config.WorkloadID); err != nil {
		return err
	}
	if strings.TrimSpace(config.StageKey) == "" {
		return shared.NewError(shared.CodeInvalidArgument, "stage_key is required")
	}
	encoded, err := encodeWorkloadConfigJSON(config)
	if err != nil {
		return err
	}
	_, err = database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO workload_stage_configs (
  id, tenant_id, project_id, application_id, workload_id, stage_key, replicas,
  network_mode,
  service_ports_json, resource_requests_json, resource_limits_json, probes_json, ingress_hosts_json,
  env_vars_json, secret_refs_json, config_files_json, writable_dirs_json, volume_mounts_json,
  init_containers_json, values_override_json, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), ?, ?)
ON DUPLICATE KEY UPDATE
  replicas = VALUES(replicas),
  network_mode = VALUES(network_mode),
  service_ports_json = VALUES(service_ports_json),
  resource_requests_json = VALUES(resource_requests_json),
  resource_limits_json = VALUES(resource_limits_json),
  probes_json = VALUES(probes_json),
  ingress_hosts_json = VALUES(ingress_hosts_json),
  env_vars_json = VALUES(env_vars_json),
  secret_refs_json = VALUES(secret_refs_json),
  config_files_json = VALUES(config_files_json),
  writable_dirs_json = VALUES(writable_dirs_json),
  volume_mounts_json = VALUES(volume_mounts_json),
  init_containers_json = VALUES(init_containers_json),
  values_override_json = VALUES(values_override_json),
  updated_at = VALUES(updated_at)`,
		config.ID, config.TenantID, config.ProjectID, config.ApplicationID, config.WorkloadID, strings.TrimSpace(config.StageKey), config.Replicas, normalizeWorkloadNetworkMode(config.NetworkMode),
		encoded.servicePorts, encoded.resourceRequests, encoded.resourceLimits, encoded.probes, encoded.ingressHosts, encoded.envVars, encoded.secretRefs, encoded.configFiles, encoded.writableDirs, encoded.volumeMounts, encoded.initContainers, encoded.valuesOverride,
		mysqlTime(config.CreatedAt), mysqlTime(config.UpdatedAt))
	return database.ConflictOrUnavailable(err, "workload stage config already exists", "save workload stage config failed")
}

func (r *MySQLRepository) GetWorkloadStageConfig(ctx context.Context, workloadID shared.ID, stageKey string) (WorkloadStageConfig, error) {
	config, err := scanWorkloadStageConfig(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, workloadStageConfigSelect()+" WHERE workload_id = ? AND stage_key = ?", workloadID, strings.TrimSpace(stageKey)))
	if err != nil {
		return WorkloadStageConfig{}, database.NotFound(err, "workload stage config not found")
	}
	return config, nil
}

func (r *MySQLRepository) ListWorkloadStageConfigs(ctx context.Context, workloadID shared.ID) ([]WorkloadStageConfig, error) {
	if _, err := r.GetWorkload(ctx, workloadID); err != nil {
		return nil, err
	}
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, workloadStageConfigSelect()+`
 WHERE workload_id = ? ORDER BY stage_key ASC`, workloadID)
	if err != nil {
		return nil, database.WrapUnavailable(err, "list workload stage configs failed")
	}
	defer rows.Close()
	items := []WorkloadStageConfig{}
	for rows.Next() {
		config, err := scanWorkloadStageConfig(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, config)
	}
	return items, rows.Err()
}

func (r *MySQLRepository) SaveWorkloadDefaultConfig(ctx context.Context, config WorkloadStageConfig) error {
	if _, err := r.GetWorkload(ctx, config.WorkloadID); err != nil {
		return err
	}
	encoded, err := encodeWorkloadConfigJSON(config)
	if err != nil {
		return err
	}
	_, err = database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO workload_default_configs (
  id, tenant_id, project_id, application_id, workload_id, replicas,
  network_mode,
  service_ports_json, resource_requests_json, resource_limits_json, probes_json, ingress_hosts_json,
  env_vars_json, secret_refs_json, config_files_json, writable_dirs_json, volume_mounts_json,
  init_containers_json, values_override_json, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), CAST(? AS JSON), ?, ?)
ON DUPLICATE KEY UPDATE
  replicas = VALUES(replicas),
  network_mode = VALUES(network_mode),
  service_ports_json = VALUES(service_ports_json),
  resource_requests_json = VALUES(resource_requests_json),
  resource_limits_json = VALUES(resource_limits_json),
  probes_json = VALUES(probes_json),
  ingress_hosts_json = VALUES(ingress_hosts_json),
  env_vars_json = VALUES(env_vars_json),
  secret_refs_json = VALUES(secret_refs_json),
  config_files_json = VALUES(config_files_json),
  writable_dirs_json = VALUES(writable_dirs_json),
  volume_mounts_json = VALUES(volume_mounts_json),
  init_containers_json = VALUES(init_containers_json),
  values_override_json = VALUES(values_override_json),
  updated_at = VALUES(updated_at)`,
		config.ID, config.TenantID, config.ProjectID, config.ApplicationID, config.WorkloadID, config.Replicas, normalizeWorkloadNetworkMode(config.NetworkMode),
		encoded.servicePorts, encoded.resourceRequests, encoded.resourceLimits, encoded.probes, encoded.ingressHosts, encoded.envVars, encoded.secretRefs, encoded.configFiles, encoded.writableDirs, encoded.volumeMounts, encoded.initContainers, encoded.valuesOverride,
		mysqlTime(config.CreatedAt), mysqlTime(config.UpdatedAt))
	return database.ConflictOrUnavailable(err, "workload default config already exists", "save workload default config failed")
}

func (r *MySQLRepository) GetWorkloadDefaultConfig(ctx context.Context, workloadID shared.ID) (WorkloadStageConfig, error) {
	config, err := scanWorkloadStageConfig(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, workloadDefaultConfigSelect()+" WHERE workload_id = ?", workloadID))
	if err != nil {
		return WorkloadStageConfig{}, database.NotFound(err, "workload default config not found")
	}
	return config, nil
}

func (r *MySQLRepository) SaveApplicationStageState(ctx context.Context, state ApplicationStageState) error {
	if _, err := r.GetApplication(ctx, state.ApplicationID); err != nil {
		return err
	}
	if strings.TrimSpace(state.StageKey) == "" {
		return shared.NewError(shared.CodeInvalidArgument, "stage_key is required")
	}
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO application_stage_states (application_id, stage_key, tenant_id, project_id, status, message, last_reported_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE status = VALUES(status), message = VALUES(message), last_reported_at = VALUES(last_reported_at), updated_at = VALUES(updated_at)`,
		state.ApplicationID, strings.TrimSpace(state.StageKey), state.TenantID, state.ProjectID, state.Status, state.Message, mysqlTimePtr(state.LastReportedAt), mysqlTime(state.UpdatedAt))
	return database.WrapUnavailable(err, "save application stage state failed")
}

func (r *MySQLRepository) GetApplicationStageState(ctx context.Context, applicationID shared.ID, stageKey string) (ApplicationStageState, error) {
	state, err := scanApplicationStageState(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, applicationStageStateSelect()+" WHERE application_id = ? AND stage_key = ?", applicationID, strings.TrimSpace(stageKey)))
	if err != nil {
		return ApplicationStageState{}, database.NotFound(err, "application stage state not found")
	}
	return state, nil
}

func (r *MySQLRepository) ListApplicationStageStatesByApplication(ctx context.Context, applicationID shared.ID) ([]ApplicationStageState, error) {
	if _, err := r.GetApplication(ctx, applicationID); err != nil {
		return nil, err
	}
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, applicationStageStateSelect()+`
 WHERE application_id = ? ORDER BY stage_key ASC`, applicationID)
	if err != nil {
		return nil, database.WrapUnavailable(err, "list application stage states failed")
	}
	defer rows.Close()
	items := []ApplicationStageState{}
	for rows.Next() {
		state, err := scanApplicationStageState(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, state)
	}
	return items, rows.Err()
}

func (r *MySQLRepository) AppendApplicationStageEvent(ctx context.Context, event ApplicationStageEvent) error {
	if _, err := r.GetApplication(ctx, event.ApplicationID); err != nil {
		return err
	}
	if strings.TrimSpace(event.StageKey) == "" {
		return shared.NewError(shared.CodeInvalidArgument, "stage_key is required")
	}
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO application_stage_events (id, tenant_id, project_id, application_id, stage_key, type, status, message, occurred_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID, event.TenantID, event.ProjectID, event.ApplicationID, strings.TrimSpace(event.StageKey), event.Type, event.Status, event.Message, mysqlTime(event.OccurredAt))
	return database.ConflictOrUnavailable(err, "application stage event already exists", "append application stage event failed")
}

func (r *MySQLRepository) ListApplicationStageEvents(ctx context.Context, applicationID shared.ID, stageKey string, page shared.PageRequest) (shared.PageResult[ApplicationStageEvent], error) {
	if _, err := r.GetApplication(ctx, applicationID); err != nil {
		return shared.PageResult[ApplicationStageEvent]{}, err
	}
	stageKey = strings.TrimSpace(stageKey)
	var total int64
	if err := database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, "SELECT COUNT(*) FROM application_stage_events WHERE application_id = ? AND stage_key = ?", applicationID, stageKey).Scan(&total); err != nil {
		return shared.PageResult[ApplicationStageEvent]{}, database.WrapUnavailable(err, "count application stage events failed")
	}
	page, limit, offset := database.LimitOffset(page)
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, `
SELECT id, tenant_id, project_id, application_id, stage_key, type, status, message, occurred_at
FROM application_stage_events WHERE application_id = ? AND stage_key = ? ORDER BY occurred_at ASC, id ASC LIMIT ? OFFSET ?`, applicationID, stageKey, limit, offset)
	if err != nil {
		return shared.PageResult[ApplicationStageEvent]{}, database.WrapUnavailable(err, "list application stage events failed")
	}
	defer rows.Close()
	items := []ApplicationStageEvent{}
	for rows.Next() {
		event, err := scanApplicationStageEvent(rows)
		if err != nil {
			return shared.PageResult[ApplicationStageEvent]{}, err
		}
		items = append(items, event)
	}
	return shared.NewPageResult(items, total, page), rows.Err()
}

func (r *MySQLRepository) insertApplication(ctx context.Context, app Application) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO applications (id, tenant_id, project_id, name, display_name, description, runtime_environment_id, status, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		app.ID, app.TenantID, app.ProjectID, app.Name, app.DisplayName, app.Description, app.RuntimeEnvironmentID, app.Status, mysqlTime(app.CreatedAt), mysqlTime(app.UpdatedAt))
	return database.ConflictOrUnavailable(err, "application name already exists in project", "create application failed")
}

func (r *MySQLRepository) insertApplicationSource(ctx context.Context, source ApplicationSource) error {
	key := normalizedSourceKey(source.Key)
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO application_sources (id, tenant_id, project_id, application_id, source_key, display_name, source_type, source_url, source_ref, svn_revision, svn_checkout_paths, jenkins_template_id, build_environment_id, source_path, build_command, artifact_copy_command, runtime_base_image, artifact_deploy_path, default_ref, is_primary, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		source.ID, source.TenantID, source.ProjectID, source.ApplicationID, key, source.DisplayName, source.SourceType, source.SourceURL, source.SourceRef, source.SVNRevision, applicationSourceCheckoutPathsValue(source), source.JenkinsTemplateID, source.BuildEnvironmentID, source.SourcePath, source.BuildSpec.BuildCommand, source.BuildSpec.ArtifactCopyCommand, source.BuildSpec.RuntimeBaseImage, source.BuildSpec.ArtifactDeployPath, source.BuildSpec.DefaultRef, source.IsPrimary, mysqlTime(source.CreatedAt), mysqlTime(source.UpdatedAt))
	return database.ConflictOrUnavailable(err, "application source already exists", "create application source failed")
}

func applicationSourceCheckoutPathsValue(source ApplicationSource) any {
	if len(source.SVNCheckoutPaths) == 0 {
		return nil
	}
	data, err := json.Marshal(source.SVNCheckoutPaths)
	if err != nil {
		return nil
	}
	return string(data)
}

func (r *MySQLRepository) listApplications(ctx context.Context, where string, args []any, page shared.PageRequest) (shared.PageResult[Application], error) {
	var total int64
	if err := database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, "SELECT COUNT(*) FROM applications WHERE "+where, args...).Scan(&total); err != nil {
		return shared.PageResult[Application]{}, database.WrapUnavailable(err, "count applications failed")
	}
	page, limit, offset := database.LimitOffset(page)
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, applicationSelect()+" WHERE "+where+" ORDER BY name ASC LIMIT ? OFFSET ?", append(args, limit, offset)...)
	if err != nil {
		return shared.PageResult[Application]{}, database.WrapUnavailable(err, "list applications failed")
	}
	defer rows.Close()
	items := []Application{}
	for rows.Next() {
		app, err := scanApplication(rows)
		if err != nil {
			return shared.PageResult[Application]{}, err
		}
		app.RuntimeEnvironments, _ = r.listApplicationRuntimeEnvironments(ctx, app.ID)
		items = append(items, app)
	}
	return shared.NewPageResult(items, total, page), rows.Err()
}

func (r *MySQLRepository) listWorkloads(ctx context.Context, where string, args ...any) ([]Workload, error) {
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, workloadSelect()+" WHERE "+where+" ORDER BY name ASC", args...)
	if err != nil {
		return nil, database.WrapUnavailable(err, "list workloads failed")
	}
	defer rows.Close()
	items := []Workload{}
	for rows.Next() {
		workload, err := scanWorkload(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, workload)
	}
	return items, rows.Err()
}

func (r *MySQLRepository) replaceApplicationRuntimeEnvironments(ctx context.Context, applicationID shared.ID, envs []ApplicationRuntimeEnvironment) error {
	exec := database.ExecutorFromContext(ctx, r.db)
	if _, err := exec.ExecContext(ctx, "DELETE FROM application_runtime_environments WHERE application_id = ?", applicationID); err != nil {
		return database.WrapUnavailable(err, "replace application runtime environments failed")
	}
	for i, env := range envs {
		labels, err := database.MarshalJSON(cleanStringMap(env.SelectorLabels))
		if err != nil {
			return err
		}
		if _, err := exec.ExecContext(ctx, `
INSERT INTO application_runtime_environments (application_id, runtime_environment_id, name, runtime_base_image, artifact_deploy_path, dockerfile_path, selector_labels_json, position)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, applicationID, env.ID, env.Name, env.RuntimeBaseImage, env.ArtifactDeployPath, env.DockerfilePath, labels, i); err != nil {
			return database.WrapUnavailable(err, "replace application runtime environments failed")
		}
	}
	return nil
}

func (r *MySQLRepository) listApplicationRuntimeEnvironments(ctx context.Context, applicationID shared.ID) ([]ApplicationRuntimeEnvironment, error) {
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, `
SELECT runtime_environment_id, name, runtime_base_image, artifact_deploy_path, dockerfile_path, selector_labels_json
FROM application_runtime_environments WHERE application_id = ? ORDER BY position ASC, runtime_environment_id ASC`, applicationID)
	if err != nil {
		return nil, database.WrapUnavailable(err, "list application runtime environments failed")
	}
	defer rows.Close()
	items := []ApplicationRuntimeEnvironment{}
	for rows.Next() {
		var env ApplicationRuntimeEnvironment
		var labels []byte
		if err := rows.Scan(&env.ID, &env.Name, &env.RuntimeBaseImage, &env.ArtifactDeployPath, &env.DockerfilePath, &labels); err != nil {
			return nil, err
		}
		if err := database.UnmarshalJSON(labels, &env.SelectorLabels); err != nil {
			return nil, err
		}
		env.SelectorLabels = cleanStringMap(env.SelectorLabels)
		items = append(items, env)
	}
	return items, rows.Err()
}

type appenvScanner interface{ Scan(dest ...any) error }

func applicationSelect() string {
	return "SELECT id, tenant_id, project_id, name, display_name, description, runtime_environment_id, status, created_at, updated_at FROM applications"
}
func applicationSourceSelect() string {
	return "SELECT id, tenant_id, project_id, application_id, source_key, display_name, source_type, source_url, source_ref, svn_revision, svn_checkout_paths, jenkins_template_id, build_environment_id, source_path, build_command, artifact_copy_command, runtime_base_image, artifact_deploy_path, default_ref, is_primary, created_at, updated_at FROM application_sources"
}
func workloadSelect() string {
	return "SELECT id, tenant_id, project_id, application_id, name, display_name, workload_type, description, status, image_source_mode, pipeline_id, created_by, created_at, updated_at FROM workloads"
}
func workloadStageConfigSelect() string {
	return `SELECT id, tenant_id, project_id, application_id, workload_id, stage_key, replicas,
network_mode,
service_ports_json, resource_requests_json, resource_limits_json, probes_json, ingress_hosts_json,
env_vars_json, secret_refs_json, config_files_json, writable_dirs_json, volume_mounts_json,
init_containers_json, values_override_json, created_at, updated_at FROM workload_stage_configs`
}
func workloadDefaultConfigSelect() string {
	return `SELECT id, tenant_id, project_id, application_id, workload_id, '' AS stage_key, replicas,
network_mode,
service_ports_json, resource_requests_json, resource_limits_json, probes_json, ingress_hosts_json,
env_vars_json, secret_refs_json, config_files_json, writable_dirs_json, volume_mounts_json,
init_containers_json, values_override_json, created_at, updated_at FROM workload_default_configs`
}
func applicationStageStateSelect() string {
	return "SELECT tenant_id, project_id, application_id, stage_key, status, message, last_reported_at, updated_at FROM application_stage_states"
}

func scanApplication(scanner appenvScanner) (Application, error) {
	var app Application
	err := scanner.Scan(&app.ID, &app.TenantID, &app.ProjectID, &app.Name, &app.DisplayName, &app.Description, &app.RuntimeEnvironmentID, &app.Status, &app.CreatedAt, &app.UpdatedAt)
	return app, err
}
func scanApplicationSource(scanner appenvScanner) (ApplicationSource, error) {
	var source ApplicationSource
	var checkoutPaths []byte
	err := scanner.Scan(&source.ID, &source.TenantID, &source.ProjectID, &source.ApplicationID, &source.Key, &source.DisplayName, &source.SourceType, &source.SourceURL, &source.SourceRef, &source.SVNRevision, &checkoutPaths, &source.JenkinsTemplateID, &source.BuildEnvironmentID, &source.SourcePath, &source.BuildSpec.BuildCommand, &source.BuildSpec.ArtifactCopyCommand, &source.BuildSpec.RuntimeBaseImage, &source.BuildSpec.ArtifactDeployPath, &source.BuildSpec.DefaultRef, &source.IsPrimary, &source.CreatedAt, &source.UpdatedAt)
	if err != nil {
		return source, err
	}
	if len(checkoutPaths) > 0 {
		if err := json.Unmarshal(checkoutPaths, &source.SVNCheckoutPaths); err != nil {
			return ApplicationSource{}, err
		}
	}
	source.BuildSpec.SourcePath = source.SourcePath
	return source, err
}
func scanWorkload(scanner appenvScanner) (Workload, error) {
	var workload Workload
	err := scanner.Scan(&workload.ID, &workload.TenantID, &workload.ProjectID, &workload.ApplicationID, &workload.Name, &workload.DisplayName, &workload.WorkloadType, &workload.Description, &workload.Status, &workload.ImageSourceMode, &workload.PipelineID, &workload.CreatedBy, &workload.CreatedAt, &workload.UpdatedAt)
	return workload, err
}
func scanWorkloadStageConfig(scanner appenvScanner) (WorkloadStageConfig, error) {
	var config WorkloadStageConfig
	var servicePorts, resourceRequests, resourceLimits, probes, ingressHosts, envVars, secretRefs, configFiles, writableDirs, volumeMounts, initContainers, valuesOverride []byte
	err := scanner.Scan(
		&config.ID, &config.TenantID, &config.ProjectID, &config.ApplicationID, &config.WorkloadID, &config.StageKey, &config.Replicas,
		&config.NetworkMode,
		&servicePorts, &resourceRequests, &resourceLimits, &probes, &ingressHosts, &envVars, &secretRefs, &configFiles, &writableDirs, &volumeMounts, &initContainers, &valuesOverride,
		&config.CreatedAt, &config.UpdatedAt,
	)
	if err != nil {
		return WorkloadStageConfig{}, err
	}
	config.NetworkMode = normalizeWorkloadNetworkMode(config.NetworkMode)
	if err := json.Unmarshal(servicePorts, &config.ServicePorts); err != nil {
		return WorkloadStageConfig{}, err
	}
	if err := json.Unmarshal(resourceRequests, &config.ResourceRequests); err != nil {
		return WorkloadStageConfig{}, err
	}
	if err := json.Unmarshal(resourceLimits, &config.ResourceLimits); err != nil {
		return WorkloadStageConfig{}, err
	}
	if err := json.Unmarshal(probes, &config.Probes); err != nil {
		return WorkloadStageConfig{}, err
	}
	if err := json.Unmarshal(ingressHosts, &config.IngressHosts); err != nil {
		return WorkloadStageConfig{}, err
	}
	if err := json.Unmarshal(envVars, &config.EnvVars); err != nil {
		return WorkloadStageConfig{}, err
	}
	if err := json.Unmarshal(secretRefs, &config.SecretRefs); err != nil {
		return WorkloadStageConfig{}, err
	}
	if err := json.Unmarshal(configFiles, &config.ConfigFiles); err != nil {
		return WorkloadStageConfig{}, err
	}
	if err := json.Unmarshal(writableDirs, &config.WritableDirs); err != nil {
		return WorkloadStageConfig{}, err
	}
	if err := json.Unmarshal(volumeMounts, &config.VolumeMounts); err != nil {
		return WorkloadStageConfig{}, err
	}
	if err := json.Unmarshal(initContainers, &config.InitContainers); err != nil {
		return WorkloadStageConfig{}, err
	}
	if err := json.Unmarshal(valuesOverride, &config.ValuesOverride); err != nil {
		return WorkloadStageConfig{}, err
	}
	return config, nil
}
func scanApplicationStageState(scanner appenvScanner) (ApplicationStageState, error) {
	var state ApplicationStageState
	err := scanner.Scan(&state.TenantID, &state.ProjectID, &state.ApplicationID, &state.StageKey, &state.Status, &state.Message, &state.LastReportedAt, &state.UpdatedAt)
	return state, err
}
func scanApplicationStageEvent(scanner appenvScanner) (ApplicationStageEvent, error) {
	var event ApplicationStageEvent
	err := scanner.Scan(&event.ID, &event.TenantID, &event.ProjectID, &event.ApplicationID, &event.StageKey, &event.Type, &event.Status, &event.Message, &event.OccurredAt)
	return event, err
}

func normalizedSourceKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return "main"
	}
	return key
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

func jsonText(value any) (string, error) {
	if value == nil {
		return "{}", nil
	}
	body, err := json.Marshal(value)
	if err != nil {
		return "", shared.WrapError(shared.CodeInvalidArgument, "invalid json field", err)
	}
	return string(body), nil
}
