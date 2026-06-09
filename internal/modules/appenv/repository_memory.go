package appenv

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/shareinto/paas/internal/shared"
)

type MemoryRepository struct {
	mu                    sync.RWMutex
	applications          map[shared.ID]Application
	applicationNameIndex  map[applicationNameKey]shared.ID
	sourcesByApplication  map[shared.ID]map[string]ApplicationSource
	environments          map[shared.ID]Environment
	environmentsByApp     map[shared.ID][]shared.ID
	configs               map[shared.ID]EnvironmentConfig
	secrets               map[shared.ID]EnvironmentSecret
	routes                map[shared.ID]EnvironmentRoute
	bindingsByEnvironment map[shared.ID]EnvironmentClusterBinding
	statesByEnvironment   map[shared.ID]EnvironmentState
	eventsByEnvironment   map[shared.ID][]EnvironmentEvent
}

func orderedApplicationSources(sources map[string]ApplicationSource) []ApplicationSource {
	items := make([]ApplicationSource, 0, len(sources))
	for _, source := range sources {
		items = append(items, source)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsPrimary != items[j].IsPrimary {
			return items[i].IsPrimary
		}
		return items[i].Key < items[j].Key
	})
	return items
}

type applicationNameKey struct {
	projectID shared.ID
	name      string
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		applications:          map[shared.ID]Application{},
		applicationNameIndex:  map[applicationNameKey]shared.ID{},
		sourcesByApplication:  map[shared.ID]map[string]ApplicationSource{},
		environments:          map[shared.ID]Environment{},
		environmentsByApp:     map[shared.ID][]shared.ID{},
		configs:               map[shared.ID]EnvironmentConfig{},
		secrets:               map[shared.ID]EnvironmentSecret{},
		routes:                map[shared.ID]EnvironmentRoute{},
		bindingsByEnvironment: map[shared.ID]EnvironmentClusterBinding{},
		statesByEnvironment:   map[shared.ID]EnvironmentState{},
		eventsByEnvironment:   map[shared.ID][]EnvironmentEvent{},
	}
}

func (r *MemoryRepository) CreateApplication(_ context.Context, application Application) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.applications[application.ID]; exists {
		return shared.NewError(shared.CodeConflict, "application already exists")
	}
	key := applicationNameKey{projectID: application.ProjectID, name: application.Name}
	if _, exists := r.applicationNameIndex[key]; exists {
		return shared.NewError(shared.CodeConflict, "application name already exists in project")
	}
	r.applications[application.ID] = application
	r.applicationNameIndex[key] = application.ID
	return nil
}

func (r *MemoryRepository) UpdateApplication(_ context.Context, application Application) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	previous, ok := r.applications[application.ID]
	if !ok {
		return shared.NewError(shared.CodeNotFound, "application not found")
	}
	if previous.TenantID != application.TenantID || previous.ProjectID != application.ProjectID {
		return shared.NewError(shared.CodeInvalidArgument, "application ownership cannot be changed")
	}
	previousKey := applicationNameKey{projectID: previous.ProjectID, name: previous.Name}
	nextKey := applicationNameKey{projectID: application.ProjectID, name: application.Name}
	if previousKey != nextKey {
		if _, exists := r.applicationNameIndex[nextKey]; exists {
			return shared.NewError(shared.CodeConflict, "application name already exists in project")
		}
		delete(r.applicationNameIndex, previousKey)
		r.applicationNameIndex[nextKey] = application.ID
	}
	r.applications[application.ID] = application
	return nil
}

