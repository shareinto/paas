package build

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
	"github.com/shareinto/paas/internal/shared"
	"github.com/shareinto/paas/internal/shared/testutil"
	"github.com/shareinto/paas/internal/testsupport"
)

type fakeApplicationQuery struct {
	apps        map[shared.ID]ApplicationRef
	sources     map[shared.ID]ApplicationSourceRef
	sourceLists map[shared.ID][]ApplicationSourceRef
}

func (q fakeApplicationQuery) GetApplication(_ context.Context, id shared.ID) (ApplicationRef, error) {
	app, ok := q.apps[id]
	if !ok {
		return ApplicationRef{}, shared.NewError(shared.CodeNotFound, "application not found")
	}
	return app, nil
}

func (q fakeApplicationQuery) GetApplicationSource(_ context.Context, applicationID shared.ID) (ApplicationSourceRef, error) {
	source, ok := q.sources[applicationID]
	if !ok {
		return ApplicationSourceRef{}, shared.NewError(shared.CodeNotFound, "application source not found")
	}
	if source.Key == "" {
		source.Key = "main"
		source.IsPrimary = true
	}
	return source, nil
}

func (q fakeApplicationQuery) ListApplicationSources(ctx context.Context, applicationID shared.ID) ([]ApplicationSourceRef, error) {
	if sources, ok := q.sourceLists[applicationID]; ok {
		return sources, nil
	}
	source, err := q.GetApplicationSource(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	return []ApplicationSourceRef{source}, nil
}

type fakeSourceRepositoryQuery struct {
	repos map[shared.ID]SourceRepositoryRef
}

func (q fakeSourceRepositoryQuery) GetSourceRepository(_ context.Context, id shared.ID) (SourceRepositoryRef, error) {
	repo, ok := q.repos[id]
	if !ok {
		return SourceRepositoryRef{}, shared.NewError(shared.CodeNotFound, "source repository not found")
	}
	return repo, nil
}

type fakeWorkloadQuery struct{ workloads map[shared.ID][]WorkloadRef }

func (q fakeWorkloadQuery) GetWorkload(_ context.Context, applicationID shared.ID, workloadID shared.ID) (WorkloadRef, error) {
	for _, workload := range q.workloads[applicationID] {
		if workload.ID == workloadID {
			return workload, nil
		}
	}
	return WorkloadRef{}, shared.NewError(shared.CodeNotFound, "workload not found")
}

func (q fakeWorkloadQuery) ListEnabledWorkloads(_ context.Context, applicationID shared.ID) ([]WorkloadRef, error) {
	if workloads, ok := q.workloads[applicationID]; ok {
		return workloads, nil
	}
	return nil, shared.NewError(shared.CodeNotFound, "workload not found")
}

func (q fakeWorkloadQuery) ListEnabledWorkloadsByPipeline(_ context.Context, applicationID shared.ID, pipelineID shared.ID) ([]WorkloadRef, error) {
	out := []WorkloadRef{}
	for _, workload := range q.workloads[applicationID] {
		if len(workload.Containers) > 0 {
			for _, container := range workload.Containers {
				if container.PipelineID == pipelineID {
					out = append(out, WorkloadRef{ID: workload.ID, TenantID: workload.TenantID, ProjectID: workload.ProjectID, ApplicationID: workload.ApplicationID, PipelineID: pipelineID, Name: workload.Name, DisplayName: workload.DisplayName, Status: workload.Status, ContainerName: container.Name})
				}
			}
			continue
		}
		if workload.PipelineID == "" || workload.PipelineID == pipelineID {
			out = append(out, workload)
		}
	}
	return out, nil
}

type fakeRunner struct {
	jobs             []BuildJobSpec
	deletedJobs      []string
	triggers         []map[string]string
	triggerErr       error
	jobErr           error
	queue            BuildQueueItem
	queueItems       map[string]BuildQueueItem
	statuses         map[int64]BuildStatus
	logs             []ProgressiveText
	logErr           error
	cancelCalls      []int64
	cancelErr        error
	cancelQueueCalls []string
	cancelQueueErr   error
}

func (r *fakeRunner) EnsureJob(_ context.Context, spec BuildJobSpec) error {
	r.jobs = append(r.jobs, spec)
	return r.jobErr
}

func (r *fakeRunner) DeleteJob(_ context.Context, jobName string) error {
	r.deletedJobs = append(r.deletedJobs, jobName)
	return nil
}

func (r *fakeRunner) TriggerBuild(_ context.Context, _ string, parameters map[string]string) (BuildQueueItem, error) {
	copied := map[string]string{}
	for k, v := range parameters {
		copied[k] = v
	}
	r.triggers = append(r.triggers, copied)
	return r.queue, r.triggerErr
}

func (r *fakeRunner) GetQueueItem(_ context.Context, queueID string) (BuildQueueItem, error) {
	if item, ok := r.queueItems[queueID]; ok {
		return item, nil
	}
	return BuildQueueItem{QueueID: queueID}, nil
}

func (r *fakeRunner) GetBuildStatus(_ context.Context, _ string, buildNumber int64) (BuildStatus, error) {
	if status, ok := r.statuses[buildNumber]; ok {
		return status, nil
	}
	return BuildStatus{BuildNumber: buildNumber, Building: true, Status: BuildRunRunning}, nil
}

func (r *fakeRunner) ProgressiveText(_ context.Context, _ string, _ int64, _ int64) (ProgressiveText, error) {
	if r.logErr != nil {
		return ProgressiveText{}, r.logErr
	}
	if len(r.logs) == 0 {
		return ProgressiveText{}, nil
	}
	next := r.logs[0]
	r.logs = r.logs[1:]
	return next, nil
}

func (r *fakeRunner) CancelBuild(_ context.Context, _ string, buildNumber int64) error {
	r.cancelCalls = append(r.cancelCalls, buildNumber)
	return r.cancelErr
}

func (r *fakeRunner) CancelQueueItem(_ context.Context, queueID string) error {
	r.cancelQueueCalls = append(r.cancelQueueCalls, queueID)
	return r.cancelQueueErr
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

type recordingRuntimeSyncer struct {
	calls []RuntimeEnvironment
	err   error
}

func (s *recordingRuntimeSyncer) SyncRuntimeEnvironment(_ context.Context, _ identityaccess.Subject, environment RuntimeEnvironment) error {
	s.calls = append(s.calls, environment)
	return s.err
}

type buildTestEnv struct {
	svc        *Service
	repo       Repository
	runner     *fakeRunner
	permission *recordingPermission
	audit      *recordingAudit
	events     *recordingPublisher
	syncer     *recordingRuntimeSyncer
}

func newBuildTestEnv(t *testing.T) buildTestEnv {
	t.Helper()
	repo, err := NewMySQLRepository(context.Background(), testsupport.MySQLDB(t, Migrations...))
	if err != nil {
		t.Fatalf("NewMySQLRepository() error = %v", err)
	}
	runner := &fakeRunner{queue: BuildQueueItem{QueueID: "queue-1"}, queueItems: map[string]BuildQueueItem{}, statuses: map[int64]BuildStatus{}}
	permission := &recordingPermission{}
	audit := &recordingAudit{}
	events := &recordingPublisher{}
	syncer := &recordingRuntimeSyncer{}
	svc := NewService(Options{
		Repository: repo,
		ApplicationQuery: fakeApplicationQuery{
			apps: map[shared.ID]ApplicationRef{
				"app_user": {ID: "app_user", TenantID: "tenant_a", ProjectID: "project_payment", TenantName: "rnd", ProjectName: "payment", Name: "user-api"},
			},
			sources: map[shared.ID]ApplicationSourceRef{
				"app_user": {ApplicationID: "app_user", SourceRepositoryID: "repo_user", BuildSpec: validBuildSpec()},
			},
		},
		SourceRepositoryQuery: fakeSourceRepositoryQuery{repos: map[shared.ID]SourceRepositoryRef{
			"repo_user": {ID: "repo_user", HTTPURL: "https://gitlab.example/payment/user-api.git", SSHURL: "git@gitlab.example:payment/user-api.git"},
		}},
		WorkloadQuery:     fakeWorkloadQuery{workloads: map[shared.ID][]WorkloadRef{"app_user": {{ID: "workload_api", TenantID: "tenant_a", ProjectID: "project_payment", ApplicationID: "app_user", Name: "api", DisplayName: "用户接口", Status: "enabled"}}}},
		BuildRunner:       runner,
		PermissionChecker: permission,
		Audit:             audit,
		EventPublisher:    events,
		RuntimeSyncer:     syncer,
		IDGenerator:       testutil.NewFakeIDGenerator(1),
		Clock:             testutil.NewFakeClock(time.Date(2026, 5, 30, 6, 0, 0, 0, time.UTC)),
		TemplateID:        "java-template-v1",
		CallbackURL:       "https://paas.example/api/build-callback",
		ImageRepository:   "registry.example/paas",
		DockerfileRepository: DockerfileRepositoryConfig{
			URL: "git@example.com:platform/dockerfiles.git",
			Ref: "v1",
		},
		SensitiveValues: []string{"literal-secret"},
	})
	return buildTestEnv{svc: svc, repo: repo, runner: runner, permission: permission, audit: audit, events: events, syncer: syncer}
}

type failingRunSourceRepository struct {
	Repository
}

func (r failingRunSourceRepository) CreateRunSource(context.Context, BuildRunSource) error {
	return shared.NewError(shared.CodeUnavailable, "create build run source failed")
}

func (r failingRunSourceRepository) CreateRunWithSources(context.Context, BuildRun, []BuildRunSource) error {
	return shared.NewError(shared.CodeUnavailable, "create build run source failed")
}

func seedRuntimeEnvironments(t *testing.T, env buildTestEnv) {
	t.Helper()
	ctx := context.Background()
	for _, runtime := range []RuntimeEnvironment{
		{
			ID:                 "runtime_env_java17",
			Name:               "java17",
			RuntimeBaseImage:   "registry.example/runtime/java17:1.0",
			ArtifactDeployPath: "/app/",
			DockerfilePath:     "java/jar/Dockerfile",
			SelectorLabels:     map[string]string{"cloud": "aliyun"},
			Images: []RuntimeEnvironmentImage{
				{
					ID:                 "runtime_image_java17_aliyun",
					Name:               "aliyun",
					DisplayName:        "阿里云 JDK 17",
					RuntimeBaseImage:   "registry.example/runtime/java17:1.0",
					ArtifactDeployPath: "/app/",
					DockerfilePath:     "java/jar/Dockerfile",
					SelectorLabels:     map[string]string{"cloud": "aliyun"},
					Status:             string(RuntimeEnvironmentEnabled),
				},
				{
					ID:                 "runtime_image_java17_aws",
					Name:               "aws",
					DisplayName:        "AWS JDK 17",
					RuntimeBaseImage:   "registry.example/runtime/java17-aws:1.0",
					ArtifactDeployPath: "/app/",
					DockerfilePath:     "java/jar/Dockerfile",
					SelectorLabels:     map[string]string{"cloud": "aws"},
					Status:             string(RuntimeEnvironmentEnabled),
				},
			},
			Status:    RuntimeEnvironmentEnabled,
			CreatedBy: "usr_admin",
			CreatedAt: time.Date(2026, 5, 30, 5, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 5, 30, 5, 0, 0, 0, time.UTC),
		},
		{
			ID:                 "runtime_env_java21",
			Name:               "java21",
			RuntimeBaseImage:   "registry.example/runtime/java21:1.0",
			ArtifactDeployPath: "/app/",
			SelectorLabels:     map[string]string{"cloud": "aws"},
			Status:             RuntimeEnvironmentEnabled,
			CreatedBy:          "usr_admin",
			CreatedAt:          time.Date(2026, 5, 30, 5, 0, 0, 0, time.UTC),
			UpdatedAt:          time.Date(2026, 5, 30, 5, 0, 0, 0, time.UTC),
		},
		{
			ID:                 "runtime_env_tomcat8",
			Name:               "tomcat8",
			RuntimeBaseImage:   "registry.example/runtime/tomcat8:1.0",
			ArtifactDeployPath: "/usr/local/tomcat/webapps/",
			DockerfilePath:     "java/tomcat/Dockerfile",
			SelectorLabels:     map[string]string{"cloud": "aliyun"},
			Status:             RuntimeEnvironmentEnabled,
			CreatedBy:          "usr_admin",
			CreatedAt:          time.Date(2026, 5, 30, 5, 0, 0, 0, time.UTC),
			UpdatedAt:          time.Date(2026, 5, 30, 5, 0, 0, 0, time.UTC),
		},
	} {
		if err := env.repo.CreateRuntimeEnvironment(ctx, runtime); err != nil && shared.CodeOf(err) != shared.CodeConflict {
			t.Fatalf("CreateRuntimeEnvironment(%s) error = %v", runtime.ID, err)
		}
	}
}

func validBuildSpec() BuildSpec {
	return BuildSpec{
		SourcePath:          "services/user-api",
		BuildCommand:        "mvn clean package -DskipTests",
		ArtifactCopyCommand: "cp -ar target/user-api.jar \"$PAAS_ARTIFACT_OUTPUT/app.jar\"",
		RuntimeBaseImage:    "registry.example/runtime/java17:1.0",
		ArtifactDeployPath:  "/app/",
		DefaultRef:          "main",
	}
}

func buildActor() identityaccess.Subject {
	return identityaccess.Subject{Type: identityaccess.SubjectUser, ID: "usr_builder"}
}

func createDefaultPipeline(t *testing.T, env buildTestEnv) BuildPipeline {
	t.Helper()
	seedRuntimeEnvironments(t, env)
	pipeline, err := env.svc.CreateBuildPipeline(context.Background(), CreateBuildPipelineInput{
		Actor:                 buildActor(),
		ApplicationID:         "app_user",
		Name:                  "main",
		DisplayName:           "主流水线",
		RuntimeEnvironmentIDs: []shared.ID{"runtime_env_java17"},
		Sources: []BuildPipelineSourceInput{{
			Key:                "main",
			DisplayName:        "主代码源",
			SourceRepositoryID: "repo_user",
			SourcePath:         validBuildSpec().SourcePath,
			BuildSpec:          validBuildSpec(),
			IsPrimary:          true,
		}},
	})
	if err != nil {
		t.Fatalf("CreateBuildPipeline() error = %v", err)
	}
	env.runner.jobs = nil
	env.runner.triggers = nil
	return pipeline
}

func TestCreateBuildPipelineRequiresRuntimeEnvironments(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()

	_, err := env.svc.CreateBuildPipeline(ctx, CreateBuildPipelineInput{
		Actor:         buildActor(),
		ApplicationID: "app_user",
		Name:          "main",
		DisplayName:   "主流水线",
		Sources: []BuildPipelineSourceInput{{
			Key:                "main",
			SourceRepositoryID: "repo_user",
			BuildSpec:          validBuildSpec(),
			IsPrimary:          true,
		}},
	})
	if shared.CodeOf(err) != shared.CodeInvalidArgument || !strings.Contains(err.Error(), "请选择一个运行时环境") {
		t.Fatalf("expected missing runtime environment ids error, got %v", err)
	}
}

func TestCreateBuildPipelinePersistsRuntimeSnapshots(t *testing.T) {
	env := newBuildTestEnv(t)
	seedRuntimeEnvironments(t, env)
	ctx := context.Background()

	pipeline, err := env.svc.CreateBuildPipeline(ctx, CreateBuildPipelineInput{
		Actor:                 buildActor(),
		ApplicationID:         "app_user",
		Name:                  "main",
		DisplayName:           "主流水线",
		RuntimeEnvironmentIDs: []shared.ID{"runtime_env_java17"},
		Sources: []BuildPipelineSourceInput{{
			Key:                "main",
			SourceRepositoryID: "repo_user",
			BuildSpec:          validBuildSpec(),
			IsPrimary:          true,
		}},
	})
	if err != nil {
		t.Fatalf("CreateBuildPipeline() error = %v", err)
	}
	if len(pipeline.RuntimeEnvironments) != 1 || pipeline.RuntimeEnvironments[0].ID != "runtime_env_java17" || len(pipeline.RuntimeEnvironments[0].Images) != 2 {
		t.Fatalf("runtime snapshots should be persisted on pipeline, got %+v", pipeline.RuntimeEnvironments)
	}
	sources, err := env.svc.ListBuildPipelineSources(ctx, pipeline.ID)
	if err != nil {
		t.Fatalf("ListBuildPipelineSources() error = %v", err)
	}
	if sources[0].BuildSpec.RuntimeBaseImage != "registry.example/runtime/java17:1.0" || sources[0].BuildSpec.ArtifactDeployPath != "/app/" {
		t.Fatalf("primary runtime should keep BuildSpec compatibility fields, got %+v", sources[0].BuildSpec)
	}
}

func TestCreateBuildPipelineBackfillsRuntimeSnapshotFromImages(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 30, 5, 0, 0, 0, time.UTC)
	if err := env.repo.CreateRuntimeEnvironment(ctx, RuntimeEnvironment{
		ID:   "runtime_env_images_only",
		Name: "images-only",
		Images: []RuntimeEnvironmentImage{{
			ID:                 "runtime_image_images_only_aliyun",
			Name:               "aliyun",
			DisplayName:        "阿里云 JDK 17",
			RuntimeBaseImage:   "registry.example/runtime/java17:1.0",
			ArtifactDeployPath: "/app/",
			DockerfilePath:     "java/jar/Dockerfile",
			SelectorLabels:     map[string]string{"cloud": "aliyun"},
			Status:             string(RuntimeEnvironmentEnabled),
		}},
		Status:    RuntimeEnvironmentEnabled,
		CreatedBy: "usr_admin",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("CreateRuntimeEnvironment() error = %v", err)
	}
	spec := validBuildSpec()
	spec.RuntimeBaseImage = ""
	spec.ArtifactDeployPath = ""

	pipeline, err := env.svc.CreateBuildPipeline(ctx, CreateBuildPipelineInput{
		Actor:                 buildActor(),
		ApplicationID:         "app_user",
		Name:                  "main",
		DisplayName:           "主流水线",
		RuntimeEnvironmentIDs: []shared.ID{"runtime_env_images_only"},
		Sources: []BuildPipelineSourceInput{{
			Key:                "main",
			SourceRepositoryID: "repo_user",
			BuildSpec:          spec,
			IsPrimary:          true,
		}},
	})
	if err != nil {
		t.Fatalf("CreateBuildPipeline() error = %v", err)
	}
	if got := pipeline.RuntimeEnvironments[0].RuntimeBaseImage; got != "registry.example/runtime/java17:1.0" {
		t.Fatalf("runtime snapshot should be backfilled from first image, got %q", got)
	}
	sources, err := env.svc.ListBuildPipelineSources(ctx, pipeline.ID)
	if err != nil {
		t.Fatalf("ListBuildPipelineSources() error = %v", err)
	}
	if sources[0].BuildSpec.RuntimeBaseImage != "registry.example/runtime/java17:1.0" || sources[0].BuildSpec.ArtifactDeployPath != "/app/" {
		t.Fatalf("BuildSpec should be backfilled from first runtime image, got %+v", sources[0].BuildSpec)
	}
}

func TestCreateBuildPipelineRejectsMultipleRuntimeEnvironments(t *testing.T) {
	env := newBuildTestEnv(t)
	seedRuntimeEnvironments(t, env)
	ctx := context.Background()

	_, err := env.svc.CreateBuildPipeline(ctx, CreateBuildPipelineInput{
		Actor:                 buildActor(),
		ApplicationID:         "app_user",
		Name:                  "main",
		DisplayName:           "主流水线",
		RuntimeEnvironmentIDs: []shared.ID{"runtime_env_java17", "runtime_env_java21"},
		Sources: []BuildPipelineSourceInput{{
			Key:                "main",
			SourceRepositoryID: "repo_user",
			BuildSpec:          validBuildSpec(),
			IsPrimary:          true,
		}},
	})
	if shared.CodeOf(err) != shared.CodeInvalidArgument || !strings.Contains(err.Error(), "请选择一个运行时环境") {
		t.Fatalf("expected single runtime environment error, got %v", err)
	}
}

func TestCreateBuildPipelineAcceptsRuntimeEnvironmentNames(t *testing.T) {
	env := newBuildTestEnv(t)
	seedRuntimeEnvironments(t, env)
	ctx := context.Background()

	pipeline, err := env.svc.CreateBuildPipeline(ctx, CreateBuildPipelineInput{
		Actor:                 buildActor(),
		ApplicationID:         "app_user",
		Name:                  "main",
		DisplayName:           "主流水线",
		RuntimeEnvironmentIDs: []shared.ID{"java17"},
		Sources: []BuildPipelineSourceInput{{
			Key:                "main",
			SourceRepositoryID: "repo_user",
			BuildSpec:          validBuildSpec(),
			IsPrimary:          true,
		}},
	})
	if err != nil {
		t.Fatalf("CreateBuildPipeline() error = %v", err)
	}
	if len(pipeline.RuntimeEnvironments) != 1 || pipeline.RuntimeEnvironments[0].ID != "runtime_env_java17" {
		t.Fatalf("runtime environment name should resolve to persisted id snapshot, got %+v", pipeline.RuntimeEnvironments)
	}
}

func TestBuildPipelineRuntimeEnvironmentJSONUsesConsoleFieldNames(t *testing.T) {
	pipeline := BuildPipeline{
		ID:            "pipeline_1",
		ApplicationID: "app_user",
		Name:          "main",
		RuntimeEnvironments: []RuntimeEnvironmentRef{{
			ID:                 "runtime_env_java17",
			Name:               "java17",
			RuntimeBaseImage:   "registry.example/runtime/java17:1.0",
			ArtifactDeployPath: "/app/",
			DockerfilePath:     "java/jar/Dockerfile",
		}},
	}

	payload, err := json.Marshal(pipeline)
	if err != nil {
		t.Fatalf("Marshal(BuildPipeline) error = %v", err)
	}
	jsonText := string(payload)
	for _, field := range []string{`"id":"runtime_env_java17"`, `"runtime_base_image":"registry.example/runtime/java17:1.0"`, `"artifact_deploy_path":"/app/"`, `"dockerfile_path":"java/jar/Dockerfile"`} {
		if !strings.Contains(jsonText, field) {
			t.Fatalf("runtime snapshot JSON missing %s: %s", field, jsonText)
		}
	}
	if strings.Contains(jsonText, `"ID"`) || strings.Contains(jsonText, `"RuntimeBaseImage"`) {
		t.Fatalf("runtime snapshot JSON should not rely on Go field names: %s", jsonText)
	}
}

func TestMapRuntimeEnvironmentPublicIncludesBuildSpecFieldsOnly(t *testing.T) {
	environment := RuntimeEnvironment{
		ID:                 "runtime_env_java17",
		Name:               "java17",
		RuntimeBaseImage:   "registry.example/runtime/java17:1.0",
		ArtifactDeployPath: "/app/",
		DockerfilePath:     "java/jar/Dockerfile",
		SelectorLabels:     map[string]string{"cloud": "aliyun"},
		Images: []RuntimeEnvironmentImage{{
			Name:             "aliyun",
			RuntimeBaseImage: "registry.example/runtime/java17:1.0",
			SelectorLabels:   map[string]string{"cloud": "aliyun"},
		}},
		Status: RuntimeEnvironmentEnabled,
	}

	payload := mapRuntimeEnvironment(environment, false)
	if payload["runtime_base_image"] != "registry.example/runtime/java17:1.0" || payload["artifact_deploy_path"] != "/app/" {
		t.Fatalf("public runtime environment should include BuildSpec fields, got %+v", payload)
	}
	for _, hidden := range []string{"images", "selector_labels", "dockerfile_path"} {
		if _, ok := payload[hidden]; ok {
			t.Fatalf("public runtime environment should hide %s: %+v", hidden, payload)
		}
	}
}

func TestEnsureDefaultBuildConfigurationDoesNotRecreateDeletedEnvironments(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()
	actor := buildActor()

	if err := env.svc.EnsureDefaultBuildConfiguration(ctx, actor.ID); err != nil {
		t.Fatalf("EnsureDefaultBuildConfiguration() error = %v", err)
	}
	for _, id := range []shared.ID{"build_env_gradle7_jdk11", "build_env_node22"} {
		if err := env.svc.DeleteBuildEnvironment(ctx, actor, id); err != nil {
			t.Fatalf("DeleteBuildEnvironment(%s) error = %v", id, err)
		}
	}
	for _, id := range []shared.ID{"runtime_env_springboot_jdk11", "runtime_env_tomcat_jdk11", "runtime_env_nginx1221"} {
		if err := env.svc.DeleteRuntimeEnvironment(ctx, actor, id); err != nil {
			t.Fatalf("DeleteRuntimeEnvironment(%s) error = %v", id, err)
		}
	}

	if err := env.svc.EnsureDefaultBuildConfiguration(ctx, actor.ID); err != nil {
		t.Fatalf("second EnsureDefaultBuildConfiguration() error = %v", err)
	}
	buildEnvironments, err := env.svc.ListBuildEnvironments(ctx, true, shared.PageRequest{Page: 1, PageSize: 100})
	if err != nil {
		t.Fatalf("ListBuildEnvironments() error = %v", err)
	}
	if buildEnvironments.Total != 0 || len(buildEnvironments.Items) != 0 {
		t.Fatalf("deleted build environments were recreated: %+v", buildEnvironments.Items)
	}
	runtimeEnvironments, err := env.svc.ListRuntimeEnvironments(ctx, true, shared.PageRequest{Page: 1, PageSize: 100})
	if err != nil {
		t.Fatalf("ListRuntimeEnvironments() error = %v", err)
	}
	if runtimeEnvironments.Total != 0 || len(runtimeEnvironments.Items) != 0 {
		t.Fatalf("deleted runtime environments were recreated: %+v", runtimeEnvironments.Items)
	}
}

func TestEnsureDefaultBuildConfigurationSeedsRequestedPresets(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()

	if err := env.svc.EnsureDefaultBuildConfiguration(ctx, buildActor().ID); err != nil {
		t.Fatalf("EnsureDefaultBuildConfiguration() error = %v", err)
	}
	buildEnvironments, err := env.svc.ListBuildEnvironments(ctx, true, shared.PageRequest{Page: 1, PageSize: 100})
	if err != nil {
		t.Fatalf("ListBuildEnvironments() error = %v", err)
	}
	buildImages := map[string]string{}
	for _, environment := range buildEnvironments.Items {
		buildImages[environment.Name] = environment.BuildImage
	}
	wantBuildImages := map[string]string{
		"gradle7-jdk11": "cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg/gradle:7-jdk11",
		"node22":        "cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg/node:22.14.0-bookworm",
	}
	for name, image := range wantBuildImages {
		if buildImages[name] != image {
			t.Fatalf("build environment %s image = %q, want %q; all=%#v", name, buildImages[name], image, buildImages)
		}
	}

	runtimeEnvironments, err := env.svc.ListRuntimeEnvironments(ctx, true, shared.PageRequest{Page: 1, PageSize: 100})
	if err != nil {
		t.Fatalf("ListRuntimeEnvironments() error = %v", err)
	}
	runtimes := map[string]RuntimeEnvironment{}
	for _, environment := range runtimeEnvironments.Items {
		runtimes[environment.Name] = environment
	}
	wantRuntimeImages := map[string]struct {
		images     int
		firstImage string
		dockerfile string
	}{
		"springboot-jdk11": {2, "cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg/dragonwell:11-anolis", "java/jar/Dockerfile"},
		"tomcat-jdk11":     {2, "cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg/tomcat:8.5.87-dragonwell11-anolis", "java/tomcat/Dockerfile"},
		"nginx1221":        {2, "cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg/nginx:1.22.1", "nginx/Dockerfile"},
	}
	for name, want := range wantRuntimeImages {
		got, ok := runtimes[name]
		if !ok {
			t.Fatalf("runtime environment %s missing; all=%#v", name, runtimes)
		}
		if len(got.Images) != want.images || got.RuntimeBaseImage != want.firstImage || got.DockerfilePath != want.dockerfile {
			t.Fatalf("runtime environment %s = images %d image %q dockerfile %q, want %d %q %q", name, len(got.Images), got.RuntimeBaseImage, got.DockerfilePath, want.images, want.firstImage, want.dockerfile)
		}
	}
}

func TestEnsureDefaultBuildConfigurationRefreshesBuiltinTemplate(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()
	oldContent := "pipeline { stages { stage('buildx-push') { steps { sh 'docker buildx build -t {{ .ImageURI }}' } } } }"
	if err := env.repo.SaveBuildTemplate(ctx, BuildTemplate{
		ID:      "global-build-template",
		Name:    "global-build-template",
		Version: 1,
		Content: oldContent,
	}); err != nil {
		t.Fatalf("SaveBuildTemplate() error = %v", err)
	}

	if err := env.svc.EnsureDefaultBuildConfiguration(ctx, buildActor().ID); err != nil {
		t.Fatalf("EnsureDefaultBuildConfiguration() error = %v", err)
	}
	template, err := env.repo.GetBuildTemplate(ctx)
	if err != nil {
		t.Fatalf("GetBuildTemplate() error = %v", err)
	}
	if template.Version != currentDefaultBuildTemplateVersion || template.Content == oldContent {
		t.Fatalf("builtin template should refresh to version %d, got version=%d content=%q", currentDefaultBuildTemplateVersion, template.Version, template.Content)
	}
	if !strings.Contains(template.Content, "artifacts: artifacts") || !strings.Contains(template.Content, "report/image-uri-") {
		t.Fatalf("refreshed template should callback with actual image artifacts, got %s", template.Content)
	}
	if strings.Contains(template.Content, "JsonSlurperClassic") {
		t.Fatalf("refreshed template should avoid Jenkins sandbox JsonSlurper constructor, got %s", template.Content)
	}
}

func TestEnsureDefaultBuildConfigurationRefreshesSandboxBlockedTemplate(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()
	oldContent := "pipeline { post { success { script { def artifacts = []; artifacts << [selector_labels: new groovy.json.JsonSlurperClassic().parseText('{}')]; writeFile file: 'report/callback-success.json', text: groovy.json.JsonOutput.toJson([artifacts: artifacts]) } } } }"
	if err := env.repo.SaveBuildTemplate(ctx, BuildTemplate{
		ID:      "global-build-template",
		Name:    "global-build-template",
		Version: 2,
		Content: oldContent,
	}); err != nil {
		t.Fatalf("SaveBuildTemplate() error = %v", err)
	}

	if err := env.svc.EnsureDefaultBuildConfiguration(ctx, buildActor().ID); err != nil {
		t.Fatalf("EnsureDefaultBuildConfiguration() error = %v", err)
	}
	template, err := env.repo.GetBuildTemplate(ctx)
	if err != nil {
		t.Fatalf("GetBuildTemplate() error = %v", err)
	}
	if template.Version != currentDefaultBuildTemplateVersion || template.Content == oldContent || strings.Contains(template.Content, "JsonSlurperClassic") {
		t.Fatalf("sandbox-blocked builtin template should refresh to version %d without JsonSlurperClassic, got version=%d content=%q", currentDefaultBuildTemplateVersion, template.Version, template.Content)
	}
}

func TestEnsureDefaultBuildConfigurationKeepsCustomTemplate(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()
	customContent := "pipeline { stages { stage('custom') { steps { sh 'echo custom' } } } }"
	if err := env.repo.SaveBuildTemplate(ctx, BuildTemplate{
		ID:      "global-build-template",
		Name:    "global-build-template",
		Version: 3,
		Content: customContent,
	}); err != nil {
		t.Fatalf("SaveBuildTemplate() error = %v", err)
	}

	if err := env.svc.EnsureDefaultBuildConfiguration(ctx, buildActor().ID); err != nil {
		t.Fatalf("EnsureDefaultBuildConfiguration() error = %v", err)
	}
	template, err := env.repo.GetBuildTemplate(ctx)
	if err != nil {
		t.Fatalf("GetBuildTemplate() error = %v", err)
	}
	if template.Version != 3 || template.Content != customContent {
		t.Fatalf("custom template should be kept, got version=%d content=%q", template.Version, template.Content)
	}
}

func TestEnsureDefaultBuildConfigurationKeepsCustomV1Template(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()
	customContent := "pipeline { stages { stage('custom') { steps { sh 'echo custom' } } } }"
	if err := env.repo.SaveBuildTemplate(ctx, BuildTemplate{
		ID:      "global-build-template",
		Name:    "global-build-template",
		Version: 1,
		Content: customContent,
	}); err != nil {
		t.Fatalf("SaveBuildTemplate() error = %v", err)
	}

	if err := env.svc.EnsureDefaultBuildConfiguration(ctx, buildActor().ID); err != nil {
		t.Fatalf("EnsureDefaultBuildConfiguration() error = %v", err)
	}
	template, err := env.repo.GetBuildTemplate(ctx)
	if err != nil {
		t.Fatalf("GetBuildTemplate() error = %v", err)
	}
	if template.Version != 1 || template.Content != customContent {
		t.Fatalf("custom v1 template should be kept, got version=%d content=%q", template.Version, template.Content)
	}
}

func TestUpdateBuildAndRuntimeEnvironmentsEditFields(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()
	actor := buildActor()

	buildEnv, err := env.svc.CreateBuildEnvironment(ctx, CreateBuildEnvironmentInput{
		Actor:       actor,
		Name:        "custom-java",
		Description: "old",
		BuildImage:  "maven:3.9.9-eclipse-temurin-17",
	})
	if err != nil {
		t.Fatalf("CreateBuildEnvironment() error = %v", err)
	}
	updatedBuildEnv, err := env.svc.UpdateBuildEnvironment(ctx, UpdateBuildEnvironmentInput{
		Actor:         actor,
		EnvironmentID: buildEnv.ID,
		Description:   "new",
		BuildImage:    "maven:3.9.9-eclipse-temurin-11",
		Status:        BuildEnvironmentDisabled,
		IsDefault:     true,
	})
	if err != nil {
		t.Fatalf("UpdateBuildEnvironment() error = %v", err)
	}
	if updatedBuildEnv.Name != buildEnv.Name || updatedBuildEnv.BuildImage != "maven:3.9.9-eclipse-temurin-11" || updatedBuildEnv.Status != BuildEnvironmentDisabled || !updatedBuildEnv.IsDefault {
		t.Fatalf("unexpected updated build environment: %+v", updatedBuildEnv)
	}

	runtimeEnv, err := env.svc.CreateRuntimeEnvironment(ctx, CreateRuntimeEnvironmentInput{
		Actor:              actor,
		Name:               "java17-custom",
		RuntimeBaseImage:   "registry.example/runtime/java17:1.0",
		ArtifactDeployPath: "/app/",
		DockerfilePath:     "java/jar/Dockerfile",
		SelectorLabels:     map[string]string{"cloud": "aliyun"},
	})
	if err != nil {
		t.Fatalf("CreateRuntimeEnvironment() error = %v", err)
	}
	updatedRuntimeEnv, err := env.svc.UpdateRuntimeEnvironment(ctx, UpdateRuntimeEnvironmentInput{
		Actor:         actor,
		EnvironmentID: runtimeEnv.ID,
		Description:   "new runtime",
		Images: []RuntimeEnvironmentImage{{
			Name:             "aliyun",
			DisplayName:      "阿里云 JDK 17",
			RuntimeBaseImage: "registry.example/runtime/java17:2.0",
			DockerfilePath:   "java/custom/Dockerfile",
			SelectorLabels:   map[string]string{"cloud": "aliyun"},
			Status:           string(RuntimeEnvironmentEnabled),
		}},
		Status: RuntimeEnvironmentDisabled,
	})
	if err != nil {
		t.Fatalf("UpdateRuntimeEnvironment() error = %v", err)
	}
	if updatedRuntimeEnv.Name != runtimeEnv.Name || updatedRuntimeEnv.RuntimeBaseImage != "registry.example/runtime/java17:2.0" || updatedRuntimeEnv.ArtifactDeployPath != "" || updatedRuntimeEnv.DockerfilePath != "java/custom/Dockerfile" || updatedRuntimeEnv.Status != RuntimeEnvironmentDisabled || len(updatedRuntimeEnv.Images) != 1 {
		t.Fatalf("unexpected updated runtime environment: %+v", updatedRuntimeEnv)
	}
	if len(env.syncer.calls) != 1 || env.syncer.calls[0].ID != runtimeEnv.ID || env.syncer.calls[0].RuntimeBaseImage != "registry.example/runtime/java17:2.0" || env.syncer.calls[0].DockerfilePath != "java/custom/Dockerfile" {
		t.Fatalf("runtime syncer should receive updated environment, got %+v", env.syncer.calls)
	}
	if len(env.audit.events) < 4 || env.audit.events[len(env.audit.events)-1].Action != "runtime_environment.update" {
		t.Fatalf("expected update audit events, got %+v", env.audit.events)
	}
}

func TestTriggerBuildCreatesPipelineAndRendersJenkinsfileWithoutParameters(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()

	pipeline := createDefaultPipeline(t, env)
	run, err := env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID, CommitSHA: "abc123", Version: "v1.0.1"})
	if err != nil {
		t.Fatalf("TriggerBuild() error = %v", err)
	}
	if run.ID == "" || run.Status != BuildRunQueued || run.JenkinsQueueID != "queue-1" {
		t.Fatalf("unexpected run: %+v", run)
	}
	if run.WorkloadID != "" {
		t.Fatalf("build run should not bind workload_id directly, got %+v", run)
	}
	if run.Version != "v1.0.1" {
		t.Fatalf("build run should keep semver version, got %+v", run)
	}
	if len(env.runner.jobs) != 1 || env.runner.jobs[0].JobName != "paas/rnd/payment/user-api/main" || env.runner.jobs[0].TemplateID != "global-build-template" {
		t.Fatalf("unexpected jobs: %+v", env.runner.jobs)
	}
	if !strings.Contains(env.runner.jobs[0].TemplateXML, "org.jenkinsci.plugins.workflow.cps.CpsFlowDefinition") || !strings.Contains(env.runner.jobs[0].TemplateXML, "git@example.com:platform/dockerfiles.git") {
		t.Fatalf("expected generated Pipeline job XML, got %s", env.runner.jobs[0].TemplateXML)
	}
	if !strings.Contains(env.runner.jobs[0].TemplateXML, "docker buildx build") || !strings.Contains(env.runner.jobs[0].TemplateXML, "java/jar/Dockerfile") || !strings.Contains(env.runner.jobs[0].TemplateXML, "curl -fsS -X POST") || strings.Contains(env.runner.jobs[0].TemplateXML, "paas-ci-helper") || strings.Contains(strings.ToLower(env.runner.jobs[0].TemplateXML), "python") {
		t.Fatalf("default Jenkinsfile should use buildx and curl without paas-ci-helper or Python, got %s", env.runner.jobs[0].TemplateXML)
	}
	if strings.Contains(env.runner.jobs[0].TemplateXML, `\(`) || strings.Contains(env.runner.jobs[0].TemplateXML, "æ") {
		t.Fatalf("rendered Jenkinsfile should avoid invalid Groovy escapes and mojibake-prone stage names, got %s", env.runner.jobs[0].TemplateXML)
	}
	if strings.Contains(env.runner.jobs[0].TemplateXML, "PAAS_BUILD_SOURCES") || strings.Contains(env.runner.jobs[0].TemplateXML, "SOURCE_REFS_JSON") || strings.Contains(env.runner.jobs[0].TemplateXML, "PAAS_RUNTIME") || strings.Contains(env.runner.jobs[0].TemplateXML, "PAAS_PACKAGE_SPEC") {
		t.Fatalf("rendered Jenkinsfile should not depend on Jenkins parameters, got %s", env.runner.jobs[0].TemplateXML)
	}
	for _, want := range []string{"abc123", "/api/builds/" + run.ID.String() + "/callback", "registry.example/paas/user-api:", "image_tag_commit", "v1.0.1", "java17-aliyun", "artifacts: artifacts", "report/image-uri-java17-aliyun.txt"} {
		if !strings.Contains(env.runner.jobs[0].TemplateXML, want) {
			t.Fatalf("rendered Jenkinsfile should contain %q for dynamic image callback, got %s", want, env.runner.jobs[0].TemplateXML)
		}
	}
	if !strings.Contains(env.runner.jobs[0].TemplateXML, "report/source-main-commit.txt") || !strings.Contains(env.runner.jobs[0].TemplateXML, "commit_sha") {
		t.Fatalf("rendered Jenkinsfile should report resolved source commit in callback, got %s", env.runner.jobs[0].TemplateXML)
	}
	if strings.Contains(env.runner.jobs[0].TemplateXML, "--metadata-file") || strings.Contains(env.runner.jobs[0].TemplateXML, "image_digest") || strings.Contains(env.runner.jobs[0].TemplateXML, "containerimage.digest") {
		t.Fatalf("rendered Jenkinsfile should not extract image digest, got %s", env.runner.jobs[0].TemplateXML)
	}
	if len(env.runner.triggers) != 1 {
		t.Fatalf("expected trigger call")
	}
	params := env.runner.triggers[0]
	if len(params) != 0 {
		t.Fatalf("Jenkins trigger should not pass parameters, got %+v", params)
	}
	if len(env.permission.calls) != 2 || env.permission.calls[0].action != "build_pipeline:create" || env.permission.calls[1].action != "build:create" || env.permission.calls[1].resource.ApplicationID != "app_user" {
		t.Fatalf("unexpected permission calls: %+v", env.permission.calls)
	}
	lastAudit := env.audit.events[len(env.audit.events)-1]
	if len(env.audit.events) != 2 || lastAudit.Details["build_command"] == "" || lastAudit.Details["runtime_base_image"] == "" || lastAudit.Details["artifact_deploy_path"] == "" || lastAudit.Details["start_command"] != "" {
		t.Fatalf("expected build spec audit details, got %+v", env.audit.events)
	}
	if len(env.events.events) != 1 || env.events.events[0].EventType != "BuildStarted" {
		t.Fatalf("expected BuildStarted, got %+v", env.events.events)
	}
}

