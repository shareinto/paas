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

type recordingBuildPipelineCommand struct {
	createInputs []CreateBuildPipelineInput
	deleteCalls  []deletePipelineCall
	createErr    error
	deleteErr    error
	pipelineID   shared.ID
	pipelines    map[shared.ID]BuildPipelineRef
}

type deletePipelineCall struct {
	actor      identityaccess.Subject
	pipelineID shared.ID
}

type recordingManifestCleaner struct {
	applicationIDs []shared.ID
	err            error
}

func (c *recordingManifestCleaner) DeleteApplicationManifests(_ context.Context, applicationID shared.ID) error {
	c.applicationIDs = append(c.applicationIDs, applicationID)
	return c.err
}

type fakeBuildPipelineQuery struct {
	pipelines map[shared.ID]BuildPipelineRef
}

func (q fakeBuildPipelineQuery) GetBuildPipeline(_ context.Context, id shared.ID) (BuildPipelineRef, error) {
	pipeline, ok := q.pipelines[id]
	if !ok {
		return BuildPipelineRef{}, shared.NewError(shared.CodeNotFound, "build pipeline not found")
	}
	return pipeline, nil
}

func (c *recordingBuildPipelineCommand) CreateBuildPipeline(_ context.Context, input CreateBuildPipelineInput) (BuildPipelineRef, error) {
	c.createInputs = append(c.createInputs, input)
	if c.createErr != nil {
		return BuildPipelineRef{}, c.createErr
	}
	id := c.pipelineID
	if id.IsZero() {
		id = "pipeline_created"
	}
	pipeline := BuildPipelineRef{ID: id, ApplicationID: input.ApplicationID, Name: input.Name, DisplayName: input.DisplayName, Status: "active"}
	if c.pipelines != nil {
		c.pipelines[pipeline.ID] = pipeline
	}
	return pipeline, nil
}

func (c *recordingBuildPipelineCommand) DeleteBuildPipeline(_ context.Context, actor identityaccess.Subject, pipelineID shared.ID) error {
	c.deleteCalls = append(c.deleteCalls, deletePipelineCall{actor: actor, pipelineID: pipelineID})
	return c.deleteErr
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
	svc         *Service
	repo        Repository
	permission  *recordingPermission
	pipelines   *recordingBuildPipelineProvisioner
	pipelineCmd *recordingBuildPipelineCommand
	cleaner     *recordingManifestCleaner
	audit       *recordingAudit
	events      *recordingPublisher
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
	_ = clusterAvailable
	pipelines := &recordingBuildPipelineProvisioner{}
	pipelineRefs := map[shared.ID]BuildPipelineRef{
		"pipeline_main":  {ID: "pipeline_main", ApplicationID: "app_1", Name: "main", DisplayName: "主流水线", Status: "active"},
		"pipeline_other": {ID: "pipeline_other", ApplicationID: "other_app", Name: "other", DisplayName: "其他流水线", Status: "active"},
	}
	pipelineQuery := fakeBuildPipelineQuery{pipelines: pipelineRefs}
	pipelineCmd := &recordingBuildPipelineCommand{pipelineID: "pipeline_created", pipelines: pipelineRefs}
	audit := &recordingAudit{}
	events := &recordingPublisher{}
	cleaner := &recordingManifestCleaner{}
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
		BuildPipelineProvisioner: pipelines,
		BuildPipelineCommand:     pipelineCmd,
		BuildPipelineQuery:       pipelineQuery,
		ManifestCleaner:          cleaner,
		PermissionChecker:        permission,
		Audit:                    audit,
		EventPublisher:           events,
		IDGenerator:              testutil.NewFakeIDGenerator(1),
		Clock:                    testutil.NewFakeClock(time.Date(2026, 5, 30, 5, 0, 0, 0, time.UTC)),
	})
	return appenvTestEnv{svc: svc, repo: repo, permission: permission, pipelines: pipelines, pipelineCmd: pipelineCmd, cleaner: cleaner, audit: audit, events: events}
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

func validPipelineInput(name string) CreateBuildPipelineInput {
	return CreateBuildPipelineInput{
		Name:                  name,
		DisplayName:           name,
		RuntimeEnvironmentIDs: []shared.ID{"runtime_env_java17"},
		Sources: []BuildPipelineSourceInput{{
			Key:                "main",
			SourceRepositoryID: "repo_user",
			BuildSpec:          validBuildSpec(),
			IsPrimary:          true,
		}},
	}
}

