package tenantproject

import (
	"context"
	"database/sql"
	"time"

	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/platform/database"
	"github.com/shareinto/paas/internal/shared"
)

type MySQLRepository struct {
	db *sql.DB
}

func NewMySQLRepository(_ context.Context, db *sql.DB) (*MySQLRepository, error) {
	return &MySQLRepository{db: db}, nil
}

func (r *MySQLRepository) CreateTenant(ctx context.Context, tenant Tenant) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO tenants (id, name, display_name, description, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)`,
		tenant.ID, tenant.Name, tenant.DisplayName, tenant.Description, mysqlTime(tenant.CreatedAt), mysqlTime(tenant.UpdatedAt))
	return database.ConflictOrUnavailable(err, "tenant already exists", "create tenant failed")
}

func (r *MySQLRepository) UpdateTenant(ctx context.Context, tenant Tenant) error {
	result, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
UPDATE tenants SET name = ?, display_name = ?, description = ?, updated_at = ? WHERE id = ?`,
		tenant.Name, tenant.DisplayName, tenant.Description, mysqlTime(tenant.UpdatedAt), tenant.ID)
	if err != nil {
		return database.ConflictOrUnavailable(err, "tenant name already exists", "update tenant failed")
	}
	return database.RequireAffected(result, "tenant not found")
}

func (r *MySQLRepository) GetTenant(ctx context.Context, id shared.ID) (Tenant, error) {
	tenant, err := scanTenant(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, name, display_name, description, created_at, updated_at FROM tenants WHERE id = ?`, id))
	if err != nil {
		return Tenant{}, database.NotFound(err, "tenant not found")
	}
	return tenant, nil
}

func (r *MySQLRepository) FindTenantByName(ctx context.Context, name string) (Tenant, error) {
	tenant, err := scanTenant(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, name, display_name, description, created_at, updated_at FROM tenants WHERE name = ?`, name))
	if err != nil {
		return Tenant{}, database.NotFound(err, "tenant not found")
	}
	return tenant, nil
}

func (r *MySQLRepository) ListTenants(ctx context.Context, page shared.PageRequest) (shared.PageResult[Tenant], error) {
	var total int64
	if err := database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, "SELECT COUNT(*) FROM tenants").Scan(&total); err != nil {
		return shared.PageResult[Tenant]{}, database.WrapUnavailable(err, "count tenants failed")
	}
	page, limit, offset := database.LimitOffset(page)
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, `
SELECT id, name, display_name, description, created_at, updated_at
FROM tenants ORDER BY name ASC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return shared.PageResult[Tenant]{}, database.WrapUnavailable(err, "list tenants failed")
	}
	defer rows.Close()
	items := []Tenant{}
	for rows.Next() {
		tenant, err := scanTenant(rows)
		if err != nil {
			return shared.PageResult[Tenant]{}, err
		}
		items = append(items, tenant)
	}
	if err := rows.Err(); err != nil {
		return shared.PageResult[Tenant]{}, database.WrapUnavailable(err, "list tenants failed")
	}
	return shared.NewPageResult(items, total, page), nil
}

func (r *MySQLRepository) SaveTenantMember(ctx context.Context, member TenantMember) error {
	if _, err := r.GetTenant(ctx, member.TenantID); err != nil {
		return err
	}
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO tenant_members (tenant_id, user_id, role_id, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE role_id = VALUES(role_id), updated_at = VALUES(updated_at)`,
		member.TenantID, member.UserID, member.RoleID, mysqlTime(member.CreatedAt), mysqlTime(member.UpdatedAt))
	return database.WrapUnavailable(err, "save tenant member failed")
}

func (r *MySQLRepository) DeleteTenantMember(ctx context.Context, tenantID shared.ID, userID shared.ID) error {
	if _, err := r.GetTenant(ctx, tenantID); err != nil {
		return err
	}
	result, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
DELETE FROM tenant_members WHERE tenant_id = ? AND user_id = ?`, tenantID, userID)
	if err != nil {
		return database.WrapUnavailable(err, "delete tenant member failed")
	}
	return database.RequireAffected(result, "tenant member not found")
}

func (r *MySQLRepository) GetTenantMember(ctx context.Context, tenantID shared.ID, userID shared.ID) (TenantMember, error) {
	member, err := scanTenantMember(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT tenant_id, user_id, role_id, created_at, updated_at
FROM tenant_members WHERE tenant_id = ? AND user_id = ?`, tenantID, userID))
	if err != nil {
		return TenantMember{}, database.NotFound(err, "tenant member not found")
	}
	return member, nil
}

func (r *MySQLRepository) ListTenantMembers(ctx context.Context, tenantID shared.ID) ([]TenantMember, error) {
	if _, err := r.GetTenant(ctx, tenantID); err != nil {
		return nil, err
	}
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, `
SELECT tenant_id, user_id, role_id, created_at, updated_at
FROM tenant_members WHERE tenant_id = ? ORDER BY user_id ASC`, tenantID)
	if err != nil {
		return nil, database.WrapUnavailable(err, "list tenant members failed")
	}
	defer rows.Close()
	items := []TenantMember{}
	for rows.Next() {
		member, err := scanTenantMember(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, member)
	}
	if err := rows.Err(); err != nil {
		return nil, database.WrapUnavailable(err, "list tenant members failed")
	}
	return items, nil
}

