package appenv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/modules/tenantproject"
	"github.com/shareinto/paas/internal/shared"
	"github.com/shareinto/paas/internal/shared/testutil"
	"github.com/shareinto/paas/internal/testsupport"
)

type fakeProjectQuery struct {
	projects map[shared.ID]tenantproject.Project
}

func (q fakeProjectQuery) GetProject(_ context.Context, id shared.ID) (tenantproject.Project, error) {
	project, ok := q.projects[id]
	if !ok {
		return tenantproject.Project{}, shared.NewError(shared.CodeNotFound, "project not found")
	}
	return project, nil
}

type fakeSourceRepositoryQuery struct {
	repositories map[shared.ID]SourceRepositoryRef
}

func (q fakeSourceRepositoryQuery) GetSourceRepository(_ context.Context, id shared.ID) (SourceRepositoryRef, error) {
	repository, ok := q.repositories[id]
	if !ok {
		return SourceRepositoryRef{}, shared.NewError(shared.CodeNotFound, "source repository not found")
	}
	return repository, nil
}

type fakeRuntimeEnvironmentQuery struct {
	environments map[shared.ID]RuntimeEnvironmentRef
	defaultID    shared.ID
}

func (q fakeRuntimeEnvironmentQuery) GetRuntimeEnvironment(_ context.Context, id shared.ID) (RuntimeEnvironmentRef, error) {
	environment, ok := q.environments[id]
	if !ok {
		return RuntimeEnvironmentRef{}, shared.NewError(shared.CodeNotFound, "runtime environment not found")
	}
	return environment, nil
}

func (q fakeRuntimeEnvironmentQuery) FindDefaultRuntimeEnvironment(ctx context.Context) (RuntimeEnvironmentRef, error) {
	return q.GetRuntimeEnvironment(ctx, q.defaultID)
}

type fakeBuildEnvironmentQuery struct {
	environments map[shared.ID]BuildEnvironmentRef
	defaultID    shared.ID
}

func (q fakeBuildEnvironmentQuery) GetBuildEnvironment(_ context.Context, id shared.ID) (BuildEnvironmentRef, error) {
	environment, ok := q.environments[id]
	if !ok {
		return BuildEnvironmentRef{}, shared.NewError(shared.CodeNotFound, "build environment not found")
	}
	return environment, nil
}

func (q fakeBuildEnvironmentQuery) FindDefaultBuildEnvironment(ctx context.Context) (BuildEnvironmentRef, error) {
	return q.GetBuildEnvironment(ctx, q.defaultID)
}

type recordingPermission struct {
	calls []permissionCall
	err   error
}

type permissionCall struct {
	resource identityaccess.ResourceScope
	action   identityaccess.Permission
}

func (p *recordingPermission) Check(_ context.Context, _ identityaccess.Subject, resource identityaccess.ResourceScope, action identityaccess.Permission) error {
	p.calls = append(p.calls, permissionCall{resource: resource, action: action})
	return p.err
}

type fakeClusterPlacement struct {
	candidate ClusterCandidate
	ok        bool
	err       error
	calls     []Environment
}

func (p *fakeClusterPlacement) SelectCluster(_ context.Context, environment Environment) (ClusterCandidate, bool, error) {
	p.calls = append(p.calls, environment)
	return p.candidate, p.ok, p.err
}

type recordingGitOps struct {
	specs []GitOpsEnvironmentSpec
	err   error
}

func (g *recordingGitOps) ProvisionEnvironment(_ context.Context, spec GitOpsEnvironmentSpec) error {
	g.specs = append(g.specs, spec)
	return g.err
}

type recordingAudit struct {
	events []AuditEvent
}

func (a *recordingAudit) Log(_ context.Context, event AuditEvent) error {
	a.events = append(a.events, event)
	return nil
}

type recordingPublisher struct {
	events []shared.DomainEvent
	err    error
}

func (p *recordingPublisher) Publish(_ context.Context, event shared.DomainEvent) error {
	p.events = append(p.events, event)
	return p.err
}

type recordingBuildPipelineProvisioner struct {
	applicationIDs []shared.ID
	deletedIDs     []shared.ID
	err            error
}

func (p *recordingBuildPipelineProvisioner) EnsureBuildPipeline(_ context.Context, applicationID shared.ID) error {
	p.applicationIDs = append(p.applicationIDs, applicationID)
	return p.err
}

func (p *recordingBuildPipelineProvisioner) DeleteBuildPipeline(_ context.Context, applicationID shared.ID) error {
	p.deletedIDs = append(p.deletedIDs, applicationID)
	return p.err
}

type appenvTestEnv struct {
	svc        *Service
	repo       Repository
	permission *recordingPermission
	clusters   *fakeClusterPlacement
	gitops     *recordingGitOps
	pipelines  *recordingBuildPipelineProvisioner
	audit      *recordingAudit
	events     *recordingPublisher
}

type failingIDGenerator struct{}

func (failingIDGenerator) NewID(string) (shared.ID, error) {
	return "", errors.New("id generation failed")
}

func newTestRepository(t *testing.T) Repository {
	t.Helper()
	repo, err := NewMySQLRepository(context.Background(), testsupport.MySQLDB(t, Migrations...))
	if err != nil {
		t.Fatalf("NewMySQLRepository() error = %v", err)
	}
	return repo
}

func newAppenvTestEnv(t *testing.T, clusterAvailable bool) appenvTestEnv {
	t.Helper()
	repo := newTestRepository(t)
	permission := &recordingPermission{}
	clusters := &fakeClusterPlacement{
		candidate: ClusterCandidate{ClusterID: "cluster_dev", ClusterName: "dev-cluster", Namespace: "payment"},
		ok:        clusterAvailable,
	}
	gitops := &recordingGitOps{}
	pipelines := &recordingBuildPipelineProvisioner{}
	audit := &recordingAudit{}
	events := &recordingPublisher{}
	svc := NewService(Options{
		Repository: repo,
		ProjectQuery: fakeProjectQuery{projects: map[shared.ID]tenantproject.Project{
			"project_payment": {ID: "project_payment", TenantID: "tenant_a", Name: "payment"},
		}},
		SourceRepositoryQuery: fakeSourceRepositoryQuery{repositories: map[shared.ID]SourceRepositoryRef{
			"repo_user":  {ID: "repo_user", TenantID: "tenant_a", ProjectID: "project_payment", DefaultBranch: "main", Status: "ready"},
			"repo_other": {ID: "repo_other", TenantID: "tenant_b", ProjectID: "project_other", DefaultBranch: "main", Status: "ready"},
			"repo_busy":  {ID: "repo_busy", TenantID: "tenant_a", ProjectID: "project_payment", DefaultBranch: "main", Status: "migrating"},
		}},
		ClusterPlacementQuery:        clusters,
		GitOpsEnvironmentProvisioner: gitops,
		BuildPipelineProvisioner:     pipelines,
		PermissionChecker:            permission,
		Audit:                        audit,
		EventPublisher:               events,
		IDGenerator:                  testutil.NewFakeIDGenerator(1),
		Clock:                        testutil.NewFakeClock(time.Date(2026, 5, 30, 5, 0, 0, 0, time.UTC)),
	})
	return appenvTestEnv{svc: svc, repo: repo, permission: permission, clusters: clusters, gitops: gitops, pipelines: pipelines, audit: audit, events: events}
}

func TestNoopAuditAndEventPublisher(t *testing.T) {
	if err := (NoopAuditLogger{}).Log(context.Background(), AuditEvent{Action: "application.create"}); err != nil {
		t.Fatalf("noop audit: %v", err)
	}
	if err := (NoopEventPublisher{}).Publish(context.Background(), shared.DomainEvent{}); err != nil {
		t.Fatalf("noop event publisher: %v", err)
	}
}

func appenvActor() identityaccess.Subject {
	return identityaccess.Subject{Type: identityaccess.SubjectUser, ID: "usr_actor"}
}

func validBuildSpec() BuildSpec {
	return BuildSpec{
		SourcePath:          "services/user-api",
		BuildCommand:        "mvn clean package -DskipTests",
		ArtifactCopyCommand: "cp -ar target/user-api.jar \"$PAAS_ARTIFACT_OUTPUT/app.jar\"",
		RuntimeBaseImage:    "registry.example/runtime/java17:1.0",
		ArtifactDeployPath:  "/app/",
	}
}