func TestTriggerBuildDoesNotLeaveRunWhenRunSourceCreationFails(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()
	pipeline := createDefaultPipeline(t, env)
	env.svc.repo = failingRunSourceRepository{Repository: env.repo}

	_, err := env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID})
	if shared.CodeOf(err) != shared.CodeUnavailable {
		t.Fatalf("TriggerBuild() error code = %s, want unavailable: %v", shared.CodeOf(err), err)
	}
	runs, err := env.repo.ListRunsByApplication(ctx, pipeline.ApplicationID, shared.PageRequest{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListRunsByApplication() error = %v", err)
	}
	if len(runs.Items) != 0 {
		t.Fatalf("failed trigger should not leave build runs, got %+v", runs.Items)
	}
}

func TestCreateNamedPipelinesAndTriggerSelectedPipeline(t *testing.T) {
	env := newBuildTestEnv(t)
	seedRuntimeEnvironments(t, env)
	ctx := context.Background()

	apiPipeline, err := env.svc.CreateBuildPipeline(ctx, CreateBuildPipelineInput{
		Actor:                 buildActor(),
		ApplicationID:         "app_user",
		Name:                  "api",
		DisplayName:           "API 构建",
		RuntimeEnvironmentIDs: []shared.ID{"runtime_env_java17"},
		Sources: []BuildPipelineSourceInput{{
			Key:                "api",
			DisplayName:        "API 源码",
			SourceRepositoryID: "repo_user",
			SourcePath:         "services/user-api",
			BuildSpec:          validBuildSpec(),
			IsPrimary:          true,
		}},
	})
	if err != nil {
		t.Fatalf("CreateBuildPipeline(api) error = %v", err)
	}
	adminPipeline, err := env.svc.CreateBuildPipeline(ctx, CreateBuildPipelineInput{
		Actor:                 buildActor(),
		ApplicationID:         "app_user",
		Name:                  "admin",
		DisplayName:           "管理端构建",
		RuntimeEnvironmentIDs: []shared.ID{"runtime_env_java17"},
		Sources: []BuildPipelineSourceInput{{
			Key:                "admin",
			DisplayName:        "管理端源码",
			SourceRepositoryID: "repo_user",
			SourcePath:         "frontend/admin",
			BuildSpec: BuildSpec{
				SourcePath:          "frontend/admin",
				BuildCommand:        "npm ci && npm run build",
				ArtifactCopyCommand: "cp -ar dist/. \"$PAAS_ARTIFACT_OUTPUT/\"",
				RuntimeBaseImage:    "nginx:1.26",
				ArtifactDeployPath:  "/usr/share/nginx/html/",
				DefaultRef:          "main",
			},
			IsPrimary: true,
		}},
	})
	if err != nil {
		t.Fatalf("CreateBuildPipeline(admin) error = %v", err)
	}
	if apiPipeline.ID == adminPipeline.ID || apiPipeline.Name != "api" || adminPipeline.Name != "admin" {
		t.Fatalf("unexpected pipelines: api=%+v admin=%+v", apiPipeline, adminPipeline)
	}
	if _, err := env.svc.CreateBuildPipeline(ctx, CreateBuildPipelineInput{Actor: buildActor(), ApplicationID: "app_user", Name: "api", RuntimeEnvironmentIDs: []shared.ID{"runtime_env_java17"}, Sources: []BuildPipelineSourceInput{{Key: "api", SourceRepositoryID: "repo_user", BuildSpec: validBuildSpec(), IsPrimary: true}}}); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate pipeline name should conflict, got %v", err)
	}

	run, err := env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: adminPipeline.ID, Sources: []TriggerBuildSourceInput{{Key: "admin", GitRef: "release/admin"}}})
	if err != nil {
		t.Fatalf("TriggerBuild() error = %v", err)
	}
	if run.ApplicationID != "app_user" || run.PipelineID != adminPipeline.ID || run.GitRef != "release/admin" {
		t.Fatalf("build run should use selected pipeline, got %+v", run)
	}
	if len(env.runner.jobs) != 1 || env.runner.jobs[0].JobName != "paas/rnd/payment/user-api/admin" {
		t.Fatalf("unexpected Jenkins job: %+v", env.runner.jobs)
	}
	if !strings.Contains(env.runner.jobs[0].TemplateXML, "frontend/admin") || !strings.Contains(env.runner.jobs[0].TemplateXML, "npm ci &amp;&amp; npm run build") {
		t.Fatalf("pipeline source should drive Jenkinsfile, got %s", env.runner.jobs[0].TemplateXML)
	}
}

