package sourcerepository

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

func (r *MySQLRepository) CreateSourceRepository(ctx context.Context, repository SourceRepository) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO source_repositories (
  id, tenant_id, project_id, name, display_name, description, git_provider,
  git_project_id, http_url, ssh_url, default_branch, status, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		repository.ID, repository.TenantID, repository.ProjectID, repository.Name, repository.DisplayName,
		repository.Description, repository.GitProvider, repository.GitProjectID, repository.HTTPURL,
		repository.SSHURL, repository.DefaultBranch, repository.Status, mysqlTime(repository.CreatedAt), mysqlTime(repository.UpdatedAt))
	return database.ConflictOrUnavailable(err, "source repository already exists", "create source repository failed")
}

func (r *MySQLRepository) UpdateSourceRepository(ctx context.Context, repository SourceRepository) error {
	previous, err := r.GetSourceRepository(ctx, repository.ID)
	if err != nil {
		return err
	}
	if previous.ProjectID != repository.ProjectID || previous.TenantID != repository.TenantID {
		return shared.NewError(shared.CodeInvalidArgument, "source repository ownership cannot be changed")
	}
	result, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
UPDATE source_repositories
SET name = ?, display_name = ?, description = ?, git_provider = ?, git_project_id = ?,
    http_url = ?, ssh_url = ?, default_branch = ?, status = ?, updated_at = ?
WHERE id = ?`,
		repository.Name, repository.DisplayName, repository.Description, repository.GitProvider,
		repository.GitProjectID, repository.HTTPURL, repository.SSHURL, repository.DefaultBranch,
		repository.Status, mysqlTime(repository.UpdatedAt), repository.ID)
	if err != nil {
		return database.ConflictOrUnavailable(err, "source repository name already exists in project", "update source repository failed")
	}
	return database.RequireAffected(result, "source repository not found")
}

func (r *MySQLRepository) DeleteSourceRepository(ctx context.Context, id shared.ID) error {
	if _, err := r.GetSourceRepository(ctx, id); err != nil {
		return err
	}
	exec := database.ExecutorFromContext(ctx, r.db)
	if _, err := exec.ExecContext(ctx, "DELETE FROM source_repository_associations WHERE source_repository_id = ?", id); err != nil {
		return database.WrapUnavailable(err, "delete source repository associations failed")
	}
	if _, err := exec.ExecContext(ctx, "DELETE FROM repository_permission_sync_jobs WHERE source_repository_id = ?", id); err != nil {
		return database.WrapUnavailable(err, "delete repository permission sync jobs failed")
	}
	if _, err := exec.ExecContext(ctx, "DELETE FROM repository_migrations WHERE source_repository_id = ?", id); err != nil {
		return database.WrapUnavailable(err, "delete repository migrations failed")
	}
	result, err := exec.ExecContext(ctx, "DELETE FROM source_repositories WHERE id = ?", id)
	if err != nil {
		return database.WrapUnavailable(err, "delete source repository failed")
	}
	return database.RequireAffected(result, "source repository not found")
}

func (r *MySQLRepository) GetSourceRepository(ctx context.Context, id shared.ID) (SourceRepository, error) {
	repository, err := scanSourceRepository(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, tenant_id, project_id, name, display_name, description, git_provider,
       git_project_id, http_url, ssh_url, default_branch, status, created_at, updated_at
FROM source_repositories WHERE id = ?`, id))
	if err != nil {
		return SourceRepository{}, database.NotFound(err, "source repository not found")
	}
	return repository, nil
}