func (r *MemoryRepository) DeleteApplicationData(_ context.Context, applicationID shared.ID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	application, ok := r.applications[applicationID]
	if !ok {
		return shared.NewError(shared.CodeNotFound, "application not found")
	}
	delete(r.applicationNameIndex, applicationNameKey{projectID: application.ProjectID, name: application.Name})
	delete(r.sourcesByApplication, applicationID)
	for _, environmentID := range r.environmentsByApp[applicationID] {
		delete(r.environments, environmentID)
		delete(r.bindingsByEnvironment, environmentID)
		delete(r.statesByEnvironment, environmentID)
		delete(r.eventsByEnvironment, environmentID)
		for id, config := range r.configs {
			if config.EnvironmentID == environmentID {
				delete(r.configs, id)
			}
		}
		for id, secret := range r.secrets {
			if secret.EnvironmentID == environmentID {
				delete(r.secrets, id)
			}
		}
		for id, route := range r.routes {
			if route.EnvironmentID == environmentID {
				delete(r.routes, id)
			}
		}
	}
	delete(r.environmentsByApp, applicationID)
	delete(r.applications, applicationID)
	return nil
}

func (r *MemoryRepository) GetApplication(_ context.Context, id shared.ID) (Application, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	application, ok := r.applications[id]
	if !ok {
		return Application{}, shared.NewError(shared.CodeNotFound, "application not found")
	}
	return application, nil
}

func (r *MemoryRepository) FindApplicationByProjectAndName(_ context.Context, projectID shared.ID, name string) (Application, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.applicationNameIndex[applicationNameKey{projectID: projectID, name: name}]
	if !ok {
		return Application{}, shared.NewError(shared.CodeNotFound, "application not found")
	}
	return r.applications[id], nil
}

