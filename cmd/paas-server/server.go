package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shareinto/paas/internal/integrations/gitlab"
	"github.com/shareinto/paas/internal/integrations/jenkins"
	"github.com/shareinto/paas/internal/modules/appenv"
	"github.com/shareinto/paas/internal/modules/audit"
	"github.com/shareinto/paas/internal/modules/build"
	"github.com/shareinto/paas/internal/modules/clusteragent"
	"github.com/shareinto/paas/internal/modules/delivery"
	"github.com/shareinto/paas/internal/modules/gitops"
	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/modules/notification"
	"github.com/shareinto/paas/internal/modules/sourcerepository"
	"github.com/shareinto/paas/internal/modules/tenantproject"
	"github.com/shareinto/paas/internal/platform/database"
	"github.com/shareinto/paas/internal/shared"
)

type application struct {
	handler http.Handler

	identity *identityaccess.Service
	tenants  *tenantproject.Service
	sources  *sourcerepository.Service
	apps     *appenv.Service
	builds   *build.Service
	delivery *delivery.Service
	audit    *audit.Service
	state    serverState
	db       *sql.DB
}

type serverState struct {
	tenantRepo   tenantproject.Repository
	sourceRepo   sourcerepository.Repository
	appRepo      appenv.Repository
	buildRepo    build.Repository
	deliveryRepo delivery.Repository
	auditRepo    audit.Repository
	gitopsRepo   gitops.Repository
	clusterRepo  clusteragent.Repository
}

type buildEventBridge struct {
	delivery *delivery.Service
}

func (b *buildEventBridge) Publish(ctx context.Context, event shared.DomainEvent) error {
	if event.EventType != "BuildSucceeded" || b.delivery == nil {
		return nil
	}
	var payload delivery.BuildSucceededPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return shared.WrapError(shared.CodeInvalidArgument, "decode build succeeded payload failed", err)
	}
	_, err := b.delivery.HandleBuildSucceeded(ctx, payload)
	return err
}

type projectDeletionGuard struct {
	sourceSvc  *sourcerepository.Service
	sourceRepo sourcerepository.Repository
	appSvc     *appenv.Service
}

func (g projectDeletionGuard) PrepareProjectDeletion(ctx context.Context, actor identityaccess.Subject, project tenantproject.Project) error {
	if g.appSvc != nil {
		for {
			apps, err := g.appSvc.ListApplicationsByProject(ctx, project.ID, shared.PageRequest{Page: 1, PageSize: 100})
			if err != nil {
				return err
			}
			if len(apps.Items) == 0 {
				break
			}
			for _, app := range apps.Items {
				if err := g.appSvc.DeleteApplication(ctx, actor, app.ID); err != nil && shared.CodeOf(err) != shared.CodeNotFound {
					return err
				}
			}
		}
	}
	if g.sourceSvc != nil {
		repositories, err := g.sourceSvc.ListSourceRepositoriesByProject(ctx, project.ID, shared.PageRequest{Page: 1, PageSize: 100})
		if err != nil {
			return err
		}
		for _, repository := range repositories.Items {
			if g.sourceRepo != nil {
				if err := g.sourceRepo.SetAssociatedApplications(ctx, repository.ID, nil); err != nil && shared.CodeOf(err) != shared.CodeNotFound {
					return err
				}
			}
			if err := g.sourceSvc.DeleteSourceRepository(ctx, sourcerepository.DeleteSourceRepositoryInput{Actor: actor, SourceRepositoryID: repository.ID}); err != nil && shared.CodeOf(err) != shared.CodeNotFound {
				return err
			}
		}
	}
	return nil
}