func (r *MySQLRepository) FindSourceRepositoryByProjectAndName(ctx context.Context, projectID shared.ID, name string) (SourceRepository, error) {
	repository, err := scanSourceRepository(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, tenant_id, project_id, name, display_name, description, git_provider,
       git_project_id, http_url, ssh_url, default_branch, status, created_at, updated_at
FROM source_repositories WHERE project_id = ? AND name = ?`, projectID, name))
	if err != nil {
		return SourceRepository{}, database.NotFound(err, "source repository not found")
	}
	return repository, nil
}

func (r *MySQLRepository) ListSourceRepositoriesByProject(ctx context.Context, projectID shared.ID, page shared.PageRequest) (shared.PageResult[SourceRepository], error) {
	var total int64
	if err := database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, "SELECT COUNT(*) FROM source_repositories WHERE project_id = ?", projectID).Scan(&total); err != nil {
		return shared.PageResult[SourceRepository]{}, database.WrapUnavailable(err, "count source repositories failed")
	}
	page, limit, offset := database.LimitOffset(page)
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, `
SELECT id, tenant_id, project_id, name, display_name, description, git_provider,
       git_project_id, http_url, ssh_url, default_branch, status, created_at, updated_at
FROM source_repositories WHERE project_id = ? ORDER BY name ASC LIMIT ? OFFSET ?`, projectID, limit, offset)
	if err != nil {
		return shared.PageResult[SourceRepository]{}, database.WrapUnavailable(err, "list source repositories failed")
	}
	defer rows.Close()
	items := []SourceRepository{}
	for rows.Next() {
		repository, err := scanSourceRepository(rows)
		if err != nil {
			return shared.PageResult[SourceRepository]{}, err
		}
		items = append(items, repository)
	}
	if err := rows.Err(); err != nil {
		return shared.PageResult[SourceRepository]{}, database.WrapUnavailable(err, "list source repositories failed")
	}
	return shared.NewPageResult(items, total, page), nil
}

func (r *MySQLRepository) ListAssociatedApplications(ctx context.Context, sourceRepositoryID shared.ID) ([]AssociatedApplication, error) {
	if _, err := r.GetSourceRepository(ctx, sourceRepositoryID); err != nil {
		return nil, err
	}
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, `
SELECT application_id, application_name, application_display_name
FROM source_repository_associations
WHERE source_repository_id = ?
ORDER BY application_name ASC`, sourceRepositoryID)
	if err != nil {
		return nil, database.WrapUnavailable(err, "list associated applications failed")
	}
	defer rows.Close()
	items := []AssociatedApplication{}
	for rows.Next() {
		var app AssociatedApplication
		if err := rows.Scan(&app.ID, &app.Name, &app.DisplayName); err != nil {
			return nil, err
		}
		items = append(items, app)
	}
	if err := rows.Err(); err != nil {
		return nil, database.WrapUnavailable(err, "list associated applications failed")
	}
	return items, nil
}

func (r *MySQLRepository) SetAssociatedApplications(ctx context.Context, sourceRepositoryID shared.ID, applications []AssociatedApplication) error {
	if _, err := r.GetSourceRepository(ctx, sourceRepositoryID); err != nil {
		return err
	}
	exec := database.ExecutorFromContext(ctx, r.db)
	if _, err := exec.ExecContext(ctx, "DELETE FROM source_repository_associations WHERE source_repository_id = ?", sourceRepositoryID); err != nil {
		return database.WrapUnavailable(err, "replace associated applications failed")
	}
	for _, app := range applications {
		if _, err := exec.ExecContext(ctx, `
INSERT INTO source_repository_associations (source_repository_id, application_id, application_name, application_display_name)
VALUES (?, ?, ?, ?)`, sourceRepositoryID, app.ID, app.Name, app.DisplayName); err != nil {
			return database.ConflictOrUnavailable(err, "associated application already exists", "replace associated applications failed")
		}
	}
	return nil
}

func (r *MySQLRepository) CreateMigration(ctx context.Context, migration RepositoryMigration) error {
	if _, err := r.GetSourceRepository(ctx, migration.SourceRepositoryID); err != nil {
		return err
	}
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO repository_migrations (
  id, tenant_id, project_id, source_repository_id, source_url, status, error_message,
  requested_by, created_at, updated_at, completed_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		migration.ID, migration.TenantID, migration.ProjectID, migration.SourceRepositoryID,
		migration.SourceURL, migration.Status, migration.ErrorMessage, migration.RequestedBy,
		mysqlTime(migration.CreatedAt), mysqlTime(migration.UpdatedAt), mysqlTimePtr(migration.CompletedAt))
	return database.ConflictOrUnavailable(err, "repository migration already exists", "create repository migration failed")
}

func (r *MySQLRepository) UpdateMigration(ctx context.Context, migration RepositoryMigration) error {
	result, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
UPDATE repository_migrations
SET status = ?, error_message = ?, updated_at = ?, completed_at = ?
WHERE id = ?`,
		migration.Status, migration.ErrorMessage, mysqlTime(migration.UpdatedAt), mysqlTimePtr(migration.CompletedAt), migration.ID)
	if err != nil {
		return database.WrapUnavailable(err, "update repository migration failed")
	}
	return database.RequireAffected(result, "repository migration not found")
}

