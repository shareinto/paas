package gitops

import (
	"context"
	"database/sql"

	"github.com/shareinto/paas/internal/platform/database"
)

type MySQLRepository struct {
	*MemoryRepository
	store *database.SnapshotStore
}

type gitopsSnapshot struct {
	Templates         []DeploymentTemplate
	Revisions         []DeploymentTemplateRevision
	ManifestRevisions []ManifestRevision
	Deployments       []Deployment
	Events            []DeploymentEvent
}

func NewMySQLRepository(ctx context.Context, db *sql.DB) (*MySQLRepository, error) {
	repo := &MySQLRepository{MemoryRepository: NewMemoryRepository(), store: database.NewSnapshotStore(db, "gitops-deployment")}
	var snapshot gitopsSnapshot
	if err := repo.store.Load(ctx, &snapshot); err != nil {
		return nil, err
	}
	for _, v := range snapshot.Templates {
		repo.templates[v.ID] = v
		if v.Scope == TemplateScopePlatform {
			repo.platformTemplateByName[v.Name] = v.ID
		}
		if v.Scope == TemplateScopeApplication {
			repo.appTemplateByApp[v.ApplicationID] = v.ID
		}
	}
	for _, v := range snapshot.Revisions {
		repo.revisions[v.ID] = v
		repo.revisionsByTemplate[v.TemplateID] = append(repo.revisionsByTemplate[v.TemplateID], v.ID)
	}
	for _, v := range snapshot.ManifestRevisions {
		repo.manifestRevisions[v.ID] = v
	}
	for _, v := range snapshot.Deployments {
		repo.deployments[v.ID] = v
		repo.deploymentByPromotion[v.PromotionID] = v.ID
		repo.deploymentsByApp[v.ApplicationID] = append(repo.deploymentsByApp[v.ApplicationID], v.ID)
	}
	for _, v := range snapshot.Events {
		repo.events[v.ID] = v
	}
	return repo, nil
}

func (r *MySQLRepository) snapshot() gitopsSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := gitopsSnapshot{
		Templates: make([]DeploymentTemplate, 0, len(r.templates)), Revisions: make([]DeploymentTemplateRevision, 0, len(r.revisions)),
		ManifestRevisions: make([]ManifestRevision, 0, len(r.manifestRevisions)), Deployments: make([]Deployment, 0, len(r.deployments)),
		Events: make([]DeploymentEvent, 0, len(r.events)),
	}
	for _, v := range r.templates {
		out.Templates = append(out.Templates, v)
	}
	for _, v := range r.revisions {
		out.Revisions = append(out.Revisions, v)
	}
	for _, v := range r.manifestRevisions {
		out.ManifestRevisions = append(out.ManifestRevisions, v)
	}
	for _, v := range r.deployments {
		out.Deployments = append(out.Deployments, v)
	}
	for _, v := range r.events {
		out.Events = append(out.Events, v)
	}
	return out
}

func (r *MySQLRepository) persist(ctx context.Context) error { return r.store.Save(ctx, r.snapshot()) }
func (r *MySQLRepository) CreateTemplate(ctx context.Context, template DeploymentTemplate) error {
	if err := r.MemoryRepository.CreateTemplate(ctx, template); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) UpdateTemplate(ctx context.Context, template DeploymentTemplate) error {
	if err := r.MemoryRepository.UpdateTemplate(ctx, template); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreateTemplateRevision(ctx context.Context, revision DeploymentTemplateRevision) error {
	if err := r.MemoryRepository.CreateTemplateRevision(ctx, revision); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreateManifestRevision(ctx context.Context, revision ManifestRevision) error {
	if err := r.MemoryRepository.CreateManifestRevision(ctx, revision); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreateDeployment(ctx context.Context, deployment Deployment) error {
	if err := r.MemoryRepository.CreateDeployment(ctx, deployment); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) UpdateDeployment(ctx context.Context, deployment Deployment) error {
	if err := r.MemoryRepository.UpdateDeployment(ctx, deployment); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreateDeploymentEvent(ctx context.Context, event DeploymentEvent) error {
	if err := r.MemoryRepository.CreateDeploymentEvent(ctx, event); err != nil {
		return err
	}
	return r.persist(ctx)
}