func (r *MemoryRepository) ListApplicationsByProject(_ context.Context, projectID shared.ID, page shared.PageRequest) (shared.PageResult[Application], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	page = page.Normalize()
	items := make([]Application, 0)
	for _, application := range r.applications {
		if application.ProjectID == projectID {
			items = append(items, application)
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

func (r *MemoryRepository) ListApplicationsByRuntimeEnvironment(_ context.Context, runtimeEnvironmentID shared.ID, page shared.PageRequest) (shared.PageResult[Application], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	page = page.Normalize()
	items := make([]Application, 0)
	for _, application := range r.applications {
		for _, environment := range application.RuntimeEnvironments {
			if environment.ID == runtimeEnvironmentID {
				items = append(items, application)
				break
			}
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

func (r *MemoryRepository) CreateApplicationSource(_ context.Context, source ApplicationSource) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.applications[source.ApplicationID]; !ok {
		return shared.NewError(shared.CodeNotFound, "application not found")
	}
	key := strings.TrimSpace(source.Key)
	if key == "" {
		key = "main"
		source.Key = key
	}
	if r.sourcesByApplication[source.ApplicationID] == nil {
		r.sourcesByApplication[source.ApplicationID] = map[string]ApplicationSource{}
	}
	if _, exists := r.sourcesByApplication[source.ApplicationID][key]; exists {
		return shared.NewError(shared.CodeConflict, "application source already exists")
	}
	r.sourcesByApplication[source.ApplicationID][key] = source
	return nil
}

func (r *MemoryRepository) UpdateApplicationSource(_ context.Context, source ApplicationSource) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := strings.TrimSpace(source.Key)
	if key == "" {
		key = "main"
		source.Key = key
	}
	if _, ok := r.sourcesByApplication[source.ApplicationID][key]; !ok {
		return shared.NewError(shared.CodeNotFound, "application source not found")
	}
	r.sourcesByApplication[source.ApplicationID][key] = source
	return nil
}

func (r *MemoryRepository) ReplaceApplicationSources(_ context.Context, applicationID shared.ID, sources []ApplicationSource) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.applications[applicationID]; !ok {
		return shared.NewError(shared.CodeNotFound, "application not found")
	}
	next := map[string]ApplicationSource{}
	for _, source := range sources {
		if source.ApplicationID != applicationID {
			return shared.NewError(shared.CodeInvalidArgument, "application source ownership cannot be changed")
		}
		key := strings.TrimSpace(source.Key)
		if key == "" {
			key = "main"
			source.Key = key
		}
		if _, exists := next[key]; exists {
			return shared.NewError(shared.CodeConflict, "application source already exists")
		}
		next[key] = source
	}
	if len(next) == 0 {
		return shared.NewError(shared.CodeInvalidArgument, "sources is required")
	}
	r.sourcesByApplication[applicationID] = next
	return nil
}

func (r *MemoryRepository) GetApplicationSource(_ context.Context, applicationID shared.ID) (ApplicationSource, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sources, ok := r.sourcesByApplication[applicationID]
	if !ok || len(sources) == 0 {
		return ApplicationSource{}, shared.NewError(shared.CodeNotFound, "application source not found")
	}
	items := orderedApplicationSources(sources)
	return items[0], nil
}

func (r *MemoryRepository) ListApplicationSources(_ context.Context, applicationID shared.ID) ([]ApplicationSource, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.applications[applicationID]; !ok {
		return nil, shared.NewError(shared.CodeNotFound, "application not found")
	}
	sources := r.sourcesByApplication[applicationID]
	if len(sources) == 0 {
		return nil, shared.NewError(shared.CodeNotFound, "application source not found")
	}
	return orderedApplicationSources(sources), nil
}

func (r *MemoryRepository) CreateEnvironment(_ context.Context, environment Environment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.applications[environment.ApplicationID]; !ok {
		return shared.NewError(shared.CodeNotFound, "application not found")
	}
	if _, exists := r.environments[environment.ID]; exists {
		return shared.NewError(shared.CodeConflict, "environment already exists")
	}
	for _, id := range r.environmentsByApp[environment.ApplicationID] {
		if r.environments[id].Name == environment.Name {
			return shared.NewError(shared.CodeConflict, "environment name already exists in application")
		}
	}
	r.environments[environment.ID] = environment
	r.environmentsByApp[environment.ApplicationID] = append(r.environmentsByApp[environment.ApplicationID], environment.ID)
	return nil
}

func (r *MemoryRepository) UpdateEnvironment(_ context.Context, environment Environment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.environments[environment.ID]; !ok {
		return shared.NewError(shared.CodeNotFound, "environment not found")
	}
	r.environments[environment.ID] = environment
	return nil
}

func (r *MemoryRepository) GetEnvironment(_ context.Context, id shared.ID) (Environment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	environment, ok := r.environments[id]
	if !ok {
		return Environment{}, shared.NewError(shared.CodeNotFound, "environment not found")
	}
	return environment, nil
}

func (r *MemoryRepository) ListEnvironmentsByApplication(_ context.Context, applicationID shared.ID) ([]Environment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.applications[applicationID]; !ok {
		return nil, shared.NewError(shared.CodeNotFound, "application not found")
	}
	items := make([]Environment, 0, len(r.environmentsByApp[applicationID]))
	for _, id := range r.environmentsByApp[applicationID] {
		items = append(items, r.environments[id])
	}
	sort.Slice(items, func(i, j int) bool { return environmentOrder(items[i].Name) < environmentOrder(items[j].Name) })
	return items, nil
}

func (r *MemoryRepository) CreateEnvironmentConfig(_ context.Context, config EnvironmentConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.environments[config.EnvironmentID]; !ok {
		return shared.NewError(shared.CodeNotFound, "environment not found")
	}
	r.configs[config.ID] = config
	return nil
}

func (r *MemoryRepository) CreateEnvironmentSecret(_ context.Context, secret EnvironmentSecret) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.environments[secret.EnvironmentID]; !ok {
		return shared.NewError(shared.CodeNotFound, "environment not found")
	}
	r.secrets[secret.ID] = secret
	return nil
}

func (r *MemoryRepository) CreateEnvironmentRoute(_ context.Context, route EnvironmentRoute) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.environments[route.EnvironmentID]; !ok {
		return shared.NewError(shared.CodeNotFound, "environment not found")
	}
	r.routes[route.ID] = route
	return nil
}

func (r *MemoryRepository) CreateEnvironmentClusterBinding(_ context.Context, binding EnvironmentClusterBinding) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.environments[binding.EnvironmentID]; !ok {
		return shared.NewError(shared.CodeNotFound, "environment not found")
	}
	if _, exists := r.bindingsByEnvironment[binding.EnvironmentID]; exists {
		return shared.NewError(shared.CodeConflict, "environment cluster binding already exists")
	}
	r.bindingsByEnvironment[binding.EnvironmentID] = binding
	return nil
}