func TestApplicationLevelBuildTriggerRequiresPipelineID(t *testing.T) {
	env := newBuildTestEnv(t)

	if _, err := env.svc.TriggerBuild(context.Background(), TriggerBuildInput{Actor: buildActor(), ApplicationID: "app_user"}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("application-level trigger should require pipeline_id, got %v", err)
	}
}

func TestTriggerBuildRendersLegacyTemplateDockerfilePath(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()
	if err := env.repo.SaveBuildTemplate(ctx, BuildTemplate{
		ID:      "global-build-template",
		Name:    "global-build-template",
		Version: 1,
		Content: "pipeline { stages { stage('prepare') { steps { sh 'echo {{ .DockerfilePath }}' } } } }",
	}); err != nil {
		t.Fatalf("SaveBuildTemplate() error = %v", err)
	}

	pipeline := createDefaultPipeline(t, env)
	if _, err := env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID}); err != nil {
		t.Fatalf("TriggerBuild() should render legacy DockerfilePath: %v", err)
	}
	if len(env.runner.jobs) != 1 || !strings.Contains(env.runner.jobs[0].TemplateXML, "java/jar/Dockerfile") {
		t.Fatalf("legacy template should render DockerfilePath, jobs=%+v", env.runner.jobs)
	}
}

func TestTriggerBuildRendersAcceleratedDefaultJenkinsfile(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()

	pipeline := createDefaultPipeline(t, env)
	if _, err := env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID, CommitSHA: "abc123"}); err != nil {
		t.Fatalf("TriggerBuild() error = %v", err)
	}
	if len(env.runner.jobs) != 1 {
		t.Fatalf("expected one Jenkins job, got %+v", env.runner.jobs)
	}
	xml := env.runner.jobs[0].TemplateXML
	for _, want := range []string{
		"git fetch --prune --tags origin",
		"git -C &#34;$checkout_dir&#34; rev-parse --verify --quiet &#34;origin/$ref^{commit}&#34;",
		"commit=&#34;$(git -C &#34;$checkout_dir&#34; rev-parse &#34;origin/$ref^{commit}&#34;)&#34;",
		"git rev-parse --verify --quiet &#34;origin/$ref^{commit}&#34;",
		"commit=&#34;$(git rev-parse &#34;origin/$ref^{commit}&#34;)&#34;",
		"git checkout --detach",
		"git reset --hard",
		"git clean -fdx",
		"git clone --no-checkout",
		"PAAS_CACHE_ROOT=/backup_data/paas-cache",
		"MAVEN_OPTS=&#34;-Dmaven.repo.local=$PAAS_DEP_CACHE/maven/repository",
		"GRADLE_USER_HOME=&#34;$PAAS_DEP_CACHE/gradle&#34;",
		"NPM_CONFIG_CACHE=&#34;$PAAS_DEP_CACHE/npm&#34;",
		"YARN_CACHE_FOLDER=&#34;$PAAS_DEP_CACHE/yarn&#34;",
		"pnpm config set store-dir &#34;$PAAS_DEP_CACHE/pnpm-store&#34;",
		"-v ${PAAS_DEP_CACHE}:${PAAS_DEP_CACHE}",
		"cache_key=$(printf &#39;%s/%s&#39; &#34;${JOB_NAME:-paas}&#34; &#39;main&#39; | sha256sum | awk &#39;{print $1}&#39; | cut -c1-16)",
		"printf &#39;/backup_data/paas-cache/dependencies/%s/main&#39; &#34;$cache_key&#34;",
		"PAAS_DEP_CACHE=${PAAS_DEP_CACHE}",
		"cache_next=&#34;${cache_dir}.next&#34;",
		"--cache-to type=local,dest=&#34;$cache_next&#34;,mode=max",
		"rm -rf &#34;$cache_dir&#34;",
		"mv &#34;$cache_next&#34; &#34;$cache_dir&#34;",
	} {
		if !strings.Contains(xml, want) {
			t.Fatalf("accelerated Jenkinsfile missing %q: %s", want, xml)
		}
	}
	if strings.Contains(xml, "/backup_data/paas-cache/dependencies/paas/rnd/payment/user-api/main") {
		t.Fatalf("dependency cache dir should use a stable flat key instead of raw job path: %s", xml)
	}
	for _, forbidden := range []string{
		"rm -rf source artifact image-context report .paas",
		"--cache-to type=local,dest=&#34;$cache_dir&#34;,mode=max",
		".DependencyCacheDir",
		"git -C &#34;$checkout_dir&#34; rev-parse --verify --quiet &#34;$ref^{commit}&#34;",
		"if git rev-parse --verify --quiet &#34;$ref^{commit}&#34;",
		"JsonSlurperClassic",
		"parseText",
	} {
		if strings.Contains(xml, forbidden) {
			t.Fatalf("accelerated Jenkinsfile should not contain %q: %s", forbidden, xml)
		}
	}
	for _, want := range []string{"selector_labels: [", "metadata: [", "runtime_environment_name"} {
		if !strings.Contains(xml, want) {
			t.Fatalf("accelerated Jenkinsfile should render sandbox-safe artifact maps, missing %q: %s", want, xml)
		}
	}
}

