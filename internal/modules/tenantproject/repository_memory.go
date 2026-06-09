package tenantproject

import (
	"context"
	"sort"
	"sync"

	"github.com/shareinto/paas/internal/shared"
)

type MemoryRepository struct {
	mu               sync.RWMutex
	tenants          map[shared.ID]Tenant
	tenantsByName    map[string]shared.ID
	members          map[shared.ID]map[shared.ID]TenantMember
	projects         map[shared.ID]Project
	projectNameIndex map[projectNameKey]shared.ID
}

type projectNameKey struct {
	tenantID shared.ID
	name     string
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		tenants:          map[shared.ID]Tenant{},
		tenantsByName:    map[string]shared.ID{},
		members:          map[shared.ID]map[shared.ID]TenantMember{},
		projects:         map[shared.ID]Project{},
		projectNameIndex: map[projectNameKey]shared.ID{},
	}
}

func (r *MemoryRepository) CreateTenant(_ context.Context, tenant Tenant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tenants[tenant.ID]; exists {
		return shared.NewError(shared.CodeConflict, "tenant already exists")
	}
	if _, exists := r.tenantsByName[tenant.Name]; exists {
		return shared.NewError(shared.CodeConflict, "tenant name already exists")
	}
	r.tenants[tenant.ID] = tenant
	r.tenantsByName[tenant.Name] = tenant.ID
	return nil
}

func (r *MemoryRepository) UpdateTenant(_ context.Context, tenant Tenant) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	previous, ok := r.tenants[tenant.ID]
	if !ok {
		return shared.NewError(shared.CodeNotFound, "tenant not found")
	}
	if previous.Name != tenant.Name {
		if _, exists := r.tenantsByName[tenant.Name]; exists {
			return shared.NewError(shared.CodeConflict, "tenant name already exists")
		}
		delete(r.tenantsByName, previous.Name)
		r.tenantsByName[tenant.Name] = tenant.ID
	}
	r.tenants[tenant.ID] = tenant
	return nil
}

func (r *MemoryRepository) GetTenant(_ context.Context, id shared.ID) (Tenant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tenant, ok := r.tenants[id]
	if !ok {
		return Tenant{}, shared.NewError(shared.CodeNotFound, "tenant not found")
	}
	return tenant, nil
}

func (r *MemoryRepository) FindTenantByName(_ context.Context, name string) (Tenant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.tenantsByName[name]
	if !ok {
		return Tenant{}, shared.NewError(shared.CodeNotFound, "tenant not found")
	}
	return r.tenants[id], nil
}

func (r *MemoryRepository) ListTenants(_ context.Context, page shared.PageRequest) (shared.PageResult[Tenant], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	page = page.Normalize()
	tenants := make([]Tenant, 0, len(r.tenants))
	for _, tenant := range r.tenants {
		tenants = append(tenants, tenant)
	}
	sort.Slice(tenants, func(i, j int) bool { return tenants[i].Name < tenants[j].Name })
	start := page.Offset()
	if start > len(tenants) {
		start = len(tenants)
	}
	end := start + page.PageSize
	if end > len(tenants) {
		end = len(tenants)
	}
	return shared.NewPageResult(tenants[start:end], int64(len(tenants)), page), nil
}

func (r *MemoryRepository) SaveTenantMember(_ context.Context, member TenantMember) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tenants[member.TenantID]; !ok {
		return shared.NewError(shared.CodeNotFound, "tenant not found")
	}
	if r.members[member.TenantID] == nil {
		r.members[member.TenantID] = map[shared.ID]TenantMember{}
	}
	r.members[member.TenantID][member.UserID] = member
	return nil
}

func (r *MemoryRepository) DeleteTenantMember(_ context.Context, tenantID shared.ID, userID shared.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tenants[tenantID]; !ok {
		return shared.NewError(shared.CodeNotFound, "tenant not found")
	}
	if _, ok := r.members[tenantID][userID]; !ok {
		return shared.NewError(shared.CodeNotFound, "tenant member not found")
	}
	delete(r.members[tenantID], userID)
	return nil
}