func validSourceInput(repoID shared.ID, spec BuildSpec) CreateApplicationSourceInput {
	return CreateApplicationSourceInput{Key: "main", SourceRepositoryID: repoID, BuildSpec: spec, IsPrimary: true}
}

func TestCreateApplicationPersistsSourceAndPendingDefaultEnvironmentsWhenNoCluster(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	ctx := context.Background()

	app, err := env.svc.CreateApplication(ctx, CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "User-API", DisplayName: "用户接口", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	if app.ID != "app_1" || app.Name != "user-api" || app.Status != ApplicationStatusActive {
		t.Fatalf("unexpected application: %+v", app)
	}
	source, err := env.svc.GetApplicationSource(ctx, app.ID)
	if err != nil {
		t.Fatalf("GetApplicationSource() error = %v", err)
	}
	if source.SourceRepositoryID != "repo_user" || source.SourcePath != "services/user-api" || source.BuildSpec.DefaultRef != "main" || source.BuildSpec.ArtifactDeployPath != "/app/" {
		t.Fatalf("build spec should be fixed on application source: %+v", source)
	}
	if len(env.pipelines.applicationIDs) != 0 {
		t.Fatalf("application creation must not provision Jenkins pipeline, got %+v", env.pipelines.applicationIDs)
	}
	environments, err := env.svc.ListEnvironments(ctx, app.ID)
	if err != nil {
		t.Fatalf("ListEnvironments() error = %v", err)
	}
	if got := environmentNames(environments); got != "dev,test,staging,prod" {
		t.Fatalf("unexpected default environments: %s", got)
	}
	states, err := env.svc.ListEnvironmentStates(ctx, app.ID)
	if err != nil {
		t.Fatalf("ListEnvironmentStates() error = %v", err)
	}
	if len(states) != 4 {
		t.Fatalf("expected states for all environments, got %+v", states)
	}
	for _, state := range states {
		if state.Status != EnvironmentStatusPendingClusterBinding {
			t.Fatalf("environment without cluster should be pending, got %+v", state)
		}
		if _, err := env.repo.GetEnvironmentClusterBinding(ctx, state.EnvironmentID); shared.CodeOf(err) != shared.CodeNotFound {
			t.Fatalf("pending environment should not have binding, err = %v", err)
		}
	}
	if len(env.gitops.specs) != 0 {
		t.Fatalf("gitops should not be called without cluster, got %+v", env.gitops.specs)
	}
	if len(env.permission.calls) != 1 || env.permission.calls[0].action != "application:create" || env.permission.calls[0].resource.ProjectID != "project_payment" {
		t.Fatalf("unexpected permission calls: %+v", env.permission.calls)
	}
	if len(env.events.events) != 1 || env.events.events[0].EventType != "ApplicationCreated" {
		t.Fatalf("expected ApplicationCreated event, got %+v", env.events.events)
	}
}

func TestCreateApplicationDoesNotRequireSourceConfiguration(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	ctx := context.Background()

	app, err := env.svc.CreateApplication(ctx, CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "User-API", DisplayName: "用户接口"})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	if app.ID != "app_1" || app.Name != "user-api" || app.Status != ApplicationStatusActive {
		t.Fatalf("unexpected application: %+v", app)
	}
	if _, err := env.svc.GetApplicationSource(ctx, app.ID); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("application source should not be created, got %v", err)
	}
	environments, err := env.svc.ListEnvironments(ctx, app.ID)
	if err != nil {
		t.Fatalf("ListEnvironments() error = %v", err)
	}
	if got := environmentNames(environments); got != "dev,test,staging,prod" {
		t.Fatalf("unexpected default environments: %s", got)
	}
	if len(env.pipelines.applicationIDs) != 0 {
		t.Fatalf("application creation must not provision Jenkins pipeline, got %+v", env.pipelines.applicationIDs)
	}
}

func TestCreateApplicationDoesNotPersistRuntimeEnvironments(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	env.svc.runtimeEnvironments = fakeRuntimeEnvironmentQuery{
		defaultID: "runtime_env_java17",
		environments: map[shared.ID]RuntimeEnvironmentRef{
			"runtime_env_java17": {ID: "runtime_env_java17", Name: "java17", Status: "enabled", RuntimeBaseImage: "registry.example/runtime/java17:1.0", ArtifactDeployPath: "/app/"},
			"runtime_env_java21": {ID: "runtime_env_java21", Name: "java21", Status: "enabled", RuntimeBaseImage: "registry.example/runtime/java21:1.0", ArtifactDeployPath: "/app/"},
		},
	}
	ctx := context.Background()

	app, err := env.svc.CreateApplication(ctx, CreateApplicationInput{
		Actor:                 appenvActor(),
		ProjectID:             "project_payment",
		Name:                  "multi-runtime-api",
		DisplayName:           "多运行时接口",
		RuntimeEnvironmentIDs: []shared.ID{"runtime_env_java17", "runtime_env_java21"},
		Sources:               []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())},
	})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	if !app.RuntimeEnvironmentID.IsZero() || len(app.RuntimeEnvironments) != 0 {
		t.Fatalf("runtime environments should not be persisted on application, got %+v", app)
	}
	source, err := env.svc.GetApplicationSource(ctx, app.ID)
	if err != nil {
		t.Fatalf("GetApplicationSource() error = %v", err)
	}
	if source.BuildSpec.RuntimeBaseImage != "registry.example/runtime/java17:1.0" {
		t.Fatalf("source compatibility BuildSpec should keep explicit runtime fields, got %+v", source.BuildSpec)
	}
}

func TestCreateApplicationDoesNotOverrideSourceRuntimeFromApplicationRuntime(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	env.svc.runtimeEnvironments = fakeRuntimeEnvironmentQuery{
		defaultID: "runtime_env_custom",
		environments: map[shared.ID]RuntimeEnvironmentRef{
			"runtime_env_custom": {ID: "runtime_env_custom", Name: "custom", Status: "enabled", RuntimeBaseImage: "registry.internal/runtime/custom:20260603", ArtifactDeployPath: "/app/"},
		},
	}
	ctx := context.Background()

	app, err := env.svc.CreateApplication(ctx, CreateApplicationInput{
		Actor:                appenvActor(),
		ProjectID:            "project_payment",
		Name:                 "custom-runtime-api",
		RuntimeEnvironmentID: "runtime_env_custom",
		RuntimeOverrides:     BuildSpec{RuntimeBaseImage: "registry.invalid/override:latest"},
		Sources:              []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())},
	})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	if !app.RuntimeEnvironmentID.IsZero() || len(app.RuntimeEnvironments) != 0 {
		t.Fatalf("runtime environment should not be persisted, got %+v", app)
	}
	source, err := env.svc.GetApplicationSource(ctx, app.ID)
	if err != nil {
		t.Fatalf("GetApplicationSource() error = %v", err)
	}
	if source.BuildSpec.RuntimeBaseImage != "registry.example/runtime/java17:1.0" {
		t.Fatalf("source runtime image should not come from application runtime field, got %+v", source.BuildSpec)
	}
}

func TestBuildEnvironmentSelectionDoesNotDefaultBuildSpecFields(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	env.svc.runtimeEnvironments = fakeRuntimeEnvironmentQuery{
		defaultID: "runtime_env_java17",
		environments: map[shared.ID]RuntimeEnvironmentRef{
			"runtime_env_java17": {ID: "runtime_env_java17", Name: "java17", Status: "enabled", RuntimeBaseImage: "registry.example/runtime/java17:1.0", ArtifactDeployPath: "/app/"},
		},
	}
	env.svc.buildEnvironments = fakeBuildEnvironmentQuery{
		environments: map[shared.ID]BuildEnvironmentRef{
			"build_env_java": {ID: "build_env_java", Status: "enabled"},
		},
	}
	ctx := context.Background()

	app, err := env.svc.CreateApplication(ctx, CreateApplicationInput{
		Actor:                appenvActor(),
		ProjectID:            "project_payment",
		Name:                 "defaulted-api",
		RuntimeEnvironmentID: "runtime_env_java17",
		Sources: []CreateApplicationSourceInput{{
			Key:                "main",
			SourceRepositoryID: "repo_user",
			BuildEnvironmentID: "build_env_java",
			BuildSpec: BuildSpec{
				SourcePath:          ".",
				BuildCommand:        "mvn test",
				ArtifactCopyCommand: "cp -ar target/custom.jar \"$PAAS_ARTIFACT_OUTPUT/app.jar\"",
				RuntimeBaseImage:    "registry.example/runtime/java17:1.0",
			},
			IsPrimary: true,
		}},
	})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	source, err := env.svc.GetApplicationSource(ctx, app.ID)
	if err != nil {
		t.Fatalf("GetApplicationSource() error = %v", err)
	}
	if source.BuildEnvironmentID != "build_env_java" || source.BuildSpec.BuildCommand != "mvn test" || !strings.Contains(source.BuildSpec.ArtifactCopyCommand, "target/custom.jar") {
		t.Fatalf("build environment should only be selected, got %+v", source)
	}
}