func TestTriggerBuildImageTagDoesNotUseBuildRunID(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()

	pipeline := createDefaultPipeline(t, env)
	if _, err := env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID}); err != nil {
		t.Fatalf("TriggerBuild() error = %v", err)
	}
	if len(env.runner.jobs) != 1 {
		t.Fatalf("expected one Jenkins job, got %+v", env.runner.jobs)
	}
	xml := env.runner.jobs[0].TemplateXML
	if strings.Contains(xml, "registry.example/paas/user-api:20260530-build_run_1-main") {
		t.Fatalf("rendered image tag must not use build run id: %s", xml)
	}
	for _, want := range []string{"image_tag_commit=&#39;main&#39;", "registry.example/paas/user-api:", "java17-aliyun"} {
		if !strings.Contains(xml, want) {
			t.Fatalf("rendered image tag should fall back to git ref when commit is empty, missing %q: %s", want, xml)
		}
	}
}

func TestSaveBuildTemplateAcceptsAcceleratedTemplateWithoutNewViewFields(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()

	if _, err := env.svc.SaveBuildTemplate(ctx, SaveBuildTemplateInput{Actor: buildActor(), Content: defaultBuildTemplateContent}); err != nil {
		t.Fatalf("SaveBuildTemplate() should accept default accelerated template, got %v", err)
	}
}

