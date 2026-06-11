package gitops

import (
	"context"
	"database/sql"
	"time"

	"github.com/shareinto/paas/internal/platform/database"
	"github.com/shareinto/paas/internal/shared"
)

type MySQLRepository struct {
	db *sql.DB
}

func NewMySQLRepository(_ context.Context, db *sql.DB) (*MySQLRepository, error) {
	return &MySQLRepository{db: db}, nil
}

func (r *MySQLRepository) CreateTemplate(ctx context.Context, template DeploymentTemplate) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO deployment_templates (id, tenant_id, project_id, application_id, name, scope, content, current_version, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		template.ID, template.TenantID, template.ProjectID, template.ApplicationID, template.Name,
		template.Scope, template.Content, template.CurrentVersion, mysqlTime(template.CreatedAt), mysqlTime(template.UpdatedAt))
	return database.ConflictOrUnavailable(err, "deployment template already exists", "create deployment template failed")
}

func (r *MySQLRepository) UpdateTemplate(ctx context.Context, template DeploymentTemplate) error {
	result, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
UPDATE deployment_templates
SET tenant_id = ?, project_id = ?, application_id = ?, name = ?, scope = ?, content = ?,
    current_version = ?, updated_at = ?
WHERE id = ?`,
		template.TenantID, template.ProjectID, template.ApplicationID, template.Name, template.Scope,
		template.Content, template.CurrentVersion, mysqlTime(template.UpdatedAt), template.ID)
	if err != nil {
		return database.ConflictOrUnavailable(err, "deployment template already exists", "update deployment template failed")
	}
	return database.RequireAffected(result, "deployment template not found")
}

func (r *MySQLRepository) GetTemplate(ctx context.Context, id shared.ID) (DeploymentTemplate, error) {
	template, err := scanTemplate(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, tenant_id, project_id, application_id, name, scope, content, current_version, created_at, updated_at
FROM deployment_templates WHERE id = ?`, id))
	if err != nil {
		return DeploymentTemplate{}, database.NotFound(err, "deployment template not found")
	}
	return template, nil
}

func (r *MySQLRepository) FindPlatformTemplate(ctx context.Context, name string) (DeploymentTemplate, error) {
	template, err := scanTemplate(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, tenant_id, project_id, application_id, name, scope, content, current_version, created_at, updated_at
FROM deployment_templates WHERE scope = ? AND name = ?`, TemplateScopePlatform, name))
	if err != nil {
		return DeploymentTemplate{}, database.NotFound(err, "platform template not found")
	}
	return template, nil
}

func (r *MySQLRepository) FindApplicationTemplate(ctx context.Context, applicationID shared.ID) (DeploymentTemplate, error) {
	template, err := scanTemplate(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, tenant_id, project_id, application_id, name, scope, content, current_version, created_at, updated_at
FROM deployment_templates WHERE scope = ? AND application_id = ?`, TemplateScopeApplication, applicationID))
	if err != nil {
		return DeploymentTemplate{}, database.NotFound(err, "application template not found")
	}
	return template, nil
}

func (r *MySQLRepository) CreateTemplateRevision(ctx context.Context, revision DeploymentTemplateRevision) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO deployment_template_revisions (id, template_id, version, content, created_by, created_at)
VALUES (?, ?, ?, ?, ?, ?)`,
		revision.ID, revision.TemplateID, revision.Version, revision.Content, revision.CreatedBy, mysqlTime(revision.CreatedAt))
	return database.ConflictOrUnavailable(err, "deployment template revision already exists", "create deployment template revision failed")
}

func (r *MySQLRepository) GetCurrentTemplateRevision(ctx context.Context, templateID shared.ID) (DeploymentTemplateRevision, error) {
	revision, err := scanTemplateRevision(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, template_id, version, content, created_by, created_at
FROM deployment_template_revisions
WHERE template_id = ?
ORDER BY version DESC, created_at DESC, id DESC LIMIT 1`, templateID))
	if err != nil {
		return DeploymentTemplateRevision{}, database.NotFound(err, "deployment template revision not found")
	}
	return revision, nil
}