func TestSyncRuntimeEnvironmentSnapshotDoesNotUpdateNewApplications(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	env.svc.runtimeEnvironments = fakeRuntimeEnvironmentQuery{
		defaultID: "runtime_env_java17",
		environments: map[shared.ID]RuntimeEnvironmentRef{
			"runtime_env_java17": {ID: "runtime_env_java17", Name: "java17", Status: "enabled", RuntimeBaseImage: "registry.example/runtime/java17:1.0", ArtifactDeployPath: "/app/"},
			"runtime_env_java21": {ID: "runtime_env_java21", Name: "java21", Status: "enabled", RuntimeBaseImage: "registry.example/runtime/java21:1.0", ArtifactDeployPath: "/app/"},
		},
	}
	ctx := context.Background()
	primaryApp, err := env.svc.CreateApplication(ctx, CreateApplicationInput{
		Actor:                 appenvActor(),
		ProjectID:             "project_payment",
		Name:                  "primary-runtime-api",
		RuntimeEnvironmentIDs: []shared.ID{"runtime_env_java17", "runtime_env_java21"},
		Sources:               []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())},
	})
	if err != nil {
		t.Fatalf("CreateApplication(primary) error = %v", err)
	}
	secondaryApp, err := env.svc.CreateApplication(ctx, CreateApplicationInput{
		Actor:                 appenvActor(),
		ProjectID:             "project_payment",
		Name:                  "secondary-runtime-api",
		RuntimeEnvironmentIDs: []shared.ID{"runtime_env_java21", "runtime_env_java17"},
		Sources:               []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())},
	})
	if err != nil {
		t.Fatalf("CreateApplication(secondary) error = %v", err)
	}
	if len(primaryApp.RuntimeEnvironments) != 0 || len(secondaryApp.RuntimeEnvironments) != 0 {
		t.Fatalf("new applications should not persist runtime environments: %+v %+v", primaryApp, secondaryApp)
	}

	count, err := env.svc.SyncRuntimeEnvironmentSnapshot(ctx, RuntimeEnvironmentSnapshotInput{
		Actor: appenvActor(),
		Environment: RuntimeEnvironmentRef{
			ID:                 "runtime_env_java17",
			Name:               "java17",
			RuntimeBaseImage:   "registry.example/runtime/java17:2.0",
			ArtifactDeployPath: "/srv/",
			DockerfilePath:     "java/custom/Dockerfile",
		},
	})
	if err != nil {
		t.Fatalf("SyncRuntimeEnvironmentSnapshot() error = %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no synced applications, got %d", count)
	}
	primary, err := env.svc.GetApplication(ctx, primaryApp.ID)
	if err != nil {
		t.Fatalf("GetApplication(primary) error = %v", err)
	}
	if len(primary.RuntimeEnvironments) != 0 {
		t.Fatalf("primary app runtime snapshots should remain empty: %+v", primary.RuntimeEnvironments)
	}
	primarySource, err := env.svc.GetApplicationSource(ctx, primaryApp.ID)
	if err != nil {
		t.Fatalf("GetApplicationSource(primary) error = %v", err)
	}
	if primarySource.BuildSpec.RuntimeBaseImage != "registry.example/runtime/java17:1.0" || primarySource.BuildSpec.ArtifactDeployPath != "/app/" {
		t.Fatalf("source BuildSpec runtime fields should not be synced from app runtime, got %+v", primarySource.BuildSpec)
	}
	secondarySource, err := env.svc.GetApplicationSource(ctx, secondaryApp.ID)
	if err != nil {
		t.Fatalf("GetApplicationSource(secondary) error = %v", err)
	}
	if secondarySource.BuildSpec.RuntimeBaseImage == "registry.example/runtime/java17:2.0" {
		t.Fatalf("secondary runtime must not overwrite primary source BuildSpec, got %+v", secondarySource.BuildSpec)
	}
	for _, event := range env.audit.events {
		if event.Action == "application.runtime_environment.sync" {
			t.Fatalf("new applications should not receive runtime sync audit events, got %+v", env.audit.events)
		}
	}
}

func TestCreateApplicationBindsDefaultEnvironmentsWhenClusterAvailable(t *testing.T) {
	env := newAppenvTestEnv(t, true)
	ctx := context.Background()

	app, err := env.svc.CreateApplication(ctx, CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "worker", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", BuildSpec{
		SourcePath:          ".",
		BuildCommand:        "mvn clean package -DskipTests",
		ArtifactCopyCommand: "cp -ar target/worker.war \"$PAAS_ARTIFACT_OUTPUT/app.war\"",
		RuntimeBaseImage:    "registry.example/runtime/tomcat8:1.0",
		DefaultRef:          "release/1.0",
	})}})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	bindings, err := env.repo.ListEnvironmentClusterBindingsByApplication(ctx, app.ID)
	if err != nil {
		t.Fatalf("ListEnvironmentClusterBindingsByApplication() error = %v", err)
	}
	if len(bindings) != 4 {
		t.Fatalf("expected binding for all default environments, got %+v", bindings)
	}
	states, err := env.svc.ListEnvironmentStates(ctx, app.ID)
	if err != nil {
		t.Fatalf("ListEnvironmentStates() error = %v", err)
	}
	for _, state := range states {
		if state.Status != EnvironmentStatusClusterBound {
			t.Fatalf("bound environment should be cluster_bound, got %+v", state)
		}
	}
	if len(env.gitops.specs) != 4 {
		t.Fatalf("expected gitops provisioning for all environments, got %+v", env.gitops.specs)
	}
	first := env.gitops.specs[0]
	if first.ApplicationName != "worker" || first.SourceRepositoryID != "repo_user" || first.SourcePath != "." || first.ClusterID != "cluster_dev" {
		t.Fatalf("unexpected gitops spec: %+v", first)
	}
}

