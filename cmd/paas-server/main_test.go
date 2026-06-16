package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shareinto/paas/internal/integrations/gitlab"
	"github.com/shareinto/paas/internal/migrations"
	"github.com/shareinto/paas/internal/modules/appenv"
	"github.com/shareinto/paas/internal/modules/build"
	"github.com/shareinto/paas/internal/modules/clusteragent"
	"github.com/shareinto/paas/internal/modules/delivery"
	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/modules/sourcerepository"
	"github.com/shareinto/paas/internal/modules/tenantproject"
	"github.com/shareinto/paas/internal/shared"
	"github.com/shareinto/paas/internal/testsupport"
)

type serverTestFixture struct {
	tenant      tenantproject.Tenant
	project     tenantproject.Project
	source      shared.ID
	application appenv.Application
	workload    appenv.Workload
	pipeline    build.BuildPipeline
	buildRun    build.BuildRun
}

func TestApplicationStartsAndServesDevelopmentAPI(t *testing.T) {
	app := newTestApplication(t)
	defer app.db.Close()
	health := httptest.NewRecorder()
	app.handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health status = %d body = %s", health.Code, health.Body.String())
	}
	ready := httptest.NewRecorder()
	app.handler.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if ready.Code != http.StatusOK || !bytes.Contains(ready.Body.Bytes(), []byte(`"status":"ready"`)) {
		t.Fatalf("ready status = %d body = %s", ready.Code, ready.Body.String())
	}
	metrics := httptest.NewRecorder()
	app.handler.ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if metrics.Code != http.StatusOK || !bytes.Contains(metrics.Body.Bytes(), []byte("paas_applications_total")) || !bytes.Contains(metrics.Body.Bytes(), []byte("paas_ready")) {
		t.Fatalf("metrics status = %d body = %s", metrics.Code, metrics.Body.String())
	}

	seedServerTestData(t, app)
	loginBody := bytes.NewBufferString(`{"account":"admin","password":"password"}`)
	login := httptest.NewRecorder()
	app.handler.ServeHTTP(login, httptest.NewRequest(http.MethodPost, "/api/auth/local/login", loginBody))
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d body = %s", login.Code, login.Body.String())
	}
	var loginPayload struct {
		Token    string `json:"token"`
		UserName string `json:"userName"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &loginPayload); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if loginPayload.Token == "" || loginPayload.UserName != "平台管理员" {
		t.Fatalf("unexpected login response: %#v", loginPayload)
	}

	projects := httptest.NewRecorder()
	app.handler.ServeHTTP(projects, httptest.NewRequest(http.MethodGet, "/api/projects", nil))
	if projects.Code != http.StatusOK {
		t.Fatalf("projects status = %d body = %s", projects.Code, projects.Body.String())
	}
	if !bytes.Contains(projects.Body.Bytes(), []byte("测试项目")) {
		t.Fatalf("projects response missing created project: %s", projects.Body.String())
	}
}

func TestNewApplicationEnsuresDevelopmentAdmin(t *testing.T) {
	app := newTestApplication(t)
	defer app.db.Close()

	loginBody := bytes.NewBufferString(`{"account":"admin","password":"password"}`)
	login := httptest.NewRecorder()
	app.handler.ServeHTTP(login, httptest.NewRequest(http.MethodPost, "/api/auth/local/login", loginBody))
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d body = %s", login.Code, login.Body.String())
	}

	user, err := app.identity.AuthenticateAccessToken(context.Background(), decodeDevelopmentLoginToken(t, login.Body.Bytes()))
	if err != nil {
		t.Fatalf("AuthenticateAccessToken() error = %v", err)
	}
	if err := app.identity.Check(context.Background(), identityaccess.Subject{Type: identityaccess.SubjectUser, ID: user.ID}, identityaccess.ResourceScope{Kind: identityaccess.ScopePlatform}, identityaccess.Permission("runtime:restart")); err != nil {
		t.Fatalf("development admin should have platform permissions: %v", err)
	}
}

func newTestApplication(t *testing.T) *application {
	t.Helper()
	testsupport.ConfigureMySQLEnv(t, migrations.All()...)
	app, err := newApplication(context.Background())
	if err != nil {
		t.Fatalf("newApplication() error = %v", err)
	}
	return app
}

func decodeDevelopmentLoginToken(t *testing.T, body []byte) string {
	t.Helper()
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if payload.Token == "" {
		t.Fatalf("login response missing token: %s", string(body))
	}
	return payload.Token
}

func TestMigrateDatabaseIfEnabledSkipsByDefault(t *testing.T) {
	t.Setenv("PAAS_AUTO_MIGRATE", "")
	t.Setenv("MYSQL_HOST", "127.0.0.1")
	t.Setenv("MYSQL_PORT", "1")
	t.Setenv("MYSQL_USER", "paas")
	t.Setenv("MYSQL_PASSWORD", "bad")
	t.Setenv("MYSQL_DATABASE", "paas")

	if err := migrateDatabaseIfEnabled(context.Background()); err != nil {
		t.Fatalf("migrateDatabaseIfEnabled() should skip when disabled: %v", err)
	}
}

func TestDevelopmentRoutesCoverConsoleContract(t *testing.T) {
	app := newTestApplication(t)
	defer app.db.Close()
	fixture := seedServerTestData(t, app)

	for _, tc := range []struct {
		method string
		path   string
		want   string
	}{
		{http.MethodPost, "/api/auth/oidc/start", "oidc/callback"},
		{http.MethodGet, "/api/applications", "测试服务"},
		{http.MethodGet, "/api/builds", fixture.buildRun.ID.String()},
		{http.MethodGet, "/api/builds/" + fixture.buildRun.ID.String() + "/logs", "构建并推送镜像"},
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(`{}`))
		req.Header.Set("Content-Type", "application/json")
		app.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s status = %d body = %s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
		if !bytes.Contains(rec.Body.Bytes(), []byte(tc.want)) {
			t.Fatalf("%s %s response missing %q: %s", tc.method, tc.path, tc.want, rec.Body.String())
		}
	}

	badLogin := httptest.NewRecorder()
	app.handler.ServeHTTP(badLogin, httptest.NewRequest(http.MethodPost, "/api/auth/local/login", bytes.NewBufferString(`{"account":"admin","password":"bad"}`)))
	if badLogin.Code != http.StatusUnauthorized {
		t.Fatalf("bad login status = %d body = %s", badLogin.Code, badLogin.Body.String())
	}

	options := httptest.NewRecorder()
	app.handler.ServeHTTP(options, httptest.NewRequest(http.MethodOptions, "/api/projects", nil))
	if options.Code != http.StatusNoContent || options.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("unexpected CORS response: status=%d headers=%v", options.Code, options.Header())
	}
}

func TestProjectManagementCreateAndDeleteRoutes(t *testing.T) {
	app := newTestApplication(t)
	defer app.db.Close()
	fixture := seedServerTestData(t, app)

	body := bytes.NewBufferString(`{"actor":{"type":"user","id":"usr_admin"},"tenant_id":"` + fixture.tenant.ID.String() + `","name":"empty-project","display_name":"空项目","description":"临时验证项目"}`)
	createRec := httptest.NewRecorder()
	app.handler.ServeHTTP(createRec, httptest.NewRequest(http.MethodPost, "/api/projects", body))
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create project status = %d body = %s", createRec.Code, createRec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created project: %v", err)
	}
	if created.ID == "" {
		t.Fatalf("created project id is empty: %s", createRec.Body.String())
	}

	listRec := httptest.NewRecorder()
	app.handler.ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/api/projects", nil))
	if listRec.Code != http.StatusOK || !bytes.Contains(listRec.Body.Bytes(), []byte("空项目")) || !bytes.Contains(listRec.Body.Bytes(), []byte("测试租户")) {
		t.Fatalf("projects response = %d %s", listRec.Code, listRec.Body.String())
	}

	deleteNonEmpty := httptest.NewRecorder()
	app.handler.ServeHTTP(deleteNonEmpty, httptest.NewRequest(http.MethodDelete, "/api/projects/"+fixture.project.ID.String(), bytes.NewBufferString(`{"actor":{"type":"user","id":"usr_admin"}}`)))
	if deleteNonEmpty.Code != http.StatusNoContent {
		t.Fatalf("non-empty project delete should cascade, status = %d body = %s", deleteNonEmpty.Code, deleteNonEmpty.Body.String())
	}
	if _, err := app.state.appRepo.GetApplication(context.Background(), fixture.application.ID); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("project delete should remove applications, got %v", err)
	}
	if _, err := app.state.sourceRepo.GetSourceRepository(context.Background(), fixture.source); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("project delete should remove source repositories, got %v", err)
	}
	if _, err := app.state.buildRepo.FindPipelineByApplication(context.Background(), fixture.application.ID); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("project delete should leave no active Jenkins pipeline, got %v", err)
	}

	associated := seedServerTestDataNamed(t, app, "assoc")
	if err := app.state.sourceRepo.SetAssociatedApplications(context.Background(), associated.source, []sourcerepository.AssociatedApplication{{ID: associated.application.ID, Name: associated.application.Name}}); err != nil {
		t.Fatalf("set associated applications: %v", err)
	}
	deleteAssociated := httptest.NewRecorder()
	app.handler.ServeHTTP(deleteAssociated, httptest.NewRequest(http.MethodDelete, "/api/projects/"+associated.project.ID.String(), bytes.NewBufferString(`{"actor":{"type":"user","id":"usr_admin"}}`)))
	if deleteAssociated.Code != http.StatusNoContent {
		t.Fatalf("associated project delete should cascade, status = %d body = %s", deleteAssociated.Code, deleteAssociated.Body.String())
	}

	deleteRec := httptest.NewRecorder()
	app.handler.ServeHTTP(deleteRec, httptest.NewRequest(http.MethodDelete, "/api/projects/"+created.ID, bytes.NewBufferString(`{"actor":{"type":"user","id":"usr_admin"}}`)))
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("delete project status = %d body = %s", deleteRec.Code, deleteRec.Body.String())
	}

	getRec := httptest.NewRecorder()
	app.handler.ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/api/projects/"+created.ID, nil))
	if getRec.Code != http.StatusNotFound {
		t.Fatalf("deleted project should be not found, status = %d body = %s", getRec.Code, getRec.Body.String())
	}
}

func TestTenantManagementUpdateRoute(t *testing.T) {
	app := newTestApplication(t)
	defer app.db.Close()
	fixture := seedServerTestData(t, app)

	body := bytes.NewBufferString(`{"actor":{"type":"user","id":"usr_admin"},"display_name":"平台租户","description":"用于平台项目"}`)
	updateRec := httptest.NewRecorder()
	app.handler.ServeHTTP(updateRec, httptest.NewRequest(http.MethodPatch, "/api/tenants/"+fixture.tenant.ID.String(), body))
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update tenant status = %d body = %s", updateRec.Code, updateRec.Body.String())
	}
	if !bytes.Contains(updateRec.Body.Bytes(), []byte("平台租户")) || !bytes.Contains(updateRec.Body.Bytes(), []byte("用于平台项目")) {
		t.Fatalf("tenant update response missing updated fields: %s", updateRec.Body.String())
	}

	listRec := httptest.NewRecorder()
	app.handler.ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/api/tenants", nil))
	if listRec.Code != http.StatusOK || !bytes.Contains(listRec.Body.Bytes(), []byte("平台租户")) {
		t.Fatalf("tenants response = %d %s", listRec.Code, listRec.Body.String())
	}
}

func TestManifestRepositoriesFromEnvUseRealGitLabWhenConfigured(t *testing.T) {
	t.Setenv("GITLAB_BASE_URL", "https://gitlab.example")
	t.Setenv("GITLAB_TOKEN", "secret")
	t.Setenv("GITLAB_MANIFEST_PROJECT_ID", "99")
	t.Setenv("GITLAB_CHART_PROJECT_ID", "100")
	manifest, chart, manifestURL, chartURL := manifestRepositoriesFromEnv()
	if _, ok := manifest.(*gitlab.ManifestRepositoryAdapter); !ok {
		t.Fatalf("manifest repo should use real GitLab adapter, got %T", manifest)
	}
	if _, ok := chart.(*gitlab.ManifestRepositoryAdapter); !ok {
		t.Fatalf("chart repo should use real GitLab adapter, got %T", chart)
	}
	if manifestURL != "https://gitlab.example/99.git" || chartURL != "https://gitlab.example/100.git" {
		t.Fatalf("unexpected repo urls manifest=%q chart=%q", manifestURL, chartURL)
	}
}

func TestManifestRepositoriesFromEnvUseSeparateGitOpsGitLabWhenConfigured(t *testing.T) {
	t.Setenv("GITLAB_BASE_URL", "https://source-gitlab.example")
	t.Setenv("GITLAB_TOKEN", "source-secret")
	t.Setenv("GITOPS_GITLAB_BASE_URL", "https://gitops-gitlab.example")
	t.Setenv("GITOPS_GITLAB_TOKEN", "gitops-secret")
	t.Setenv("GITLAB_MANIFEST_PROJECT_ID", "99")
	t.Setenv("GITLAB_CHART_PROJECT_ID", "100")

	manifest, chart, manifestURL, chartURL := manifestRepositoriesFromEnv()
	if _, ok := manifest.(*gitlab.ManifestRepositoryAdapter); !ok {
		t.Fatalf("manifest repo should use real GitLab adapter, got %T", manifest)
	}
	if _, ok := chart.(*gitlab.ManifestRepositoryAdapter); !ok {
		t.Fatalf("chart repo should use real GitLab adapter, got %T", chart)
	}
	if manifestURL != "https://gitops-gitlab.example/99.git" || chartURL != "https://gitops-gitlab.example/100.git" {
		t.Fatalf("gitops repo urls should use separate gitops gitlab, manifest=%q chart=%q", manifestURL, chartURL)
	}
}

func TestManifestRepositoriesFromEnvFallbackToFakeWhenUnconfigured(t *testing.T) {
	t.Setenv("GITLAB_BASE_URL", "")
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("GITLAB_MANIFEST_PROJECT_ID", "")
	t.Setenv("GITLAB_CHART_PROJECT_ID", "")
	manifest, chart, _, _ := manifestRepositoriesFromEnv()
	if _, ok := manifest.(*gitlab.FakeManifestRepositoryAdapter); !ok {
		t.Fatalf("manifest repo should use fake adapter, got %T", manifest)
	}
	if _, ok := chart.(*gitlab.FakeManifestRepositoryAdapter); !ok {
		t.Fatalf("chart repo should use fake adapter, got %T", chart)
	}
}

func TestModuleRoutesUseWiredPorts(t *testing.T) {
	app := newTestApplication(t)
	defer app.db.Close()
	fixture := seedServerTestData(t, app)

	body := bytes.NewBufferString(`{"actor":{"type":"user","id":"usr_admin"},"git_ref":"main","commit_sha":"abc123"}`)
	buildRec := httptest.NewRecorder()
	app.handler.ServeHTTP(buildRec, httptest.NewRequest(http.MethodPost, "/api/build-pipelines/"+fixture.pipeline.ID.String()+"/builds", body))
	if buildRec.Code != http.StatusCreated {
		t.Fatalf("trigger build status = %d body = %s", buildRec.Code, buildRec.Body.String())
	}

	templateRec := httptest.NewRecorder()
	app.handler.ServeHTTP(templateRec, httptest.NewRequest(http.MethodGet, "/api/apps/"+fixture.application.ID.String()+"/deployment-template", nil))
	if templateRec.Code != http.StatusOK || !bytes.Contains(templateRec.Body.Bytes(), []byte("init-data")) {
		t.Fatalf("template response = %d %s", templateRec.Code, templateRec.Body.String())
	}

	stagesRec := httptest.NewRecorder()
	app.handler.ServeHTTP(stagesRec, httptest.NewRequest(http.MethodGet, "/api/apps/"+fixture.application.ID.String()+"/stages", nil))
	if stagesRec.Code != http.StatusOK || !bytes.Contains(stagesRec.Body.Bytes(), []byte("dev")) {
		t.Fatalf("stages response = %d %s", stagesRec.Code, stagesRec.Body.String())
	}
}

func TestBuildCallbackCreatesFreightReleaseCandidateThroughWiredEvent(t *testing.T) {
	app := newTestApplication(t)
	defer app.db.Close()
	fixture := seedServerTestData(t, app)
	ctx := context.Background()
	actor := identityaccess.Subject{Type: identityaccess.SubjectUser, ID: "usr_admin"}

	run, err := app.builds.TriggerBuild(ctx, build.TriggerBuildInput{Actor: actor, PipelineID: fixture.pipeline.ID, GitRef: "main"})
	if err != nil {
		t.Fatalf("trigger build: %v", err)
	}
	if _, err := app.builds.HandleBuildCallback(ctx, build.BuildCallbackInput{BuildRunID: run.ID, Status: build.BuildRunSucceeded, JenkinsBuildNumber: 7, ImageURI: "registry.local/test-api:v2"}); err != nil {
		t.Fatalf("finish build: %v", err)
	}

	contextRec := httptest.NewRecorder()
	app.handler.ServeHTTP(contextRec, httptest.NewRequest(http.MethodGet, "/api/apps/"+fixture.application.ID.String()+"/freights/creation-context", nil))
	if contextRec.Code != http.StatusOK {
		t.Fatalf("creation context status = %d body = %s", contextRec.Code, contextRec.Body.String())
	}
	var contextOut delivery.FreightCreationContext
	if err := json.Unmarshal(contextRec.Body.Bytes(), &contextOut); err != nil {
		t.Fatalf("decode creation context: %v", err)
	}
	contextRelease := contextOut.LatestReleasesByWorkload[fixture.workload.ID]
	if contextRelease.ID.IsZero() || contextRelease.WorkloadID != fixture.workload.ID {
		t.Fatalf("creation context should expose workload release candidate, got %+v in context %+v", contextRelease, contextOut.LatestReleasesByWorkload)
	}
	if contextRelease.ImageURI != "registry.local/test-api:v2" || contextRelease.ImageDigest != "" {
		t.Fatalf("creation context should expose latest callback release without digest, got %+v", contextRelease)
	}
	release, err := app.state.deliveryRepo.FindReleaseByBuildRunAndWorkload(ctx, run.ID, fixture.workload.ID)
	if err != nil {
		t.Fatalf("find release created from build callback: %v", err)
	}
	if release.ImageURI != "registry.local/test-api:v2" || release.ImageDigest != "" || release.PipelineID != fixture.pipeline.ID {
		t.Fatalf("build callback should create workload release candidate from wired event, got %+v", release)
	}
}

func TestTriggerBuildCreatesManagedJenkinsPipeline(t *testing.T) {
	app := newTestApplication(t)
	defer app.db.Close()
	fixture := seedServerTestData(t, app)

	repoBody := bytes.NewBufferString(`{"actor":{"type":"user","id":"usr_admin"},"project_id":"` + fixture.project.ID.String() + `","name":"invoice-api","display_name":"发票服务仓库","default_branch":"main"}`)
	repoRec := httptest.NewRecorder()
	app.handler.ServeHTTP(repoRec, httptest.NewRequest(http.MethodPost, "/api/source-repositories", repoBody))
	if repoRec.Code != http.StatusCreated {
		t.Fatalf("create source repository status = %d body = %s", repoRec.Code, repoRec.Body.String())
	}
	var source struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(repoRec.Body.Bytes(), &source); err != nil {
		t.Fatalf("decode source repository response: %v", err)
	}

	appBody, _ := json.Marshal(appenv.CreateApplicationInput{
		Actor:       identityaccess.Subject{Type: identityaccess.SubjectUser, ID: "usr_admin"},
		ProjectID:   fixture.project.ID,
		Name:        "invoice-api",
		DisplayName: "发票服务",
		Sources: []appenv.CreateApplicationSourceInput{{Key: "main", SourceRepositoryID: shared.ID(source.ID), IsPrimary: true, BuildSpec: appenv.BuildSpec{
			SourcePath:          ".",
			BuildCommand:        "mvn clean package -DskipTests",
			ArtifactCopyCommand: "cp -ar target/invoice-api.jar \"$PAAS_ARTIFACT_OUTPUT/app.jar\"",
			RuntimeBaseImage:    "registry.example/runtime/java17:1.0",
			DefaultRef:          "main",
		}}},
	})
	appRec := httptest.NewRecorder()
	app.handler.ServeHTTP(appRec, httptest.NewRequest(http.MethodPost, "/api/applications", bytes.NewReader(appBody)))
	if appRec.Code != http.StatusCreated {
		t.Fatalf("create application status = %d body = %s", appRec.Code, appRec.Body.String())
	}
	var created appenv.Application
	if err := json.Unmarshal(appRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode application response: %v", err)
	}
	workload, err := app.apps.CreateWorkload(context.Background(), appenv.CreateWorkloadInput{Actor: identityaccess.Subject{Type: identityaccess.SubjectUser, ID: "usr_admin"}, ApplicationID: created.ID, Name: "api", DisplayName: "发票服务", WorkloadType: appenv.WorkloadTypeDeployment})
	if err != nil {
		t.Fatalf("create workload: %v", err)
	}
	pipeline, err := app.builds.CreateBuildPipeline(context.Background(), build.CreateBuildPipelineInput{
		Actor:         identityaccess.Subject{Type: identityaccess.SubjectUser, ID: "usr_admin"},
		ApplicationID: created.ID,
		Name:          "main",
		DisplayName:   "主流水线",
		RuntimeEnvironmentIDs: []shared.ID{
			"runtime_env_springboot_jdk11_aliyun",
		},
		Sources: []build.BuildPipelineSourceInput{{Key: "main", SourceRepositoryID: shared.ID(source.ID), SourcePath: ".", IsPrimary: true, BuildSpec: build.BuildSpec{
			SourcePath:          ".",
			BuildCommand:        "mvn clean package -DskipTests",
			ArtifactCopyCommand: "cp -ar target/invoice-api.jar \"$PAAS_ARTIFACT_OUTPUT/app.jar\"",
			RuntimeBaseImage:    "registry.example/runtime/java17:1.0",
			DefaultRef:          "main",
		}}},
	})
	if err != nil {
		t.Fatalf("create build pipeline: %v", err)
	}
	if _, err := app.apps.UpdateWorkload(context.Background(), appenv.UpdateWorkloadInput{Actor: identityaccess.Subject{Type: identityaccess.SubjectUser, ID: "usr_admin"}, ApplicationID: created.ID, WorkloadID: workload.ID, Name: workload.Name, DisplayName: workload.DisplayName, WorkloadType: workload.WorkloadType, ImageSourceMode: string(workload.ImageSourceMode), PipelineID: pipeline.ID}); err != nil {
		t.Fatalf("bind workload pipeline: %v", err)
	}
	run, err := app.builds.TriggerBuild(context.Background(), build.TriggerBuildInput{Actor: identityaccess.Subject{Type: identityaccess.SubjectUser, ID: "usr_admin"}, PipelineID: pipeline.ID, GitRef: "main"})
	if err != nil || run.ID.IsZero() {
		t.Fatalf("trigger build should create run: %+v, %v", run, err)
	}
	pipeline, err = app.state.buildRepo.FindPipelineByApplication(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("expected managed Jenkins pipeline to be persisted: %v", err)
	}
	if pipeline.ExternalJobName != "paas/rnd-test/order-test/invoice-api/main" || pipeline.Provider != "jenkins" || !pipeline.ManagedByPlatform || pipeline.ConfigHash != "" {
		t.Fatalf("unexpected pipeline: %+v", pipeline)
	}
}

func TestServerAdaptersAndHelpers(t *testing.T) {
	app := newTestApplication(t)
	defer app.db.Close()
	fixture := seedServerTestData(t, app)
	ctx := context.Background()

	sourceApp := sourceForAppEnv{service: app.sources}
	if ref, err := sourceApp.GetSourceRepository(ctx, fixture.source); err != nil || ref.DefaultBranch != "main" {
		t.Fatalf("source app ref = %#v err=%v", ref, err)
	}
	if _, err := sourceApp.GetSourceRepository(ctx, "missing"); err == nil {
		t.Fatalf("expected source app ref error")
	}

	sourceBuild := sourceForBuild{service: app.sources}
	if ref, err := sourceBuild.GetSourceRepository(ctx, fixture.source); err != nil || ref.HTTPURL == "" {
		t.Fatalf("source build ref = %#v err=%v", ref, err)
	}

	appBuild := appForBuild{service: app.apps}
	if ref, err := appBuild.GetApplication(ctx, fixture.application.ID); err != nil || ref.Name != "test-api-test" {
		t.Fatalf("app build ref = %#v err=%v", ref, err)
	}
	if source, err := appBuild.GetApplicationSource(ctx, fixture.application.ID); err != nil || source.BuildSpec.BuildCommand == "" {
		t.Fatalf("app source ref = %#v err=%v", source, err)
	}

	envDelivery := stageRuntimeForDelivery{service: app.apps}
	if _, err := app.apps.UpdateApplicationStageState(ctx, appenv.UpdateApplicationStageStateInput{ApplicationID: fixture.application.ID, StageKey: "dev", Status: appenv.ApplicationStageStatusRunning, Message: "健康"}); err != nil {
		t.Fatalf("UpdateApplicationStageState(running) error = %v", err)
	}
	if _, err := app.apps.UpdateApplicationStageState(ctx, appenv.UpdateApplicationStageStateInput{ApplicationID: fixture.application.ID, StageKey: "test", Status: appenv.ApplicationStageStatusDegraded, Message: "Pod ImagePullBackOff"}); err != nil {
		t.Fatalf("UpdateApplicationStageState(degraded) error = %v", err)
	}
	runtimeStates, err := envDelivery.ListStageRuntimeStates(ctx, fixture.application.ID)
	if err != nil {
		t.Fatalf("ListStageRuntimeStates() error = %v", err)
	}
	if got := runtimeStates["dev"]; got.SyncStatus != "Synced" || got.HealthStatus != "Healthy" || got.OperationState != "succeeded" || got.Message != "健康" {
		t.Fatalf("running runtime state = %#v", got)
	}
	if got := runtimeStates["test"]; got.SyncStatus != "OutOfSync" || got.HealthStatus != "Degraded" || got.OperationState != "failed" || got.Message != "Pod ImagePullBackOff" {
		t.Fatalf("degraded runtime state = %#v", got)
	}

	buildDelivery := buildForDelivery{service: app.builds, repo: app.state.buildRepo}
	if run, err := buildDelivery.GetBuildRun(ctx, fixture.buildRun.ID); err != nil || run.CommitSHA != "8c1a09f" {
		t.Fatalf("delivery run = %#v err=%v", run, err)
	}
	if artifact, err := buildDelivery.GetBuildArtifact(ctx, fixture.buildRun.PrimaryArtifactID); err != nil || !artifact.IsPrimary {
		t.Fatalf("delivery artifact = %#v err=%v", artifact, err)
	}

	appGitOps := appForGitOps{service: app.apps}
	if ref, err := appGitOps.GetApplication(ctx, fixture.application.ID); err != nil || ref.ProjectID != fixture.project.ID {
		t.Fatalf("gitops app = %#v err=%v", ref, err)
	}

	updater := stageUpdater{service: app.apps}
	reportedAt := time.Now()
	if err := updater.UpdateFromAgent(ctx, clusteragent.StatusReport{Applications: []clusteragent.ApplicationStatus{{ApplicationID: fixture.application.ID, StageKey: "dev", SyncStatus: "Synced", HealthStatus: "Healthy", Message: "健康"}, {ApplicationID: fixture.application.ID, StageKey: "test", HealthStatus: "Degraded", Message: "异常"}, {}}, ReportedAt: reportedAt}); err != nil {
		t.Fatalf("update from agent: %v", err)
	}

	if applicationType(appenv.BuildSpec{}) != "Spring Boot" {
		t.Fatalf("unexpected build type")
	}
	if buildStatusText(build.BuildRunFailed) != "失败" || buildStatusText(build.BuildRunRunning) != "运行中" || buildStatusText(build.BuildRunAborted) != "已中止" || buildStatusText(build.BuildRunQueued) != "排队中" {
		t.Fatalf("unexpected build status text")
	}
	if durationText(nil, nil) != "-" || durationPart(0, "秒") != "" || firstNonEmpty("", " value ") != "value" {
		t.Fatalf("unexpected helper result")
	}
	now := time.Now()
	if formatOptional(&now) == "-" || formatLocal(time.Time{}) != "-" {
		t.Fatalf("unexpected time formatting")
	}
	if got := toBuildSpec(appenv.BuildSpec{ArtifactDeployPath: "/usr/local/tomcat/webapps/"}); got.ArtifactDeployPath != "/usr/local/tomcat/webapps/" {
		t.Fatalf("unexpected converted build spec: %#v", got)
	}
	if _, err := sourceBuild.GetSourceRepository(ctx, shared.ID("missing")); err == nil {
		t.Fatalf("expected source build error")
	}
}

func seedServerTestData(t *testing.T, app *application) serverTestFixture {
	return seedServerTestDataNamed(t, app, "test")
}

func seedServerTestDataNamed(t *testing.T, app *application, suffix string) serverTestFixture {
	t.Helper()
	ctx := context.Background()
	actor := identityaccess.Subject{Type: identityaccess.SubjectUser, ID: "usr_admin"}

	if _, err := app.identity.CreateLocalUser(ctx, identityaccess.CreateLocalUserInput{ActorID: actor.ID, Username: "admin", Password: "password", DisplayName: "平台管理员"}); err != nil && shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("create local user: %v", err)
	}
	tenant, err := app.tenants.CreateTenant(ctx, tenantproject.CreateTenantInput{Actor: actor, Name: "rnd-" + suffix, DisplayName: "测试租户"})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	project, err := app.tenants.CreateProject(ctx, tenantproject.CreateProjectInput{Actor: actor, TenantID: tenant.ID, Name: "order-" + suffix, DisplayName: "测试项目"})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	repo, err := app.sources.CreateSourceRepository(ctx, sourcerepository.CreateSourceRepositoryInput{Actor: actor, ProjectID: project.ID, Name: "test-api-" + suffix, DisplayName: "测试服务仓库", DefaultBranch: "main"})
	if err != nil {
		t.Fatalf("create source repository: %v", err)
	}
	created, err := app.apps.CreateApplication(ctx, appenv.CreateApplicationInput{
		Actor:       actor,
		ProjectID:   project.ID,
		Name:        "test-api-" + suffix,
		DisplayName: "测试服务",
		Sources: []appenv.CreateApplicationSourceInput{{Key: "main", SourceRepositoryID: repo.ID, IsPrimary: true, BuildSpec: appenv.BuildSpec{
			SourcePath:          ".",
			BuildCommand:        "mvn clean package -DskipTests",
			ArtifactCopyCommand: "cp -ar target/test-api-" + suffix + ".jar \"$PAAS_ARTIFACT_OUTPUT/app.jar\"",
			RuntimeBaseImage:    "registry.example/runtime/java17:1.0",
			DefaultRef:          "main",
		}}},
	})
	if err != nil {
		t.Fatalf("create application: %v", err)
	}
	now := time.Now().UTC()
	if err := app.state.clusterRepo.CreateCluster(ctx, clusteragent.Cluster{
		ID:             "cluster_test_" + shared.ID(suffix),
		TenantID:       tenant.ID,
		Name:           "测试集群",
		Region:         "cn",
		Status:         clusteragent.ClusterReady,
		AgentTokenHash: "test-token-hash",
		CreatedAt:      now,
		UpdatedAt:      now,
	}); err != nil && shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("create test cluster: %v", err)
	}
	workload, err := app.apps.CreateWorkload(ctx, appenv.CreateWorkloadInput{Actor: actor, ApplicationID: created.ID, Name: "api", DisplayName: "测试服务", WorkloadType: appenv.WorkloadTypeDeployment})
	if err != nil {
		t.Fatalf("create workload: %v", err)
	}
	if _, err := app.apps.UpdateApplicationStageState(ctx, appenv.UpdateApplicationStageStateInput{ApplicationID: created.ID, StageKey: "dev", Status: appenv.ApplicationStageStatusRunning, Message: "运行中"}); err != nil {
		t.Fatalf("set stage state: %v", err)
	}
	ensureDeploymentTemplate(t, app, created.ID)

	pipeline, err := app.builds.CreateBuildPipeline(ctx, build.CreateBuildPipelineInput{
		Actor:         actor,
		ApplicationID: created.ID,
		Name:          "main",
		DisplayName:   "主流水线",
		RuntimeEnvironmentIDs: []shared.ID{
			"runtime_env_springboot_jdk11_aliyun",
		},
		Sources: []build.BuildPipelineSourceInput{{Key: "main", DisplayName: "主代码源", SourceRepositoryID: repo.ID, SourcePath: ".", IsPrimary: true, BuildSpec: build.BuildSpec{
			SourcePath:          ".",
			BuildCommand:        "mvn clean package -DskipTests",
			ArtifactCopyCommand: "cp -ar target/test-api-" + suffix + ".jar \"$PAAS_ARTIFACT_OUTPUT/app.jar\"",
			RuntimeBaseImage:    "registry.example/runtime/java17:1.0",
			DefaultRef:          "main",
		}}},
	})
	if err != nil {
		t.Fatalf("create build pipeline: %v", err)
	}
	if _, err := app.apps.UpdateWorkload(ctx, appenv.UpdateWorkloadInput{Actor: actor, ApplicationID: created.ID, WorkloadID: workload.ID, Name: workload.Name, DisplayName: workload.DisplayName, WorkloadType: workload.WorkloadType, ImageSourceMode: string(workload.ImageSourceMode), PipelineID: pipeline.ID}); err != nil {
		t.Fatalf("bind workload pipeline: %v", err)
	}
	run, err := app.builds.TriggerBuild(ctx, build.TriggerBuildInput{Actor: actor, PipelineID: pipeline.ID, GitRef: "main", CommitSHA: "8c1a09f"})
	if err != nil {
		t.Fatalf("trigger build: %v", err)
	}
	run, err = app.builds.HandleBuildCallback(ctx, build.BuildCallbackInput{BuildRunID: run.ID, Status: build.BuildRunSucceeded, JenkinsBuildNumber: 1, CommitSHA: "8c1a09f", ImageURI: "registry.local/test-api:v1", ImageDigest: "sha256:test"})
	if err != nil {
		t.Fatalf("finish build: %v", err)
	}
	release, err := app.delivery.HandleBuildSucceeded(ctx, delivery.BuildSucceededPayload{BuildRunID: run.ID, ApplicationID: created.ID, WorkloadID: workload.ID, BuildArtifactID: run.PrimaryArtifactID, ImageURI: "registry.local/test-api:v1", ImageDigest: "sha256:test", CommitSHA: "8c1a09f"})
	if err != nil {
		t.Fatalf("create release candidate: %v", err)
	}
	if _, err := app.delivery.CreateFreight(ctx, delivery.CreateFreightInput{Actor: actor, ApplicationID: created.ID, Name: "测试发布包", Items: []delivery.CreateFreightItemInput{{WorkloadID: workload.ID, SourceType: delivery.FreightItemPipelineArtifact, ReleaseID: release.ID}}}); err != nil {
		t.Fatalf("create freight: %v", err)
	}
	return serverTestFixture{tenant: tenant, project: project, source: repo.ID, application: created, workload: workload, pipeline: pipeline, buildRun: run}
}

func ensureDeploymentTemplate(t *testing.T, app *application, applicationID shared.ID) {
	t.Helper()
	platformBody := bytes.NewBufferString(`{"name":"java","content":"initContainers:\n- name: init-data\n  image: busybox\nsecurityContext:\n  runAsNonRoot: true"}`)
	platform := httptest.NewRecorder()
	app.handler.ServeHTTP(platform, httptest.NewRequest(http.MethodPost, "/api/deployment-templates/platform", platformBody))
	if platform.Code != http.StatusCreated {
		t.Fatalf("create platform template status = %d body = %s", platform.Code, platform.Body.String())
	}
	appBody := bytes.NewBufferString(`{"base_template_name":"java","actor_id":"usr_admin"}`)
	template := httptest.NewRecorder()
	app.handler.ServeHTTP(template, httptest.NewRequest(http.MethodPost, "/api/apps/"+applicationID.String()+"/deployment-template", appBody))
	if template.Code != http.StatusCreated {
		t.Fatalf("create application template status = %d body = %s", template.Code, template.Body.String())
	}
}
