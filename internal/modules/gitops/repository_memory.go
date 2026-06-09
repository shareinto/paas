package gitops

import (
	"context"
	"sort"
	"sync"

	"github.com/shareinto/paas/internal/shared"
)

type MemoryRepository struct {
	mu                     sync.RWMutex
	templates              map[shared.ID]DeploymentTemplate
	platformTemplateByName map[string]shared.ID
	appTemplateByApp       map[shared.ID]shared.ID
	revisions              map[shared.ID]DeploymentTemplateRevision
	revisionsByTemplate    map[shared.ID][]shared.ID
	manifestRevisions      map[shared.ID]ManifestRevision
	deployments            map[shared.ID]Deployment
	deploymentByPromotion  map[shared.ID]shared.ID
	deploymentsByApp       map[shared.ID][]shared.ID
	events                 map[shared.ID]DeploymentEvent
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		templates: map[shared.ID]DeploymentTemplate{}, platformTemplateByName: map[string]shared.ID{}, appTemplateByApp: map[shared.ID]shared.ID{},
		revisions: map[shared.ID]DeploymentTemplateRevision{}, revisionsByTemplate: map[shared.ID][]shared.ID{},
		manifestRevisions: map[shared.ID]ManifestRevision{}, deployments: map[shared.ID]Deployment{}, deploymentByPromotion: map[shared.ID]shared.ID{},
		deploymentsByApp: map[shared.ID][]shared.ID{}, events: map[shared.ID]DeploymentEvent{},
	}
}

func (r *MemoryRepository) CreateTemplate(_ context.Context, template DeploymentTemplate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.templates[template.ID]; ok {
		return shared.NewError(shared.CodeConflict, "deployment template already exists")
	}
	if template.Scope == TemplateScopeApplication {
		if _, ok := r.appTemplateByApp[template.ApplicationID]; ok {
			return shared.NewError(shared.CodeConflict, "application template already exists")
		}
		r.appTemplateByApp[template.ApplicationID] = template.ID
	}
	if template.Scope == TemplateScopePlatform {
		r.platformTemplateByName[template.Name] = template.ID
	}
	r.templates[template.ID] = template
	return nil
}

func (r *MemoryRepository) UpdateTemplate(_ context.Context, template DeploymentTemplate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.templates[template.ID]; !ok {
		return shared.NewError(shared.CodeNotFound, "deployment template not found")
	}
	r.templates[template.ID] = template
	return nil
}

func (r *MemoryRepository) GetTemplate(_ context.Context, id shared.ID) (DeploymentTemplate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	template, ok := r.templates[id]
	if !ok {
		return DeploymentTemplate{}, shared.NewError(shared.CodeNotFound, "deployment template not found")
	}
	return template, nil
}

func (r *MemoryRepository) FindPlatformTemplate(_ context.Context, name string) (DeploymentTemplate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.platformTemplateByName[name]
	if !ok {
		return DeploymentTemplate{}, shared.NewError(shared.CodeNotFound, "platform template not found")
	}
	return r.templates[id], nil
}

func (r *MemoryRepository) FindApplicationTemplate(_ context.Context, applicationID shared.ID) (DeploymentTemplate, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.appTemplateByApp[applicationID]
	if !ok {
		return DeploymentTemplate{}, shared.NewError(shared.CodeNotFound, "application template not found")
	}
	return r.templates[id], nil
}

func (r *MemoryRepository) CreateTemplateRevision(_ context.Context, revision DeploymentTemplateRevision) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.revisions[revision.ID]; ok {
		return shared.NewError(shared.CodeConflict, "deployment template revision already exists")
	}
	r.revisions[revision.ID] = revision
	r.revisionsByTemplate[revision.TemplateID] = append(r.revisionsByTemplate[revision.TemplateID], revision.ID)
	return nil
}

func (r *MemoryRepository) GetCurrentTemplateRevision(_ context.Context, templateID shared.ID) (DeploymentTemplateRevision, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := r.revisionsByTemplate[templateID]
	if len(ids) == 0 {
		return DeploymentTemplateRevision{}, shared.NewError(shared.CodeNotFound, "deployment template revision not found")
	}
	return r.revisions[ids[len(ids)-1]], nil
}

func (r *MemoryRepository) CreateManifestRevision(_ context.Context, revision ManifestRevision) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.manifestRevisions[revision.ID] = revision
	return nil
}

func (r *MemoryRepository) GetManifestRevision(_ context.Context, id shared.ID) (ManifestRevision, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	revision, ok := r.manifestRevisions[id]
	if !ok {
		return ManifestRevision{}, shared.NewError(shared.CodeNotFound, "manifest revision not found")
	}
	return revision, nil
}

func (r *MemoryRepository) CreateDeployment(_ context.Context, deployment Deployment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.deployments[deployment.ID]; ok {
		return shared.NewError(shared.CodeConflict, "deployment already exists")
	}
	r.deployments[deployment.ID] = deployment
	r.deploymentByPromotion[deployment.PromotionID] = deployment.ID
	r.deploymentsByApp[deployment.ApplicationID] = append(r.deploymentsByApp[deployment.ApplicationID], deployment.ID)
	return nil
}

func (r *MemoryRepository) UpdateDeployment(_ context.Context, deployment Deployment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.deployments[deployment.ID]; !ok {
		return shared.NewError(shared.CodeNotFound, "deployment not found")
	}
	r.deployments[deployment.ID] = deployment
	return nil
}

func (r *MemoryRepository) GetDeployment(_ context.Context, id shared.ID) (Deployment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	deployment, ok := r.deployments[id]
	if !ok {
		return Deployment{}, shared.NewError(shared.CodeNotFound, "deployment not found")
	}
	return deployment, nil
}

func (r *MemoryRepository) FindDeploymentByPromotion(_ context.Context, promotionID shared.ID) (Deployment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.deploymentByPromotion[promotionID]
	if !ok {
		return Deployment{}, shared.NewError(shared.CodeNotFound, "deployment not found")
	}
	return r.deployments[id], nil
}

func (r *MemoryRepository) ListDeployments(_ context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[Deployment], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	page = page.Normalize()
	items := make([]Deployment, 0, len(r.deploymentsByApp[applicationID]))
	for _, id := range r.deploymentsByApp[applicationID] {
		items = append(items, r.deployments[id])
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
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

func (r *MemoryRepository) CreateDeploymentEvent(_ context.Context, event DeploymentEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events[event.ID] = event
	return nil
}