func TestCreateApplicationAcceptsNodeStaticSource(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	ctx := context.Background()
	spec := BuildSpec{
		SourcePath:          ".",
		BuildCommand:        "yarn install && yarn build",
		ArtifactCopyCommand: "cp -ar dist/. \"$PAAS_ARTIFACT_OUTPUT/\"",
		RuntimeBaseImage:    "nginx:1.26.2",
		DefaultRef:          "main",
	}

	app, err := env.svc.CreateApplication(ctx, CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "macc-frontend", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", spec)}})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	source, err := env.svc.GetApplicationSource(ctx, app.ID)
	if err != nil {
		t.Fatalf("GetApplicationSource() error = %v", err)
	}
	if source.BuildSpec.ArtifactCopyCommand != "cp -ar dist/. \"$PAAS_ARTIFACT_OUTPUT/\"" {
		t.Fatalf("artifact copy command should be fixed, got %+v", source.BuildSpec)
	}
}

func TestSetEnvironmentConfigAndSecretAuditWithoutSecretValue(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	ctx := context.Background()
	app, err := env.svc.CreateApplication(ctx, CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "config-api", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	environments, err := env.svc.ListEnvironments(ctx, app.ID)
	if err != nil {
		t.Fatalf("ListEnvironments() error = %v", err)
	}
	config, err := env.svc.SetEnvironmentConfig(ctx, SetEnvironmentConfigInput{Actor: appenvActor(), EnvironmentID: environments[0].ID, Key: "JAVA_OPTS", Value: "-Xmx512m"})
	if err != nil {
		t.Fatalf("SetEnvironmentConfig() error = %v", err)
	}
	secret, err := env.svc.SetEnvironmentSecret(ctx, SetEnvironmentSecretInput{Actor: appenvActor(), EnvironmentID: environments[0].ID, Key: "DB_PASSWORD", SecretRef: "secret/data/order/db"})
	if err != nil {
		t.Fatalf("SetEnvironmentSecret() error = %v", err)
	}
	if config.Value != "-Xmx512m" || secret.SecretRef != "secret/data/order/db" {
		t.Fatalf("unexpected config or secret metadata: %+v %+v", config, secret)
	}
	var configAudit, secretAudit bool
	for _, event := range env.audit.events {
		if event.Action == "environment_config.update" {
			configAudit = true
		}
		if event.Action == "environment_secret.update" {
			secretAudit = true
			if event.Summary == "secret/data/order/db" {
				t.Fatalf("secret ref should not be used as audit summary")
			}
		}
	}
	if !configAudit || !secretAudit {
		t.Fatalf("expected config and secret audit events, got %+v", env.audit.events)
	}
}

func TestWorkloadLifecycleValidationEnabledListAndAudit(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	ctx := context.Background()
	app, err := env.svc.CreateApplication(ctx, CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "workload-api", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}

	workload, err := env.svc.CreateWorkload(ctx, CreateWorkloadInput{
		Actor:         appenvActor(),
		ApplicationID: app.ID,
		Name:          "api",
		DisplayName:   "接口服务",
		WorkloadType:  WorkloadTypeDeployment,
	})
	if err != nil {
		t.Fatalf("CreateWorkload() error = %v", err)
	}
	if workload.ApplicationID != app.ID || workload.Name != "api" || workload.WorkloadType != WorkloadTypeDeployment || workload.Status != WorkloadStatusEnabled {
		t.Fatalf("unexpected workload: %+v", workload)
	}
	lowercase, err := env.svc.CreateWorkload(ctx, CreateWorkloadInput{
		Actor:         appenvActor(),
		ApplicationID: app.ID,
		Name:          "lowercase-api",
		WorkloadType:  WorkloadType("deployment"),
	})
	if err != nil {
		t.Fatalf("CreateWorkload(lowercase) error = %v", err)
	}
	if lowercase.WorkloadType != WorkloadTypeDeployment {
		t.Fatalf("lowercase workload_type should be normalized, got %+v", lowercase)
	}
	if _, err := env.svc.CreateWorkload(ctx, CreateWorkloadInput{Actor: appenvActor(), ApplicationID: app.ID, Name: "api", WorkloadType: WorkloadTypeStatefulSet}); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate workload name should fail, got %v", err)
	}
	if _, err := env.svc.CreateWorkload(ctx, CreateWorkloadInput{Actor: appenvActor(), ApplicationID: app.ID, Name: "worker", WorkloadType: WorkloadType("DaemonSet")}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("unsupported workload type should fail, got %v", err)
	}

	worker, err := env.svc.CreateWorkload(ctx, CreateWorkloadInput{Actor: appenvActor(), ApplicationID: app.ID, Name: "worker", WorkloadType: WorkloadTypeStatefulSet})
	if err != nil {
		t.Fatalf("CreateWorkload(worker) error = %v", err)
	}
	updated, err := env.svc.UpdateWorkload(ctx, UpdateWorkloadInput{Actor: appenvActor(), ApplicationID: app.ID, WorkloadID: worker.ID, DisplayName: "后台任务", Description: "handles async jobs"})
	if err != nil {
		t.Fatalf("UpdateWorkload() error = %v", err)
	}
	if updated.DisplayName != "后台任务" || updated.Description != "handles async jobs" || updated.WorkloadType != WorkloadTypeStatefulSet {
		t.Fatalf("unexpected updated workload: %+v", updated)
	}
	if _, err := env.svc.DisableWorkload(ctx, WorkloadStatusInput{Actor: appenvActor(), ApplicationID: app.ID, WorkloadID: worker.ID}); err != nil {
		t.Fatalf("DisableWorkload() error = %v", err)
	}
	enabled, err := env.svc.ListEnabledWorkloads(ctx, app.ID)
	if err != nil {
		t.Fatalf("ListEnabledWorkloads() error = %v", err)
	}
	if len(enabled) != 2 || enabled[0].ID != workload.ID || enabled[1].ID != lowercase.ID {
		t.Fatalf("disabled workload should not be listed as enabled, got %+v", enabled)
	}
	if _, err := env.svc.EnableWorkload(ctx, WorkloadStatusInput{Actor: appenvActor(), ApplicationID: app.ID, WorkloadID: worker.ID}); err != nil {
		t.Fatalf("EnableWorkload() error = %v", err)
	}
	if _, err := env.svc.DeleteWorkload(ctx, WorkloadStatusInput{Actor: appenvActor(), ApplicationID: app.ID, WorkloadID: worker.ID}); err != nil {
		t.Fatalf("DeleteWorkload() error = %v", err)
	}
	if _, err := env.svc.EnableWorkload(ctx, WorkloadStatusInput{Actor: appenvActor(), ApplicationID: app.ID, WorkloadID: worker.ID}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("deleted workload should not be re-enabled, got %v", err)
	}
	if _, err := env.svc.DisableWorkload(ctx, WorkloadStatusInput{Actor: appenvActor(), ApplicationID: app.ID, WorkloadID: worker.ID}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("deleted workload should not be disabled again, got %v", err)
	}
	enabled, err = env.svc.ListEnabledWorkloads(ctx, app.ID)
	if err != nil {
		t.Fatalf("ListEnabledWorkloads(after delete) error = %v", err)
	}
	if len(enabled) != 2 || enabled[0].ID != workload.ID || enabled[1].ID != lowercase.ID {
		t.Fatalf("deleted workload should not be listed as enabled, got %+v", enabled)
	}

	actions := map[string]int{}
	for _, event := range env.audit.events {
		actions[event.Action]++
	}
	if actions["workload.create"] != 3 || actions["workload.status_change"] != 3 {
		t.Fatalf("expected workload create and status audits, got %+v", env.audit.events)
	}
}

func TestWorkloadEnvironmentConfigSaveQueryAndAudit(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	ctx := context.Background()
	app, err := env.svc.CreateApplication(ctx, CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "config-workload", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	environments, err := env.svc.ListEnvironments(ctx, app.ID)
	if err != nil {
		t.Fatalf("ListEnvironments() error = %v", err)
	}
	workload, err := env.svc.CreateWorkload(ctx, CreateWorkloadInput{Actor: appenvActor(), ApplicationID: app.ID, Name: "api", WorkloadType: WorkloadTypeDeployment})
	if err != nil {
		t.Fatalf("CreateWorkload() error = %v", err)
	}

	config, err := env.svc.SaveWorkloadEnvironmentConfig(ctx, SaveWorkloadEnvironmentConfigInput{
		Actor:         appenvActor(),
		ApplicationID: app.ID,
		WorkloadID:    workload.ID,
		EnvironmentID: environments[0].ID,
		Replicas:      2,
		ServicePorts:  []WorkloadServicePort{{Name: "http", Port: 80, TargetPort: 8080, Protocol: "TCP"}},
		ResourceRequests: WorkloadResourceList{
			CPU:    "100m",
			Memory: "128Mi",
		},
		ResourceLimits: WorkloadResourceList{
			CPU:    "500m",
			Memory: "512Mi",
		},
		Probes:         []WorkloadProbe{{Name: "readiness", Type: "http", Path: "/healthz", Port: 8080, InitialDelaySeconds: 5, PeriodSeconds: 10}},
		IngressHosts:   []WorkloadIngressHost{{Host: "api.dev.example.com", Path: "/"}},
		EnvVars:        []WorkloadEnvVar{{Name: "JAVA_OPTS", Value: "-Xmx256m"}},
		SecretRefs:     []WorkloadSecretRef{{Name: "DB_PASSWORD", SecretRef: "secret/data/payment/db"}},
		ConfigFiles:    []WorkloadConfigFile{{MountPath: "/etc/app/app.yml", Content: "server.port: 8080"}},
		WritableDirs:   []WorkloadWritableDir{{MountPath: "/data/uploads", SizeLimit: "1Gi"}},
		VolumeMounts:   []WorkloadVolumeMount{{Name: "cache", MountPath: "/cache"}},
		InitContainers: []WorkloadInitContainer{{Name: "init-db", Image: "busybox:1.36", Command: []string{"sh", "-c", "echo init"}}},
		ValuesOverride: map[string]any{"podAnnotations": map[string]any{"prometheus.io/scrape": "true"}},
	})
	if err != nil {
		t.Fatalf("SaveWorkloadEnvironmentConfig() error = %v", err)
	}
	if config.Replicas != 2 || len(config.ServicePorts) != 1 || len(config.Probes) != 1 || len(config.IngressHosts) != 1 || len(config.ConfigFiles) != 1 || len(config.WritableDirs) != 1 {
		t.Fatalf("deployment config should preserve structured fields: %+v", config)
	}
	got, err := env.svc.GetWorkloadEnvironmentConfig(ctx, workload.ID, environments[0].ID)
	if err != nil {
		t.Fatalf("GetWorkloadEnvironmentConfig() error = %v", err)
	}
	if got.ID != config.ID || got.ServicePorts[0].TargetPort != 8080 || got.ResourceLimits.Memory != "512Mi" || got.ConfigFiles[0].MountPath != "/etc/app/app.yml" || got.WritableDirs[0].MountPath != "/data/uploads" {
		t.Fatalf("unexpected saved config: %+v", got)
	}
	configs, err := env.svc.ListWorkloadEnvironmentConfigs(ctx, workload.ID)
	if err != nil {
		t.Fatalf("ListWorkloadEnvironmentConfigs() error = %v", err)
	}
	if len(configs) != 1 || configs[0].ID != config.ID {
		t.Fatalf("unexpected config list: %+v", configs)
	}
	var audited bool
	for _, event := range env.audit.events {
		if event.Action == "workload_environment_config.update" && event.ResourceID == config.ID {
			audited = true
		}
	}
	if !audited {
		t.Fatalf("config change should be audited, got %+v", env.audit.events)
	}

	if _, err := env.svc.DeleteWorkload(ctx, WorkloadStatusInput{Actor: appenvActor(), ApplicationID: app.ID, WorkloadID: workload.ID}); err != nil {
		t.Fatalf("DeleteWorkload() error = %v", err)
	}
	if _, err := env.svc.SaveWorkloadEnvironmentConfig(ctx, SaveWorkloadEnvironmentConfigInput{
		Actor:         appenvActor(),
		ApplicationID: app.ID,
		WorkloadID:    workload.ID,
		EnvironmentID: environments[0].ID,
		Replicas:      1,
	}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("deleted workload should not accept environment config, got %v", err)
	}
}

func TestWorkloadEnvironmentConfigRequiresApplicationOwnership(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	ctx := context.Background()
	firstApp, err := env.svc.CreateApplication(ctx, CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "first-app", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}})
	if err != nil {
		t.Fatalf("CreateApplication(first) error = %v", err)
	}
	secondApp, err := env.svc.CreateApplication(ctx, CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "second-app", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}})
	if err != nil {
		t.Fatalf("CreateApplication(second) error = %v", err)
	}
	environments, err := env.svc.ListEnvironments(ctx, firstApp.ID)
	if err != nil {
		t.Fatalf("ListEnvironments() error = %v", err)
	}
	workload, err := env.svc.CreateWorkload(ctx, CreateWorkloadInput{Actor: appenvActor(), ApplicationID: firstApp.ID, Name: "api", WorkloadType: WorkloadTypeDeployment})
	if err != nil {
		t.Fatalf("CreateWorkload() error = %v", err)
	}
	if _, err := env.svc.SaveWorkloadEnvironmentConfig(ctx, SaveWorkloadEnvironmentConfigInput{
		Actor:         appenvActor(),
		ApplicationID: firstApp.ID,
		WorkloadID:    workload.ID,
		EnvironmentID: environments[0].ID,
		Replicas:      1,
	}); err != nil {
		t.Fatalf("SaveWorkloadEnvironmentConfig() error = %v", err)
	}
	if _, err := env.svc.ListWorkloadEnvironmentConfigsForApplication(ctx, secondApp.ID, workload.ID); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("cross-application config list should fail, got %v", err)
	}
}