func (r *MySQLRepository) GetMigration(ctx context.Context, id shared.ID) (RepositoryMigration, error) {
	migration, err := scanRepositoryMigration(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, tenant_id, project_id, source_repository_id, source_url, status, error_message,
       requested_by, created_at, updated_at, completed_at
FROM repository_migrations WHERE id = ?`, id))
	if err != nil {
		return RepositoryMigration{}, database.NotFound(err, "repository migration not found")
	}
	return migration, nil
}

func (r *MySQLRepository) ListMigrationsByRepository(ctx context.Context, sourceRepositoryID shared.ID, page shared.PageRequest) (shared.PageResult[RepositoryMigration], error) {
	if _, err := r.GetSourceRepository(ctx, sourceRepositoryID); err != nil {
		return shared.PageResult[RepositoryMigration]{}, err
	}
	var total int64
	if err := database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, "SELECT COUNT(*) FROM repository_migrations WHERE source_repository_id = ?", sourceRepositoryID).Scan(&total); err != nil {
		return shared.PageResult[RepositoryMigration]{}, database.WrapUnavailable(err, "count repository migrations failed")
	}
	page, limit, offset := database.LimitOffset(page)
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, `
SELECT id, tenant_id, project_id, source_repository_id, source_url, status, error_message,
       requested_by, created_at, updated_at, completed_at
FROM repository_migrations
WHERE source_repository_id = ?
ORDER BY created_at ASC, id ASC LIMIT ? OFFSET ?`, sourceRepositoryID, limit, offset)
	if err != nil {
		return shared.PageResult[RepositoryMigration]{}, database.WrapUnavailable(err, "list repository migrations failed")
	}
	defer rows.Close()
	items := []RepositoryMigration{}
	for rows.Next() {
		migration, err := scanRepositoryMigration(rows)
		if err != nil {
			return shared.PageResult[RepositoryMigration]{}, err
		}
		items = append(items, migration)
	}
	if err := rows.Err(); err != nil {
		return shared.PageResult[RepositoryMigration]{}, database.WrapUnavailable(err, "list repository migrations failed")
	}
	return shared.NewPageResult(items, total, page), nil
}

func (r *MySQLRepository) CreatePermissionSyncJob(ctx context.Context, job RepositoryPermissionSyncJob) error {
	if _, err := r.GetSourceRepository(ctx, job.SourceRepositoryID); err != nil {
		return err
	}
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO repository_permission_sync_jobs (
  id, tenant_id, project_id, source_repository_id, status, error_message,
  requested_by, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.TenantID, job.ProjectID, job.SourceRepositoryID, job.Status,
		job.ErrorMessage, job.RequestedBy, mysqlTime(job.CreatedAt), mysqlTime(job.UpdatedAt))
	return database.ConflictOrUnavailable(err, "repository permission sync job already exists", "create repository permission sync job failed")
}

func (r *MySQLRepository) UpdatePermissionSyncJob(ctx context.Context, job RepositoryPermissionSyncJob) error {
	result, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
UPDATE repository_permission_sync_jobs
SET status = ?, error_message = ?, updated_at = ?
WHERE id = ?`,
		job.Status, job.ErrorMessage, mysqlTime(job.UpdatedAt), job.ID)
	if err != nil {
		return database.WrapUnavailable(err, "update repository permission sync job failed")
	}
	return database.RequireAffected(result, "repository permission sync job not found")
}

func (r *MySQLRepository) GetPermissionSyncJob(ctx context.Context, id shared.ID) (RepositoryPermissionSyncJob, error) {
	job, err := scanPermissionSyncJob(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, tenant_id, project_id, source_repository_id, status, error_message,
       requested_by, created_at, updated_at
FROM repository_permission_sync_jobs WHERE id = ?`, id))
	if err != nil {
		return RepositoryPermissionSyncJob{}, database.NotFound(err, "repository permission sync job not found")
	}
	return job, nil
}

type sourceRepositoryScanner interface {
	Scan(dest ...any) error
}

func scanSourceRepository(scanner sourceRepositoryScanner) (SourceRepository, error) {
	var repository SourceRepository
	err := scanner.Scan(&repository.ID, &repository.TenantID, &repository.ProjectID, &repository.Name, &repository.DisplayName, &repository.Description, &repository.GitProvider, &repository.GitProjectID, &repository.HTTPURL, &repository.SSHURL, &repository.DefaultBranch, &repository.Status, &repository.CreatedAt, &repository.UpdatedAt)
	return repository, err
}

func scanRepositoryMigration(scanner sourceRepositoryScanner) (RepositoryMigration, error) {
	var migration RepositoryMigration
	err := scanner.Scan(&migration.ID, &migration.TenantID, &migration.ProjectID, &migration.SourceRepositoryID, &migration.SourceURL, &migration.Status, &migration.ErrorMessage, &migration.RequestedBy, &migration.CreatedAt, &migration.UpdatedAt, &migration.CompletedAt)
	return migration, err
}

func scanPermissionSyncJob(scanner sourceRepositoryScanner) (RepositoryPermissionSyncJob, error) {
	var job RepositoryPermissionSyncJob
	err := scanner.Scan(&job.ID, &job.TenantID, &job.ProjectID, &job.SourceRepositoryID, &job.Status, &job.ErrorMessage, &job.RequestedBy, &job.CreatedAt, &job.UpdatedAt)
	return job, err
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