func TestTriggerBuildFailsWhenDockerfileRepositoryIsNotConfigured(t *testing.T) {
	env := newBuildTestEnv(t)
	env.svc.dockerfileRepo = DockerfileRepositoryConfig{}
	ctx := context.Background()

	pipeline := createDefaultPipeline(t, env)
	if _, err := env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID}); err != nil {
		t.Fatalf("TriggerBuild() error = %v", err)
	}
	xml := env.runner.jobs[0].TemplateXML
	if !strings.Contains(xml, "platform Dockerfile repository is not configured") {
		t.Fatalf("rendered Jenkinsfile should fail clearly when Dockerfile repository is missing: %s", xml)
	}
	if strings.Contains(xml, "generated Dockerfile fallback") || strings.Contains(xml, "cat &gt; &#34;image-context/main/Dockerfile&#34;") || strings.Contains(xml, "FROM ${RUNTIME_BASE}") {
		t.Fatalf("rendered Jenkinsfile should not generate fallback Dockerfile: %s", xml)
	}
}

func TestCreateBuildPipelineDoesNotProvisionJenkinsUntilTriggered(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()

	pipeline := createDefaultPipeline(t, env)
	if pipeline.Name != "main" {
		t.Fatalf("unexpected pipeline: %+v", pipeline)
	}
	if len(env.runner.jobs) != 0 || len(env.runner.triggers) != 0 {
		t.Fatalf("pipeline creation must not touch Jenkins, jobs=%+v triggers=%+v", env.runner.jobs, env.runner.triggers)
	}
	result, err := env.repo.ListPipelinesByApplication(ctx, "app_user", shared.PageRequest{Page: 1, PageSize: 10})
	if err != nil || len(result.Items) != 1 {
		t.Fatalf("pipeline should be persisted, result=%+v err=%v", result, err)
	}
}

func TestListBuildPipelinesReturnsNewestFirst(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()

	createDefaultPipeline(t, env)
	second, err := env.svc.CreateBuildPipeline(ctx, CreateBuildPipelineInput{
		Actor:                 buildActor(),
		ApplicationID:         "app_user",
		Name:                  "release",
		DisplayName:           "发布流水线",
		RuntimeEnvironmentIDs: []shared.ID{"runtime_env_java17"},
		Sources: []BuildPipelineSourceInput{{
			Key:                "main",
			DisplayName:        "主代码源",
			SourceRepositoryID: "repo_user",
			SourcePath:         validBuildSpec().SourcePath,
			BuildSpec:          validBuildSpec(),
			IsPrimary:          true,
		}},
	})
	if err != nil {
		t.Fatalf("CreateBuildPipeline(release) error = %v", err)
	}

	result, err := env.svc.ListBuildPipelines(ctx, "app_user", shared.PageRequest{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListBuildPipelines() error = %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 pipelines, got %+v", result.Items)
	}
	if result.Items[0].ID != second.ID {
		t.Fatalf("newest pipeline should be first, got %+v", result.Items)
	}
}

func TestDeleteBuildPipelineDeletesManagedJenkinsJob(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()
	pipeline := createDefaultPipeline(t, env)
	run, err := env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID})
	if err != nil {
		t.Fatalf("TriggerBuild() error = %v", err)
	}
	if _, err := env.svc.HandleBuildCallback(ctx, BuildCallbackInput{BuildRunID: run.ID, Status: BuildRunSucceeded, JenkinsBuildNumber: 1, ImageURI: "registry.example/paas/user-api:delete"}); err != nil {
		t.Fatalf("finish build before delete: %v", err)
	}
	if err := env.svc.DeleteBuildPipeline(ctx, "app_user"); err != nil {
		t.Fatalf("DeleteBuildPipeline() error = %v", err)
	}
	createdJobName := env.runner.jobs[0].JobName
	if len(env.runner.deletedJobs) != 1 || env.runner.deletedJobs[0] != createdJobName {
		t.Fatalf("unexpected deleted jobs: %+v", env.runner.deletedJobs)
	}
	pipeline, err = env.repo.GetPipeline(ctx, pipeline.ID)
	if err != nil {
		t.Fatalf("GetPipeline() error = %v", err)
	}
	if pipeline.Status != BuildPipelineStatusDisabled {
		t.Fatalf("pipeline should be disabled, got %+v", pipeline)
	}
}

func TestDeleteNamedBuildPipelineRejectsBoundWorkload(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()
	pipeline := createDefaultPipeline(t, env)
	env.svc.SetWorkloadQuery(fakeWorkloadQuery{workloads: map[shared.ID][]WorkloadRef{
		"app_user": {
			{ID: "workload_api", TenantID: "tenant_a", ProjectID: "project_payment", ApplicationID: "app_user", PipelineID: pipeline.ID, Name: "api", DisplayName: "用户接口", Status: "enabled"},
		},
	}})

	err := env.svc.DeleteNamedBuildPipeline(ctx, buildActor(), pipeline.ID)
	if shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("DeleteNamedBuildPipeline() code = %s, err = %v", shared.CodeOf(err), err)
	}
	if len(env.runner.deletedJobs) != 0 {
		t.Fatalf("bound pipeline should not delete Jenkins job, got %+v", env.runner.deletedJobs)
	}
	got, err := env.repo.GetPipeline(ctx, pipeline.ID)
	if err != nil {
		t.Fatalf("GetPipeline() error = %v", err)
	}
	if got.Status != BuildPipelineStatusActive {
		t.Fatalf("bound pipeline should stay active, got %+v", got)
	}
}

func TestEnsureBuildPipelineRendersFlatMultiSourceArtifactJenkinsfile(t *testing.T) {
	env := newBuildTestEnv(t)
	seedRuntimeEnvironments(t, env)
	ctx := context.Background()
	frontSources := []BuildPipelineSourceInput{
		{Key: "frontcomponents", SourceRepositoryID: "repo_frontcomponents", BuildSpec: BuildSpec{
			SourcePath:          ".",
			BuildCommand:        "yarn install && yarn build",
			ArtifactCopyCommand: "cp -ar frontcomponents/. \"$PAAS_ARTIFACT_OUTPUT/\"",
			RuntimeBaseImage:    "nginx:1.26.2",
			DefaultRef:          "main-backup",
		}, IsPrimary: true},
		{Key: "frontmacc5", SourceRepositoryID: "repo_frontmacc5", BuildSpec: BuildSpec{
			SourcePath:          ".",
			BuildCommand:        "make clean && make public_web_static6",
			ArtifactCopyCommand: "cp -ar dist/. \"$PAAS_ARTIFACT_OUTPUT/\"",
			RuntimeBaseImage:    "nginx:1.26.2",
			DefaultRef:          "main",
		}},
	}
	env.svc.SetApplicationQuery(fakeApplicationQuery{
		apps: map[shared.ID]ApplicationRef{
			"app_user": {
				ID:          "app_user",
				TenantID:    "tenant_a",
				ProjectID:   "project_payment",
				TenantName:  "rnd",
				ProjectName: "payment",
				Name:        "macc-frontend",
			},
		},
	})
	env.svc.sourceRepos = fakeSourceRepositoryQuery{repos: map[shared.ID]SourceRepositoryRef{
		"repo_frontcomponents": {ID: "repo_frontcomponents", SSHURL: "git@gitlab.example:macc/basic-component-cloud.git"},
		"repo_frontmacc5":      {ID: "repo_frontmacc5", SSHURL: "git@gitlab.example:macc/macc-cloud.git"},
	}}

	pipeline, err := env.svc.CreateBuildPipeline(ctx, CreateBuildPipelineInput{Actor: buildActor(), ApplicationID: "app_user", Name: "frontend", RuntimeEnvironmentIDs: []shared.ID{"runtime_env_java17"}, Sources: frontSources})
	if err != nil {
		t.Fatalf("CreateBuildPipeline() error = %v", err)
	}
	run, err := env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID, Sources: []TriggerBuildSourceInput{{Key: "frontcomponents", GitRef: "release/a"}, {Key: "frontmacc5", GitRef: "release/b"}}})
	if err != nil {
		t.Fatalf("TriggerBuild() error = %v", err)
	}
	if len(env.runner.jobs) != 1 {
		t.Fatalf("expected one rendered job, got %+v", env.runner.jobs)
	}
	xml := env.runner.jobs[0].TemplateXML
	for _, want := range []string{
		"checkout frontcomponents",
		"checkout frontmacc5",
		"build frontcomponents",
		"build frontmacc5",
		"collect frontcomponents",
		"collect frontmacc5",
		"cp -ar dist/. &#34;$PAAS_ARTIFACT_OUTPUT/&#34;",
		"cp -ar artifact/. &#34;image-context/java17-aliyun/&#34;",
		"cp -ar artifact/. &#34;image-context/java17-aws/&#34;",
		"PAAS_ARTIFACT_OUTPUT",
		"docker buildx build",
		"java/jar/Dockerfile",
		"buildx-push",
	} {
		if !strings.Contains(xml, want) {
			t.Fatalf("rendered Jenkinsfile missing %q: %s", want, xml)
		}
	}
	for _, forbidden := range []string{"paas-package-spec.json", "package_strategy", "nginx_static_bundle", "artifact_path", "PAAS_PRIMARY_ARTIFACT_PATH", "artifact_file", "ARTIFACT_FILE_NAME", "ArtifactFileName", "artifact/${PAAS_PRIMARY_SOURCE_KEY}"} {
		if strings.Contains(xml, forbidden) {
			t.Fatalf("rendered Jenkinsfile should not contain static aggregation marker %q: %s", forbidden, xml)
		}
	}
	if run.SourceRepositoryID != "repo_frontcomponents" || run.GitRef != "release/a" {
		t.Fatalf("primary source should drive compatibility fields, got %+v", run)
	}
	params := env.runner.triggers[0]
	if len(params) != 0 {
		t.Fatalf("Jenkins trigger should not pass parameters, got %+v", params)
	}
	renderedRunXML := env.runner.jobs[len(env.runner.jobs)-1].TemplateXML
	if !strings.Contains(renderedRunXML, "release/a") || !strings.Contains(renderedRunXML, "release/b") {
		t.Fatalf("source refs should be rendered into Jenkinsfile: %s", renderedRunXML)
	}
}

func TestEnsureBuildPipelineRejectsMissingArtifactCopyCommand(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()
	spec := validBuildSpec()
	spec.ArtifactCopyCommand = " "

	seedRuntimeEnvironments(t, env)
	_, err := env.svc.CreateBuildPipeline(ctx, CreateBuildPipelineInput{Actor: buildActor(), ApplicationID: "app_user", Name: "bad", RuntimeEnvironmentIDs: []shared.ID{"runtime_env_java17"}, Sources: []BuildPipelineSourceInput{{Key: "main", SourceRepositoryID: "repo_user", BuildSpec: spec, IsPrimary: true}}})
	if shared.CodeOf(err) != shared.CodeInvalidArgument || !strings.Contains(err.Error(), "artifact_copy_command is required") {
		t.Fatalf("expected missing artifact_copy_command error, got %v", err)
	}
}

