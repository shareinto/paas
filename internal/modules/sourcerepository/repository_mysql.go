package sourcerepository

import (
	"context"
	"database/sql"

	"github.com/shareinto/paas/internal/platform/database"
	"github.com/shareinto/paas/internal/shared"
)

type MySQLRepository struct {
	*MemoryRepository
	store *database.SnapshotStore
}

type sourceRepositorySnapshot struct {
	Repositories   []SourceRepository
	Applications   map[shared.ID][]AssociatedApplication
	Migrations     []RepositoryMigration
	PermissionJobs []RepositoryPermissionSyncJob
}

func NewMySQLRepository(ctx context.Context, db *sql.DB) (*MySQLRepository, error) {
	repo := &MySQLRepository{MemoryRepository: NewMemoryRepository(), store: database.NewSnapshotStore(db, "source-repository")}
	var snapshot sourceRepositorySnapshot
	if err := repo.store.Load(ctx, &snapshot); err != nil {
		return nil, err
	}
	repo.restore(snapshot)
	return repo, nil
}

func (r *MySQLRepository) restore(snapshot sourceRepositorySnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, repository := range snapshot.Repositories {
		r.repositories[repository.ID] = repository
		r.repositoryNameIndex[repositoryNameKey{projectID: repository.ProjectID, name: repository.Name}] = repository.ID
	}
	if snapshot.Applications != nil {
		r.applications = snapshot.Applications
	}
	for _, migration := range snapshot.Migrations {
		r.migrations[migration.ID] = migration
		r.migrationsByRepo[migration.SourceRepositoryID] = append(r.migrationsByRepo[migration.SourceRepositoryID], migration.ID)
	}
	for _, job := range snapshot.PermissionJobs {
		r.permissionJobs[job.ID] = job
	}
}

func (r *MySQLRepository) snapshot() sourceRepositorySnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := sourceRepositorySnapshot{
		Repositories:   make([]SourceRepository, 0, len(r.repositories)),
		Applications:   map[shared.ID][]AssociatedApplication{},
		Migrations:     make([]RepositoryMigration, 0, len(r.migrations)),
		PermissionJobs: make([]RepositoryPermissionSyncJob, 0, len(r.permissionJobs)),
	}
	for _, repository := range r.repositories {
		out.Repositories = append(out.Repositories, repository)
	}
	for id, applications := range r.applications {
		out.Applications[id] = append([]AssociatedApplication(nil), applications...)
	}
	for _, migration := range r.migrations {
		out.Migrations = append(out.Migrations, migration)
	}
	for _, job := range r.permissionJobs {
		out.PermissionJobs = append(out.PermissionJobs, job)
	}
	return out
}

func (r *MySQLRepository) persist(ctx context.Context) error { return r.store.Save(ctx, r.snapshot()) }

func (r *MySQLRepository) CreateSourceRepository(ctx context.Context, repository SourceRepository) error {
	if err := r.MemoryRepository.CreateSourceRepository(ctx, repository); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) UpdateSourceRepository(ctx context.Context, repository SourceRepository) error {
	if err := r.MemoryRepository.UpdateSourceRepository(ctx, repository); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) DeleteSourceRepository(ctx context.Context, id shared.ID) error {
	if err := r.MemoryRepository.DeleteSourceRepository(ctx, id); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) SetAssociatedApplications(ctx context.Context, sourceRepositoryID shared.ID, applications []AssociatedApplication) error {
	if err := r.MemoryRepository.SetAssociatedApplications(ctx, sourceRepositoryID, applications); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreateMigration(ctx context.Context, migration RepositoryMigration) error {
	if err := r.MemoryRepository.CreateMigration(ctx, migration); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) UpdateMigration(ctx context.Context, migration RepositoryMigration) error {
	if err := r.MemoryRepository.UpdateMigration(ctx, migration); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreatePermissionSyncJob(ctx context.Context, job RepositoryPermissionSyncJob) error {
	if err := r.MemoryRepository.CreatePermissionSyncJob(ctx, job); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) UpdatePermissionSyncJob(ctx context.Context, job RepositoryPermissionSyncJob) error {
	if err := r.MemoryRepository.UpdatePermissionSyncJob(ctx, job); err != nil {
		return err
	}
	return r.persist(ctx)
}