func newApplication(ctx context.Context) (*application, error) {
	clock := shared.SystemClock{}
	ids := shared.RandomIDGenerator{}
	repos, db, err := openRepositories(ctx)
	if err != nil {
		return nil, err
	}

	auditRepo := repos.audit
	auditSvc := audit.NewService(audit.Options{Repository: auditRepo, IDGenerator: ids, Clock: clock})
	identityRepo := repos.identity
	localRegistrationEnabled := localRegistrationEnabledFromEnv()
	identitySvc := identityaccess.NewService(identityaccess.Options{Repository: identityRepo, Audit: audit.IdentityAccessLogger{Logger: auditSvc}, IDGenerator: ids, Clock: clock, LocalRegistrationEnabled: &localRegistrationEnabled})
	if err := ensureDevelopmentAdmin(ctx, identityRepo, identitySvc); err != nil {
		return nil, err
	}

	tenantRepo := repos.tenant
	sourceRepo := repos.source
	appRepo := repos.app
	tenantSvc := tenantproject.NewService(tenantproject.Options{Repository: tenantRepo, PermissionChecker: identitySvc, RoleBindings: identitySvc, SubjectQuery: identitySvc, Audit: audit.TenantProjectLogger{Logger: auditSvc}, IDGenerator: ids, Clock: clock})

	gitAdapter, webhookURL := sourceGitAdapterFromEnv()
	sourceSvc := sourcerepository.NewService(sourcerepository.Options{Repository: sourceRepo, Git: gitAdapter, ProjectQuery: tenantSvc, MembershipQuery: tenantSvc, Audit: audit.SourceRepositoryLogger{Logger: auditSvc}, IDGenerator: ids, Clock: clock, WebhookCallbackURL: webhookURL})

	buildRepo := repos.build
	jenkinsAdapter := buildRunnerFromEnv()
	templateID := firstNonEmpty(os.Getenv("JENKINS_DEFAULT_TEMPLATE_ID"), "java-unified-v1")
	buildEvents := &buildEventBridge{}
	buildSvc := build.NewService(build.Options{Repository: buildRepo, SourceRepositoryQuery: sourceForBuild{service: sourceSvc}, BuildRunner: jenkinsAdapter, Audit: audit.BuildLogger{Logger: auditSvc}, EventPublisher: buildEvents, IDGenerator: ids, Clock: clock, TemplateID: templateID, CallbackURL: firstNonEmpty(os.Getenv("JENKINS_CALLBACK_BASE_URL"), "http://127.0.0.1:8080"), ImageRepository: firstNonEmpty(os.Getenv("IMAGE_REPOSITORY"), "registry.local/order-api"), DockerfileRepository: build.DockerfileRepositoryConfig{URL: firstNonEmpty(os.Getenv("DOCKERFILE_REPOSITORY_URL"), "ssh://git@192.168.100.80:2422/paas/dockerfiles.git"), Ref: os.Getenv("DOCKERFILE_REPOSITORY_REF"), CredentialsID: os.Getenv("DOCKERFILE_REPOSITORY_CREDENTIALS_ID")}})
	appSvc := appenv.NewService(appenv.Options{
		Repository:               appRepo,
		ProjectQuery:             tenantSvc,
		SourceRepositoryQuery:    sourceForAppEnv{service: sourceSvc},
		JenkinsTemplateQuery:     jenkinsTemplateForAppEnv{repo: buildRepo},
		BuildEnvironmentQuery:    buildEnvironmentForAppEnv{repo: buildRepo},
		RuntimeEnvironmentQuery:  runtimeEnvironmentForAppEnv{repo: buildRepo},
		BuildPipelineProvisioner: buildSvc,
		BuildPipelineQuery:       buildPipelineForAppEnv{service: buildSvc},
		Audit:                    audit.ApplicationEnvironmentLogger{Logger: auditSvc},
		IDGenerator:              ids,
		Clock:                    clock,
	})
	buildSvc.SetRuntimeEnvironmentSyncer(runtimeEnvironmentSyncerForAppEnv{service: appSvc})
	buildSvc.SetApplicationQuery(appForBuild{service: appSvc, projects: tenantSvc})
	buildSvc.SetWorkloadQuery(workloadForBuild{service: appSvc})
	tenantSvc.SetProjectDeletionGuard(projectDeletionGuard{sourceSvc: sourceSvc, sourceRepo: sourceRepo, appSvc: appSvc})

	deliveryRepo := repos.delivery
	gitopsRepo := repos.gitops
	manifestRepo, chartRepo, manifestRepoURL, chartRepoURL := manifestRepositoriesFromEnv()
	gitopsSvc := gitops.NewService(gitops.Options{
		Repository:      gitopsRepo,
		ManifestRepo:    manifestRepo,
		ChartRepo:       chartRepo,
		ManifestRepoURL: manifestRepoURL,
		ChartRepoURL:    chartRepoURL,
		ChartName:       firstNonEmpty(os.Getenv("PAAS_PLATFORM_CHART_NAME"), "paas-app"),
		ChartVersion:    firstNonEmpty(os.Getenv("PAAS_PLATFORM_CHART_VERSION"), "0.1.0"),
		Application:     appForGitOps{service: appSvc},
		Workload:        workloadForGitOps{service: appSvc},
		Audit:           audit.GitOpsLogger{Logger: auditSvc},
		IDGenerator:     ids,
		Clock:           clock,
	})
	appSvc.SetManifestCleaner(gitopsSvc)
	stageRuntimeDelivery := stageRuntimeForDelivery{service: appSvc}
	deliverySvc := delivery.NewService(delivery.Options{Repository: deliveryRepo, BuildQuery: buildForDelivery{service: buildSvc, repo: buildRepo}, ApplicationQuery: appForDelivery{service: appSvc, projects: tenantSvc}, WorkloadQuery: workloadForDelivery{service: appSvc}, StageRuntimeStateQuery: stageRuntimeDelivery, StageSync: stageSyncForDelivery{service: appSvc}, ClusterQuery: clusterForDelivery{repo: repos.cluster}, GitOpsDeployment: gitopsSvc, Audit: audit.DeliveryLogger{Logger: auditSvc}, IDGenerator: ids, Clock: clock})
	buildEvents.delivery = deliverySvc
	if err := buildSvc.EnsureDefaultJenkinsJobTemplate(ctx, "usr_admin"); err != nil {
		return nil, err
	}
	if err := buildSvc.EnsureDefaultBuildConfiguration(ctx, "usr_admin"); err != nil {
		return nil, err
	}

	clusterRepo := repos.cluster
	runtimeGateway := clusteragent.NewWebSocketRuntimeGateway()
	clusterSvc := clusteragent.NewService(clusteragent.Options{Repository: clusterRepo, TenantQuery: tenantForClusterAgent{service: tenantSvc}, PermissionChecker: identitySvc, RuntimeGateway: runtimeGateway, StageClusters: stageClusterForRuntime{apps: appSvc, delivery: deliveryRepo}, StageState: stageUpdater{service: appSvc}, DeploymentStatus: gitopsSvc, Audit: audit.ClusterAgentLogger{Logger: auditSvc}, IDGenerator: ids, Clock: clock})

	notificationSvc := notification.NewService(notification.Options{Repository: repos.notification, IDGenerator: ids, Clock: clock})
	if err := notificationSvc.EnsureDefaults(ctx); err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	state := serverState{tenantRepo: tenantRepo, sourceRepo: sourceRepo, appRepo: appRepo, buildRepo: buildRepo, deliveryRepo: deliveryRepo, auditRepo: auditRepo, gitopsRepo: gitopsRepo, clusterRepo: clusterRepo}
	registerDevelopmentRoutes(mux, developmentAPI{identity: identitySvc, tenants: tenantSvc, apps: appSvc, builds: buildSvc, delivery: deliverySvc, state: state})
	tenantproject.NewHandler(tenantSvc, tenantproject.HandlerOptions{SubjectQuery: identitySvc}).Register(mux)
	identityaccess.NewHandler(identitySvc).Register(mux)
	sourcerepository.NewHandler(sourceSvc).Register(mux)
	appenv.NewHandler(appSvc).Register(mux)
	build.NewHandler(buildSvc).Register(mux)
	delivery.NewHandler(deliverySvc).Register(mux)
	gitops.NewHandler(gitopsSvc).Register(mux)
	audit.NewHandler(auditSvc).Register(mux)
	notification.NewHandler(notificationSvc).Register(mux)
	clusteragent.NewHandler(clusterSvc).Register(mux)

	return &application{handler: withCORS(mux), identity: identitySvc, tenants: tenantSvc, sources: sourceSvc, apps: appSvc, builds: buildSvc, delivery: deliverySvc, audit: auditSvc, state: state, db: db}, nil
}