func TestTriggerBuildUpdatesPipelineWhenConfigChangesAndSupportsTomcatWar(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()
	pipeline := createDefaultPipeline(t, env)
	firstRun, err := env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID})
	if err != nil {
		t.Fatalf("first TriggerBuild() error = %v", err)
	}
	if _, err := env.svc.HandleBuildCallback(ctx, BuildCallbackInput{BuildRunID: firstRun.ID, Status: BuildRunSucceeded, JenkinsBuildNumber: 1, CommitSHA: "abc123", ImageURI: "registry.example/paas/user-api-main:abc123"}); err != nil {
		t.Fatalf("finish first build: %v", err)
	}
	tomcatSpec := BuildSpec{
		SourcePath:          "apps/legacy-web",
		BuildCommand:        "mvn clean package -DskipTests",
		ArtifactCopyCommand: "cp -ar target/legacy-web.war \"$PAAS_ARTIFACT_OUTPUT/app.war\"",
		RuntimeBaseImage:    "registry.example/runtime/tomcat8:1.0",
		DefaultRef:          "release/1.0",
	}
	if _, err := env.svc.UpdateBuildPipeline(ctx, UpdateBuildPipelineInput{Actor: buildActor(), PipelineID: pipeline.ID, DisplayName: pipeline.DisplayName, RuntimeEnvironmentIDs: []shared.ID{"runtime_env_tomcat8"}, Sources: []BuildPipelineSourceInput{{Key: "main", SourceRepositoryID: "repo_user", SourcePath: tomcatSpec.SourcePath, BuildSpec: tomcatSpec, IsPrimary: true}}}); err != nil {
		t.Fatalf("UpdateBuildPipeline() error = %v", err)
	}
	env.runner.queue = BuildQueueItem{QueueID: "queue-2", BuildNumber: 7}
	run, err := env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID})
	if err != nil {
		t.Fatalf("second TriggerBuild() error = %v", err)
	}
	if run.Status != BuildRunRunning || run.JenkinsBuildNumber != 7 {
		t.Fatalf("queue with build number should mark running, got %+v", run)
	}
	if len(env.runner.jobs) != 2 || len(env.runner.deletedJobs) != 0 || env.runner.jobs[1].JobName != "paas/rnd/payment/user-api/main" {
		t.Fatalf("pipeline should be updated in place, jobs = %+v deleted = %+v", env.runner.jobs, env.runner.deletedJobs)
	}
	params := env.runner.triggers[1]
	if len(params) != 0 {
		t.Fatalf("Jenkins trigger should not pass parameters, got %+v", params)
	}
	if !strings.Contains(env.runner.jobs[1].TemplateXML, "cp -ar target/legacy-web.war") || !strings.Contains(env.runner.jobs[1].TemplateXML, "mvn clean package") || !strings.Contains(env.runner.jobs[1].TemplateXML, "tomcat8") || !strings.Contains(env.runner.jobs[1].TemplateXML, "java/tomcat/Dockerfile") || !strings.Contains(env.runner.jobs[1].TemplateXML, "app.war") || !strings.Contains(env.runner.jobs[1].TemplateXML, "/usr/local/tomcat/webapps") {
		t.Fatalf("tomcat build data should be rendered into Jenkinsfile: %s", env.runner.jobs[1].TemplateXML)
	}
}

func TestTriggerBuildUpdatesPipelineBeforeBuildWhenPreviousRunIsActive(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()
	env.runner.queue = BuildQueueItem{QueueID: "queue-1", BuildNumber: 9}
	pipeline := createDefaultPipeline(t, env)
	firstRun, err := env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID})
	if err != nil {
		t.Fatalf("first TriggerBuild() error = %v", err)
	}
	gradleSpec := BuildSpec{
		SourcePath:          "services/user-api",
		BuildCommand:        "gradle build -x test",
		ArtifactCopyCommand: "cp -ar build/libs/. \"$PAAS_ARTIFACT_OUTPUT/\"",
		RuntimeBaseImage:    "registry.example/runtime/java17:1.0",
		DefaultRef:          "main",
	}
	if _, err := env.svc.UpdateBuildPipeline(ctx, UpdateBuildPipelineInput{Actor: buildActor(), PipelineID: pipeline.ID, DisplayName: pipeline.DisplayName, Sources: []BuildPipelineSourceInput{{Key: "main", SourceRepositoryID: "repo_user", SourcePath: gradleSpec.SourcePath, BuildSpec: gradleSpec, IsPrimary: true}}}); err != nil {
		t.Fatalf("UpdateBuildPipeline() error = %v", err)
	}
	env.runner.queue = BuildQueueItem{QueueID: "queue-2", BuildNumber: 10}
	run, err := env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID})
	if err != nil {
		t.Fatalf("second TriggerBuild() should update existing pipeline before triggering: %v", err)
	}
	storedFirst, _ := env.svc.GetBuildRun(ctx, firstRun.ID)
	if storedFirst.Status != BuildRunRunning || storedFirst.FinishedAt != nil {
		t.Fatalf("old run should remain active, got %+v", storedFirst)
	}
	if run.Status != BuildRunRunning || run.JenkinsBuildNumber != 10 {
		t.Fatalf("new run should be triggered, got %+v", run)
	}
	if len(env.runner.deletedJobs) != 0 || len(env.runner.jobs) != 2 || env.runner.jobs[1].JobName != "paas/rnd/payment/user-api/main" {
		t.Fatalf("pipeline should be updated in place, jobs=%+v deleted=%+v", env.runner.jobs, env.runner.deletedJobs)
	}
}

func TestTriggerBuildExpandsRuntimeEnvironmentImages(t *testing.T) {
	env := newBuildTestEnv(t)
	seedRuntimeEnvironments(t, env)
	ctx := context.Background()
	env.svc.SetApplicationQuery(fakeApplicationQuery{
		apps: map[shared.ID]ApplicationRef{
			"app_user": {
				ID:        "app_user",
				TenantID:  "tenant_a",
				ProjectID: "project_payment",
				Name:      "user-api",
			},
		},
		sources: map[shared.ID]ApplicationSourceRef{
			"app_user": {ApplicationID: "app_user", Key: "main", SourceRepositoryID: "repo_user", BuildSpec: validBuildSpec(), IsPrimary: true},
		},
	})

	pipeline, err := env.svc.CreateBuildPipeline(ctx, CreateBuildPipelineInput{
		Actor:                 buildActor(),
		ApplicationID:         "app_user",
		Name:                  "multi-runtime",
		DisplayName:           "多镜像流水线",
		RuntimeEnvironmentIDs: []shared.ID{"runtime_env_java17"},
		Sources: []BuildPipelineSourceInput{{
			Key:                "main",
			DisplayName:        "主代码源",
			SourceRepositoryID: "repo_user",
			SourcePath:         validBuildSpec().SourcePath,
			BuildSpec:          validBuildSpec(),
			IsPrimary:          true,
		}},
	})
	if err != nil {
		t.Fatalf("CreateBuildPipeline() error = %v", err)
	}
	run, err := env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID, CommitSHA: "abc123"})
	if err != nil {
		t.Fatalf("TriggerBuild() error = %v", err)
	}
	params := env.runner.triggers[0]
	if len(params) != 0 {
		t.Fatalf("Jenkins trigger should not pass parameters, got %+v", params)
	}
	if !strings.Contains(env.runner.jobs[0].TemplateXML, "java17-aliyun") || !strings.Contains(env.runner.jobs[0].TemplateXML, "java17-aws") || !strings.Contains(env.runner.jobs[0].TemplateXML, "java/jar/Dockerfile") {
		t.Fatalf("runtime snapshot should be rendered into Jenkinsfile, got %s", env.runner.jobs[0].TemplateXML)
	}
	if strings.Contains(env.runner.jobs[0].TemplateXML, "PAAS_RUNTIME") {
		t.Fatalf("Jenkinsfile should not depend on PAAS_RUNTIME parameter, got %s", env.runner.jobs[0].TemplateXML)
	}

	succeeded, err := env.svc.HandleBuildCallback(ctx, BuildCallbackInput{BuildRunID: run.ID, Status: BuildRunSucceeded})
	if err != nil || succeeded.Status != BuildRunSucceeded {
		t.Fatalf("HandleBuildCallback() error = %v, run=%+v", err, succeeded)
	}
	artifacts, err := env.svc.ListBuildArtifacts(ctx, run.ID)
	if err != nil || len(artifacts) != 2 {
		t.Fatalf("expected two runtime image artifacts, got %+v, %v", artifacts, err)
	}
	if artifacts[0].URI != "registry.example/paas/user-api:20260530-abc123-v0.0.0" || artifacts[1].URI != "registry.example/paas/user-api:20260530-abc123-v0.0.0" {
		t.Fatalf("unexpected runtime image URIs: %+v", artifacts)
	}
	if !artifacts[0].IsPrimary || artifacts[1].IsPrimary {
		t.Fatalf("first runtime image should be primary, got %+v", artifacts)
	}
	if artifacts[0].SelectorLabels["cloud"] != "aliyun" || artifacts[1].SelectorLabels["cloud"] != "aws" {
		t.Fatalf("runtime image artifacts should keep selector labels, got %+v", artifacts)
	}
}

func TestTriggerBuildValidationAndFailures(t *testing.T) {
	tests := []struct {
		name string
		mut  func(*BuildSpec)
	}{
		{name: "empty command", mut: func(spec *BuildSpec) { spec.BuildCommand = " " }},
		{name: "empty artifact copy command", mut: func(spec *BuildSpec) { spec.ArtifactCopyCommand = " " }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newBuildTestEnv(t)
			seedRuntimeEnvironments(t, env)
			spec := validBuildSpec()
			tt.mut(&spec)
			env.svc.apps = fakeApplicationQuery{
				apps: map[shared.ID]ApplicationRef{"app_user": {ID: "app_user", TenantID: "tenant_a", ProjectID: "project_payment", Name: "user-api"}},
			}
			_, err := env.svc.CreateBuildPipeline(context.Background(), CreateBuildPipelineInput{Actor: buildActor(), ApplicationID: "app_user", Name: "bad", RuntimeEnvironmentIDs: []shared.ID{"runtime_env_java17"}, Sources: []BuildPipelineSourceInput{{Key: "main", SourceRepositoryID: "repo_user", BuildSpec: spec, IsPrimary: true}}})
			if shared.CodeOf(err) != shared.CodeInvalidArgument {
				t.Fatalf("expected invalid_argument, got %v", err)
			}
		})
	}

	env := newBuildTestEnv(t)
	pipeline := createDefaultPipeline(t, env)
	if _, err := env.svc.TriggerBuild(context.Background(), TriggerBuildInput{Actor: identityaccess.Subject{}, PipelineID: pipeline.ID}); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("missing actor should fail, got %v", err)
	}
	env = newBuildTestEnv(t)
	pipeline = createDefaultPipeline(t, env)
	env.permission.err = shared.NewError(shared.CodePermissionDenied, "denied")
	if _, err := env.svc.TriggerBuild(context.Background(), TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID}); shared.CodeOf(err) != shared.CodePermissionDenied {
		t.Fatalf("permission denial should fail, got %v", err)
	}
	env = newBuildTestEnv(t)
	pipeline = createDefaultPipeline(t, env)
	env.runner.jobErr = errors.New("jenkins job failed")
	if _, err := env.svc.TriggerBuild(context.Background(), TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID}); err == nil {
		t.Fatalf("job failure should fail")
	}
	env = newBuildTestEnv(t)
	pipeline = createDefaultPipeline(t, env)
	env.runner.triggerErr = errors.New("jenkins trigger failed")
	if _, err := env.svc.TriggerBuild(context.Background(), TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID}); err == nil {
		t.Fatalf("trigger failure should fail")
	}
	env = newBuildTestEnv(t)
	pipeline = createDefaultPipeline(t, env)
	env.svc.runner = nil
	if _, err := env.svc.TriggerBuild(context.Background(), TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("missing runner should fail, got %v", err)
	}
	env = newBuildTestEnv(t)
	pipeline = createDefaultPipeline(t, env)
	env.svc.apps = nil
	if _, err := env.svc.TriggerBuild(context.Background(), TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("missing app query should fail, got %v", err)
	}
	env = newBuildTestEnv(t)
	pipeline = createDefaultPipeline(t, env)
	env.svc.sourceRepos = nil
	if _, err := env.svc.TriggerBuild(context.Background(), TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("missing source repository query should fail, got %v", err)
	}
	env = newBuildTestEnv(t)
	if _, err := env.svc.TriggerBuild(context.Background(), TriggerBuildInput{Actor: buildActor()}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("missing pipeline_id should fail, got %v", err)
	}
}

