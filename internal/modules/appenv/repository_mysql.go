package appenv

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

type appenvSnapshot struct {
	Applications []Application
	Sources      []ApplicationSource
	Environments []Environment
	Configs      []EnvironmentConfig
	Secrets      []EnvironmentSecret
	Routes       []EnvironmentRoute
	Bindings     []EnvironmentClusterBinding
	States       []EnvironmentState
	Events       map[shared.ID][]EnvironmentEvent
}

func NewMySQLRepository(ctx context.Context, db *sql.DB) (*MySQLRepository, error) {
	repo := &MySQLRepository{MemoryRepository: NewMemoryRepository(), store: database.NewSnapshotStore(db, "application-environment")}
	var snapshot appenvSnapshot
	if err := repo.store.Load(ctx, &snapshot); err != nil {
		return nil, err
	}
	repo.restore(snapshot)
	return repo, nil
}

func (r *MySQLRepository) restore(snapshot appenvSnapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, app := range snapshot.Applications {
		r.applications[app.ID] = app
		r.applicationNameIndex[applicationNameKey{projectID: app.ProjectID, name: app.Name}] = app.ID
	}
	for _, source := range snapshot.Sources {
		key := source.Key
		if key == "" {
			key = "main"
			source.Key = key
		}
		if r.sourcesByApplication[source.ApplicationID] == nil {
			r.sourcesByApplication[source.ApplicationID] = map[string]ApplicationSource{}
		}
		r.sourcesByApplication[source.ApplicationID][key] = source
	}
	for _, env := range snapshot.Environments {
		r.environments[env.ID] = env
		r.environmentsByApp[env.ApplicationID] = append(r.environmentsByApp[env.ApplicationID], env.ID)
	}
	for _, config := range snapshot.Configs {
		r.configs[config.ID] = config
	}
	for _, secret := range snapshot.Secrets {
		r.secrets[secret.ID] = secret
	}
	for _, route := range snapshot.Routes {
		r.routes[route.ID] = route
	}
	for _, binding := range snapshot.Bindings {
		r.bindingsByEnvironment[binding.EnvironmentID] = binding
	}
	for _, state := range snapshot.States {
		r.statesByEnvironment[state.EnvironmentID] = state
	}
	if snapshot.Events != nil {
		r.eventsByEnvironment = snapshot.Events
	}
}

func (r *MySQLRepository) snapshot() appenvSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := appenvSnapshot{
		Applications: make([]Application, 0, len(r.applications)),
		Sources:      []ApplicationSource{},
		Environments: make([]Environment, 0, len(r.environments)),
		Configs:      make([]EnvironmentConfig, 0, len(r.configs)),
		Secrets:      make([]EnvironmentSecret, 0, len(r.secrets)),
		Routes:       make([]EnvironmentRoute, 0, len(r.routes)),
		Bindings:     make([]EnvironmentClusterBinding, 0, len(r.bindingsByEnvironment)),
		States:       make([]EnvironmentState, 0, len(r.statesByEnvironment)),
		Events:       map[shared.ID][]EnvironmentEvent{},
	}
	for _, v := range r.applications {
		out.Applications = append(out.Applications, v)
	}
	for _, sources := range r.sourcesByApplication {
		for _, v := range sources {
			out.Sources = append(out.Sources, v)
		}
	}
	for _, v := range r.environments {
		out.Environments = append(out.Environments, v)
	}
	for _, v := range r.configs {
		out.Configs = append(out.Configs, v)
	}
	for _, v := range r.secrets {
		out.Secrets = append(out.Secrets, v)
	}
	for _, v := range r.routes {
		out.Routes = append(out.Routes, v)
	}
	for _, v := range r.bindingsByEnvironment {
		out.Bindings = append(out.Bindings, v)
	}
	for _, v := range r.statesByEnvironment {
		out.States = append(out.States, v)
	}
	for id, events := range r.eventsByEnvironment {
		out.Events[id] = append([]EnvironmentEvent(nil), events...)
	}
	return out
}

func (r *MySQLRepository) persist(ctx context.Context) error { return r.store.Save(ctx, r.snapshot()) }

func (r *MySQLRepository) CreateApplication(ctx context.Context, application Application) error {
	if err := r.MemoryRepository.CreateApplication(ctx, application); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) UpdateApplication(ctx context.Context, application Application) error {
	if err := r.MemoryRepository.UpdateApplication(ctx, application); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) DeleteApplicationData(ctx context.Context, applicationID shared.ID) error {
	if err := r.MemoryRepository.DeleteApplicationData(ctx, applicationID); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreateApplicationSource(ctx context.Context, source ApplicationSource) error {
	if err := r.MemoryRepository.CreateApplicationSource(ctx, source); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) UpdateApplicationSource(ctx context.Context, source ApplicationSource) error {
	if err := r.MemoryRepository.UpdateApplicationSource(ctx, source); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) ReplaceApplicationSources(ctx context.Context, applicationID shared.ID, sources []ApplicationSource) error {
	if err := r.MemoryRepository.ReplaceApplicationSources(ctx, applicationID, sources); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreateEnvironment(ctx context.Context, environment Environment) error {
	if err := r.MemoryRepository.CreateEnvironment(ctx, environment); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) UpdateEnvironment(ctx context.Context, environment Environment) error {
	if err := r.MemoryRepository.UpdateEnvironment(ctx, environment); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreateEnvironmentConfig(ctx context.Context, config EnvironmentConfig) error {
	if err := r.MemoryRepository.CreateEnvironmentConfig(ctx, config); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreateEnvironmentSecret(ctx context.Context, secret EnvironmentSecret) error {
	if err := r.MemoryRepository.CreateEnvironmentSecret(ctx, secret); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreateEnvironmentRoute(ctx context.Context, route EnvironmentRoute) error {
	if err := r.MemoryRepository.CreateEnvironmentRoute(ctx, route); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) CreateEnvironmentClusterBinding(ctx context.Context, binding EnvironmentClusterBinding) error {
	if err := r.MemoryRepository.CreateEnvironmentClusterBinding(ctx, binding); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) SaveEnvironmentState(ctx context.Context, state EnvironmentState) error {
	if err := r.MemoryRepository.SaveEnvironmentState(ctx, state); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) AppendEnvironmentEvent(ctx context.Context, event EnvironmentEvent) error {
	if err := r.MemoryRepository.AppendEnvironmentEvent(ctx, event); err != nil {
		return err
	}
	return r.persist(ctx)
}