func TestCreateApplicationValidatesExplicitSourceRepository(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	ctx := context.Background()

	if _, err := env.svc.CreateApplication(ctx, CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "api", Sources: []CreateApplicationSourceInput{validSourceInput("repo_other", validBuildSpec())}}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("foreign source repository should fail, got %v", err)
	}
	if _, err := env.svc.CreateApplication(ctx, CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "api", Sources: []CreateApplicationSourceInput{validSourceInput("repo_busy", validBuildSpec())}}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("non-ready source repository should fail, got %v", err)
	}
}

func TestCreateApplicationBuildSpecValidation(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*BuildSpec)
	}{
		{name: "empty command", mut: func(spec *BuildSpec) { spec.BuildCommand = " " }},
		{name: "empty artifact copy command", mut: func(spec *BuildSpec) { spec.ArtifactCopyCommand = " " }},
		{name: "bad artifact deploy path", mut: func(spec *BuildSpec) { spec.ArtifactDeployPath = "../app" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newAppenvTestEnv(t, false)
			spec := validBuildSpec()
			tt.mut(&spec)
			_, err := env.svc.CreateApplication(context.Background(), CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "api", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", spec)}})
			if shared.CodeOf(err) != shared.CodeInvalidArgument {
				t.Fatalf("expected invalid_argument, got %v", err)
			}
		})
	}
}

func TestCreateApplicationPropagatesPermissionAndProvisionFailures(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	env.permission.err = shared.NewError(shared.CodePermissionDenied, "denied")
	if _, err := env.svc.CreateApplication(context.Background(), CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "api", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}}); shared.CodeOf(err) != shared.CodePermissionDenied {
		t.Fatalf("permission denial should fail, got %v", err)
	}

	env = newAppenvTestEnv(t, true)
	env.gitops.err = errors.New("gitops unavailable")
	if _, err := env.svc.CreateApplication(context.Background(), CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "api", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}}); err == nil {
		t.Fatalf("gitops provisioning failure should fail")
	}
}

func TestCreateApplicationPropagatesClusterAndEventFailures(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	env.clusters.err = errors.New("cluster placement failed")
	if _, err := env.svc.CreateApplication(context.Background(), CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "api", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}}); err == nil {
		t.Fatalf("cluster placement failure should fail")
	}

	env = newAppenvTestEnv(t, false)
	env.events.err = errors.New("event bus failed")
	if _, err := env.svc.CreateApplication(context.Background(), CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "api", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}}); err == nil {
		t.Fatalf("event publish failure should fail")
	}

	env = newAppenvTestEnv(t, false)
	env.svc.ids = failingIDGenerator{}
	if _, err := env.svc.CreateApplication(context.Background(), CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "api", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}}); err == nil {
		t.Fatalf("id generation failure should fail")
	}
}