func TestQueueSyncLogsCancelAndCallback(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()
	pipeline := createDefaultPipeline(t, env)
	run, err := env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID})
	if err != nil {
		t.Fatalf("TriggerBuild() error = %v", err)
	}
	env.runner.queueItems["queue-1"] = BuildQueueItem{QueueID: "queue-1", BuildNumber: 11, Started: true}
	run, err = env.svc.SyncQueueItem(ctx, run.ID)
	if err != nil {
		t.Fatalf("SyncQueueItem() error = %v", err)
	}
	if run.Status != BuildRunRunning || run.JenkinsBuildNumber != 11 {
		t.Fatalf("unexpected synced run: %+v", run)
	}
	env.runner.logs = []ProgressiveText{
		{Text: "start PAAS_TOKEN=abc literal-secret\n", NextOffset: 32, MoreData: true},
		{Text: "push REGISTRY_PASSWORD=secret\n", NextOffset: 64, MoreData: false},
	}
	events, err := env.svc.StreamBuildLogs(ctx, run.ID)
	if err != nil {
		t.Fatalf("StreamBuildLogs() error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected streamed log chunks, got %+v", events)
	}
	streamed := events[0].Data + events[1].Data
	if strings.Contains(streamed, "abc") || strings.Contains(streamed, "literal-secret") || strings.Contains(streamed, "secret") {
		t.Fatalf("logs should be redacted, got %+v", events)
	}
	stored, _ := env.svc.GetBuildRun(ctx, run.ID)
	if stored.LogOffset != 64 {
		t.Fatalf("expected log offset 64, got %+v", stored)
	}
	history, err := env.svc.BuildLogEvents(ctx, run.ID)
	if err != nil || len(history) < 2 || !strings.Contains(history[0].Data, "start") || !strings.Contains(history[1].Data, "push") {
		t.Fatalf("expected stored build logs, got %+v, %v", history, err)
	}
	run, err = env.svc.HandleBuildCallback(ctx, BuildCallbackInput{BuildRunID: run.ID, Status: BuildRunSucceeded, ImageURI: "registry.example/paas/user-api:abc123", ImageDigest: "sha256:123", CommitSHA: "abc123"})
	if err != nil {
		t.Fatalf("HandleBuildCallback() error = %v", err)
	}
	if run.Status != BuildRunSucceeded || run.PrimaryArtifactID == "" {
		t.Fatalf("expected succeeded run with primary artifact, got %+v", run)
	}
	artifacts, err := env.svc.ListBuildArtifacts(ctx, run.ID)
	if err != nil || len(artifacts) != 1 || !artifacts[0].IsPrimary {
		t.Fatalf("unexpected artifacts: %+v, %v", artifacts, err)
	}
	if artifacts[0].WorkloadID != "" {
		t.Fatalf("artifact should not bind workload_id directly, got %+v", artifacts[0])
	}
	if len(env.events.events) < 2 || env.events.events[len(env.events.events)-1].EventType != "BuildSucceeded" {
		t.Fatalf("expected BuildSucceeded, got %+v", env.events.events)
	}
	var payload BuildSucceededPayload
	if err := json.Unmarshal(env.events.events[len(env.events.events)-1].Payload, &payload); err != nil {
		t.Fatalf("decode BuildSucceeded payload: %v", err)
	}
	if payload.ApplicationID != "app_user" || payload.WorkloadID != "" || len(payload.WorkloadIDs) != 1 || payload.WorkloadIDs[0] != "workload_api" || len(payload.WorkloadTargets) != 1 || payload.WorkloadTargets[0].WorkloadID != "workload_api" || payload.WorkloadTargets[0].ContainerName != "app" || payload.BuildRunID != run.ID || payload.BuildArtifactID != artifacts[0].ID {
		t.Fatalf("BuildSucceeded payload should include app/workload fan-out/run/artifact ids, got %+v", payload)
	}
	again, err := env.svc.HandleBuildCallback(ctx, BuildCallbackInput{BuildRunID: run.ID, Status: BuildRunFailed, ErrorMessage: "late failure"})
	if err != nil || again.Status != BuildRunSucceeded {
		t.Fatalf("callback should be idempotent for terminal run, got %+v, %v", again, err)
	}

	env = newBuildTestEnv(t)
	env.runner.queue = BuildQueueItem{QueueID: "queue-2", BuildNumber: 22}
	pipeline = createDefaultPipeline(t, env)
	cancelRun, _ := env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID})
	cancelRun, err = env.svc.CancelBuild(ctx, buildActor(), cancelRun.ID)
	if err != nil || cancelRun.Status != BuildRunAborted || len(env.runner.cancelCalls) != 1 || env.runner.cancelCalls[0] != 22 {
		t.Fatalf("cancel failed: %+v, calls=%+v, err=%v", cancelRun, env.runner.cancelCalls, err)
	}
}

func TestListBuildRunsRefreshesQueuedRunToRunning(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()
	pipeline := createDefaultPipeline(t, env)
	run, err := env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID})
	if err != nil {
		t.Fatalf("TriggerBuild() error = %v", err)
	}
	if run.Status != BuildRunQueued {
		t.Fatalf("build should start queued, got %+v", run)
	}
	env.runner.queueItems["queue-1"] = BuildQueueItem{QueueID: "queue-1", BuildNumber: 19, Started: true}

	runs, err := env.svc.ListBuildRuns(ctx, pipeline.ApplicationID, shared.PageRequest{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListBuildRuns() error = %v", err)
	}
	if len(runs.Items) == 0 {
		t.Fatalf("expected build run in list")
	}
	if runs.Items[0].ID != run.ID || runs.Items[0].Status != BuildRunRunning || runs.Items[0].JenkinsBuildNumber != 19 {
		t.Fatalf("list should refresh queued run to running, got %+v", runs.Items[0])
	}
}

func TestBuildCallbackRecordsSucceededEventPublishFailure(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()
	env.events.err = errors.New("delivery rejected build event")
	pipeline := createDefaultPipeline(t, env)
	run, err := env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID, CommitSHA: "abc123"})
	if err != nil {
		t.Fatalf("TriggerBuild() error = %v", err)
	}

	succeeded, err := env.svc.HandleBuildCallback(ctx, BuildCallbackInput{BuildRunID: run.ID, Status: BuildRunSucceeded, ImageURI: "registry.example/paas/user-api:abc123"})
	if err != nil {
		t.Fatalf("HandleBuildCallback() should keep Jenkins success even when event publish fails: %v", err)
	}
	if succeeded.Status != BuildRunSucceeded {
		t.Fatalf("build status should stay succeeded, got %+v", succeeded)
	}
	stored, err := env.svc.GetBuildRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetBuildRun() error = %v", err)
	}
	if !strings.Contains(stored.ErrorMessage, "BuildSucceeded event publish failed") || !strings.Contains(stored.ErrorMessage, "delivery rejected build event") {
		t.Fatalf("build run should record event publish failure, got %+v", stored)
	}
}

func TestBuildCallbackDrainsFinalLogsBeforeTerminalStatus(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()
	env.runner.queue = BuildQueueItem{QueueID: "queue-final", BuildNumber: 71}
	pipeline := createDefaultPipeline(t, env)
	run, err := env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID})
	if err != nil {
		t.Fatalf("TriggerBuild() error = %v", err)
	}
	env.runner.logs = []ProgressiveText{
		{Text: "post action one\n", NextOffset: 16, MoreData: true},
		{Text: "post action two\n", NextOffset: 32, MoreData: false},
	}
	if _, err := env.svc.HandleBuildCallback(ctx, BuildCallbackInput{BuildRunID: run.ID, Status: BuildRunFailed, JenkinsBuildNumber: 71, ErrorMessage: "compile failed"}); err != nil {
		t.Fatalf("HandleBuildCallback() error = %v", err)
	}
	events, err := env.svc.BuildLogEvents(ctx, run.ID)
	if err != nil {
		t.Fatalf("BuildLogEvents() error = %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected two final log chunks plus terminal status, got %+v", events)
	}
	if events[0].Event != "log" || !strings.Contains(events[0].Data, "post action one") {
		t.Fatalf("expected first final log before status, got %+v", events)
	}
	if events[1].Event != "log" || !strings.Contains(events[1].Data, "post action two") {
		t.Fatalf("expected second final log before status, got %+v", events)
	}
	if events[2].Event != "status" || events[2].Data != string(BuildRunFailed) {
		t.Fatalf("expected terminal status after final logs, got %+v", events)
	}
	stored, _ := env.svc.GetBuildRun(ctx, run.ID)
	if stored.LogOffset != 32 {
		t.Fatalf("expected final log offset 32, got %+v", stored)
	}
}

func TestCancelQueuedBuildCancelsJenkinsQueueItem(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()
	env.runner.queue = BuildQueueItem{QueueID: "queue-pending"}
	pipeline := createDefaultPipeline(t, env)
	run, err := env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID})
	if err != nil {
		t.Fatalf("TriggerBuild() error = %v", err)
	}

	canceled, err := env.svc.CancelBuild(ctx, buildActor(), run.ID)
	if err != nil {
		t.Fatalf("CancelBuild() error = %v", err)
	}
	if canceled.Status != BuildRunAborted || canceled.FinishedAt == nil {
		t.Fatalf("queued cancel should abort run, got %+v", canceled)
	}
	if len(env.runner.cancelQueueCalls) != 1 || env.runner.cancelQueueCalls[0] != "queue-pending" {
		t.Fatalf("expected queued item to be canceled, calls=%+v", env.runner.cancelQueueCalls)
	}
	if len(env.runner.cancelCalls) != 0 {
		t.Fatalf("queued item without build number should not stop build, calls=%+v", env.runner.cancelCalls)
	}
}

func TestCancelQueuedBuildSyncsStartedQueueItemBeforeStoppingBuild(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()
	env.runner.queue = BuildQueueItem{QueueID: "queue-started"}
	env.runner.queueItems["queue-started"] = BuildQueueItem{QueueID: "queue-started", Started: true, BuildNumber: 55}
	pipeline := createDefaultPipeline(t, env)
	run, err := env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID})
	if err != nil {
		t.Fatalf("TriggerBuild() error = %v", err)
	}

	canceled, err := env.svc.CancelBuild(ctx, buildActor(), run.ID)
	if err != nil {
		t.Fatalf("CancelBuild() error = %v", err)
	}
	if canceled.Status != BuildRunAborted || canceled.JenkinsBuildNumber != 55 {
		t.Fatalf("started queued cancel should persist build number and abort run, got %+v", canceled)
	}
	if len(env.runner.cancelCalls) != 1 || env.runner.cancelCalls[0] != 55 {
		t.Fatalf("expected started queue item to stop Jenkins build, calls=%+v", env.runner.cancelCalls)
	}
	if len(env.runner.cancelQueueCalls) != 0 {
		t.Fatalf("started queue item should not call queue cancel, calls=%+v", env.runner.cancelQueueCalls)
	}
}

func TestTerminalBuildLogStreamWaitsForJenkinsLogCompletion(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()
	env.runner.queue = BuildQueueItem{QueueID: "queue-terminal", BuildNumber: 81}
	pipeline := createDefaultPipeline(t, env)
	run, err := env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID})
	if err != nil {
		t.Fatalf("TriggerBuild() error = %v", err)
	}
	if _, err := env.svc.HandleBuildCallback(ctx, BuildCallbackInput{BuildRunID: run.ID, Status: BuildRunSucceeded, JenkinsBuildNumber: 81}); err != nil {
		t.Fatalf("HandleBuildCallback() error = %v", err)
	}
	env.runner.logs = []ProgressiveText{{MoreData: true}}
	events, err := env.svc.StreamBuildLogs(ctx, run.ID)
	if err != nil {
		t.Fatalf("StreamBuildLogs() pending final logs error = %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("terminal status should wait for Jenkins MoreData=false, got %+v", events)
	}
	env.runner.logs = []ProgressiveText{{Text: "final line\n", NextOffset: 11, MoreData: false}}
	events, err = env.svc.StreamBuildLogs(ctx, run.ID)
	if err != nil {
		t.Fatalf("StreamBuildLogs() final logs error = %v", err)
	}
	if len(events) != 2 || events[0].Event != "log" || events[1].Event != "status" || events[1].Data != string(BuildRunSucceeded) {
		t.Fatalf("expected final log followed by terminal status, got %+v", events)
	}
}

func TestCallbackFailureAndPreconditions(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()
	pipeline := createDefaultPipeline(t, env)
	run, _ := env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID})
	events, err := env.svc.StreamBuildLogs(ctx, run.ID)
	if err != nil || len(events) != 1 || events[0].Event != "status" || events[0].Data != string(BuildRunQueued) {
		t.Fatalf("logs before build number should return queued status, got %+v, %v", events, err)
	}
	if _, err := env.svc.HandleBuildCallback(ctx, BuildCallbackInput{BuildRunID: run.ID, Status: "bad"}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("bad callback status should fail, got %v", err)
	}
	succeeded, err := env.svc.HandleBuildCallback(ctx, BuildCallbackInput{BuildRunID: run.ID, Status: BuildRunSucceeded})
	if err != nil || succeeded.Status != BuildRunSucceeded || succeeded.PrimaryArtifactID == "" {
		t.Fatalf("succeeded callback without image should synthesize artifact, got %+v, %v", succeeded, err)
	}
	artifact, _ := env.repo.GetArtifact(ctx, succeeded.PrimaryArtifactID)
	if artifact.URI != "registry.example/paas/user-api:20260530-main-v0.0.0" {
		t.Fatalf("unexpected synthesized image URI: %+v", artifact)
	}
	run, _ = env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID})
	failed, err := env.svc.HandleBuildCallback(ctx, BuildCallbackInput{BuildRunID: run.ID, Status: BuildRunFailed, ErrorMessage: "compile failed"})
	if err != nil || failed.Status != BuildRunFailed {
		t.Fatalf("failed callback should update run, got %+v, %v", failed, err)
	}
	if env.events.events[len(env.events.events)-1].EventType != "BuildFailed" {
		t.Fatalf("expected BuildFailed event, got %+v", env.events.events)
	}
	if _, err := env.svc.CancelBuild(ctx, buildActor(), failed.ID); err != nil {
		t.Fatalf("cancel terminal run should be idempotent: %v", err)
	}
}

