package tenantproject

import (
	"context"
	"database/sql"

	"github.com/shareinto/paas/internal/platform/database"
	"github.com/shareinto/paas/internal/shared"
)

type MySQLRepository struct {
	inner *MemoryRepository
	store *database.SnapshotStore
}

type tenantProjectSnapshot struct {
	Tenants  []Tenant
	Members  []TenantMember
	Projects []Project
}

func NewMySQLRepository(ctx context.Context, db *sql.DB) (*MySQLRepository, error) {
	repo := &MySQLRepository{inner: NewMemoryRepository(), store: database.NewSnapshotStore(db, "tenant-project")}
	var snapshot tenantProjectSnapshot
	if err := repo.store.Load(ctx, &snapshot); err != nil {
		return nil, err
	}
	repo.restore(snapshot)
	return repo, nil
}

func (r *MySQLRepository) restore(snapshot tenantProjectSnapshot) {
	r.inner.mu.Lock()
	defer r.inner.mu.Unlock()
	for _, tenant := range snapshot.Tenants {
		r.inner.tenants[tenant.ID] = tenant
		r.inner.tenantsByName[tenant.Name] = tenant.ID
	}
	for _, member := range snapshot.Members {
		if r.inner.members[member.TenantID] == nil {
			r.inner.members[member.TenantID] = map[shared.ID]TenantMember{}
		}
		r.inner.members[member.TenantID][member.UserID] = member
	}
	for _, project := range snapshot.Projects {
		r.inner.projects[project.ID] = project
		r.inner.projectNameIndex[projectNameKey{tenantID: project.TenantID, name: project.Name}] = project.ID
	}
}

func (r *MySQLRepository) snapshot() tenantProjectSnapshot {
	r.inner.mu.RLock()
	defer r.inner.mu.RUnlock()
	out := tenantProjectSnapshot{Tenants: make([]Tenant, 0, len(r.inner.tenants)), Projects: make([]Project, 0, len(r.inner.projects))}
	for _, value := range r.inner.tenants {
		out.Tenants = append(out.Tenants, value)
	}
	for _, byUser := range r.inner.members {
		for _, value := range byUser {
			out.Members = append(out.Members, value)
		}
	}
	for _, value := range r.inner.projects {
		out.Projects = append(out.Projects, value)
	}
	return out
}

func (r *MySQLRepository) persist(ctx context.Context) error { return r.store.Save(ctx, r.snapshot()) }

func (r *MySQLRepository) CreateTenant(ctx context.Context, tenant Tenant) error {
	if err := r.inner.CreateTenant(ctx, tenant); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) UpdateTenant(ctx context.Context, tenant Tenant) error {
	if err := r.inner.UpdateTenant(ctx, tenant); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) GetTenant(ctx context.Context, id shared.ID) (Tenant, error) {
	return r.inner.GetTenant(ctx, id)
}
func (r *MySQLRepository) FindTenantByName(ctx context.Context, name string) (Tenant, error) {
	return r.inner.FindTenantByName(ctx, name)
}
func (r *MySQLRepository) ListTenants(ctx context.Context, page shared.PageRequest) (shared.PageResult[Tenant], error) {
	return r.inner.ListTenants(ctx, page)
}
func (r *MySQLRepository) SaveTenantMember(ctx context.Context, member TenantMember) error {
	if err := r.inner.SaveTenantMember(ctx, member); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) DeleteTenantMember(ctx context.Context, tenantID shared.ID, userID shared.ID) error {
	if err := r.inner.DeleteTenantMember(ctx, tenantID, userID); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) GetTenantMember(ctx context.Context, tenantID shared.ID, userID shared.ID) (TenantMember, error) {
	return r.inner.GetTenantMember(ctx, tenantID, userID)
}
func (r *MySQLRepository) ListTenantMembers(ctx context.Context, tenantID shared.ID) ([]TenantMember, error) {
	return r.inner.ListTenantMembers(ctx, tenantID)
}
func (r *MySQLRepository) CreateProject(ctx context.Context, project Project) error {
	if err := r.inner.CreateProject(ctx, project); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) UpdateProject(ctx context.Context, project Project) error {
	if err := r.inner.UpdateProject(ctx, project); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) DeleteProject(ctx context.Context, id shared.ID) error {
	if err := r.inner.DeleteProject(ctx, id); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) GetProject(ctx context.Context, id shared.ID) (Project, error) {
	return r.inner.GetProject(ctx, id)
}
func (r *MySQLRepository) FindProjectByTenantAndName(ctx context.Context, tenantID shared.ID, name string) (Project, error) {
	return r.inner.FindProjectByTenantAndName(ctx, tenantID, name)
}
func (r *MySQLRepository) ListProjectsByTenant(ctx context.Context, tenantID shared.ID, page shared.PageRequest) (shared.PageResult[Project], error) {
	return r.inner.ListProjectsByTenant(ctx, tenantID, page)
}