func TestServiceGuardBranches(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	ctx := context.Background()
	if _, err := env.svc.CreateApplication(ctx, CreateApplicationInput{Actor: identityaccess.Subject{}, ProjectID: "project_payment", Name: "api", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}}); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("missing actor should fail, got %v", err)
	}
	if _, err := env.svc.CreateApplication(ctx, CreateApplicationInput{Actor: appenvActor(), ProjectID: "", Name: "api", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("missing project should fail, got %v", err)
	}
	if _, err := env.svc.CreateApplication(ctx, CreateApplicationInput{Actor: appenvActor(), ProjectID: "missing", Name: "api", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing project lookup should fail, got %v", err)
	}
	if _, err := env.svc.CreateApplication(ctx, CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "1bad", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("bad app name should fail, got %v", err)
	}
	if _, err := env.svc.ListApplicationsByProject(ctx, "missing", shared.PageRequest{}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("list missing project should fail, got %v", err)
	}

	noProjectService := NewService(Options{Repository: newTestRepository(t)})
	if _, err := noProjectService.ListApplicationsByProject(ctx, "project_payment", shared.PageRequest{}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("nil project query should fail, got %v", err)
	}
	noRepoService := NewService(Options{Repository: newTestRepository(t), ProjectQuery: env.svc.projects})
	if _, err := noRepoService.CreateApplication(ctx, CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "api", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("nil source repository query should fail, got %v", err)
	}
}

func TestEnvironmentStateAndEvents(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	ctx := context.Background()
	app, err := env.svc.CreateApplication(ctx, CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "api", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	environments, _ := env.svc.ListEnvironments(ctx, app.ID)
	reportedAt := time.Date(2026, 5, 30, 5, 10, 0, 0, time.UTC)
	state, err := env.svc.UpdateEnvironmentState(ctx, UpdateEnvironmentStateInput{EnvironmentID: environments[0].ID, Status: EnvironmentStatusRunning, Message: "运行中", ReportedAt: &reportedAt})
	if err != nil {
		t.Fatalf("UpdateEnvironmentState() error = %v", err)
	}
	if state.Status != EnvironmentStatusRunning || state.LastReportedAt == nil || !state.LastReportedAt.Equal(reportedAt) {
		t.Fatalf("unexpected state: %+v", state)
	}
	events, err := env.svc.ListEnvironmentEvents(ctx, environments[0].ID, shared.PageRequest{})
	if err != nil {
		t.Fatalf("ListEnvironmentEvents() error = %v", err)
	}
	if events.Total != 2 {
		t.Fatalf("expected creation and state events, got %+v", events)
	}
	if _, err := env.svc.UpdateEnvironmentState(ctx, UpdateEnvironmentStateInput{EnvironmentID: environments[0].ID, Status: "unknown"}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("invalid status should fail, got %v", err)
	}
}

func TestApplicationQueriesUpdateDeleteAndManualBinding(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	ctx := context.Background()
	app, err := env.svc.CreateApplication(ctx, CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "api", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	if got, err := env.svc.GetApplication(ctx, app.ID); err != nil || got.ID != app.ID {
		t.Fatalf("GetApplication() = %+v, %v", got, err)
	}
	page, err := env.svc.ListApplicationsByProject(ctx, "project_payment", shared.PageRequest{Page: 1, PageSize: 10})
	if err != nil || page.Total != 1 || page.Items[0].ID != app.ID {
		t.Fatalf("ListApplicationsByProject() = %+v, %v", page, err)
	}
	updated, err := env.svc.UpdateApplication(ctx, UpdateApplicationInput{Actor: appenvActor(), ApplicationID: app.ID, DisplayName: "接口服务", Description: " updated "})
	if err != nil {
		t.Fatalf("UpdateApplication() error = %v", err)
	}
	if updated.DisplayName != "接口服务" || updated.Description != "updated" || updated.Status != ApplicationStatusActive {
		t.Fatalf("unexpected updated app: %+v", updated)
	}
	editedSpec := validBuildSpec()
	editedSpec.BuildCommand = "mvn verify"
	editedSpec.ArtifactCopyCommand = "cp -ar target/edited.jar \"$PAAS_ARTIFACT_OUTPUT/app.jar\""
	updated, err = env.svc.UpdateApplication(ctx, UpdateApplicationInput{
		Actor:                 appenvActor(),
		ApplicationID:         app.ID,
		DisplayName:           "接口服务",
		RuntimeEnvironmentIDs: []shared.ID{"runtime_env_java17"},
		Sources:               []CreateApplicationSourceInput{validSourceInput("repo_user", editedSpec)},
	})
	if err != nil {
		t.Fatalf("UpdateApplication() with source error = %v", err)
	}
	if !updated.RuntimeEnvironmentID.IsZero() || len(updated.RuntimeEnvironments) != 0 {
		t.Fatalf("runtime environment should not be updated on application, got %+v", updated)
	}
	editedSource, err := env.svc.GetApplicationSource(ctx, app.ID)
	if err != nil || editedSource.BuildSpec.BuildCommand != "mvn verify" || editedSource.BuildSpec.ArtifactCopyCommand != "cp -ar target/edited.jar \"$PAAS_ARTIFACT_OUTPUT/app.jar\"" {
		t.Fatalf("source should be editable, got %+v, %v", editedSource, err)
	}
	environments, _ := env.svc.ListEnvironments(ctx, app.ID)
	gotEnv, err := env.svc.GetEnvironment(ctx, environments[0].ID)
	if err != nil || gotEnv.Name != "dev" {
		t.Fatalf("GetEnvironment() = %+v, %v", gotEnv, err)
	}
	binding, err := env.svc.BindEnvironmentCluster(ctx, BindEnvironmentClusterInput{Actor: appenvActor(), EnvironmentID: gotEnv.ID, ClusterID: "cluster_manual", ClusterName: "manual", Namespace: "api-dev"})
	if err != nil {
		t.Fatalf("BindEnvironmentCluster() error = %v", err)
	}
	if binding.EnvironmentID != gotEnv.ID || binding.Namespace != "api-dev" {
		t.Fatalf("unexpected binding: %+v", binding)
	}
	state, err := env.svc.GetEnvironmentState(ctx, gotEnv.ID)
	if err != nil || state.Status != EnvironmentStatusClusterBound {
		t.Fatalf("GetEnvironmentState() = %+v, %v", state, err)
	}
	if len(env.gitops.specs) != 1 || env.gitops.specs[0].EnvironmentName != "dev" {
		t.Fatalf("manual binding should provision gitops once, got %+v", env.gitops.specs)
	}
	if _, err := env.svc.BindEnvironmentCluster(ctx, BindEnvironmentClusterInput{Actor: appenvActor(), EnvironmentID: environments[1].ID, ClusterName: "manual", Namespace: "api-test"}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("incomplete binding should fail, got %v", err)
	}
	if _, err := env.svc.BindEnvironmentCluster(ctx, BindEnvironmentClusterInput{Actor: identityaccess.Subject{}, EnvironmentID: environments[1].ID, ClusterID: "cluster_2", ClusterName: "manual", Namespace: "api-test"}); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("binding without actor should fail, got %v", err)
	}
	if err := env.svc.DeleteApplication(ctx, appenvActor(), app.ID); err != nil {
		t.Fatalf("DeleteApplication() error = %v", err)
	}
	if len(env.pipelines.deletedIDs) != 1 || env.pipelines.deletedIDs[0] != app.ID {
		t.Fatalf("expected Jenkins pipeline cleanup, got %+v", env.pipelines.deletedIDs)
	}
	if _, err := env.svc.GetApplication(ctx, app.ID); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("deleted app should be removed, got %v", err)
	}
	if _, err := env.svc.GetApplicationSource(ctx, app.ID); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("deleted app source should be removed, got %v", err)
	}
	if _, err := env.svc.ListEnvironments(ctx, app.ID); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("deleted app environments should be removed, got %v", err)
	}
	if _, err := env.svc.GetEnvironment(ctx, gotEnv.ID); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("deleted app environment should be removed, got %v", err)
	}
}