func (r *MemoryRepository) GetTenantMember(_ context.Context, tenantID shared.ID, userID shared.ID) (TenantMember, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	member, ok := r.members[tenantID][userID]
	if !ok {
		return TenantMember{}, shared.NewError(shared.CodeNotFound, "tenant member not found")
	}
	return member, nil
}

func (r *MemoryRepository) ListTenantMembers(_ context.Context, tenantID shared.ID) ([]TenantMember, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.tenants[tenantID]; !ok {
		return nil, shared.NewError(shared.CodeNotFound, "tenant not found")
	}
	members := make([]TenantMember, 0, len(r.members[tenantID]))
	for _, member := range r.members[tenantID] {
		members = append(members, member)
	}
	sort.Slice(members, func(i, j int) bool { return members[i].UserID < members[j].UserID })
	return members, nil
}

func (r *MemoryRepository) CreateProject(_ context.Context, project Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tenants[project.TenantID]; !ok {
		return shared.NewError(shared.CodeNotFound, "tenant not found")
	}
	if _, exists := r.projects[project.ID]; exists {
		return shared.NewError(shared.CodeConflict, "project already exists")
	}
	key := projectNameKey{tenantID: project.TenantID, name: project.Name}
	if _, exists := r.projectNameIndex[key]; exists {
		return shared.NewError(shared.CodeConflict, "project name already exists in tenant")
	}
	r.projects[project.ID] = project
	r.projectNameIndex[key] = project.ID
	return nil
}

func (r *MemoryRepository) UpdateProject(_ context.Context, project Project) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	previous, ok := r.projects[project.ID]
	if !ok {
		return shared.NewError(shared.CodeNotFound, "project not found")
	}
	if _, ok := r.tenants[project.TenantID]; !ok {
		return shared.NewError(shared.CodeNotFound, "tenant not found")
	}
	if previous.TenantID != project.TenantID {
		return shared.NewError(shared.CodeInvalidArgument, "project tenant cannot be changed")
	}
	previousKey := projectNameKey{tenantID: previous.TenantID, name: previous.Name}
	nextKey := projectNameKey{tenantID: project.TenantID, name: project.Name}
	if previousKey != nextKey {
		if _, exists := r.projectNameIndex[nextKey]; exists {
			return shared.NewError(shared.CodeConflict, "project name already exists in tenant")
		}
		delete(r.projectNameIndex, previousKey)
		r.projectNameIndex[nextKey] = project.ID
	}
	r.projects[project.ID] = project
	return nil
}

func (r *MemoryRepository) DeleteProject(_ context.Context, id shared.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	project, ok := r.projects[id]
	if !ok {
		return shared.NewError(shared.CodeNotFound, "project not found")
	}
	delete(r.projects, id)
	delete(r.projectNameIndex, projectNameKey{tenantID: project.TenantID, name: project.Name})
	return nil
}

func (r *MemoryRepository) GetProject(_ context.Context, id shared.ID) (Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	project, ok := r.projects[id]
	if !ok {
		return Project{}, shared.NewError(shared.CodeNotFound, "project not found")
	}
	return project, nil
}

func (r *MemoryRepository) FindProjectByTenantAndName(_ context.Context, tenantID shared.ID, name string) (Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.projectNameIndex[projectNameKey{tenantID: tenantID, name: name}]
	if !ok {
		return Project{}, shared.NewError(shared.CodeNotFound, "project not found")
	}
	return r.projects[id], nil
}

func (r *MemoryRepository) ListProjectsByTenant(_ context.Context, tenantID shared.ID, page shared.PageRequest) (shared.PageResult[Project], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.tenants[tenantID]; !ok {
		return shared.PageResult[Project]{}, shared.NewError(shared.CodeNotFound, "tenant not found")
	}
	page = page.Normalize()
	projects := make([]Project, 0)
	for _, project := range r.projects {
		if project.TenantID == tenantID {
			projects = append(projects, project)
		}
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })
	start := page.Offset()
	if start > len(projects) {
		start = len(projects)
	}
	end := start + page.PageSize
	if end > len(projects) {
		end = len(projects)
	}
	return shared.NewPageResult(projects[start:end], int64(len(projects)), page), nil
}