func ensureDevelopmentAdmin(ctx context.Context, repo identityaccess.Repository, service *identityaccess.Service) error {
	users, err := repo.ListUsers(ctx, shared.PageRequest{Page: 1, PageSize: 1})
	if err != nil {
		return err
	}
	var adminID shared.ID
	if users.Total > 0 {
		if existing, findErr := repo.FindUserByUsername(ctx, "admin"); findErr == nil {
			adminID = existing.ID
		}
	} else {
		admin, err := service.CreateLocalUser(ctx, identityaccess.CreateLocalUserInput{
			ActorID:     "usr_admin",
			Username:    "admin",
			Password:    "password",
			DisplayName: "平台管理员",
		})
		if err != nil && shared.CodeOf(err) != shared.CodeConflict {
			return err
		}
		adminID = admin.ID
		if adminID.IsZero() {
			existing, findErr := repo.FindUserByUsername(ctx, "admin")
			if findErr != nil {
				return findErr
			}
			adminID = existing.ID
		}
	}
	for _, subjectID := range []shared.ID{adminID, "usr_admin"} {
		if subjectID.IsZero() {
			continue
		}
		_, err = service.ReplaceRoleBindingForSubjectScope(ctx, identityaccess.RoleBinding{
			SubjectType: identityaccess.SubjectUser,
			SubjectID:   subjectID,
			RoleID:      identityaccess.RolePlatformAdmin,
			ScopeKind:   identityaccess.ScopePlatform,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func localRegistrationEnabledFromEnv() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("PAAS_LOCAL_REGISTRATION_ENABLED")))
	switch value {
	case "", "true", "1", "yes", "on":
		return true
	case "false", "0", "no", "off":
		return false
	default:
		return true
	}
}

type repositories struct {
	identity     identityaccess.Repository
	tenant       tenantproject.Repository
	source       sourcerepository.Repository
	app          appenv.Repository
	build        build.Repository
	delivery     delivery.Repository
	audit        audit.Repository
	notification notification.Repository
	cluster      clusteragent.Repository
	gitops       gitops.Repository
}

func openRepositories(ctx context.Context) (repositories, *sql.DB, error) {
	db, err := database.Open(ctx, database.ConfigFromEnv())
	if err != nil {
		return repositories{}, nil, err
	}
	repos, err := openMySQLRepositories(ctx, db)
	if err != nil {
		_ = db.Close()
		return repositories{}, nil, err
	}
	return repos, db, nil
}

func openMySQLRepositories(ctx context.Context, db *sql.DB) (repositories, error) {
	identityRepo, err := identityaccess.NewMySQLRepository(ctx, db)
	if err != nil {
		return repositories{}, err
	}
	tenantRepo, err := tenantproject.NewMySQLRepository(ctx, db)
	if err != nil {
		return repositories{}, err
	}
	sourceRepo, err := sourcerepository.NewMySQLRepository(ctx, db)
	if err != nil {
		return repositories{}, err
	}
	appRepo, err := appenv.NewMySQLRepository(ctx, db)
	if err != nil {
		return repositories{}, err
	}
	buildRepo, err := build.NewMySQLRepository(ctx, db)
	if err != nil {
		return repositories{}, err
	}
	deliveryRepo, err := delivery.NewMySQLRepository(ctx, db)
	if err != nil {
		return repositories{}, err
	}
	auditRepo, err := audit.NewMySQLRepository(ctx, db)
	if err != nil {
		return repositories{}, err
	}
	notificationRepo, err := notification.NewMySQLRepository(ctx, db)
	if err != nil {
		return repositories{}, err
	}
	clusterRepo, err := clusteragent.NewMySQLRepository(ctx, db)
	if err != nil {
		return repositories{}, err
	}
	gitopsRepo, err := gitops.NewMySQLRepository(ctx, db)
	if err != nil {
		return repositories{}, err
	}
	return repositories{identity: identityRepo, tenant: tenantRepo, source: sourceRepo, app: appRepo, build: buildRepo, delivery: deliveryRepo, audit: auditRepo, notification: notificationRepo, cluster: clusterRepo, gitops: gitopsRepo}, nil
}

func sourceGitAdapterFromEnv() (sourcerepository.GitSourceRepositoryPort, string) {
	baseURL := strings.TrimSpace(os.Getenv("GITLAB_BASE_URL"))
	token := strings.TrimSpace(os.Getenv("GITLAB_TOKEN"))
	webhookURL := strings.TrimSpace(os.Getenv("GITLAB_WEBHOOK_CALLBACK_URL"))
	if baseURL == "" || token == "" {
		return &gitlab.FakeSourceRepositoryAdapter{Files: []sourcerepository.RepositoryFile{{Path: "pom.xml", Type: "blob"}, {Path: "target/order-api.jar", Type: "blob"}}}, webhookURL
	}
	cfg := gitlabConfigFromValues(baseURL, token, os.Getenv("GITLAB_TIMEOUT_SECONDS"), os.Getenv("GITLAB_RETRY_MAX"))
	cfg.HTTPURLAliases = commaSeparatedValues(os.Getenv("GITLAB_HTTP_URL_ALIASES"))
	return gitlab.NewSourceRepositoryAdapterWithNamespace(gitlab.NewClient(cfg), os.Getenv("GITLAB_ROOT_GROUP_PATH")), webhookURL
}

func manifestRepositoriesFromEnv() (gitops.ManifestRepositoryPort, gitops.ManifestRepositoryPort, string, string) {
	baseURL := firstNonEmpty(os.Getenv("GITOPS_GITLAB_BASE_URL"), os.Getenv("GITLAB_BASE_URL"))
	token := firstNonEmpty(os.Getenv("GITOPS_GITLAB_TOKEN"), os.Getenv("GITLAB_TOKEN"))
	manifestProjectID := strings.TrimSpace(os.Getenv("GITLAB_MANIFEST_PROJECT_ID"))
	chartProjectID := strings.TrimSpace(os.Getenv("GITLAB_CHART_PROJECT_ID"))
	manifestRepoURL := strings.TrimSpace(os.Getenv("GITLAB_MANIFEST_REPO_URL"))
	chartRepoURL := strings.TrimSpace(os.Getenv("GITLAB_CHART_REPO_URL"))
	if baseURL == "" || token == "" || manifestProjectID == "" || chartProjectID == "" {
		return gitlab.NewFakeManifestRepositoryAdapter(), gitlab.NewFakeManifestRepositoryAdapter(), manifestRepoURL, chartRepoURL
	}
	cfg := gitlabConfigFromValues(
		baseURL,
		token,
		firstNonEmpty(os.Getenv("GITOPS_GITLAB_TIMEOUT_SECONDS"), os.Getenv("GITLAB_TIMEOUT_SECONDS")),
		firstNonEmpty(os.Getenv("GITOPS_GITLAB_RETRY_MAX"), os.Getenv("GITLAB_RETRY_MAX")),
	)
	client := gitlab.NewClient(cfg)
	if manifestRepoURL == "" {
		manifestRepoURL = defaultGitLabProjectRepoURL(baseURL, manifestProjectID)
	}
	if chartRepoURL == "" {
		chartRepoURL = defaultGitLabProjectRepoURL(baseURL, chartProjectID)
	}
	return gitlab.NewManifestRepositoryAdapter(client, manifestProjectID), gitlab.NewManifestRepositoryAdapter(client, chartProjectID), manifestRepoURL, chartRepoURL
}

func gitlabConfigFromValues(baseURL string, token string, timeoutValue string, retryValue string) gitlab.Config {
	cfg := gitlab.Config{BaseURL: baseURL, Token: token}
	if seconds, err := strconv.Atoi(timeoutValue); err == nil && seconds > 0 {
		cfg.Timeout = time.Duration(seconds) * time.Second
	}
	if retries, err := strconv.Atoi(retryValue); err == nil && retries >= 0 {
		cfg.RetryMax = retries
	}
	return cfg
}

func defaultGitLabProjectRepoURL(baseURL string, projectID string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/") + "/" + strings.TrimSpace(projectID) + ".git"
}

func commaSeparatedValues(value string) []string {
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			values = append(values, part)
		}
	}
	return values
}

func buildRunnerFromEnv() build.BuildRunnerPort {
	baseURL := strings.TrimSpace(os.Getenv("JENKINS_BASE_URL"))
	username := strings.TrimSpace(os.Getenv("JENKINS_USERNAME"))
	token := strings.TrimSpace(os.Getenv("JENKINS_TOKEN"))
	if baseURL == "" || token == "" {
		return &jenkins.FakeAdapter{Logs: map[int64]build.ProgressiveText{1: {Text: strings.Join([]string{
			"[INFO] 检出平台托管源码仓库",
			"[INFO] 执行构建命令 mvn clean package -DskipTests",
			"[INFO] 校验构建产物",
			"[INFO] 构建并推送镜像",
			"[INFO] 回调 PaaS 控制面完成",
		}, "\n"), NextOffset: 180}}}
	}
	cfg := jenkins.Config{BaseURL: baseURL, Username: username, Token: token}
	if seconds, err := strconv.Atoi(strings.TrimSpace(os.Getenv("JENKINS_TIMEOUT_SECONDS"))); err == nil && seconds > 0 {
		cfg.Timeout = time.Duration(seconds) * time.Second
	}
	if retries, err := strconv.Atoi(strings.TrimSpace(os.Getenv("JENKINS_RETRY_MAX"))); err == nil && retries >= 0 {
		cfg.RetryMax = retries
	}
	return jenkins.NewAdapter(jenkins.NewClient(cfg))
}

func registerDevelopmentRoutes(mux *http.ServeMux, api developmentAPI) {
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeDevelopmentJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", api.readyz)
	mux.HandleFunc("GET /metrics", api.metrics)
	mux.HandleFunc("POST /api/auth/local/login", api.localLogin)
	mux.HandleFunc("POST /api/auth/local/register", api.localRegister)
	mux.HandleFunc("POST /api/auth/oidc/start", api.oidcStart)
	mux.HandleFunc("GET /api/applications", api.listApplications)
	mux.HandleFunc("GET /api/builds", api.listBuilds)
	mux.HandleFunc("GET /api/builds/{buildRunId}/logs", api.buildLog)
}