func TestRepositoryDirectMethods(t *testing.T) {
	repo := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 30, 6, 0, 0, 0, time.UTC)
	app := Application{ID: "app_1", TenantID: "tenant_a", ProjectID: "project_payment", Name: "api", Status: ApplicationStatusActive, CreatedAt: now, UpdatedAt: now}
	if err := repo.CreateApplication(ctx, app); err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	if err := repo.CreateApplication(ctx, app); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate app should conflict, got %v", err)
	}
	app.DisplayName = "接口"
	if err := repo.UpdateApplication(ctx, app); err != nil {
		t.Fatalf("UpdateApplication() error = %v", err)
	}
	found, err := repo.FindApplicationByProjectAndName(ctx, "project_payment", "api")
	if err != nil || found.DisplayName != "接口" {
		t.Fatalf("FindApplicationByProjectAndName() = %+v, %v", found, err)
	}
	source := ApplicationSource{ID: "source_1", TenantID: app.TenantID, ProjectID: app.ProjectID, ApplicationID: app.ID, Key: "main", SourceRepositoryID: "repo_user", SourcePath: ".", BuildSpec: validBuildSpec(), IsPrimary: true, CreatedAt: now, UpdatedAt: now}
	if err := repo.CreateApplicationSource(ctx, source); err != nil {
		t.Fatalf("CreateApplicationSource() error = %v", err)
	}
	source.SourcePath = "services/api"
	if err := repo.UpdateApplicationSource(ctx, source); err != nil {
		t.Fatalf("UpdateApplicationSource() error = %v", err)
	}
	environment := Environment{ID: "env_1", TenantID: app.TenantID, ProjectID: app.ProjectID, ApplicationID: app.ID, Name: "qa", CreatedAt: now, UpdatedAt: now}
	if err := repo.CreateEnvironment(ctx, environment); err != nil {
		t.Fatalf("CreateEnvironment() error = %v", err)
	}
	environment.DisplayName = "验证环境"
	if err := repo.UpdateEnvironment(ctx, environment); err != nil {
		t.Fatalf("UpdateEnvironment() error = %v", err)
	}
	if err := repo.CreateEnvironmentConfig(ctx, EnvironmentConfig{ID: "cfg_1", TenantID: app.TenantID, ProjectID: app.ProjectID, ApplicationID: app.ID, EnvironmentID: environment.ID, Key: "LOG_LEVEL", Value: "info"}); err != nil {
		t.Fatalf("CreateEnvironmentConfig() error = %v", err)
	}
	if err := repo.CreateEnvironmentSecret(ctx, EnvironmentSecret{ID: "sec_1", TenantID: app.TenantID, ProjectID: app.ProjectID, ApplicationID: app.ID, EnvironmentID: environment.ID, Key: "DB_PASSWORD", SecretRef: "secret/db"}); err != nil {
		t.Fatalf("CreateEnvironmentSecret() error = %v", err)
	}
	if err := repo.CreateEnvironmentRoute(ctx, EnvironmentRoute{ID: "route_1", TenantID: app.TenantID, ProjectID: app.ProjectID, ApplicationID: app.ID, EnvironmentID: environment.ID, Host: "api.example.com", Path: "/"}); err != nil {
		t.Fatalf("CreateEnvironmentRoute() error = %v", err)
	}
	if _, err := repo.GetApplication(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing app should be not_found, got %v", err)
	}
	if err := repo.UpdateApplication(ctx, Application{ID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("updating missing app should be not_found, got %v", err)
	}
	changedOwner := app
	changedOwner.ProjectID = "project_other"
	if err := repo.UpdateApplication(ctx, changedOwner); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("changing app ownership should fail, got %v", err)
	}
	renamed := app
	renamed.Name = "api-renamed"
	if err := repo.UpdateApplication(ctx, renamed); err != nil {
		t.Fatalf("renaming app should succeed: %v", err)
	}
	if _, err := repo.FindApplicationByProjectAndName(ctx, "project_payment", "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing app name should be not_found, got %v", err)
	}
	if _, err := repo.ListApplicationsByProject(ctx, "project_payment", shared.PageRequest{Page: 99, PageSize: 10}); err != nil {
		t.Fatalf("out of range list should succeed: %v", err)
	}
	if err := repo.CreateApplicationSource(ctx, ApplicationSource{ID: "bad", ApplicationID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("source for missing app should fail, got %v", err)
	}
	if err := repo.CreateApplicationSource(ctx, source); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate source should conflict, got %v", err)
	}
	if err := repo.UpdateApplicationSource(ctx, ApplicationSource{ApplicationID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("update missing source should fail, got %v", err)
	}
	if _, err := repo.GetApplicationSource(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("get missing source should fail, got %v", err)
	}
	if err := repo.CreateEnvironment(ctx, Environment{ID: "bad", ApplicationID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("environment for missing app should fail, got %v", err)
	}
	if err := repo.CreateEnvironment(ctx, Environment{ID: "env_2", TenantID: app.TenantID, ProjectID: app.ProjectID, ApplicationID: app.ID, Name: "qa"}); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate environment name should conflict, got %v", err)
	}
	if err := repo.UpdateEnvironment(ctx, Environment{ID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("update missing environment should fail, got %v", err)
	}
	if _, err := repo.GetEnvironment(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("get missing environment should fail, got %v", err)
	}
	if _, err := repo.ListEnvironmentsByApplication(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("list missing environments should fail, got %v", err)
	}
	if err := repo.CreateEnvironmentConfig(ctx, EnvironmentConfig{ID: "bad", EnvironmentID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("config missing environment should fail, got %v", err)
	}
	if err := repo.CreateEnvironmentSecret(ctx, EnvironmentSecret{ID: "bad", EnvironmentID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("secret missing environment should fail, got %v", err)
	}
	if err := repo.CreateEnvironmentRoute(ctx, EnvironmentRoute{ID: "bad", EnvironmentID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("route missing environment should fail, got %v", err)
	}
	binding := EnvironmentClusterBinding{ID: "binding_1", TenantID: app.TenantID, ProjectID: app.ProjectID, ApplicationID: app.ID, EnvironmentID: environment.ID, ClusterID: "cluster_1", ClusterName: "cluster", Namespace: "api", Status: EnvironmentClusterBindingActive}
	if err := repo.CreateEnvironmentClusterBinding(ctx, EnvironmentClusterBinding{ID: "bad", EnvironmentID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("binding missing environment should fail, got %v", err)
	}
	if err := repo.CreateEnvironmentClusterBinding(ctx, binding); err != nil {
		t.Fatalf("CreateEnvironmentClusterBinding() error = %v", err)
	}
	if err := repo.CreateEnvironmentClusterBinding(ctx, binding); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate binding should conflict, got %v", err)
	}
	if _, err := repo.GetEnvironmentClusterBinding(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing binding should fail, got %v", err)
	}
	if _, err := repo.ListEnvironmentClusterBindingsByApplication(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("list missing bindings should fail, got %v", err)
	}
	if err := repo.SaveEnvironmentState(ctx, EnvironmentState{EnvironmentID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("state missing environment should fail, got %v", err)
	}
	if _, err := repo.GetEnvironmentState(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing state should fail, got %v", err)
	}
	if _, err := repo.ListEnvironmentStatesByApplication(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("list missing states should fail, got %v", err)
	}
	if err := repo.AppendEnvironmentEvent(ctx, EnvironmentEvent{ID: "bad", EnvironmentID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("event missing environment should fail, got %v", err)
	}
	if _, err := repo.ListEnvironmentEvents(ctx, "missing", shared.PageRequest{}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("list missing events should fail, got %v", err)
	}
}

func TestHandlerApplicationEnvironmentFlow(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	mux := http.NewServeMux()
	NewHandler(env.svc).Register(mux)
	body, _ := json.Marshal(CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "api", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}})

	rec := serveJSON(mux, http.MethodPost, "/api/applications", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var app Application
	if err := json.NewDecoder(rec.Body).Decode(&app); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if app.ID == "" || app.Name != "api" {
		t.Fatalf("unexpected response: %+v", app)
	}
	duplicate := serveJSON(mux, http.MethodPost, "/api/applications", body)
	assertStatus(t, duplicate, http.StatusConflict)
	if !strings.Contains(duplicate.Body.String(), "application name already exists in project") {
		t.Fatalf("expected conflict message, got %s", duplicate.Body.String())
	}
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/applications/"+app.ID.String(), nil), http.StatusOK)
	patchBody, _ := json.Marshal(UpdateApplicationInput{Actor: appenvActor(), DisplayName: "接口服务"})
	assertStatus(t, serveJSON(mux, http.MethodPatch, "/api/applications/"+app.ID.String(), patchBody), http.StatusOK)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/projects/project_payment/applications?page=1&page_size=10", nil), http.StatusOK)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/applications/"+app.ID.String()+"/source", nil), http.StatusOK)

	envListRec := serveJSON(mux, http.MethodGet, "/api/applications/"+app.ID.String()+"/environments", nil)
	assertStatus(t, envListRec, http.StatusOK)
	var envList struct {
		Items []Environment `json:"items"`
	}
	if err := json.NewDecoder(envListRec.Body).Decode(&envList); err != nil {
		t.Fatalf("decode environments: %v", err)
	}
	if len(envList.Items) != 4 {
		t.Fatalf("expected 4 environments, got %+v", envList)
	}
	environmentID := envList.Items[0].ID.String()
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/environments/"+environmentID, nil), http.StatusOK)
	bindBody, _ := json.Marshal(BindEnvironmentClusterInput{Actor: appenvActor(), ClusterID: "cluster_manual", ClusterName: "manual", Namespace: "api-dev"})
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/environments/"+environmentID+"/cluster-binding", bindBody), http.StatusCreated)
	configBody, _ := json.Marshal(SetEnvironmentConfigInput{Actor: appenvActor(), Key: "JAVA_OPTS", Value: "-Xmx512m"})
	assertStatus(t, serveJSON(mux, http.MethodPut, "/api/environments/"+environmentID+"/configs", configBody), http.StatusOK)
	secretBody, _ := json.Marshal(SetEnvironmentSecretInput{Actor: appenvActor(), Key: "DB_PASSWORD", SecretRef: "secret/data/api/db"})
	assertStatus(t, serveJSON(mux, http.MethodPut, "/api/environments/"+environmentID+"/secrets", secretBody), http.StatusOK)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/environments/"+environmentID+"/state", nil), http.StatusOK)
	stateBody, _ := json.Marshal(UpdateEnvironmentStateInput{Status: EnvironmentStatusRunning, Message: "运行中"})
	assertStatus(t, serveJSON(mux, http.MethodPut, "/api/environments/"+environmentID+"/state", stateBody), http.StatusOK)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/environments/"+environmentID+"/events?page=1&page_size=5", nil), http.StatusOK)

	actorBody, _ := json.Marshal(struct {
		Actor identityaccess.Subject `json:"actor"`
	}{Actor: appenvActor()})
	workloadBody, _ := json.Marshal(CreateWorkloadInput{Actor: appenvActor(), Name: "api", DisplayName: "接口服务", WorkloadType: WorkloadTypeDeployment})
	workloadRec := serveJSON(mux, http.MethodPost, "/api/applications/"+app.ID.String()+"/workloads", workloadBody)
	assertStatus(t, workloadRec, http.StatusCreated)
	var workload Workload
	if err := json.NewDecoder(workloadRec.Body).Decode(&workload); err != nil {
		t.Fatalf("decode workload: %v", err)
	}
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/applications/"+app.ID.String()+"/workloads", nil), http.StatusOK)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/applications/"+app.ID.String()+"/workloads?enabled=true", nil), http.StatusOK)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/applications/"+app.ID.String()+"/workloads/"+workload.ID.String(), nil), http.StatusOK)
	updateWorkloadBody, _ := json.Marshal(UpdateWorkloadInput{Actor: appenvActor(), DisplayName: "接口服务 v2", WorkloadType: WorkloadTypeDeployment})
	assertStatus(t, serveJSON(mux, http.MethodPut, "/api/applications/"+app.ID.String()+"/workloads/"+workload.ID.String(), updateWorkloadBody), http.StatusOK)
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/applications/"+app.ID.String()+"/workloads/"+workload.ID.String()+":disable", actorBody), http.StatusOK)
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/applications/"+app.ID.String()+"/workloads/"+workload.ID.String()+":enable", actorBody), http.StatusOK)
	configPayload, _ := json.Marshal(SaveWorkloadEnvironmentConfigInput{
		Actor:        appenvActor(),
		Replicas:     1,
		ServicePorts: []WorkloadServicePort{{Name: "http", Port: 80, TargetPort: 8080}},
		ConfigFiles:  []WorkloadConfigFile{{MountPath: "/etc/app/app.yml", Content: "server.port: 8080"}},
		WritableDirs: []WorkloadWritableDir{{MountPath: "/data"}},
	})
	assertStatus(t, serveJSON(mux, http.MethodPut, "/api/applications/"+app.ID.String()+"/workloads/"+workload.ID.String()+"/environment-configs/"+environmentID, configPayload), http.StatusOK)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/applications/"+app.ID.String()+"/workloads/"+workload.ID.String()+"/environment-configs", nil), http.StatusOK)
	otherBody, _ := json.Marshal(CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "other-api", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}})
	otherRec := serveJSON(mux, http.MethodPost, "/api/applications", otherBody)
	assertStatus(t, otherRec, http.StatusCreated)
	var otherApp Application
	if err := json.NewDecoder(otherRec.Body).Decode(&otherApp); err != nil {
		t.Fatalf("decode other app: %v", err)
	}
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/applications/"+otherApp.ID.String()+"/workloads/"+workload.ID.String()+"/environment-configs", nil), http.StatusNotFound)
	assertStatus(t, serveJSON(mux, http.MethodDelete, "/api/applications/"+app.ID.String()+"/workloads/"+workload.ID.String(), actorBody), http.StatusOK)

	assertStatus(t, serveJSON(mux, http.MethodDelete, "/api/applications/"+app.ID.String(), actorBody), http.StatusNoContent)
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/applications", []byte("{")), http.StatusBadRequest)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/applications/missing", nil), http.StatusNotFound)
}