func (r *MemoryRepository) GetEnvironmentClusterBinding(_ context.Context, environmentID shared.ID) (EnvironmentClusterBinding, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	binding, ok := r.bindingsByEnvironment[environmentID]
	if !ok {
		return EnvironmentClusterBinding{}, shared.NewError(shared.CodeNotFound, "environment cluster binding not found")
	}
	return binding, nil
}

func (r *MemoryRepository) ListEnvironmentClusterBindingsByApplication(_ context.Context, applicationID shared.ID) ([]EnvironmentClusterBinding, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.applications[applicationID]; !ok {
		return nil, shared.NewError(shared.CodeNotFound, "application not found")
	}
	items := make([]EnvironmentClusterBinding, 0)
	for _, environmentID := range r.environmentsByApp[applicationID] {
		if binding, ok := r.bindingsByEnvironment[environmentID]; ok {
			items = append(items, binding)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return environmentOrder(r.environments[items[i].EnvironmentID].Name) < environmentOrder(r.environments[items[j].EnvironmentID].Name)
	})
	return items, nil
}

func (r *MemoryRepository) SaveEnvironmentState(_ context.Context, state EnvironmentState) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.environments[state.EnvironmentID]; !ok {
		return shared.NewError(shared.CodeNotFound, "environment not found")
	}
	r.statesByEnvironment[state.EnvironmentID] = state
	return nil
}

func (r *MemoryRepository) GetEnvironmentState(_ context.Context, environmentID shared.ID) (EnvironmentState, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	state, ok := r.statesByEnvironment[environmentID]
	if !ok {
		return EnvironmentState{}, shared.NewError(shared.CodeNotFound, "environment state not found")
	}
	return state, nil
}

func (r *MemoryRepository) ListEnvironmentStatesByApplication(_ context.Context, applicationID shared.ID) ([]EnvironmentState, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.applications[applicationID]; !ok {
		return nil, shared.NewError(shared.CodeNotFound, "application not found")
	}
	items := make([]EnvironmentState, 0, len(r.environmentsByApp[applicationID]))
	for _, environmentID := range r.environmentsByApp[applicationID] {
		if state, ok := r.statesByEnvironment[environmentID]; ok {
			items = append(items, state)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		return environmentOrder(r.environments[items[i].EnvironmentID].Name) < environmentOrder(r.environments[items[j].EnvironmentID].Name)
	})
	return items, nil
}

func (r *MemoryRepository) AppendEnvironmentEvent(_ context.Context, event EnvironmentEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.environments[event.EnvironmentID]; !ok {
		return shared.NewError(shared.CodeNotFound, "environment not found")
	}
	r.eventsByEnvironment[event.EnvironmentID] = append(r.eventsByEnvironment[event.EnvironmentID], event)
	return nil
}

func (r *MemoryRepository) ListEnvironmentEvents(_ context.Context, environmentID shared.ID, page shared.PageRequest) (shared.PageResult[EnvironmentEvent], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.environments[environmentID]; !ok {
		return shared.PageResult[EnvironmentEvent]{}, shared.NewError(shared.CodeNotFound, "environment not found")
	}
	page = page.Normalize()
	items := append([]EnvironmentEvent(nil), r.eventsByEnvironment[environmentID]...)
	sort.Slice(items, func(i, j int) bool { return items[i].OccurredAt.Before(items[j].OccurredAt) })
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

func environmentOrder(name string) int {
	for i, candidate := range defaultEnvironmentNames {
		if name == candidate {
			return i
		}
	}
	return len(defaultEnvironmentNames)
}