type developmentAPI struct {
	identity *identityaccess.Service
	tenants  *tenantproject.Service
	apps     *appenv.Service
	builds   *build.Service
	delivery *delivery.Service
	state    serverState
}

func (api developmentAPI) readyz(w http.ResponseWriter, r *http.Request) {
	tenants, err := api.tenants.ListTenants(r.Context(), shared.PageRequest{Page: 1, PageSize: 1})
	if err != nil {
		writeDevelopmentJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "not_ready", "error": map[string]string{"code": string(shared.CodeOf(err)), "message": "依赖检查失败"}})
		return
	}
	writeDevelopmentJSON(w, http.StatusOK, map[string]any{"status": "ready", "checks": map[string]any{"tenant_repository": "ok", "tenant_count": tenants.Total}})
}

func (api developmentAPI) metrics(w http.ResponseWriter, r *http.Request) {
	snapshot := api.metricsSnapshot(r.Context())
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	for name, value := range snapshot {
		_, _ = fmt.Fprintf(w, "# TYPE %s gauge\n%s %d\n", name, name, value)
	}
}

func (api developmentAPI) metricsSnapshot(ctx context.Context) map[string]int64 {
	out := map[string]int64{
		"paas_projects_total":     0,
		"paas_applications_total": 0,
		"paas_build_runs_total":   0,
		"paas_freights_total":     0,
		"paas_audit_logs_total":   0,
		"paas_stage_states":       0,
		"paas_ready":              1,
	}
	for _, project := range api.allProjects(ctx) {
		out["paas_projects_total"]++
		apps, err := api.apps.ListApplicationsByProject(ctx, project.ID, shared.PageRequest{Page: 1, PageSize: 100})
		if err != nil {
			out["paas_ready"] = 0
			continue
		}
		out["paas_applications_total"] += int64(len(apps.Items))
		for _, app := range apps.Items {
			runs, err := api.builds.ListBuildRuns(ctx, app.ID, shared.PageRequest{Page: 1, PageSize: 100})
			if err != nil {
				out["paas_ready"] = 0
			} else {
				out["paas_build_runs_total"] += int64(len(runs.Items))
			}
			freights, err := api.delivery.ListFreights(ctx, app.ID, shared.PageRequest{Page: 1, PageSize: 100})
			if err != nil {
				out["paas_ready"] = 0
			} else {
				out["paas_freights_total"] += int64(len(freights.Items))
			}
			states, err := api.apps.ListApplicationStageStates(ctx, app.ID)
			if err != nil {
				out["paas_ready"] = 0
			} else {
				out["paas_stage_states"] += int64(len(states))
			}
		}
	}
	logs, err := api.state.auditRepo.List(ctx, audit.Query{}, shared.PageRequest{Page: 1, PageSize: 100})
	if err != nil {
		out["paas_ready"] = 0
	} else {
		out["paas_audit_logs_total"] = int64(len(logs.Items))
	}
	return out
}