func (r *MySQLRepository) CreateManifestRevision(ctx context.Context, revision ManifestRevision) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO manifest_revisions (id, deployment_id, promotion_id, application_id, environment_id, template_revision_id, path, commit_sha, merge_request_id, change_type, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		revision.ID, revision.DeploymentID, revision.PromotionID, revision.ApplicationID, revision.EnvironmentID,
		revision.TemplateRevisionID, revision.Path, revision.CommitSHA, revision.MergeRequestID, revision.ChangeType, mysqlTime(revision.CreatedAt))
	return database.ConflictOrUnavailable(err, "manifest revision already exists", "create manifest revision failed")
}

func (r *MySQLRepository) GetManifestRevision(ctx context.Context, id shared.ID) (ManifestRevision, error) {
	revision, err := scanManifestRevision(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, deployment_id, promotion_id, application_id, environment_id, template_revision_id, path, commit_sha, merge_request_id, change_type, created_at
FROM manifest_revisions WHERE id = ?`, id))
	if err != nil {
		return ManifestRevision{}, database.NotFound(err, "manifest revision not found")
	}
	return revision, nil
}

func (r *MySQLRepository) CreateDeployment(ctx context.Context, deployment Deployment) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO deployments (
  id, tenant_id, project_id, application_id, environment_id, cluster_binding_id, promotion_id,
  freight_id, manifest_revision_id, image_repository, image_tag, image_digest, workload_summary, status, message,
  created_at, updated_at, completed_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		deployment.ID, deployment.TenantID, deployment.ProjectID, deployment.ApplicationID, deployment.EnvironmentID,
		deployment.ClusterBindingID, deployment.PromotionID, deployment.FreightID, deployment.ManifestRevisionID,
		deployment.ImageRepository, deployment.ImageTag, deployment.ImageDigest, deployment.WorkloadSummary, deployment.Status, deployment.Message,
		mysqlTime(deployment.CreatedAt), mysqlTime(deployment.UpdatedAt), mysqlTimePtr(deployment.CompletedAt))
	return database.ConflictOrUnavailable(err, "deployment already exists", "create deployment failed")
}

func (r *MySQLRepository) UpdateDeployment(ctx context.Context, deployment Deployment) error {
	result, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
UPDATE deployments
SET manifest_revision_id = ?, image_repository = ?, image_tag = ?, image_digest = ?, workload_summary = ?,
    status = ?, message = ?, updated_at = ?, completed_at = ?
WHERE id = ?`,
		deployment.ManifestRevisionID, deployment.ImageRepository, deployment.ImageTag, deployment.ImageDigest,
		deployment.WorkloadSummary, deployment.Status, deployment.Message, mysqlTime(deployment.UpdatedAt), mysqlTimePtr(deployment.CompletedAt), deployment.ID)
	if err != nil {
		return database.WrapUnavailable(err, "update deployment failed")
	}
	return database.RequireAffected(result, "deployment not found")
}

func (r *MySQLRepository) GetDeployment(ctx context.Context, id shared.ID) (Deployment, error) {
	deployment, err := scanDeployment(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, tenant_id, project_id, application_id, environment_id, cluster_binding_id, promotion_id,
       freight_id, manifest_revision_id, image_repository, image_tag, image_digest, workload_summary, status, message,
       created_at, updated_at, completed_at
FROM deployments WHERE id = ?`, id))
	if err != nil {
		return Deployment{}, database.NotFound(err, "deployment not found")
	}
	return deployment, nil
}

func (r *MySQLRepository) FindDeploymentByPromotion(ctx context.Context, promotionID shared.ID) (Deployment, error) {
	deployment, err := scanDeployment(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, tenant_id, project_id, application_id, environment_id, cluster_binding_id, promotion_id,
       freight_id, manifest_revision_id, image_repository, image_tag, image_digest, workload_summary, status, message,
       created_at, updated_at, completed_at
FROM deployments WHERE promotion_id = ?`, promotionID))
	if err != nil {
		return Deployment{}, database.NotFound(err, "deployment not found")
	}
	return deployment, nil
}

