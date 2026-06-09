package sourcerepository

import (
	"context"
	"sort"
	"sync"

	"github.com/shareinto/paas/internal/shared"
)

type MemoryRepository struct {
	mu                  sync.RWMutex
	repositories        map[shared.ID]SourceRepository
	repositoryNameIndex map[repositoryNameKey]shared.ID
	applications        map[shared.ID][]AssociatedApplication
	migrations          map[shared.ID]RepositoryMigration
	migrationsByRepo    map[shared.ID][]shared.ID
	permissionJobs      map[shared.ID]RepositoryPermissionSyncJob
}

type repositoryNameKey struct {
	projectID shared.ID
	name      string
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		repositories:        map[shared.ID]SourceRepository{},
		repositoryNameIndex: map[repositoryNameKey]shared.ID{},
		applications:        map[shared.ID][]AssociatedApplication{},
		migrations:          map[shared.ID]RepositoryMigration{},
		migrationsByRepo:    map[shared.ID][]shared.ID{},
		permissionJobs:      map[shared.ID]RepositoryPermissionSyncJob{},
	}
}

func (r *MemoryRepository) CreateSourceRepository(_ context.Context, repository SourceRepository) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.repositories[repository.ID]; exists {
		return shared.NewError(shared.CodeConflict, "source repository already exists")
	}
	key := repositoryNameKey{projectID: repository.ProjectID, name: repository.Name}
	if _, exists := r.repositoryNameIndex[key]; exists {
		return shared.NewError(shared.CodeConflict, "source repository name already exists in project")
	}
	r.repositories[repository.ID] = repository
	r.repositoryNameIndex[key] = repository.ID
	return nil
}

func (r *MemoryRepository) UpdateSourceRepository(_ context.Context, repository SourceRepository) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	previous, ok := r.repositories[repository.ID]
	if !ok {
		return shared.NewError(shared.CodeNotFound, "source repository not found")
	}
	if previous.ProjectID != repository.ProjectID || previous.TenantID != repository.TenantID {
		return shared.NewError(shared.CodeInvalidArgument, "source repository ownership cannot be changed")
	}
	previousKey := repositoryNameKey{projectID: previous.ProjectID, name: previous.Name}
	nextKey := repositoryNameKey{projectID: repository.ProjectID, name: repository.Name}
	if previousKey != nextKey {
		if _, exists := r.repositoryNameIndex[nextKey]; exists {
			return shared.NewError(shared.CodeConflict, "source repository name already exists in project")
		}
		delete(r.repositoryNameIndex, previousKey)
		r.repositoryNameIndex[nextKey] = repository.ID
	}
	r.repositories[repository.ID] = repository
	return nil
}

func (r *MemoryRepository) DeleteSourceRepository(_ context.Context, id shared.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	repository, ok := r.repositories[id]
	if !ok {
		return shared.NewError(shared.CodeNotFound, "source repository not found")
	}
	delete(r.repositories, id)
	delete(r.repositoryNameIndex, repositoryNameKey{projectID: repository.ProjectID, name: repository.Name})
	delete(r.applications, id)
	delete(r.migrationsByRepo, id)
	return nil
}

func (r *MemoryRepository) GetSourceRepository(_ context.Context, id shared.ID) (SourceRepository, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	repository, ok := r.repositories[id]
	if !ok {
		return SourceRepository{}, shared.NewError(shared.CodeNotFound, "source repository not found")
	}
	return repository, nil
}

func (r *MemoryRepository) FindSourceRepositoryByProjectAndName(_ context.Context, projectID shared.ID, name string) (SourceRepository, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.repositoryNameIndex[repositoryNameKey{projectID: projectID, name: name}]
	if !ok {
		return SourceRepository{}, shared.NewError(shared.CodeNotFound, "source repository not found")
	}
	return r.repositories[id], nil
}