func (api developmentAPI) localLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Account  string `json:"account"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decodeDevelopmentJSON(w, r, &req) {
		return
	}
	username := firstNonEmpty(req.Username, req.Account)
	pair, user, err := api.identity.LoginLocal(r.Context(), username, req.Password)
	if err != nil {
		writeDevelopmentError(w, err)
		return
	}
	writeDevelopmentJSON(w, http.StatusOK, map[string]string{"token": pair.AccessToken, "userName": firstNonEmpty(user.DisplayName, user.Username)})
}

func (api developmentAPI) localRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Account          string `json:"account"`
		Username         string `json:"username"`
		Password         string `json:"password"`
		DisplayName      string `json:"displayName"`
		DisplayNameSnake string `json:"display_name"`
		Email            string `json:"email"`
	}
	if !decodeDevelopmentJSON(w, r, &req) {
		return
	}
	pair, user, err := api.identity.RegisterLocal(r.Context(), identityaccess.RegisterLocalUserInput{
		Username:    firstNonEmpty(req.Username, req.Account),
		Password:    req.Password,
		DisplayName: firstNonEmpty(req.DisplayName, req.DisplayNameSnake),
		Email:       req.Email,
	})
	if err != nil {
		writeDevelopmentError(w, err)
		return
	}
	writeDevelopmentJSON(w, http.StatusCreated, map[string]string{"token": pair.AccessToken, "userName": firstNonEmpty(user.DisplayName, user.Username)})
}

func (api developmentAPI) oidcStart(w http.ResponseWriter, _ *http.Request) {
	writeDevelopmentJSON(w, http.StatusOK, map[string]string{"redirect_url": "/oidc/callback?code=mock"})
}

type projectRow struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Display   string `json:"displayName"`
	Tenant    string `json:"tenant"`
	Owner     string `json:"owner"`
	UpdatedAt string `json:"updatedAt"`
}

func (api developmentAPI) listProjects(w http.ResponseWriter, r *http.Request) {
	tenants, err := api.tenants.ListTenants(r.Context(), shared.PageRequest{Page: 1, PageSize: 100})
	if err != nil {
		writeDevelopmentError(w, err)
		return
	}
	rows := make([]projectRow, 0)
	for _, tenant := range tenants.Items {
		projects, err := api.tenants.ListProjectsByTenant(r.Context(), tenant.ID, shared.PageRequest{Page: 1, PageSize: 100})
		if err != nil {
			writeDevelopmentError(w, err)
			return
		}
		for _, project := range projects.Items {
			rows = append(rows, projectRow{ID: project.ID.String(), Name: project.Name, Display: project.DisplayName, Tenant: tenant.DisplayName, Owner: "平台管理员", UpdatedAt: formatLocal(project.UpdatedAt)})
		}
	}
	writeDevelopmentJSON(w, http.StatusOK, shared.NewPageResult(rows, int64(len(rows)), pageFromRequest(r)))
}

type applicationRow struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Display     string `json:"displayName"`
	Project     string `json:"project"`
	Type        string `json:"type"`
	StageStatus string `json:"stageStatus"`
	Build       string `json:"build"`
	Release     string `json:"release"`
	Owner       string `json:"owner"`
	UpdatedAt   string `json:"updatedAt"`
}

func (api developmentAPI) listApplications(w http.ResponseWriter, r *http.Request) {
	projects := api.allProjects(r.Context())
	rows := make([]applicationRow, 0)
	for _, project := range projects {
		apps, err := api.apps.ListApplicationsByProject(r.Context(), project.ID, shared.PageRequest{Page: 1, PageSize: 100})
		if err != nil {
			writeDevelopmentError(w, err)
			return
		}
		for _, app := range apps.Items {
			source, _ := api.apps.GetApplicationSource(r.Context(), app.ID)
			rows = append(rows, applicationRow{ID: app.ID.String(), Name: app.Name, Display: app.DisplayName, Project: project.DisplayName, Type: applicationType(source.BuildSpec), StageStatus: api.stageSummary(r.Context(), app.ID), Build: api.latestBuildSummary(r.Context(), app.ID), Release: api.latestFreightSummary(r.Context(), app.ID), Owner: "平台管理员", UpdatedAt: formatLocal(app.UpdatedAt)})
		}
	}
	writeDevelopmentJSON(w, http.StatusOK, shared.NewPageResult(rows, int64(len(rows)), pageFromRequest(r)))
}

type buildRow struct {
	ID          string `json:"id"`
	Application string `json:"application"`
	Status      string `json:"status"`
	Ref         string `json:"ref"`
	Commit      string `json:"commit"`
	StartedAt   string `json:"startedAt"`
	Duration    string `json:"duration"`
}

func (api developmentAPI) listBuilds(w http.ResponseWriter, r *http.Request) {
	rows := make([]buildRow, 0)
	for _, project := range api.allProjects(r.Context()) {
		apps, err := api.apps.ListApplicationsByProject(r.Context(), project.ID, shared.PageRequest{Page: 1, PageSize: 100})
		if err != nil {
			writeDevelopmentError(w, err)
			return
		}
		for _, app := range apps.Items {
			runs, err := api.builds.ListBuildRuns(r.Context(), app.ID, shared.PageRequest{Page: 1, PageSize: 100})
			if err != nil {
				writeDevelopmentError(w, err)
				return
			}
			for _, run := range runs.Items {
				rows = append(rows, buildRow{ID: run.ID.String(), Application: app.DisplayName, Status: buildStatusText(run.Status), Ref: run.GitRef, Commit: run.CommitSHA, StartedAt: formatOptional(run.StartedAt), Duration: durationText(run.StartedAt, run.FinishedAt)})
			}
		}
	}
	writeDevelopmentJSON(w, http.StatusOK, shared.NewPageResult(rows, int64(len(rows)), pageFromRequest(r)))
}

func (api developmentAPI) buildLog(w http.ResponseWriter, r *http.Request) {
	events, err := api.builds.BuildLogEvents(r.Context(), shared.ID(r.PathValue("buildRunId")))
	if err != nil {
		writeDevelopmentError(w, err)
		return
	}
	lines := make([]string, 0, len(events))
	for _, event := range events {
		if event.Event == "log" {
			lines = append(lines, event.Data)
		}
	}
	writeDevelopmentJSON(w, http.StatusOK, strings.Join(lines, "\n"))
}

func (api developmentAPI) allProjects(ctx context.Context) []tenantproject.Project {
	tenants, err := api.tenants.ListTenants(ctx, shared.PageRequest{Page: 1, PageSize: 100})
	if err != nil {
		return nil
	}
	var out []tenantproject.Project
	for _, tenant := range tenants.Items {
		projects, err := api.tenants.ListProjectsByTenant(ctx, tenant.ID, shared.PageRequest{Page: 1, PageSize: 100})
		if err == nil {
			out = append(out, projects.Items...)
		}
	}
	return out
}

func (api developmentAPI) stageSummary(ctx context.Context, applicationID shared.ID) string {
	states, err := api.apps.ListApplicationStageStates(ctx, applicationID)
	if err != nil {
		return "未知"
	}
	for _, state := range states {
		if state.Status == appenv.ApplicationStageStatusRunning {
			return "运行中"
		}
	}
	return "待绑定集群"
}

func (api developmentAPI) latestBuildSummary(ctx context.Context, applicationID shared.ID) string {
	runs, err := api.builds.ListBuildRuns(ctx, applicationID, shared.PageRequest{Page: 1, PageSize: 1})
	if err != nil || len(runs.Items) == 0 {
		return "-"
	}
	return "#" + strings.TrimPrefix(runs.Items[0].ID.String(), "build_") + " " + buildStatusText(runs.Items[0].Status)
}

func (api developmentAPI) latestFreightSummary(ctx context.Context, applicationID shared.ID) string {
	freights, err := api.delivery.ListFreights(ctx, applicationID, shared.PageRequest{Page: 1, PageSize: 1})
	if err != nil || len(freights.Items) == 0 {
		return "-"
	}
	return freights.Items[0].Name
}

type sourceForAppEnv struct{ service *sourcerepository.Service }

func (q sourceForAppEnv) GetSourceRepository(ctx context.Context, id shared.ID) (appenv.SourceRepositoryRef, error) {
	repo, err := q.service.GetSourceRepository(ctx, id)
	if err != nil {
		return appenv.SourceRepositoryRef{}, err
	}
	return appenv.SourceRepositoryRef{ID: repo.ID, TenantID: repo.TenantID, ProjectID: repo.ProjectID, DefaultBranch: repo.DefaultBranch, Status: string(repo.Status)}, nil
}

type sourceForBuild struct{ service *sourcerepository.Service }

func (q sourceForBuild) GetSourceRepository(ctx context.Context, id shared.ID) (build.SourceRepositoryRef, error) {
	repo, err := q.service.GetSourceRepository(ctx, id)
	if err != nil {
		return build.SourceRepositoryRef{}, err
	}
	return build.SourceRepositoryRef{ID: repo.ID, HTTPURL: repo.HTTPURL, SSHURL: repo.SSHURL}, nil
}

type appForBuild struct {
	service  *appenv.Service
	projects *tenantproject.Service
}

func (q appForBuild) GetApplication(ctx context.Context, id shared.ID) (build.ApplicationRef, error) {
	app, err := q.service.GetApplication(ctx, id)
	if err != nil {
		return build.ApplicationRef{}, err
	}
	ref := build.ApplicationRef{ID: app.ID, TenantID: app.TenantID, ProjectID: app.ProjectID, Name: app.Name, RuntimeEnvironments: toBuildRuntimeEnvironments(app.RuntimeEnvironments)}
	if q.projects != nil {
		project, err := q.projects.GetProject(ctx, app.ProjectID)
		if err != nil {
			return build.ApplicationRef{}, err
		}
		tenant, err := q.projects.GetTenant(ctx, project.TenantID)
		if err != nil {
			return build.ApplicationRef{}, err
		}
		ref.ProjectName = project.Name
		ref.TenantName = tenant.Name
	}
	return ref, nil
}

func (q appForBuild) GetApplicationSource(ctx context.Context, applicationID shared.ID) (build.ApplicationSourceRef, error) {
	source, err := q.service.GetApplicationSource(ctx, applicationID)
	if err != nil {
		return build.ApplicationSourceRef{}, err
	}
	return toBuildApplicationSourceRef(source), nil
}

func (q appForBuild) ListApplicationSources(ctx context.Context, applicationID shared.ID) ([]build.ApplicationSourceRef, error) {
	sources, err := q.service.ListApplicationSources(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	out := make([]build.ApplicationSourceRef, 0, len(sources))
	for _, source := range sources {
		out = append(out, toBuildApplicationSourceRef(source))
	}
	return out, nil
}

type workloadForBuild struct{ service *appenv.Service }

func (q workloadForBuild) GetWorkload(ctx context.Context, applicationID shared.ID, workloadID shared.ID) (build.WorkloadRef, error) {
	workload, err := q.service.GetWorkload(ctx, applicationID, workloadID)
	if err != nil {
		return build.WorkloadRef{}, err
	}
	return build.WorkloadRef{ID: workload.ID, TenantID: workload.TenantID, ProjectID: workload.ProjectID, ApplicationID: workload.ApplicationID, PipelineID: workload.PipelineID, Name: workload.Name, DisplayName: workload.DisplayName, Status: string(workload.Status)}, nil
}

func (q workloadForBuild) ListEnabledWorkloads(ctx context.Context, applicationID shared.ID) ([]build.WorkloadRef, error) {
	workloads, err := q.service.ListEnabledWorkloads(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	out := make([]build.WorkloadRef, 0, len(workloads))
	for _, workload := range workloads {
		out = append(out, build.WorkloadRef{ID: workload.ID, TenantID: workload.TenantID, ProjectID: workload.ProjectID, ApplicationID: workload.ApplicationID, PipelineID: workload.PipelineID, Name: workload.Name, DisplayName: workload.DisplayName, Status: string(workload.Status)})
	}
	return out, nil
}

func (q workloadForBuild) ListEnabledWorkloadsByPipeline(ctx context.Context, applicationID shared.ID, pipelineID shared.ID) ([]build.WorkloadRef, error) {
	workloads, err := q.service.ListEnabledWorkloads(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	out := make([]build.WorkloadRef, 0, len(workloads))
	for _, workload := range workloads {
		if workload.PipelineID == pipelineID {
			out = append(out, build.WorkloadRef{ID: workload.ID, TenantID: workload.TenantID, ProjectID: workload.ProjectID, ApplicationID: workload.ApplicationID, PipelineID: workload.PipelineID, Name: workload.Name, DisplayName: workload.DisplayName, Status: string(workload.Status)})
		}
	}
	return out, nil
}

type buildPipelineForAppEnv struct{ service *build.Service }

func (q buildPipelineForAppEnv) GetBuildPipeline(ctx context.Context, id shared.ID) (appenv.BuildPipelineRef, error) {
	pipeline, err := q.service.GetBuildPipeline(ctx, id)
	if err != nil {
		return appenv.BuildPipelineRef{}, err
	}
	return appenv.BuildPipelineRef{ID: pipeline.ID, ApplicationID: pipeline.ApplicationID, Name: pipeline.Name, DisplayName: pipeline.DisplayName, Status: string(pipeline.Status)}, nil
}

type workloadForDelivery struct{ service *appenv.Service }

func (q workloadForDelivery) ListEnabledWorkloads(ctx context.Context, applicationID shared.ID) ([]delivery.WorkloadRef, error) {
	workloads, err := q.service.ListEnabledWorkloads(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	out := make([]delivery.WorkloadRef, 0, len(workloads))
	for _, workload := range workloads {
		out = append(out, delivery.WorkloadRef{ID: workload.ID, TenantID: workload.TenantID, ProjectID: workload.ProjectID, ApplicationID: workload.ApplicationID, Name: workload.Name, DisplayName: workload.DisplayName, Status: string(workload.Status)})
	}
	return out, nil
}

type jenkinsTemplateForAppEnv struct{ repo build.Repository }

func (q jenkinsTemplateForAppEnv) GetJenkinsJobTemplate(ctx context.Context, id shared.ID) (appenv.JenkinsTemplateRef, error) {
	template, err := q.repo.GetJenkinsJobTemplate(ctx, id)
	if err != nil {
		return appenv.JenkinsTemplateRef{}, err
	}
	return appenv.JenkinsTemplateRef{ID: template.ID, Status: string(template.Status)}, nil
}

func (q jenkinsTemplateForAppEnv) FindDefaultJenkinsJobTemplate(ctx context.Context) (appenv.JenkinsTemplateRef, error) {
	template, err := q.repo.FindDefaultJenkinsJobTemplate(ctx)
	if err != nil {
		return appenv.JenkinsTemplateRef{}, err
	}
	return appenv.JenkinsTemplateRef{ID: template.ID, Status: string(template.Status)}, nil
}

type buildEnvironmentForAppEnv struct{ repo build.Repository }

func (q buildEnvironmentForAppEnv) GetBuildEnvironment(ctx context.Context, id shared.ID) (appenv.BuildEnvironmentRef, error) {
	environment, err := q.repo.GetBuildEnvironment(ctx, id)
	if err != nil {
		return appenv.BuildEnvironmentRef{}, err
	}
	return toAppenvBuildEnvironmentRef(environment), nil
}

func (q buildEnvironmentForAppEnv) FindDefaultBuildEnvironment(ctx context.Context) (appenv.BuildEnvironmentRef, error) {
	environment, err := q.repo.FindDefaultBuildEnvironment(ctx)
	if err != nil {
		return appenv.BuildEnvironmentRef{}, err
	}
	return toAppenvBuildEnvironmentRef(environment), nil
}

func toAppenvBuildEnvironmentRef(environment build.BuildEnvironment) appenv.BuildEnvironmentRef {
	return appenv.BuildEnvironmentRef{ID: environment.ID, Status: string(environment.Status)}
}

type runtimeEnvironmentForAppEnv struct{ repo build.Repository }

func (q runtimeEnvironmentForAppEnv) GetRuntimeEnvironment(ctx context.Context, id shared.ID) (appenv.RuntimeEnvironmentRef, error) {
	environment, err := q.repo.GetRuntimeEnvironment(ctx, id)
	if err != nil {
		return appenv.RuntimeEnvironmentRef{}, err
	}
	return toAppenvRuntimeEnvironmentRef(environment), nil
}

func (q runtimeEnvironmentForAppEnv) FindDefaultRuntimeEnvironment(ctx context.Context) (appenv.RuntimeEnvironmentRef, error) {
	environment, err := q.repo.FindDefaultRuntimeEnvironment(ctx)
	if err != nil {
		return appenv.RuntimeEnvironmentRef{}, err
	}
	return toAppenvRuntimeEnvironmentRef(environment), nil
}

func toAppenvRuntimeEnvironmentRef(environment build.RuntimeEnvironment) appenv.RuntimeEnvironmentRef {
	return appenv.RuntimeEnvironmentRef{ID: environment.ID, Name: environment.Name, Status: string(environment.Status), RuntimeBaseImage: environment.RuntimeBaseImage, ArtifactDeployPath: environment.ArtifactDeployPath, DockerfilePath: environment.DockerfilePath, SelectorLabels: environment.SelectorLabels}
}

type clusterForDelivery struct{ repo clusteragent.Repository }

func (q clusterForDelivery) GetCluster(ctx context.Context, id shared.ID) (delivery.ClusterRef, error) {
	cluster, err := q.repo.GetCluster(ctx, id)
	if err != nil {
		return delivery.ClusterRef{}, err
	}
	return delivery.ClusterRef{ID: cluster.ID, TenantID: cluster.TenantID, Name: cluster.Name, Labels: cluster.Labels}, nil
}

type tenantForClusterAgent struct{ service *tenantproject.Service }

func (q tenantForClusterAgent) GetTenant(ctx context.Context, id shared.ID) (clusteragent.TenantRef, error) {
	tenant, err := q.service.GetTenant(ctx, id)
	if err != nil {
		return clusteragent.TenantRef{}, err
	}
	return clusteragent.TenantRef{ID: tenant.ID}, nil
}

type stageClusterForRuntime struct {
	apps     *appenv.Service
	delivery delivery.Repository
}

func (q stageClusterForRuntime) ResolveStageCluster(ctx context.Context, applicationID shared.ID, stageKey string) (clusteragent.StageClusterRef, error) {
	app, err := q.apps.GetApplication(ctx, applicationID)
	if err != nil {
		return clusteragent.StageClusterRef{}, err
	}
	bindings, err := q.delivery.ListStageClusterBindings(ctx, app.TenantID, stageKey)
	if err != nil {
		return clusteragent.StageClusterRef{}, err
	}
	for _, binding := range bindings {
		if binding.Status == delivery.StageClusterBindingActive {
			return clusteragent.StageClusterRef{ClusterID: binding.ClusterID, TenantID: app.TenantID}, nil
		}
	}
	return clusteragent.StageClusterRef{TenantID: app.TenantID}, nil
}

type runtimeEnvironmentSyncerForAppEnv struct{ service *appenv.Service }

func (s runtimeEnvironmentSyncerForAppEnv) SyncRuntimeEnvironment(ctx context.Context, actor identityaccess.Subject, environment build.RuntimeEnvironment) error {
	if s.service == nil {
		return nil
	}
	_, err := s.service.SyncRuntimeEnvironmentSnapshot(ctx, appenv.RuntimeEnvironmentSnapshotInput{Actor: actor, Environment: toAppenvRuntimeEnvironmentRef(environment)})
	return err
}

type appForDelivery struct {
	service  *appenv.Service
	projects *tenantproject.Service
}

func (q appForDelivery) GetApplication(ctx context.Context, id shared.ID) (delivery.ApplicationRef, error) {
	app, err := q.service.GetApplication(ctx, id)
	if err != nil {
		return delivery.ApplicationRef{}, err
	}
	ref := delivery.ApplicationRef{ID: app.ID, TenantID: app.TenantID, ProjectID: app.ProjectID, Name: app.Name}
	if q.projects != nil {
		project, err := q.projects.GetProject(ctx, app.ProjectID)
		if err != nil {
			return delivery.ApplicationRef{}, err
		}
		ref.ProjectName = project.Name
	}
	return ref, nil
}

type stageRuntimeForDelivery struct{ service *appenv.Service }

type stageSyncForDelivery struct{ service *appenv.Service }

func (s stageSyncForDelivery) SyncApplicationStages(ctx context.Context, input delivery.SyncApplicationStagesInput) error {
	return nil
}

func (q stageRuntimeForDelivery) ListStageRuntimeStates(ctx context.Context, applicationID shared.ID) (map[string]delivery.StageRuntimeState, error) {
	states, err := q.service.ListApplicationStageStates(ctx, applicationID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]delivery.StageRuntimeState, len(states))
	for _, state := range states {
		runtimeState := delivery.StageRuntimeState{
			ApplicationID: state.ApplicationID,
			StageKey:      state.StageKey,
			Message:       state.Message,
		}
		switch state.Status {
		case appenv.ApplicationStageStatusRunning:
			runtimeState.SyncStatus = "Synced"
			runtimeState.HealthStatus = "Healthy"
			runtimeState.OperationState = "succeeded"
		case appenv.ApplicationStageStatusDegraded:
			runtimeState.SyncStatus = "OutOfSync"
			runtimeState.HealthStatus = "Degraded"
			runtimeState.OperationState = "failed"
		case appenv.ApplicationStageStatusDeploying:
			runtimeState.SyncStatus = "OutOfSync"
			runtimeState.HealthStatus = "Progressing"
			runtimeState.OperationState = "running"
		}
		out[state.StageKey] = runtimeState
	}
	return out, nil
}

type buildForDelivery struct {
	service *build.Service
	repo    build.Repository
}

func (q buildForDelivery) GetBuildRun(ctx context.Context, id shared.ID) (delivery.BuildRunRef, error) {
	run, err := q.service.GetBuildRun(ctx, id)
	if err != nil {
		return delivery.BuildRunRef{}, err
	}
	return toDeliveryBuildRunRef(run), nil
}

func (q buildForDelivery) ListBuildRuns(ctx context.Context, applicationID shared.ID, page shared.PageRequest) (shared.PageResult[delivery.BuildRunRef], error) {
	runs, err := q.service.ListBuildRuns(ctx, applicationID, page)
	if err != nil {
		return shared.PageResult[delivery.BuildRunRef]{}, err
	}
	items := make([]delivery.BuildRunRef, 0, len(runs.Items))
	for _, run := range runs.Items {
		items = append(items, toDeliveryBuildRunRef(run))
	}
	return shared.PageResult[delivery.BuildRunRef]{Items: items, Total: runs.Total, Page: runs.Page, PageSize: runs.PageSize}, nil
}

func (q buildForDelivery) GetBuildArtifact(ctx context.Context, id shared.ID) (delivery.BuildArtifactRef, error) {
	artifact, err := q.repo.GetArtifact(ctx, id)
	if err != nil {
		return delivery.BuildArtifactRef{}, err
	}
	return toDeliveryBuildArtifactRef(artifact), nil
}

func (q buildForDelivery) ListBuildArtifacts(ctx context.Context, buildRunID shared.ID) ([]delivery.BuildArtifactRef, error) {
	artifacts, err := q.service.ListBuildArtifacts(ctx, buildRunID)
	if err != nil {
		return nil, err
	}
	out := make([]delivery.BuildArtifactRef, 0, len(artifacts))
	for _, artifact := range artifacts {
		out = append(out, toDeliveryBuildArtifactRef(artifact))
	}
	return out, nil
}

type appForGitOps struct{ service *appenv.Service }

func (q appForGitOps) GetApplication(ctx context.Context, id shared.ID) (gitops.ApplicationRef, error) {
	app, err := q.service.GetApplication(ctx, id)
	if err != nil {
		return gitops.ApplicationRef{}, err
	}
	return gitops.ApplicationRef{ID: app.ID, TenantID: app.TenantID, ProjectID: app.ProjectID, Name: app.Name}, nil
}

type workloadForGitOps struct{ service *appenv.Service }

func (q workloadForGitOps) GetWorkload(ctx context.Context, applicationID shared.ID, workloadID shared.ID) (gitops.WorkloadRef, error) {
	workload, err := q.service.GetWorkload(ctx, applicationID, workloadID)
	if err != nil {
		return gitops.WorkloadRef{}, err
	}
	return gitops.WorkloadRef{ID: workload.ID, TenantID: workload.TenantID, ProjectID: workload.ProjectID, ApplicationID: workload.ApplicationID, Name: workload.Name, DisplayName: workload.DisplayName, WorkloadType: string(workload.WorkloadType)}, nil
}

func (q workloadForGitOps) GetWorkloadStageConfig(ctx context.Context, workloadID shared.ID, stageKey string) (gitops.WorkloadStageConfigRef, error) {
	config, err := q.service.GetWorkloadStageConfig(ctx, workloadID, stageKey)
	if err != nil {
		return gitops.WorkloadStageConfigRef{}, err
	}
	return toGitOpsWorkloadStageConfig(config), nil
}

func (q workloadForGitOps) GetWorkloadDefaultConfig(ctx context.Context, workloadID shared.ID) (gitops.WorkloadStageConfigRef, error) {
	config, err := q.service.GetWorkloadDefaultConfig(ctx, workloadID)
	if err != nil {
		return gitops.WorkloadStageConfigRef{}, err
	}
	return toGitOpsWorkloadStageConfig(config), nil
}

func toGitOpsWorkloadStageConfig(config appenv.WorkloadStageConfig) gitops.WorkloadStageConfigRef {
	out := gitops.WorkloadStageConfigRef{
		Replicas:         config.Replicas,
		ResourceRequests: gitops.WorkloadResourceListRef{CPU: config.ResourceRequests.CPU, Memory: config.ResourceRequests.Memory},
		ResourceLimits:   gitops.WorkloadResourceListRef{CPU: config.ResourceLimits.CPU, Memory: config.ResourceLimits.Memory},
		ValuesOverride:   config.ValuesOverride,
	}
	for _, port := range config.ServicePorts {
		out.ServicePorts = append(out.ServicePorts, gitops.WorkloadServicePortRef{Name: port.Name, Port: port.Port, TargetPort: port.TargetPort, Protocol: port.Protocol})
	}
	for _, probe := range config.Probes {
		out.Probes = append(out.Probes, gitops.WorkloadProbeRef{Name: probe.Name, Type: probe.Type, Path: probe.Path, Port: probe.Port, Command: probe.Command, InitialDelaySeconds: probe.InitialDelaySeconds, PeriodSeconds: probe.PeriodSeconds})
	}
	for _, env := range config.EnvVars {
		out.EnvVars = append(out.EnvVars, gitops.WorkloadEnvVarRef{Name: env.Name, Value: env.Value})
	}
	for _, host := range config.IngressHosts {
		out.IngressHosts = append(out.IngressHosts, gitops.WorkloadIngressHostRef{Host: host.Host, Path: host.Path})
	}
	for _, secret := range config.SecretRefs {
		out.SecretRefs = append(out.SecretRefs, gitops.WorkloadSecretRef{Name: secret.Name, SecretRef: secret.SecretRef})
	}
	for _, file := range config.ConfigFiles {
		out.ConfigFiles = append(out.ConfigFiles, gitops.WorkloadConfigFileRef{MountPath: file.MountPath, Content: file.Content, Base64Encoded: file.Base64Encoded})
	}
	for _, dir := range config.WritableDirs {
		out.WritableDirs = append(out.WritableDirs, gitops.WorkloadWritableDirRef{MountPath: dir.MountPath, SizeLimit: dir.SizeLimit, OwnerGroup: dir.OwnerGroup, Mode: dir.Mode})
	}
	for _, mount := range config.VolumeMounts {
		out.VolumeMounts = append(out.VolumeMounts, gitops.WorkloadVolumeMountRef{Name: mount.Name, MountPath: mount.MountPath})
	}
	for _, init := range config.InitContainers {
		out.InitContainers = append(out.InitContainers, gitops.WorkloadInitContainerRef{Name: init.Name, Image: init.Image, Command: init.Command})
	}
	return out
}

type stageUpdater struct{ service *appenv.Service }

func (u stageUpdater) UpdateFromAgent(ctx context.Context, report clusteragent.StatusReport) error {
	for _, appStatus := range report.Applications {
		if appStatus.ApplicationID.IsZero() || strings.TrimSpace(appStatus.StageKey) == "" {
			continue
		}
		status := appenv.ApplicationStageStatusDeploying
		if strings.EqualFold(appStatus.HealthStatus, "Healthy") && strings.EqualFold(appStatus.SyncStatus, "Synced") {
			status = appenv.ApplicationStageStatusRunning
		} else if strings.EqualFold(appStatus.HealthStatus, "Degraded") {
			status = appenv.ApplicationStageStatusDegraded
		}
		reportedAt := report.ReportedAt
		if reportedAt.IsZero() {
			reportedAt = time.Now()
		}
		if _, err := u.service.UpdateApplicationStageState(ctx, appenv.UpdateApplicationStageStateInput{ApplicationID: appStatus.ApplicationID, StageKey: appStatus.StageKey, Status: status, Message: appStatus.Message, ReportedAt: &reportedAt}); err != nil {
			if shared.CodeOf(err) == shared.CodeNotFound {
				continue
			}
			return err
		}
	}
	return nil
}

func toBuildSpec(spec appenv.BuildSpec) build.BuildSpec {
	return build.BuildSpec{SourcePath: spec.SourcePath, BuildCommand: spec.BuildCommand, ArtifactCopyCommand: spec.ArtifactCopyCommand, RuntimeBaseImage: spec.RuntimeBaseImage, ArtifactDeployPath: spec.ArtifactDeployPath, DefaultRef: spec.DefaultRef}
}

func toBuildApplicationSourceRef(source appenv.ApplicationSource) build.ApplicationSourceRef {
	return build.ApplicationSourceRef{ApplicationID: source.ApplicationID, Key: source.Key, DisplayName: source.DisplayName, SourceRepositoryID: source.SourceRepositoryID, JenkinsTemplateID: source.JenkinsTemplateID, BuildEnvironmentID: source.BuildEnvironmentID, SourcePath: source.SourcePath, BuildSpec: toBuildSpec(source.BuildSpec), IsPrimary: source.IsPrimary}
}

func toBuildRuntimeEnvironments(environments []appenv.ApplicationRuntimeEnvironment) []build.RuntimeEnvironmentRef {
	out := make([]build.RuntimeEnvironmentRef, 0, len(environments))
	for _, environment := range environments {
		out = append(out, build.RuntimeEnvironmentRef{ID: environment.ID, Name: environment.Name, RuntimeBaseImage: environment.RuntimeBaseImage, ArtifactDeployPath: environment.ArtifactDeployPath, DockerfilePath: environment.DockerfilePath, SelectorLabels: environment.SelectorLabels})
	}
	return out
}

func toDeliveryBuildArtifactRef(artifact build.BuildArtifact) delivery.BuildArtifactRef {
	return delivery.BuildArtifactRef{ID: artifact.ID, BuildRunID: artifact.BuildRunID, ApplicationID: artifact.ApplicationID, WorkloadID: artifact.WorkloadID, SourceKey: artifact.SourceKey, URI: artifact.URI, Digest: artifact.Digest, IsPrimary: artifact.IsPrimary, SelectorLabels: artifact.SelectorLabels, Metadata: artifact.Metadata}
}

func toDeliveryBuildRunRef(run build.BuildRun) delivery.BuildRunRef {
	return delivery.BuildRunRef{ID: run.ID, TenantID: run.TenantID, ProjectID: run.ProjectID, ApplicationID: run.ApplicationID, PipelineID: run.PipelineID, PipelineName: run.PipelineName, PipelineDisplayName: run.PipelineDisplayName, CommitSHA: run.CommitSHA, Status: string(run.Status)}
}

func applicationType(spec appenv.BuildSpec) string {
	return "Spring Boot"
}

func buildStatusText(status build.BuildRunStatus) string {
	switch status {
	case build.BuildRunSucceeded:
		return "成功"
	case build.BuildRunFailed:
		return "失败"
	case build.BuildRunRunning:
		return "运行中"
	case build.BuildRunAborted:
		return "已中止"
	default:
		return "排队中"
	}
}

func pageFromRequest(r *http.Request) shared.PageRequest {
	return shared.PageRequest{Page: 1, PageSize: 100}.Normalize()
}

func formatLocal(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04")
}

func formatOptional(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return formatLocal(*t)
}

func durationText(start *time.Time, finish *time.Time) string {
	if start == nil || finish == nil || finish.Before(*start) {
		return "-"
	}
	seconds := int(finish.Sub(*start).Seconds())
	return strings.TrimSpace(strings.Join([]string{durationPart(seconds/60, "分"), durationPart(seconds%60, "秒")}, " "))
}

func durationPart(value int, unit string) string {
	if value <= 0 {
		return ""
	}
	return strconv.Itoa(value) + " " + unit
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func decodeDevelopmentJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeDevelopmentJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": string(shared.CodeInvalidArgument), "message": "请求体不是有效 JSON"}})
		return false
	}
	return true
}

func writeDevelopmentJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeDevelopmentError(w http.ResponseWriter, err error) {
	writeDevelopmentJSON(w, shared.HTTPStatusOf(err), map[string]any{"error": map[string]string{"code": string(shared.CodeOf(err)), "message": developmentErrorMessage(err)}})
}

func developmentErrorMessage(err error) string {
	var appErr *shared.AppError
	if errors.As(err, &appErr) && strings.TrimSpace(appErr.Message) != "" {
		return appErr.Message
	}
	return "请求处理失败"
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