func (r *MySQLRepository) ListDeployments(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[Deployment], error) {
	var total int64
	if err := database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, "SELECT COUNT(*) FROM deployments WHERE application_id = ?", applicationID).Scan(&total); err != nil {
		return shared.PageResult[Deployment]{}, database.WrapUnavailable(err, "count deployments failed")
	}
	page, limit, offset := database.LimitOffset(page)
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, `
SELECT id, tenant_id, project_id, application_id, environment_id, cluster_binding_id, promotion_id,
       freight_id, manifest_revision_id, image_repository, image_tag, image_digest, workload_summary, status, message,
       created_at, updated_at, completed_at
FROM deployments
WHERE application_id = ?
ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`, applicationID, limit, offset)
	if err != nil {
		return shared.PageResult[Deployment]{}, database.WrapUnavailable(err, "list deployments failed")
	}
	defer rows.Close()
	items := []Deployment{}
	for rows.Next() {
		deployment, err := scanDeployment(rows)
		if err != nil {
			return shared.PageResult[Deployment]{}, err
		}
		items = append(items, deployment)
	}
	if err := rows.Err(); err != nil {
		return shared.PageResult[Deployment]{}, database.WrapUnavailable(err, "list deployments failed")
	}
	return shared.NewPageResult(items, total, page), nil
}

func (r *MySQLRepository) CreateDeploymentEvent(ctx context.Context, event DeploymentEvent) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO deployment_events (id, deployment_id, status, message, occurred_at)
VALUES (?, ?, ?, ?, ?)`,
		event.ID, event.DeploymentID, event.Status, event.Message, mysqlTime(event.OccurredAt))
	return database.ConflictOrUnavailable(err, "deployment event already exists", "create deployment event failed")
}

type gitopsScanner interface {
	Scan(dest ...any) error
}

func scanTemplate(scanner gitopsScanner) (DeploymentTemplate, error) {
	var template DeploymentTemplate
	err := scanner.Scan(&template.ID, &template.TenantID, &template.ProjectID, &template.ApplicationID, &template.Name, &template.Scope, &template.Content, &template.CurrentVersion, &template.CreatedAt, &template.UpdatedAt)
	return template, err
}

func scanTemplateRevision(scanner gitopsScanner) (DeploymentTemplateRevision, error) {
	var revision DeploymentTemplateRevision
	err := scanner.Scan(&revision.ID, &revision.TemplateID, &revision.Version, &revision.Content, &revision.CreatedBy, &revision.CreatedAt)
	return revision, err
}

func scanManifestRevision(scanner gitopsScanner) (ManifestRevision, error) {
	var revision ManifestRevision
	err := scanner.Scan(&revision.ID, &revision.DeploymentID, &revision.PromotionID, &revision.ApplicationID, &revision.EnvironmentID, &revision.TemplateRevisionID, &revision.Path, &revision.CommitSHA, &revision.MergeRequestID, &revision.ChangeType, &revision.CreatedAt)
	return revision, err
}

func scanDeployment(scanner gitopsScanner) (Deployment, error) {
	var deployment Deployment
	err := scanner.Scan(&deployment.ID, &deployment.TenantID, &deployment.ProjectID, &deployment.ApplicationID, &deployment.EnvironmentID, &deployment.ClusterBindingID, &deployment.PromotionID, &deployment.FreightID, &deployment.ManifestRevisionID, &deployment.ImageRepository, &deployment.ImageTag, &deployment.ImageDigest, &deployment.WorkloadSummary, &deployment.Status, &deployment.Message, &deployment.CreatedAt, &deployment.UpdatedAt, &deployment.CompletedAt)
	return deployment, err
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