func TestAdditionalServiceBranches(t *testing.T) {
	ctx := context.Background()
	env := newBuildTestEnv(t)
	env.runner.queue = BuildQueueItem{QueueID: "queue-canceled"}
	pipeline := createDefaultPipeline(t, env)
	run, _ := env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID})
	env.runner.queueItems["queue-canceled"] = BuildQueueItem{QueueID: "queue-canceled", Canceled: true}
	synced, err := env.svc.SyncQueueItem(ctx, run.ID)
	if err != nil || synced.Status != BuildRunAborted {
		t.Fatalf("canceled queue should abort run, got %+v, %v", synced, err)
	}
	again, err := env.svc.SyncQueueItem(ctx, synced.ID)
	if err != nil || again.Status != BuildRunAborted {
		t.Fatalf("sync completed queue should be no-op, got %+v, %v", again, err)
	}

	env = newBuildTestEnv(t)
	env.runner.queue = BuildQueueItem{QueueID: "queue-log", BuildNumber: 44}
	pipeline = createDefaultPipeline(t, env)
	run, _ = env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID})
	env.runner.logs = []ProgressiveText{{Text: "bad", NextOffset: -1}}
	if _, err := env.svc.StreamBuildLogs(ctx, run.ID); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("backwards log offset should fail, got %v", err)
	}
	env.runner.logErr = errors.New("log unavailable")
	if _, err := env.svc.StreamBuildLogs(ctx, run.ID); err == nil {
		t.Fatalf("runner log error should fail")
	}

	env = newBuildTestEnv(t)
	env.runner.queue = BuildQueueItem{QueueID: "queue-cancel", BuildNumber: 45}
	env.runner.cancelErr = errors.New("cancel unavailable")
	pipeline = createDefaultPipeline(t, env)
	run, _ = env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID})
	if _, err := env.svc.CancelBuild(ctx, buildActor(), run.ID); err == nil {
		t.Fatalf("cancel runner failure should fail")
	}
	env.permission.err = shared.NewError(shared.CodePermissionDenied, "denied")
	if _, err := env.svc.CancelBuild(ctx, buildActor(), run.ID); shared.CodeOf(err) != shared.CodePermissionDenied {
		t.Fatalf("cancel permission failure should fail, got %v", err)
	}
}

func TestHelpersAndNoopAdapters(t *testing.T) {
	ctx := context.Background()
	if err := (NoopAuditLogger{}).Log(ctx, AuditEvent{}); err != nil {
		t.Fatalf("noop audit should not fail: %v", err)
	}
	if err := (NoopEventPublisher{}).Publish(ctx, shared.DomainEvent{}); err != nil {
		t.Fatalf("noop publisher should not fail: %v", err)
	}
	if got := repoCloneURL(SourceRepositoryRef{HTTPURL: "https://gitlab.example/repo.git", SSHURL: "git@gitlab.example:repo.git"}); got != "git@gitlab.example:repo.git" {
		t.Fatalf("repoCloneURL should prefer SSH URL, got %q", got)
	}
	if got := repoCloneURL(SourceRepositoryRef{HTTPURL: "https://gitlab.example/repo.git"}); got != "https://gitlab.example/repo.git" {
		t.Fatalf("repoCloneURL should fall back to HTTP URL, got %q", got)
	}
	svc := NewService(Options{SensitiveValues: []string{"", "a", "a"}})
	if len(svc.sensitiveValues) != 1 || svc.sensitiveValues[0] != "a" {
		t.Fatalf("sensitive values should be normalized, got %+v", svc.sensitiveValues)
	}
	runner := failingRunner{}
	if _, err := runner.TriggerBuild(ctx, "", nil); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("failing trigger should fail, got %v", err)
	}
	if _, err := runner.GetQueueItem(ctx, "q"); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("failing queue should fail, got %v", err)
	}
	if _, err := runner.ProgressiveText(ctx, "", 1, 0); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("failing logs should fail, got %v", err)
	}
	if err := runner.CancelBuild(ctx, "", 1); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("failing cancel should fail, got %v", err)
	}
	if err := runner.CancelQueueItem(ctx, "q"); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("failing queue cancel should fail, got %v", err)
	}
}

func TestJenkinsfileTemplateValidationAndWrapping(t *testing.T) {
	jenkinsfile := `pipeline { agent any stages { stage('Build') { steps { sh "$BUILD_COMMAND" } } } }`
	if err := validateJenkinsfile(jenkinsfile); err != nil {
		t.Fatalf("Jenkinsfile should be accepted: %v", err)
	}
	if err := validateJenkinsfile("pipeline { environment { PASSWORD=plain } }"); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("plaintext secret should be rejected, got %v", err)
	}
	xml := jenkinsPipelineJobXML(`echo "<tag>"`)
	if strings.Contains(xml, "ParametersDefinitionProperty") || strings.Contains(xml, "PAAS_BUILD_SOURCES") || strings.Contains(xml, "CALLBACK_URL") {
		t.Fatalf("generated job XML should not expose Jenkins parameters: %s", xml)
	}
	if strings.Contains(xml, "BUILD_RUN_ID") || !strings.Contains(xml, "&lt;tag&gt;") || !strings.Contains(xml, "<properties/>") {
		t.Fatalf("generated job XML should escape script and keep empty properties: %s", xml)
	}
}

func TestDeleteJenkinsJobTemplateRejectsReferencedTemplate(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()
	template, err := env.svc.CreateJenkinsJobTemplate(ctx, CreateJenkinsJobTemplateInput{Actor: buildActor(), Name: "custom-java", JenkinsfileContent: defaultJenkinsfile})
	if err != nil {
		t.Fatalf("CreateJenkinsJobTemplate() error = %v", err)
	}
	if err := env.svc.DeleteJenkinsJobTemplate(ctx, buildActor(), template.ID); err != nil {
		t.Fatalf("unreferenced template should delete: %v", err)
	}
	template, err = env.svc.CreateJenkinsJobTemplate(ctx, CreateJenkinsJobTemplateInput{Actor: buildActor(), Name: "used-java", JenkinsfileContent: defaultJenkinsfile})
	if err != nil {
		t.Fatalf("CreateJenkinsJobTemplate() error = %v", err)
	}
	if err := env.repo.CreatePipeline(ctx, BuildPipeline{ID: "build_pipeline_used", TenantID: "tenant_a", ProjectID: "project_payment", ApplicationID: "app_used", Provider: "jenkins", ExternalJobName: "paas/rnd/payment/used", TemplateID: template.ID.String(), Status: BuildPipelineStatusActive, ManagedByPlatform: true}); err != nil {
		t.Fatalf("CreatePipeline() error = %v", err)
	}
	if err := env.svc.DeleteJenkinsJobTemplate(ctx, buildActor(), template.ID); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("referenced template should be rejected, got %v", err)
	}
}

func TestRepositoryAndHandler(t *testing.T) {
	env := newBuildTestEnv(t)
	ctx := context.Background()
	pipeline := createDefaultPipeline(t, env)
	run, err := env.svc.TriggerBuild(ctx, TriggerBuildInput{Actor: buildActor(), PipelineID: pipeline.ID})
	if err != nil {
		t.Fatalf("TriggerBuild() error = %v", err)
	}
	pipeline, err = env.repo.FindPipelineByApplication(ctx, "app_user")
	if err != nil {
		t.Fatalf("FindPipelineByApplication() error = %v", err)
	}
	if err := env.repo.UpdatePipeline(ctx, pipeline); err != nil {
		t.Fatalf("UpdatePipeline() error = %v", err)
	}
	if err := env.repo.CreatePipeline(ctx, pipeline); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate pipeline should conflict, got %v", err)
	}
	changedPipeline := pipeline
	changedPipeline.ApplicationID = "other"
	if err := env.repo.UpdatePipeline(ctx, changedPipeline); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("changing pipeline ownership should fail, got %v", err)
	}
	if err := env.repo.UpdatePipeline(ctx, BuildPipeline{ID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("updating missing pipeline should fail, got %v", err)
	}
	if _, err := env.repo.GetPipeline(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing pipeline should fail, got %v", err)
	}
	if _, err := env.repo.FindPipelineByApplication(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing pipeline by app should fail, got %v", err)
	}
	if err := env.repo.CreateRun(ctx, run); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate run should conflict, got %v", err)
	}
	if err := env.repo.CreateRun(ctx, BuildRun{ID: "bad", PipelineID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("run for missing pipeline should fail, got %v", err)
	}
	if err := env.repo.UpdateRun(ctx, BuildRun{ID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("updating missing run should fail, got %v", err)
	}
	changed := run
	changed.ApplicationID = "other"
	if err := env.repo.UpdateRun(ctx, changed); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("changing run ownership should fail, got %v", err)
	}
	if _, err := env.repo.GetRun(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing run should fail, got %v", err)
	}
	if err := env.repo.CreateArtifact(ctx, BuildArtifact{ID: "bad", BuildRunID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("artifact for missing run should fail, got %v", err)
	}
	if _, err := env.repo.GetArtifact(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing artifact should fail, got %v", err)
	}
	if _, err := env.repo.ListArtifactsByRun(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing artifact list should fail, got %v", err)
	}
	artifact := BuildArtifact{ID: "artifact_1", TenantID: run.TenantID, ProjectID: run.ProjectID, BuildRunID: run.ID, ApplicationID: run.ApplicationID, Type: BuildArtifactImage, Name: "主镜像", URI: "registry.example/image:tag", IsPrimary: true}
	if err := env.repo.CreateArtifact(ctx, artifact); err != nil {
		t.Fatalf("CreateArtifact() error = %v", err)
	}
	if err := env.repo.CreateArtifact(ctx, artifact); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate artifact should conflict, got %v", err)
	}
	if _, err := env.repo.ListRunsByApplication(ctx, "app_user", shared.PageRequest{Page: 99, PageSize: 10}); err != nil {
		t.Fatalf("out of range run list should succeed: %v", err)
	}

	mux := http.NewServeMux()
	NewHandler(env.svc).Register(mux)
	env.runner.queue = BuildQueueItem{QueueID: "queue-http", BuildNumber: 33}
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/apps/app_user/build-pipelines?page=1&page_size=10", nil), http.StatusOK)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/build-pipelines/"+pipeline.ID.String(), nil), http.StatusOK)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/build-pipelines/"+pipeline.ID.String()+"/sources", nil), http.StatusOK)
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/apps/app_user/builds", []byte(`{"actor":{"type":"user","id":"usr_builder"}}`)), http.StatusBadRequest)
	body, _ := json.Marshal(TriggerBuildInput{Actor: buildActor(), GitRef: "main"})
	rec := serveJSON(mux, http.MethodPost, "/api/build-pipelines/"+pipeline.ID.String()+"/builds", body)
	assertStatus(t, rec, http.StatusCreated)
	var httpRun BuildRun
	if err := json.NewDecoder(rec.Body).Decode(&httpRun); err != nil {
		t.Fatalf("decode build run: %v", err)
	}
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/apps/app_user/builds?page=1&page_size=10", nil), http.StatusOK)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/builds/"+httpRun.ID.String(), nil), http.StatusOK)
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/builds/"+httpRun.ID.String()+"/queue-sync", nil), http.StatusOK)
	env.runner.logs = []ProgressiveText{{Text: "ok\nnext\n", NextOffset: 8}}
	cbBody, _ := json.Marshal(BuildCallbackInput{Status: BuildRunSucceeded, ImageURI: "registry.example/paas/user-api:http", ImageDigest: "sha256:http"})
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/builds/"+httpRun.ID.String()+"/callback", cbBody), http.StatusOK)
	logRec := serveJSON(mux, http.MethodGet, "/api/builds/"+httpRun.ID.String()+"/logs/stream", nil)
	assertStatus(t, logRec, http.StatusOK)
	if !strings.Contains(logRec.Body.String(), "event: log") || !strings.Contains(logRec.Body.String(), "data: ok") || !strings.Contains(logRec.Body.String(), "data: next") {
		t.Fatalf("expected SSE log output, got %q", logRec.Body.String())
	}
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/builds/"+httpRun.ID.String()+"/artifacts", nil), http.StatusOK)
	cancelBody, _ := json.Marshal(struct {
		Actor identityaccess.Subject `json:"actor"`
	}{Actor: buildActor()})
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/builds/"+httpRun.ID.String()+"/cancel", cancelBody), http.StatusOK)
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/build-pipelines/"+pipeline.ID.String()+"/builds", []byte("{")), http.StatusBadRequest)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/builds/missing", nil), http.StatusNotFound)
}

func TestHandlerErrorBranches(t *testing.T) {
	env := newBuildTestEnv(t)
	mux := http.NewServeMux()
	NewHandler(env.svc).Register(mux)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/apps/missing/builds", nil), http.StatusNotFound)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/builds/missing/artifacts", nil), http.StatusNotFound)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/builds/missing/logs/stream", nil), http.StatusNotFound)
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/builds/missing/cancel", []byte("{")), http.StatusBadRequest)
	cancelBody, _ := json.Marshal(struct {
		Actor identityaccess.Subject `json:"actor"`
	}{Actor: buildActor()})
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/builds/missing/cancel", cancelBody), http.StatusNotFound)
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/builds/missing/callback", []byte("{")), http.StatusBadRequest)
	cbBody, _ := json.Marshal(BuildCallbackInput{Status: BuildRunFailed})
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/builds/missing/callback", cbBody), http.StatusNotFound)
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/builds/missing/queue-sync", nil), http.StatusNotFound)
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