func TestHandlerErrorBranches(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	mux := http.NewServeMux()
	NewHandler(env.svc).Register(mux)
	assertStatus(t, serveJSON(mux, http.MethodPatch, "/api/applications/missing", []byte("{")), http.StatusBadRequest)
	patchBody, _ := json.Marshal(UpdateApplicationInput{Actor: appenvActor(), DisplayName: "接口"})
	assertStatus(t, serveJSON(mux, http.MethodPatch, "/api/applications/missing", patchBody), http.StatusNotFound)
	assertStatus(t, serveJSON(mux, http.MethodDelete, "/api/applications/missing", []byte("{")), http.StatusBadRequest)
	deleteBody, _ := json.Marshal(struct {
		Actor identityaccess.Subject `json:"actor"`
	}{Actor: appenvActor()})
	assertStatus(t, serveJSON(mux, http.MethodDelete, "/api/applications/missing", deleteBody), http.StatusNotFound)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/projects/missing/applications", nil), http.StatusNotFound)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/applications/missing/source", nil), http.StatusNotFound)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/applications/missing/environments", nil), http.StatusNotFound)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/environments/missing", nil), http.StatusNotFound)
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/environments/missing/cluster-binding", []byte("{")), http.StatusBadRequest)
	bindBody, _ := json.Marshal(BindEnvironmentClusterInput{Actor: appenvActor(), ClusterID: "cluster", ClusterName: "cluster", Namespace: "api"})
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/environments/missing/cluster-binding", bindBody), http.StatusNotFound)
	assertStatus(t, serveJSON(mux, http.MethodPut, "/api/environments/missing/configs", []byte("{")), http.StatusBadRequest)
	configBody, _ := json.Marshal(SetEnvironmentConfigInput{Actor: appenvActor(), Key: "JAVA_OPTS", Value: "-Xmx512m"})
	assertStatus(t, serveJSON(mux, http.MethodPut, "/api/environments/missing/configs", configBody), http.StatusNotFound)
	assertStatus(t, serveJSON(mux, http.MethodPut, "/api/environments/missing/secrets", []byte("{")), http.StatusBadRequest)
	secretBody, _ := json.Marshal(SetEnvironmentSecretInput{Actor: appenvActor(), Key: "DB_PASSWORD", SecretRef: "secret/data/api/db"})
	assertStatus(t, serveJSON(mux, http.MethodPut, "/api/environments/missing/secrets", secretBody), http.StatusNotFound)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/environments/missing/state", nil), http.StatusNotFound)
	assertStatus(t, serveJSON(mux, http.MethodPut, "/api/environments/missing/state", []byte("{")), http.StatusBadRequest)
	stateBody, _ := json.Marshal(UpdateEnvironmentStateInput{Status: EnvironmentStatusRunning})
	assertStatus(t, serveJSON(mux, http.MethodPut, "/api/environments/missing/state", stateBody), http.StatusNotFound)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/environments/missing/events", nil), http.StatusNotFound)
}

func environmentNames(environments []Environment) string {
	names := make([]string, 0, len(environments))
	for _, environment := range environments {
		names = append(names, environment.Name)
	}
	return stringsJoin(names, ",")
}

func stringsJoin(values []string, sep string) string {
	if len(values) == 0 {
		return ""
	}
	result := values[0]
	for _, value := range values[1:] {
		result += sep + value
	}
	return result
}

func serveJSON(mux *http.ServeMux, method string, target string, body []byte) *httptest.ResponseRecorder {
	if body == nil {
		body = []byte("{}")
	}
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, status int) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, status, rec.Body.String())
	}
}