func (r *MySQLRepository) CreateProject(ctx context.Context, project Project) error {
	if _, err := r.GetTenant(ctx, project.TenantID); err != nil {
		return err
	}
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO projects (id, tenant_id, name, display_name, description, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
		project.ID, project.TenantID, project.Name, project.DisplayName, project.Description, mysqlTime(project.CreatedAt), mysqlTime(project.UpdatedAt))
	return database.ConflictOrUnavailable(err, "project already exists", "create project failed")
}

func (r *MySQLRepository) UpdateProject(ctx context.Context, project Project) error {
	previous, err := r.GetProject(ctx, project.ID)
	if err != nil {
		return err
	}
	if _, err := r.GetTenant(ctx, project.TenantID); err != nil {
		return err
	}
	if previous.TenantID != project.TenantID {
		return shared.NewError(shared.CodeInvalidArgument, "project tenant cannot be changed")
	}
	result, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
UPDATE projects SET name = ?, display_name = ?, description = ?, updated_at = ? WHERE id = ?`,
		project.Name, project.DisplayName, project.Description, mysqlTime(project.UpdatedAt), project.ID)
	if err != nil {
		return database.ConflictOrUnavailable(err, "project name already exists in tenant", "update project failed")
	}
	return database.RequireAffected(result, "project not found")
}

func (r *MySQLRepository) DeleteProject(ctx context.Context, id shared.ID) error {
	result, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, "DELETE FROM projects WHERE id = ?", id)
	if err != nil {
		return database.WrapUnavailable(err, "delete project failed")
	}
	return database.RequireAffected(result, "project not found")
}

func (r *MySQLRepository) GetProject(ctx context.Context, id shared.ID) (Project, error) {
	project, err := scanProject(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, tenant_id, name, display_name, description, created_at, updated_at FROM projects WHERE id = ?`, id))
	if err != nil {
		return Project{}, database.NotFound(err, "project not found")
	}
	return project, nil
}

func (r *MySQLRepository) FindProjectByTenantAndName(ctx context.Context, tenantID shared.ID, name string) (Project, error) {
	project, err := scanProject(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, tenant_id, name, display_name, description, created_at, updated_at
FROM projects WHERE tenant_id = ? AND name = ?`, tenantID, name))
	if err != nil {
		return Project{}, database.NotFound(err, "project not found")
	}
	return project, nil
}

func (r *MySQLRepository) ListProjectsByTenant(ctx context.Context, tenantID shared.ID, page shared.PageRequest) (shared.PageResult[Project], error) {
	if _, err := r.GetTenant(ctx, tenantID); err != nil {
		return shared.PageResult[Project]{}, err
	}
	var total int64
	if err := database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, "SELECT COUNT(*) FROM projects WHERE tenant_id = ?", tenantID).Scan(&total); err != nil {
		return shared.PageResult[Project]{}, database.WrapUnavailable(err, "count projects failed")
	}
	page, limit, offset := database.LimitOffset(page)
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, `
SELECT id, tenant_id, name, display_name, description, created_at, updated_at
FROM projects WHERE tenant_id = ? ORDER BY name ASC LIMIT ? OFFSET ?`, tenantID, limit, offset)
	if err != nil {
		return shared.PageResult[Project]{}, database.WrapUnavailable(err, "list projects failed")
	}
	defer rows.Close()
	items := []Project{}
	for rows.Next() {
		project, err := scanProject(rows)
		if err != nil {
			return shared.PageResult[Project]{}, err
		}
		items = append(items, project)
	}
	if err := rows.Err(); err != nil {
		return shared.PageResult[Project]{}, database.WrapUnavailable(err, "list projects failed")
	}
	return shared.NewPageResult(items, total, page), nil
}

type tenantProjectScanner interface {
	Scan(dest ...any) error
}

func scanTenant(scanner tenantProjectScanner) (Tenant, error) {
	var tenant Tenant
	err := scanner.Scan(&tenant.ID, &tenant.Name, &tenant.DisplayName, &tenant.Description, &tenant.CreatedAt, &tenant.UpdatedAt)
	return tenant, err
}

func scanTenantMember(scanner tenantProjectScanner) (TenantMember, error) {
	var member TenantMember
	var roleID string
	err := scanner.Scan(&member.TenantID, &member.UserID, &roleID, &member.CreatedAt, &member.UpdatedAt)
	member.RoleID = identityaccess.RoleID(roleID)
	return member, err
}

func scanProject(scanner tenantProjectScanner) (Project, error) {
	var project Project
	err := scanner.Scan(&project.ID, &project.TenantID, &project.Name, &project.DisplayName, &project.Description, &project.CreatedAt, &project.UpdatedAt)
	return project, err
}

func mysqlTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value
}