func TestCreateApplicationPersistsSourceWithoutApplicationEnvironments(t *testing.T) {
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
	states, err := env.svc.ListApplicationStageStates(ctx, app.ID)
	if err != nil {
		t.Fatalf("ListApplicationStageStates() error = %v", err)
	}
	if len(states) != 0 {
		t.Fatalf("application creation must not create stage states before reports, got %+v", states)
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

func TestWorkloadLifecycleValidationEnabledListAndAudit(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	ctx := context.Background()
	app, err := env.svc.CreateApplication(ctx, CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "workload-api", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}

	workload, err := env.svc.CreateWorkload(ctx, CreateWorkloadInput{
		Actor:           appenvActor(),
		ApplicationID:   app.ID,
		Name:            "api",
		DisplayName:     "接口服务",
		WorkloadType:    WorkloadTypeDeployment,
		ImageSourceMode: "custom_image",
		PipelineID:      "pipeline_main",
	})
	if err != nil {
		t.Fatalf("CreateWorkload() error = %v", err)
	}
	if workload.ApplicationID != app.ID || workload.Name != "api" || workload.WorkloadType != WorkloadTypeDeployment || workload.Status != WorkloadStatusEnabled || workload.ImageSourceMode != "custom_image" || !workload.PipelineID.IsZero() {
		t.Fatalf("unexpected workload: %+v", workload)
	}
	persisted, err := env.repo.GetWorkload(ctx, workload.ID)
	if err != nil {
		t.Fatalf("GetWorkload() error = %v", err)
	}
	if persisted.ImageSourceMode != "custom_image" || !persisted.PipelineID.IsZero() {
		t.Fatalf("custom image workload should not keep pipeline_id, got %+v", persisted)
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
	if _, err := env.svc.CreateWorkload(ctx, CreateWorkloadInput{Actor: appenvActor(), ApplicationID: app.ID, Name: "bad-image-source", WorkloadType: WorkloadTypeDeployment, ImageSourceMode: "external_registry"}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("unsupported image_source_mode should fail, got %v", err)
	}

	worker, err := env.svc.CreateWorkload(ctx, CreateWorkloadInput{Actor: appenvActor(), ApplicationID: app.ID, Name: "worker", WorkloadType: WorkloadTypeStatefulSet})
	if err != nil {
		t.Fatalf("CreateWorkload(worker) error = %v", err)
	}
	updated, err := env.svc.UpdateWorkload(ctx, UpdateWorkloadInput{Actor: appenvActor(), ApplicationID: app.ID, WorkloadID: worker.ID, DisplayName: "后台任务", Description: "handles async jobs", ImageSourceMode: "mixed", PipelineID: "pipeline_main"})
	if err != nil {
		t.Fatalf("UpdateWorkload() error = %v", err)
	}
	if updated.DisplayName != "后台任务" || updated.Description != "handles async jobs" || updated.WorkloadType != WorkloadTypeStatefulSet || updated.ImageSourceMode != "mixed" || updated.PipelineID != "pipeline_main" {
		t.Fatalf("unexpected updated workload: %+v", updated)
	}
	if _, err := env.svc.UpdateWorkload(ctx, UpdateWorkloadInput{Actor: appenvActor(), ApplicationID: app.ID, WorkloadID: worker.ID, DisplayName: "后台任务", PipelineID: "pipeline_other"}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("cross-application pipeline should fail, got %v", err)
	}
	if _, err := env.svc.UpdateWorkload(ctx, UpdateWorkloadInput{Actor: appenvActor(), ApplicationID: app.ID, WorkloadID: worker.ID, DisplayName: "后台任务", ImageSourceMode: "bad_mode"}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("unsupported update image_source_mode should fail, got %v", err)
	}
	customImage, err := env.svc.UpdateWorkload(ctx, UpdateWorkloadInput{Actor: appenvActor(), ApplicationID: app.ID, WorkloadID: worker.ID, DisplayName: "后台任务", ImageSourceMode: "custom_image"})
	if err != nil {
		t.Fatalf("UpdateWorkload(custom_image) error = %v", err)
	}
	if customImage.ImageSourceMode != "custom_image" || !customImage.PipelineID.IsZero() {
		t.Fatalf("custom_image update should clear pipeline_id, got %+v", customImage)
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

func TestWorkloadNameCanBeReusedAfterSoftDelete(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	ctx := context.Background()
	app, err := env.svc.CreateApplication(ctx, CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "reuse-api", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	first, err := env.svc.CreateWorkload(ctx, CreateWorkloadInput{Actor: appenvActor(), ApplicationID: app.ID, Name: "api", WorkloadType: WorkloadTypeDeployment})
	if err != nil {
		t.Fatalf("CreateWorkload(first) error = %v", err)
	}
	if _, err := env.svc.DeleteWorkload(ctx, WorkloadStatusInput{Actor: appenvActor(), ApplicationID: app.ID, WorkloadID: first.ID}); err != nil {
		t.Fatalf("DeleteWorkload() error = %v", err)
	}

	second, err := env.svc.CreateWorkload(ctx, CreateWorkloadInput{Actor: appenvActor(), ApplicationID: app.ID, Name: "api", WorkloadType: WorkloadTypeDeployment})
	if err != nil {
		t.Fatalf("CreateWorkload(second) should allow deleted name reuse: %v", err)
	}
	if second.ID == first.ID || second.Name != "api" || second.Status != WorkloadStatusEnabled {
		t.Fatalf("recreated workload should be a new enabled record with same name, first=%+v second=%+v", first, second)
	}
	workloads, err := env.svc.ListWorkloads(ctx, app.ID)
	if err != nil {
		t.Fatalf("ListWorkloads() error = %v", err)
	}
	if len(workloads) != 1 || workloads[0].ID != second.ID {
		t.Fatalf("list should hide deleted workload and include recreated workload, got %+v", workloads)
	}
	if _, err := env.svc.CreateWorkload(ctx, CreateWorkloadInput{Actor: appenvActor(), ApplicationID: app.ID, Name: "api", WorkloadType: WorkloadTypeStatefulSet}); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("active workload should still reserve name, got %v", err)
	}
	if _, err := env.svc.DisableWorkload(ctx, WorkloadStatusInput{Actor: appenvActor(), ApplicationID: app.ID, WorkloadID: second.ID}); err != nil {
		t.Fatalf("DisableWorkload() error = %v", err)
	}
	if _, err := env.svc.CreateWorkload(ctx, CreateWorkloadInput{Actor: appenvActor(), ApplicationID: app.ID, Name: "api", WorkloadType: WorkloadTypeStatefulSet}); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("disabled workload should still reserve name, got %v", err)
	}
}

func TestWorkloadNameAllowsDisplayStyleNames(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	ctx := context.Background()
	app, err := env.svc.CreateApplication(ctx, CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "relaxed-workload-name", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	workload, err := env.svc.CreateWorkload(ctx, CreateWorkloadInput{Actor: appenvActor(), ApplicationID: app.ID, Name: "订单 API_主服务", WorkloadType: WorkloadTypeDeployment})
	if err != nil {
		t.Fatalf("CreateWorkload() should allow display-style workload names: %v", err)
	}
	if workload.Name != "订单 API_主服务" {
		t.Fatalf("workload name should be preserved, got %q", workload.Name)
	}
	updated, err := env.svc.UpdateWorkload(ctx, UpdateWorkloadInput{Actor: appenvActor(), ApplicationID: app.ID, WorkloadID: workload.ID, Name: "Order API_主服务 v2", WorkloadType: WorkloadTypeDeployment})
	if err != nil {
		t.Fatalf("UpdateWorkload() should allow display-style workload names: %v", err)
	}
	if updated.Name != "Order API_主服务 v2" {
		t.Fatalf("updated workload name should be preserved, got %q", updated.Name)
	}
	if _, err := env.svc.CreateWorkload(ctx, CreateWorkloadInput{Actor: appenvActor(), ApplicationID: app.ID, Name: "   ", WorkloadType: WorkloadTypeDeployment}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("blank workload name should still fail, got %v", err)
	}
}

func TestApplicationStageStateAndWorkloadStageConfigUseStageKey(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	ctx := context.Background()
	app, err := env.svc.CreateApplication(ctx, CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "stage-api", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	workload, err := env.svc.CreateWorkload(ctx, CreateWorkloadInput{Actor: appenvActor(), ApplicationID: app.ID, Name: "api", WorkloadType: WorkloadTypeDeployment})
	if err != nil {
		t.Fatalf("CreateWorkload() error = %v", err)
	}
	reportedAt := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	state, err := env.svc.UpdateApplicationStageState(ctx, UpdateApplicationStageStateInput{ApplicationID: app.ID, StageKey: "dev", Status: ApplicationStageStatusRunning, Message: "运行中", ReportedAt: &reportedAt})
	if err != nil {
		t.Fatalf("UpdateApplicationStageState() error = %v", err)
	}
	if state.ApplicationID != app.ID || state.StageKey != "dev" || state.Status != ApplicationStageStatusRunning || state.LastReportedAt == nil || !state.LastReportedAt.Equal(reportedAt) {
		t.Fatalf("unexpected stage state: %+v", state)
	}
	states, err := env.svc.ListApplicationStageStates(ctx, app.ID)
	if err != nil {
		t.Fatalf("ListApplicationStageStates() error = %v", err)
	}
	if len(states) != 1 || states[0].StageKey != "dev" {
		t.Fatalf("unexpected stage states: %+v", states)
	}
	events, err := env.svc.ListApplicationStageEvents(ctx, app.ID, "dev", shared.PageRequest{})
	if err != nil {
		t.Fatalf("ListApplicationStageEvents() error = %v", err)
	}
	if events.Total != 1 {
		t.Fatalf("expected one stage state event, got %+v", events)
	}
	config, err := env.svc.SaveWorkloadStageConfig(ctx, SaveWorkloadStageConfigInput{
		Actor:         appenvActor(),
		ApplicationID: app.ID,
		WorkloadID:    workload.ID,
		StageKey:      "dev",
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
		t.Fatalf("SaveWorkloadStageConfig() error = %v", err)
	}
	if config.ApplicationID != app.ID || config.WorkloadID != workload.ID || config.StageKey != "dev" || config.Replicas != 2 {
		t.Fatalf("unexpected stage config: %+v", config)
	}
	got, err := env.svc.GetWorkloadStageConfig(ctx, workload.ID, "dev")
	if err != nil {
		t.Fatalf("GetWorkloadStageConfig() error = %v", err)
	}
	if got.ID != config.ID || got.StageKey != "dev" || got.ServicePorts[0].TargetPort != 8080 || got.ResourceLimits.Memory != "512Mi" || got.ConfigFiles[0].MountPath != "/etc/app/app.yml" || got.WritableDirs[0].MountPath != "/data/uploads" {
		t.Fatalf("unexpected saved stage config: %+v", got)
	}
	configs, err := env.svc.ListWorkloadStageConfigs(ctx, workload.ID)
	if err != nil {
		t.Fatalf("ListWorkloadStageConfigs() error = %v", err)
	}
	if len(configs) != 1 || configs[0].ID != config.ID {
		t.Fatalf("unexpected stage config list: %+v", configs)
	}
	var audited bool
	for _, event := range env.audit.events {
		if event.Action == "workload_stage_config.update" && event.ResourceID == config.ID {
			audited = true
		}
	}
	if !audited {
		t.Fatalf("stage config change should be audited, got %+v", env.audit.events)
	}
	if _, err := env.svc.UpdateApplicationStageState(ctx, UpdateApplicationStageStateInput{ApplicationID: app.ID, StageKey: "dev", Status: "unknown"}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("invalid stage status should fail, got %v", err)
	}
	if _, err := env.svc.DeleteWorkload(ctx, WorkloadStatusInput{Actor: appenvActor(), ApplicationID: app.ID, WorkloadID: workload.ID}); err != nil {
		t.Fatalf("DeleteWorkload() error = %v", err)
	}
	if _, err := env.svc.SaveWorkloadStageConfig(ctx, SaveWorkloadStageConfigInput{Actor: appenvActor(), ApplicationID: app.ID, WorkloadID: workload.ID, StageKey: "dev", Replicas: 1}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("deleted workload should not accept stage config, got %v", err)
	}
}

func TestWorkloadDefaultConfigSaveQueryAndAudit(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	ctx := context.Background()
	app, err := env.svc.CreateApplication(ctx, CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "default-workload", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	workload, err := env.svc.CreateWorkload(ctx, CreateWorkloadInput{Actor: appenvActor(), ApplicationID: app.ID, Name: "api", WorkloadType: WorkloadTypeDeployment})
	if err != nil {
		t.Fatalf("CreateWorkload() error = %v", err)
	}

	config, err := env.svc.SaveWorkloadDefaultConfig(ctx, SaveWorkloadDefaultConfigInput{
		Actor:         appenvActor(),
		ApplicationID: app.ID,
		WorkloadID:    workload.ID,
		Replicas:      2,
		ServicePorts:  []WorkloadServicePort{{Name: "http", Port: 80, TargetPort: 8080, Protocol: "TCP"}},
		EnvVars:       []WorkloadEnvVar{{Name: "SPRING_PROFILES_ACTIVE", Value: "default"}},
		ConfigFiles:   []WorkloadConfigFile{{MountPath: "/etc/app/application.yaml", Content: "spring.profiles.active: default", Base64Encoded: true}},
		WritableDirs:  []WorkloadWritableDir{{MountPath: "/data", OwnerGroup: "app:app", Mode: "0775"}},
	})
	if err != nil {
		t.Fatalf("SaveWorkloadDefaultConfig() error = %v", err)
	}
	got, err := env.svc.GetWorkloadDefaultConfig(ctx, workload.ID)
	if err != nil {
		t.Fatalf("GetWorkloadDefaultConfig() error = %v", err)
	}
	if got.ID != config.ID || got.ConfigFiles[0].Base64Encoded != true || got.WritableDirs[0].OwnerGroup != "app:app" || got.WritableDirs[0].Mode != "0775" {
		t.Fatalf("unexpected default config: %+v", got)
	}
	var audited bool
	for _, event := range env.audit.events {
		if event.Action == "workload_default_config.update" && event.ResourceID == config.ID {
			audited = true
		}
	}
	if !audited {
		t.Fatalf("default config change should be audited, got %+v", env.audit.events)
	}

	if _, err := env.svc.DeleteWorkload(ctx, WorkloadStatusInput{Actor: appenvActor(), ApplicationID: app.ID, WorkloadID: workload.ID}); err != nil {
		t.Fatalf("DeleteWorkload() error = %v", err)
	}
	if _, err := env.svc.SaveWorkloadDefaultConfig(ctx, SaveWorkloadDefaultConfigInput{Actor: appenvActor(), ApplicationID: app.ID, WorkloadID: workload.ID, Replicas: 1}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("deleted workload should not accept default config, got %v", err)
	}
}

func TestCreateWorkloadWithPipelineCreatesPipelineWorkloadAndDefaultConfig(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	ctx := context.Background()
	app, err := env.svc.CreateApplication(ctx, CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "combo-api"})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	result, err := env.svc.CreateWorkloadWithPipeline(ctx, CreateWorkloadWithPipelineInput{
		Actor:    appenvActor(),
		Workload: CreateWorkloadInput{Name: "api", DisplayName: "接口服务", WorkloadType: WorkloadTypeDeployment},
		Pipeline: validPipelineInput("api"),
		DefaultConfig: SaveWorkloadDefaultConfigInput{
			Replicas:     2,
			ServicePorts: []WorkloadServicePort{{Name: "http", Port: 80, TargetPort: 8080}},
			EnvVars:      []WorkloadEnvVar{{Name: "SPRING_PROFILES_ACTIVE", Value: "default"}},
		},
		ApplicationID: app.ID,
	})
	if err != nil {
		t.Fatalf("CreateWorkloadWithPipeline() error = %v", err)
	}
	if result.Pipeline.ID != "pipeline_created" || result.Pipeline.ApplicationID != app.ID {
		t.Fatalf("unexpected pipeline result: %+v", result.Pipeline)
	}
	if result.Workload.ApplicationID != app.ID || result.Workload.PipelineID != result.Pipeline.ID || result.Workload.ImageSourceMode != "pipeline_artifact" {
		t.Fatalf("unexpected workload result: %+v", result.Workload)
	}
	if result.DefaultConfig.WorkloadID != result.Workload.ID || result.DefaultConfig.Replicas != 2 || result.DefaultConfig.ServicePorts[0].TargetPort != 8080 {
		t.Fatalf("unexpected default config result: %+v", result.DefaultConfig)
	}
	if len(env.pipelineCmd.createInputs) != 1 || env.pipelineCmd.createInputs[0].Actor.ID != appenvActor().ID || env.pipelineCmd.createInputs[0].ApplicationID != app.ID {
		t.Fatalf("unexpected pipeline create inputs: %+v", env.pipelineCmd.createInputs)
	}
	if len(env.pipelineCmd.deleteCalls) != 0 {
		t.Fatalf("successful combo create should not clean pipeline, got %+v", env.pipelineCmd.deleteCalls)
	}
}

func TestCreateWorkloadWithPipelineCleansPipelineWhenWorkloadFails(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	ctx := context.Background()
	app, err := env.svc.CreateApplication(ctx, CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "combo-dup"})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	if _, err := env.svc.CreateWorkload(ctx, CreateWorkloadInput{Actor: appenvActor(), ApplicationID: app.ID, Name: "api", WorkloadType: WorkloadTypeDeployment}); err != nil {
		t.Fatalf("CreateWorkload(existing) error = %v", err)
	}
	_, err = env.svc.CreateWorkloadWithPipeline(ctx, CreateWorkloadWithPipelineInput{
		Actor:         appenvActor(),
		ApplicationID: app.ID,
		Workload:      CreateWorkloadInput{Name: "api", WorkloadType: WorkloadTypeDeployment},
		Pipeline:      validPipelineInput("api"),
		DefaultConfig: SaveWorkloadDefaultConfigInput{Replicas: 1},
	})
	if shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate workload should fail with conflict, got %v", err)
	}
	if len(env.pipelineCmd.deleteCalls) != 1 || env.pipelineCmd.deleteCalls[0].pipelineID != "pipeline_created" || env.pipelineCmd.deleteCalls[0].actor.ID != appenvActor().ID {
		t.Fatalf("expected created pipeline cleanup, got %+v", env.pipelineCmd.deleteCalls)
	}
}

func TestCreateWorkloadWithPipelineDeletesWorkloadAndPipelineWhenDefaultConfigFails(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	ctx := context.Background()
	app, err := env.svc.CreateApplication(ctx, CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "combo-config"})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	_, err = env.svc.CreateWorkloadWithPipeline(ctx, CreateWorkloadWithPipelineInput{
		Actor:         appenvActor(),
		ApplicationID: app.ID,
		Workload:      CreateWorkloadInput{Name: "api", WorkloadType: WorkloadTypeDeployment},
		Pipeline:      validPipelineInput("api"),
		DefaultConfig: SaveWorkloadDefaultConfigInput{Replicas: -1},
	})
	if shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("invalid default config should fail, got %v", err)
	}
	if len(env.pipelineCmd.deleteCalls) != 1 || env.pipelineCmd.deleteCalls[0].pipelineID != "pipeline_created" {
		t.Fatalf("expected created pipeline cleanup, got %+v", env.pipelineCmd.deleteCalls)
	}
	workloads, err := env.svc.ListWorkloads(ctx, app.ID)
	if err != nil {
		t.Fatalf("ListWorkloads() error = %v", err)
	}
	if len(workloads) != 0 {
		t.Fatalf("failed combo create should soft-delete created workload, got %+v", workloads)
	}
}

func TestWorkloadStageConfigRequiresApplicationOwnership(t *testing.T) {
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
	workload, err := env.svc.CreateWorkload(ctx, CreateWorkloadInput{Actor: appenvActor(), ApplicationID: firstApp.ID, Name: "api", WorkloadType: WorkloadTypeDeployment})
	if err != nil {
		t.Fatalf("CreateWorkload() error = %v", err)
	}
	if _, err := env.svc.SaveWorkloadStageConfig(ctx, SaveWorkloadStageConfigInput{
		Actor:         appenvActor(),
		ApplicationID: firstApp.ID,
		WorkloadID:    workload.ID,
		StageKey:      "dev",
		Replicas:      1,
	}); err != nil {
		t.Fatalf("SaveWorkloadStageConfig() error = %v", err)
	}
	if _, err := env.svc.GetWorkload(ctx, secondApp.ID, workload.ID); shared.CodeOf(err) != shared.CodeNotFound {
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
}

func TestCreateApplicationPropagatesEventAndIDFailures(t *testing.T) {
	env := newAppenvTestEnv(t, false)
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

func TestApplicationQueriesUpdateDelete(t *testing.T) {
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
	if _, err := env.svc.UpdateApplicationStageState(ctx, UpdateApplicationStageStateInput{ApplicationID: app.ID, StageKey: "dev", Status: ApplicationStageStatusRunning, Message: "运行中"}); err != nil {
		t.Fatalf("UpdateApplicationStageState() error = %v", err)
	}
	if err := env.svc.DeleteApplication(ctx, appenvActor(), app.ID); err != nil {
		t.Fatalf("DeleteApplication() error = %v", err)
	}
	if len(env.cleaner.applicationIDs) != 1 || env.cleaner.applicationIDs[0] != app.ID {
		t.Fatalf("expected manifest cleanup before deletion, got %+v", env.cleaner.applicationIDs)
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
	if _, err := env.svc.ListApplicationStageStates(ctx, app.ID); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("deleted app stage states should be removed with app, got %v", err)
	}
}

func TestDeleteApplicationStopsWhenManifestCleanupFails(t *testing.T) {
	env := newAppenvTestEnv(t, false)
	ctx := context.Background()
	app, err := env.svc.CreateApplication(ctx, CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "api", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}})
	if err != nil {
		t.Fatalf("CreateApplication() error = %v", err)
	}
	env.cleaner.err = shared.NewError(shared.CodeInternal, "gitlab manifest cleanup failed")

	err = env.svc.DeleteApplication(ctx, appenvActor(), app.ID)
	if shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("manifest cleanup failure should block delete, got %v", err)
	}
	if !strings.Contains(err.Error(), "删除应用部署清单失败") {
		t.Fatalf("cleanup failure should use Chinese message, got %v", err)
	}
	if len(env.cleaner.applicationIDs) != 1 || env.cleaner.applicationIDs[0] != app.ID {
		t.Fatalf("expected one manifest cleanup attempt, got %+v", env.cleaner.applicationIDs)
	}
	if len(env.pipelines.deletedIDs) != 0 {
		t.Fatalf("pipeline cleanup should not run after manifest cleanup failure, got %+v", env.pipelines.deletedIDs)
	}
	if _, err := env.svc.GetApplication(ctx, app.ID); err != nil {
		t.Fatalf("application should remain when manifest cleanup fails, got %v", err)
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
	stageState := ApplicationStageState{TenantID: app.TenantID, ProjectID: app.ProjectID, ApplicationID: app.ID, StageKey: "qa", Status: ApplicationStageStatusRunning, Message: "运行中", UpdatedAt: now}
	if err := repo.SaveApplicationStageState(ctx, stageState); err != nil {
		t.Fatalf("SaveApplicationStageState() error = %v", err)
	}
	if err := repo.AppendApplicationStageEvent(ctx, ApplicationStageEvent{ID: "stage_event_1", TenantID: app.TenantID, ProjectID: app.ProjectID, ApplicationID: app.ID, StageKey: "qa", Type: "application_stage_state.updated", Status: ApplicationStageStatusRunning, Message: "运行中", OccurredAt: now}); err != nil {
		t.Fatalf("AppendApplicationStageEvent() error = %v", err)
	}
	stageEvents, err := repo.ListApplicationStageEvents(ctx, app.ID, "qa", shared.PageRequest{})
	if err != nil || stageEvents.Total != 1 {
		t.Fatalf("ListApplicationStageEvents() = %+v, %v", stageEvents, err)
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
	if err := repo.SaveApplicationStageState(ctx, ApplicationStageState{ApplicationID: "missing", StageKey: "dev"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("stage state missing application should fail, got %v", err)
	}
	if _, err := repo.GetApplicationStageState(ctx, "missing", "dev"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing stage state should fail, got %v", err)
	}
	if _, err := repo.ListApplicationStageStatesByApplication(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("list missing stage states should fail, got %v", err)
	}
	if err := repo.AppendApplicationStageEvent(ctx, ApplicationStageEvent{ID: "bad", ApplicationID: "missing", StageKey: "dev"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("stage event missing application should fail, got %v", err)
	}
	if _, err := repo.ListApplicationStageEvents(ctx, "missing", "dev", shared.PageRequest{}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("list missing stage events should fail, got %v", err)
	}
}

func TestHandlerApplicationAndWorkloadFlow(t *testing.T) {
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

	comboBody, _ := json.Marshal(CreateWorkloadWithPipelineInput{
		Actor:    appenvActor(),
		Workload: CreateWorkloadInput{Name: "worker", DisplayName: "后台任务", WorkloadType: WorkloadTypeDeployment},
		Pipeline: validPipelineInput("worker"),
		DefaultConfig: SaveWorkloadDefaultConfigInput{
			Replicas:     1,
			ServicePorts: []WorkloadServicePort{{Name: "http", Port: 80, TargetPort: 8080}},
		},
	})
	comboRec := serveJSON(mux, http.MethodPost, "/api/applications/"+app.ID.String()+"/workloads:create-with-pipeline", comboBody)
	assertStatus(t, comboRec, http.StatusCreated)
	var comboResult CreateWorkloadWithPipelineResult
	if err := json.NewDecoder(comboRec.Body).Decode(&comboResult); err != nil {
		t.Fatalf("decode combo workload: %v", err)
	}
	if comboResult.Workload.Name != "worker" || comboResult.Workload.PipelineID != comboResult.Pipeline.ID || comboResult.DefaultConfig.WorkloadID != comboResult.Workload.ID {
		t.Fatalf("unexpected combo response: %+v", comboResult)
	}

	actorBody, _ := json.Marshal(struct {
		Actor identityaccess.Subject `json:"actor"`
	}{Actor: appenvActor()})
	workloadBody, _ := json.Marshal(CreateWorkloadInput{Actor: appenvActor(), Name: "api", DisplayName: "接口服务", WorkloadType: WorkloadTypeDeployment, ImageSourceMode: "custom_image"})
	workloadRec := serveJSON(mux, http.MethodPost, "/api/applications/"+app.ID.String()+"/workloads", workloadBody)
	assertStatus(t, workloadRec, http.StatusCreated)
	var workload Workload
	if err := json.NewDecoder(workloadRec.Body).Decode(&workload); err != nil {
		t.Fatalf("decode workload: %v", err)
	}
	if workload.ImageSourceMode != "custom_image" {
		t.Fatalf("create workload should return requested image_source_mode, got %+v", workload)
	}
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/applications/"+app.ID.String()+"/workloads", nil), http.StatusOK)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/applications/"+app.ID.String()+"/workloads?enabled=true", nil), http.StatusOK)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/applications/"+app.ID.String()+"/workloads/"+workload.ID.String(), nil), http.StatusOK)
	updateWorkloadBody, _ := json.Marshal(UpdateWorkloadInput{Actor: appenvActor(), DisplayName: "接口服务 v2", WorkloadType: WorkloadTypeDeployment, ImageSourceMode: "mixed"})
	updateWorkloadRec := serveJSON(mux, http.MethodPut, "/api/applications/"+app.ID.String()+"/workloads/"+workload.ID.String(), updateWorkloadBody)
	assertStatus(t, updateWorkloadRec, http.StatusOK)
	var updatedWorkload Workload
	if err := json.NewDecoder(updateWorkloadRec.Body).Decode(&updatedWorkload); err != nil {
		t.Fatalf("decode updated workload: %v", err)
	}
	if updatedWorkload.ImageSourceMode != "mixed" {
		t.Fatalf("update workload should return requested image_source_mode, got %+v", updatedWorkload)
	}
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/applications/"+app.ID.String()+"/workloads/"+workload.ID.String()+":disable", actorBody), http.StatusOK)
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/applications/"+app.ID.String()+"/workloads/"+workload.ID.String()+":enable", actorBody), http.StatusOK)
	defaultConfigPayload, _ := json.Marshal(SaveWorkloadDefaultConfigInput{
		Actor:        appenvActor(),
		Replicas:     2,
		ServicePorts: []WorkloadServicePort{{Name: "http", Port: 80, TargetPort: 8080}},
		ConfigFiles:  []WorkloadConfigFile{{MountPath: "/etc/app/default.yml", Content: "server.port: 8080", Base64Encoded: true}},
		WritableDirs: []WorkloadWritableDir{{MountPath: "/data", OwnerGroup: "app:app", Mode: "0775"}},
	})
	assertStatus(t, serveJSON(mux, http.MethodPut, "/api/applications/"+app.ID.String()+"/workloads/"+workload.ID.String()+"/default-config", defaultConfigPayload), http.StatusOK)
	defaultConfigRec := serveJSON(mux, http.MethodGet, "/api/applications/"+app.ID.String()+"/workloads/"+workload.ID.String()+"/default-config", nil)
	assertStatus(t, defaultConfigRec, http.StatusOK)
	var defaultConfig WorkloadStageConfig
	if err := json.NewDecoder(defaultConfigRec.Body).Decode(&defaultConfig); err != nil {
		t.Fatalf("decode default config: %v", err)
	}
	if !defaultConfig.ConfigFiles[0].Base64Encoded || defaultConfig.WritableDirs[0].OwnerGroup != "app:app" || defaultConfig.WritableDirs[0].Mode != "0775" {
		t.Fatalf("unexpected default config response: %+v", defaultConfig)
	}
	otherBody, _ := json.Marshal(CreateApplicationInput{Actor: appenvActor(), ProjectID: "project_payment", Name: "other-api", Sources: []CreateApplicationSourceInput{validSourceInput("repo_user", validBuildSpec())}})
	otherRec := serveJSON(mux, http.MethodPost, "/api/applications", otherBody)
	assertStatus(t, otherRec, http.StatusCreated)
	var otherApp Application
	if err := json.NewDecoder(otherRec.Body).Decode(&otherApp); err != nil {
		t.Fatalf("decode other app: %v", err)
	}
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/applications/"+otherApp.ID.String()+"/workloads/"+workload.ID.String(), nil), http.StatusNotFound)
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