func (r *MemoryRepository) ListSourceRepositoriesByProject(_ context.Context, projectID shared.ID, page shared.PageRequest) (shared.PageResult[SourceRepository], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	page = page.Normalize()
	items := make([]SourceRepository, 0)
	for _, repository := range r.repositories {
		if repository.ProjectID == projectID {
			items = append(items, repository)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
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

func (r *MemoryRepository) ListAssociatedApplications(_ context.Context, sourceRepositoryID shared.ID) ([]AssociatedApplication, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.repositories[sourceRepositoryID]; !ok {
		return nil, shared.NewError(shared.CodeNotFound, "source repository not found")
	}
	applications := append([]AssociatedApplication(nil), r.applications[sourceRepositoryID]...)
	sort.Slice(applications, func(i, j int) bool { return applications[i].Name < applications[j].Name })
	return applications, nil
}

func (r *MemoryRepository) SetAssociatedApplications(_ context.Context, sourceRepositoryID shared.ID, applications []AssociatedApplication) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.repositories[sourceRepositoryID]; !ok {
		return shared.NewError(shared.CodeNotFound, "source repository not found")
	}
	r.applications[sourceRepositoryID] = append([]AssociatedApplication(nil), applications...)
	return nil
}

func (r *MemoryRepository) CreateMigration(_ context.Context, migration RepositoryMigration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.repositories[migration.SourceRepositoryID]; !ok {
		return shared.NewError(shared.CodeNotFound, "source repository not found")
	}
	if _, exists := r.migrations[migration.ID]; exists {
		return shared.NewError(shared.CodeConflict, "repository migration already exists")
	}
	r.migrations[migration.ID] = migration
	r.migrationsByRepo[migration.SourceRepositoryID] = append(r.migrationsByRepo[migration.SourceRepositoryID], migration.ID)
	return nil
}

func (r *MemoryRepository) UpdateMigration(_ context.Context, migration RepositoryMigration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.migrations[migration.ID]; !ok {
		return shared.NewError(shared.CodeNotFound, "repository migration not found")
	}
	r.migrations[migration.ID] = migration
	return nil
}

func (r *MemoryRepository) GetMigration(_ context.Context, id shared.ID) (RepositoryMigration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	migration, ok := r.migrations[id]
	if !ok {
		return RepositoryMigration{}, shared.NewError(shared.CodeNotFound, "repository migration not found")
	}
	return migration, nil
}

func (r *MemoryRepository) ListMigrationsByRepository(_ context.Context, sourceRepositoryID shared.ID, page shared.PageRequest) (shared.PageResult[RepositoryMigration], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.repositories[sourceRepositoryID]; !ok {
		return shared.PageResult[RepositoryMigration]{}, shared.NewError(shared.CodeNotFound, "source repository not found")
	}
	page = page.Normalize()
	items := make([]RepositoryMigration, 0, len(r.migrationsByRepo[sourceRepositoryID]))
	for _, id := range r.migrationsByRepo[sourceRepositoryID] {
		items = append(items, r.migrations[id])
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

func (r *MemoryRepository) CreatePermissionSyncJob(_ context.Context, job RepositoryPermissionSyncJob) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.repositories[job.SourceRepositoryID]; !ok {
		return shared.NewError(shared.CodeNotFound, "source repository not found")
	}
	if _, exists := r.permissionJobs[job.ID]; exists {
		return shared.NewError(shared.CodeConflict, "repository permission sync job already exists")
	}
	r.permissionJobs[job.ID] = job
	return nil
}

func (r *MemoryRepository) UpdatePermissionSyncJob(_ context.Context, job RepositoryPermissionSyncJob) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.permissionJobs[job.ID]; !ok {
		return shared.NewError(shared.CodeNotFound, "repository permission sync job not found")
	}
	r.permissionJobs[job.ID] = job
	return nil
}

func (r *MemoryRepository) GetPermissionSyncJob(_ context.Context, id shared.ID) (RepositoryPermissionSyncJob, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	job, ok := r.permissionJobs[id]
	if !ok {
		return RepositoryPermissionSyncJob{}, shared.NewError(shared.CodeNotFound, "repository permission sync job not found")
	}
	return job, nil
}
