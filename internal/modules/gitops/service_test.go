package gitops

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/shareinto/paas/internal/modules/clusteragent"
	"github.com/shareinto/paas/internal/modules/delivery"
	"github.com/shareinto/paas/internal/shared"
)

type staticIDs struct{ ids []shared.ID }

func (s *staticIDs) NewID(string) (shared.ID, error) {
	id := s.ids[0]
	s.ids = s.ids[1:]
	return id, nil
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type auditRecorder struct{ events []AuditEvent }

func (r *auditRecorder) Log(_ context.Context, event AuditEvent) error {
	r.events = append(r.events, event)
	return nil
}

type gitopsFailingIDs struct{ err error }

func (g gitopsFailingIDs) NewID(string) (shared.ID, error) {
	return "", g.err
}

type gitopsErrRepo struct {
	*MemoryRepository
	createTemplateErr         error
	updateTemplateErr         error
	createTemplateRevisionErr error
	createDeploymentErr       error
	createManifestRevisionErr error
	updateDeploymentErr       error
	createDeploymentEventErr  error
}

func (r *gitopsErrRepo) CreateTemplate(ctx context.Context, template DeploymentTemplate) error {
	if r.createTemplateErr != nil {
		return r.createTemplateErr
	}
	return r.MemoryRepository.CreateTemplate(ctx, template)
}

func (r *gitopsErrRepo) UpdateTemplate(ctx context.Context, template DeploymentTemplate) error {
	if r.updateTemplateErr != nil {
		return r.updateTemplateErr
	}
	return r.MemoryRepository.UpdateTemplate(ctx, template)
}

func (r *gitopsErrRepo) CreateTemplateRevision(ctx context.Context, revision DeploymentTemplateRevision) error {
	if r.createTemplateRevisionErr != nil {
		return r.createTemplateRevisionErr
	}
	return r.MemoryRepository.CreateTemplateRevision(ctx, revision)
}

func (r *gitopsErrRepo) CreateDeployment(ctx context.Context, deployment Deployment) error {
	if r.createDeploymentErr != nil {
		return r.createDeploymentErr
	}
	return r.MemoryRepository.CreateDeployment(ctx, deployment)
}

func (r *gitopsErrRepo) CreateManifestRevision(ctx context.Context, revision ManifestRevision) error {
	if r.createManifestRevisionErr != nil {
		return r.createManifestRevisionErr
	}
	return r.MemoryRepository.CreateManifestRevision(ctx, revision)
}

func (r *gitopsErrRepo) UpdateDeployment(ctx context.Context, deployment Deployment) error {
	if r.updateDeploymentErr != nil {
		return r.updateDeploymentErr
	}
	return r.MemoryRepository.UpdateDeployment(ctx, deployment)
}

func (r *gitopsErrRepo) CreateDeploymentEvent(ctx context.Context, event DeploymentEvent) error {
	if r.createDeploymentEventErr != nil {
		return r.createDeploymentEventErr
	}
	return r.MemoryRepository.CreateDeploymentEvent(ctx, event)
}

type gitopsErrManifest struct{ err error }

func (m gitopsErrManifest) ReadFile(context.Context, string, string) (string, error) {
	return "", m.err
}
func (m gitopsErrManifest) CommitFiles(context.Context, CommitSpec) (CommitResult, error) {
	return CommitResult{}, m.err
}
func (m gitopsErrManifest) CreateMergeRequest(context.Context, MergeRequestSpec) (MergeRequestResult, error) {
	return MergeRequestResult{}, m.err
}
func (m gitopsErrManifest) GetMergeRequest(context.Context, string) (MergeRequest, error) {
	return MergeRequest{}, m.err
}

type errAppQuery struct{ err error }

func (q errAppQuery) GetApplication(context.Context, shared.ID) (ApplicationRef, error) {
	return ApplicationRef{}, q.err
}

type errEnvQuery struct{ err error }

func (q errEnvQuery) GetEnvironment(context.Context, shared.ID) (EnvironmentRef, error) {
	return EnvironmentRef{}, q.err
}
func (q errEnvQuery) GetActiveBinding(context.Context, shared.ID) (ClusterBindingRef, error) {
	return ClusterBindingRef{}, q.err
}

type appQuery map[shared.ID]ApplicationRef

func (q appQuery) GetApplication(_ context.Context, id shared.ID) (ApplicationRef, error) {
	return q[id], nil
}

type envQuery struct {
	envs     map[shared.ID]EnvironmentRef
	bindings map[shared.ID]ClusterBindingRef
}

func (q envQuery) GetEnvironment(_ context.Context, id shared.ID) (EnvironmentRef, error) {
	return q.envs[id], nil
}

func (q envQuery) GetActiveBinding(_ context.Context, id shared.ID) (ClusterBindingRef, error) {
	return q.bindings[id], nil
}

func newTestService(ids []shared.ID) (*Service, *FakeManifestRepository, *auditRecorder) {
	manifest := NewFakeManifestRepository()
	audit := &auditRecorder{}
	svc := NewService(Options{
		Repository: NewMemoryRepository(), ManifestRepo: manifest,
		Application: appQuery{"app_1": {ID: "app_1", TenantID: "tenant_1", ProjectID: "project_1", Name: "order-api"}},
		Environment: envQuery{envs: map[shared.ID]EnvironmentRef{"env_dev": {ID: "env_dev", TenantID: "tenant_1", ProjectID: "project_1", ApplicationID: "app_1", Name: "dev"}, "env_prod": {ID: "env_prod", TenantID: "tenant_1", ProjectID: "project_1", ApplicationID: "app_1", Name: "prod"}}, bindings: map[shared.ID]ClusterBindingRef{"env_dev": {ID: "binding_dev", EnvironmentID: "env_dev", Namespace: "order-dev", Active: true}, "env_prod": {ID: "binding_prod", EnvironmentID: "env_prod", Namespace: "order-prod", Active: true}}},
		Audit:       audit, IDGenerator: &staticIDs{ids: ids}, Clock: fixedClock{now: time.Date(2026, 5, 30, 13, 0, 0, 0, time.UTC)},
	})
	return svc, manifest, audit
}

func TestApplicationTemplateCopyDoesNotMutatePlatformTemplate(t *testing.T) {
	svc, _, _ := newTestService([]shared.ID{"deployment_template_platform", "deployment_template_revision_platform", "deployment_template_app", "deployment_template_revision_app", "deployment_template_revision_app2"})
	base, err := svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app")
	if err != nil {
		t.Fatalf("platform template: %v", err)
	}
	appTemplate, err := svc.CreateApplicationTemplate(context.Background(), "app_1", "java", "user_1")
	if err != nil {
		t.Fatalf("app template: %v", err)
	}
	if _, err := svc.UpdateApplicationTemplate(context.Background(), "app_1", "initContainers:\n- name: init\n  command: ['mkdir','-p','/data']\nvolumeMounts: []\nsecurityContext: {}", "user_1"); err != nil {
		t.Fatalf("update template: %v", err)
	}
	unchanged, _ := svc.repo.GetTemplate(context.Background(), base.ID)
	changed, _ := svc.repo.GetTemplate(context.Background(), appTemplate.ID)
	if unchanged.Content == changed.Content || !strings.Contains(changed.Content, "initContainers") {
		t.Fatalf("application template should be independent")
	}
}

func TestTemplatePolicyValidationRejectsPrivilegedAndAllowsInitContainer(t *testing.T) {
	svc, _, _ := newTestService(nil)
	allowed := svc.ValidateTemplate(context.Background(), "initContainers:\n- name: init\nsecurityContext:\n  runAsNonRoot: true")
	if !allowed.Valid {
		t.Fatalf("expected initContainer template to be valid: %#v", allowed)
	}
	rejected := svc.ValidateTemplate(context.Background(), "securityContext:\n  privileged: true")
	if rejected.Valid {
		t.Fatalf("expected privileged template to be rejected")
	}
}

func TestApplyPromotionCommitsDevCreatesMRForProdAndUpdatesDeploymentFromAgent(t *testing.T) {
	ids := []shared.ID{
		"deployment_template_platform", "deployment_template_revision_platform", "deployment_template_app", "deployment_template_revision_app",
		"deployment_1", "manifest_revision_1", "deployment_event_1",
		"deployment_2", "manifest_revision_2", "deployment_event_2", "deployment_event_3",
	}
	svc, manifest, audit := newTestService(ids)
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app")
	_, _ = svc.CreateApplicationTemplate(context.Background(), "app_1", "java", "user_1")
	dev, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{PromotionID: "promotion_dev", FreightID: "freight_1", ApplicationID: "app_1", EnvironmentID: "env_dev", ImageURI: "registry/order-api:v1", ImageDigest: "sha256:old"})
	if err != nil {
		t.Fatalf("apply dev: %v", err)
	}
	prod, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{PromotionID: "promotion_prod", FreightID: "freight_2", ApplicationID: "app_1", EnvironmentID: "env_prod", ImageURI: "registry/order-api:v0", ImageDigest: "sha256:rollback", IsRollback: true})
	if err != nil {
		t.Fatalf("apply prod: %v", err)
	}
	if dev.ManifestRevision == "" || len(manifest.Commits) != 1 || len(manifest.MRs) != 1 || prod.ManifestRevision == "" {
		t.Fatalf("unexpected manifest operations: commits=%d mrs=%d", len(manifest.Commits), len(manifest.MRs))
	}
	if !strings.Contains(manifest.Files["apps/order-api/dev/values.yaml"], "sha256:old") || !strings.Contains(manifest.Files["apps/order-api/prod/values.yaml"], "sha256:rollback") {
		t.Fatalf("values files did not contain expected digests: %#v", manifest.Files)
	}
	if err := svc.UpdateFromAgent(context.Background(), clusteragent.StatusReport{Applications: []clusteragent.ApplicationStatus{{DeploymentID: "deployment_1", SyncStatus: "Synced", HealthStatus: "Healthy", Message: "ok"}}}); err != nil {
		t.Fatalf("agent update: %v", err)
	}
	deployment, _ := svc.GetDeployment(context.Background(), "deployment_1")
	if deployment.Status != DeploymentSucceeded || deployment.CompletedAt == nil {
		t.Fatalf("deployment not updated from agent: %#v", deployment)
	}
	if len(audit.events) < 5 {
		t.Fatalf("expected template and manifest audit events, got %#v", audit.events)
	}
	if audit.events[len(audit.events)-2].Action != "manifest_revision.create" || audit.events[len(audit.events)-1].Action != "deployment.create" {
		t.Fatalf("unexpected final audit events: %#v", audit.events)
	}
}

func TestGitOpsHTTPHandlerCoversTemplateAndDeploymentAPIs(t *testing.T) {
	ids := []shared.ID{
		"deployment_template_platform", "deployment_template_revision_platform",
		"deployment_template_app", "deployment_template_revision_app",
		"deployment_template_revision_app2",
		"deployment_1", "manifest_revision_1", "deployment_event_1",
	}
	svc, manifest, _ := newTestService(ids)
	mux := http.NewServeMux()
	NewHandler(svc).Register(mux)

	createPlatform := httptest.NewRecorder()
	mux.ServeHTTP(createPlatform, httptest.NewRequest(http.MethodPost, "/api/deployment-templates/platform", bytes.NewBufferString(`{"name":"java","content":"containers:\n- name: app"}`)))
	if createPlatform.Code != http.StatusCreated {
		t.Fatalf("create platform status=%d body=%s", createPlatform.Code, createPlatform.Body.String())
	}
	createApp := httptest.NewRecorder()
	mux.ServeHTTP(createApp, httptest.NewRequest(http.MethodPost, "/api/apps/app_1/deployment-template", bytes.NewBufferString(`{"base_template_name":"java","actor_id":"user_1"}`)))
	if createApp.Code != http.StatusCreated {
		t.Fatalf("create app template status=%d body=%s", createApp.Code, createApp.Body.String())
	}
	update := httptest.NewRecorder()
	mux.ServeHTTP(update, httptest.NewRequest(http.MethodPut, "/api/apps/app_1/deployment-template", bytes.NewBufferString(`{"content":"containers:\n- name: app\nsecurityContext:\n  runAsNonRoot: true","actor_id":"user_1"}`)))
	if update.Code != http.StatusOK {
		t.Fatalf("update app template status=%d body=%s", update.Code, update.Body.String())
	}
	getTemplate := httptest.NewRecorder()
	mux.ServeHTTP(getTemplate, httptest.NewRequest(http.MethodGet, "/api/apps/app_1/deployment-template", nil))
	if getTemplate.Code != http.StatusOK || !bytes.Contains(getTemplate.Body.Bytes(), []byte("current_revision")) {
		t.Fatalf("get template status=%d body=%s", getTemplate.Code, getTemplate.Body.String())
	}
	validate := httptest.NewRecorder()
	mux.ServeHTTP(validate, httptest.NewRequest(http.MethodPost, "/api/apps/app_1/deployment-template/validate", bytes.NewBufferString(`{"content":"securityContext:\n  privileged: true"}`)))
	if validate.Code != http.StatusOK || !bytes.Contains(validate.Body.Bytes(), []byte(`"valid":false`)) {
		t.Fatalf("validate status=%d body=%s", validate.Code, validate.Body.String())
	}

	if _, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{PromotionID: "promotion_dev", FreightID: "freight_1", ApplicationID: "app_1", EnvironmentID: "env_dev", ImageURI: "registry/order-api:v1", ImageDigest: "sha256:1"}); err != nil {
		t.Fatalf("apply promotion: %v", err)
	}
	deployment, err := svc.repo.FindDeploymentByPromotion(context.Background(), "promotion_dev")
	if err != nil {
		t.Fatalf("get deployment by promotion: %v", err)
	}
	if len(manifest.Commits) != 1 {
		t.Fatalf("expected manifest commit")
	}
	listDeployments := httptest.NewRecorder()
	mux.ServeHTTP(listDeployments, httptest.NewRequest(http.MethodGet, "/api/apps/app_1/deployments?page=1&page_size=10", nil))
	if listDeployments.Code != http.StatusOK {
		t.Fatalf("list deployments status=%d body=%s", listDeployments.Code, listDeployments.Body.String())
	}
	var page shared.PageResult[Deployment]
	if err := json.Unmarshal(listDeployments.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode deployments: %v", err)
	}
	if page.Total != 1 || page.Items[0].ID != deployment.ID {
		t.Fatalf("unexpected deployments: %#v", page)
	}
	getDeployment := httptest.NewRecorder()
	mux.ServeHTTP(getDeployment, httptest.NewRequest(http.MethodGet, "/api/deployments/"+deployment.ID.String(), nil))
	if getDeployment.Code != http.StatusOK {
		t.Fatalf("get deployment status=%d body=%s", getDeployment.Code, getDeployment.Body.String())
	}
	invalid := httptest.NewRecorder()
	mux.ServeHTTP(invalid, httptest.NewRequest(http.MethodPost, "/api/apps/app_1/deployment-template/validate", bytes.NewBufferString(`{bad`)))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid json status=%d body=%s", invalid.Code, invalid.Body.String())
	}
}

func TestFakeManifestRepositoryReadAndMRLookup(t *testing.T) {
	repo := NewFakeManifestRepository()
	if _, err := repo.ReadFile(context.Background(), "apps/order/dev/values.yaml", "main"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing file should return not found, got %v", err)
	}
	if _, err := repo.CommitFiles(context.Background(), CommitSpec{Files: []CommitFile{{Path: "apps/order/dev/values.yaml", Content: "image: v1"}}}); err != nil {
		t.Fatalf("commit: %v", err)
	}
	content, err := repo.ReadFile(context.Background(), "apps/order/dev/values.yaml", "main")
	if err != nil || content != "image: v1" {
		t.Fatalf("read content=%q err=%v", content, err)
	}
	mr, err := repo.GetMergeRequest(context.Background(), "mr_1")
	if err != nil || mr.State != "opened" {
		t.Fatalf("unexpected mr: %#v err=%v", mr, err)
	}
}

func TestNoopAuditLogger(t *testing.T) {
	if err := (NoopAuditLogger{}).Log(context.Background(), AuditEvent{Action: "deployment.create"}); err != nil {
		t.Fatalf("noop audit: %v", err)
	}
}

func TestGitOpsValidationErrorsRepositoryQueriesAndStatusMapping(t *testing.T) {
	svc, _, _ := newTestService([]shared.ID{"deployment_template_platform", "deployment_template_revision_platform"})
	if _, err := svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app"); err != nil {
		t.Fatalf("platform template: %v", err)
	}
	if _, err := svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app"); err != nil {
		t.Fatalf("idempotent platform template should return existing: %v", err)
	}
	if _, err := svc.CreateApplicationTemplate(context.Background(), "app_1", "missing", "user_1"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing base template should fail, got %v", err)
	}
	if _, err := svc.UpdateApplicationTemplate(context.Background(), "missing_app", "containers: []", "user_1"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing app template should fail, got %v", err)
	}
	if result := svc.ValidateTemplate(context.Background(), ""); result.Valid {
		t.Fatalf("empty template should be invalid")
	}
	if result := svc.ValidateTemplate(context.Background(), "hostPath:\n  path: /var/run/docker.sock"); result.Valid {
		t.Fatalf("hostPath template should be invalid")
	}
	if _, err := svc.repo.GetManifestRevision(context.Background(), "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing manifest revision should fail, got %v", err)
	}
	if status := mapAgentStatus(clusteragent.ApplicationStatus{SyncStatus: "Synced", HealthStatus: "Healthy", OperationState: "Succeeded"}); status != DeploymentSucceeded {
		t.Fatalf("healthy synced should succeed, got %s", status)
	}
	if status := mapAgentStatus(clusteragent.ApplicationStatus{SyncStatus: "Synced", HealthStatus: "Degraded"}); status != DeploymentDegraded {
		t.Fatalf("degraded should map to degraded, got %s", status)
	}
	if status := mapAgentStatus(clusteragent.ApplicationStatus{SyncStatus: "OutOfSync", HealthStatus: "Progressing", OperationState: "Running"}); status != DeploymentSyncing {
		t.Fatalf("running out-of-sync should map to syncing, got %s", status)
	}
	if status := mapAgentStatus(clusteragent.ApplicationStatus{OperationState: "Failed"}); status != DeploymentFailed {
		t.Fatalf("failed operation should map to failed, got %s", status)
	}
	if status := mapAgentStatus(clusteragent.ApplicationStatus{}); status != DeploymentUnknown {
		t.Fatalf("empty status should map unknown, got %s", status)
	}
	if repository, tag := splitImage("registry.example/order:v1"); repository != "registry.example/order" || tag != "v1" {
		t.Fatalf("unexpected image split: repository=%q tag=%q", repository, tag)
	}
	if repository, tag := splitImage("registry.example/order"); repository != "registry.example/order" || tag != "" {
		t.Fatalf("unexpected image without tag split: repository=%q tag=%q", repository, tag)
	}
	if repository, tag := splitImage(" "); repository != "" || tag != "" {
		t.Fatalf("blank image should split empty, got repository=%q tag=%q", repository, tag)
	}
	if got := firstNonEmpty(" ", "\t", " commit_1 "); got != "commit_1" {
		t.Fatalf("firstNonEmpty trimmed value = %q", got)
	}
	if got := firstNonEmpty(" ", ""); got != "" {
		t.Fatalf("firstNonEmpty empty value = %q", got)
	}
	repo := NewMemoryRepository()
	svcNoRevision := NewService(Options{Repository: repo})
	if err := repo.CreateTemplate(context.Background(), DeploymentTemplate{ID: "template_without_revision", Scope: TemplateScopeApplication, ApplicationID: "app_without_revision"}); err != nil {
		t.Fatalf("create template without revision: %v", err)
	}
	if _, _, err := svcNoRevision.GetApplicationTemplate(context.Background(), "app_without_revision"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing current revision should fail, got %v", err)
	}
}

func TestGitOpsMemoryRepositoryConflictAndMissingBranches(t *testing.T) {
	repo := NewMemoryRepository()
	ctx := context.Background()
	template := DeploymentTemplate{ID: "template_1", Name: "java", Scope: TemplateScopeApplication, ApplicationID: "app_1"}
	if err := repo.CreateTemplate(ctx, template); err != nil {
		t.Fatalf("create template: %v", err)
	}
	if err := repo.CreateTemplate(ctx, template); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate template should conflict, got %v", err)
	}
	if err := repo.CreateTemplate(ctx, DeploymentTemplate{ID: "template_2", Scope: TemplateScopeApplication, ApplicationID: "app_1"}); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate app template should conflict, got %v", err)
	}
	if err := repo.UpdateTemplate(ctx, DeploymentTemplate{ID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("update missing template should fail, got %v", err)
	}
	if _, err := repo.GetTemplate(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("get missing template should fail, got %v", err)
	}
	if _, err := repo.GetCurrentTemplateRevision(ctx, template.ID); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing current revision should fail, got %v", err)
	}
	revision := DeploymentTemplateRevision{ID: "revision_1", TemplateID: template.ID, Version: 1}
	if err := repo.CreateTemplateRevision(ctx, revision); err != nil {
		t.Fatalf("create revision: %v", err)
	}
	if err := repo.CreateTemplateRevision(ctx, revision); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate revision should conflict, got %v", err)
	}
	if current, err := repo.GetCurrentTemplateRevision(ctx, template.ID); err != nil || current.ID != revision.ID {
		t.Fatalf("current revision: %#v err=%v", current, err)
	}
	deployment := Deployment{ID: "deployment_1", ApplicationID: "app_1", PromotionID: "promotion_1", CreatedAt: time.Date(2026, 5, 30, 13, 0, 0, 0, time.UTC)}
	if err := repo.CreateDeployment(ctx, deployment); err != nil {
		t.Fatalf("create deployment: %v", err)
	}
	if err := repo.CreateDeployment(ctx, deployment); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate deployment should conflict, got %v", err)
	}
	if err := repo.UpdateDeployment(ctx, Deployment{ID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("update missing deployment should fail, got %v", err)
	}
	if _, err := repo.GetDeployment(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("get missing deployment should fail, got %v", err)
	}
	if _, err := repo.FindDeploymentByPromotion(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("find missing deployment should fail, got %v", err)
	}
	if _, err := repo.ListDeployments(ctx, "app_1", shared.PageRequest{Page: 2, PageSize: 10}); err != nil {
		t.Fatalf("list deployments page: %v", err)
	}
}

func TestGitOpsServicePropagatesTemplateAndQueryErrors(t *testing.T) {
	errBoom := shared.NewError(shared.CodeInternal, "boom")
	repo := &gitopsErrRepo{MemoryRepository: NewMemoryRepository()}
	svc := NewService(Options{Repository: repo, ManifestRepo: NewFakeManifestRepository(), Application: appQuery{"app_1": {ID: "app_1", TenantID: "tenant_1", ProjectID: "project_1", Name: "order-api"}}, IDGenerator: &staticIDs{ids: []shared.ID{"template_bad", "revision_bad", "template_bad2", "revision_bad2", "template_1", "revision_1", "template_2", "revision_2", "revision_3", "revision_4"}}, Clock: fixedClock{now: time.Date(2026, 5, 30, 13, 0, 0, 0, time.UTC)}})
	repo.createTemplateErr = errBoom
	if _, err := svc.EnsurePlatformTemplate(context.Background(), "java", "containers: []"); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("create template error should propagate, got %v", err)
	}
	repo.createTemplateErr = nil
	repo.createTemplateRevisionErr = errBoom
	if _, err := svc.EnsurePlatformTemplate(context.Background(), "java", "containers: []"); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("create platform revision error should propagate, got %v", err)
	}
	repo.createTemplateRevisionErr = nil
	if _, err := svc.EnsurePlatformTemplate(context.Background(), "java", "containers: []"); err != nil {
		t.Fatalf("platform template: %v", err)
	}
	if _, err := svc.CreateApplicationTemplate(context.Background(), "app_1", "java", "user_1"); err != nil {
		t.Fatalf("app template: %v", err)
	}
	if existing, err := svc.CreateApplicationTemplate(context.Background(), "app_1", "java", "user_1"); err != nil || existing.ApplicationID != "app_1" {
		t.Fatalf("existing app template should be returned: %#v err=%v", existing, err)
	}
	repo.updateTemplateErr = errBoom
	if _, err := svc.UpdateApplicationTemplate(context.Background(), "app_1", "containers: []", "user_1"); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("update template error should propagate, got %v", err)
	}
	repo.updateTemplateErr = nil
	repo.createTemplateRevisionErr = errBoom
	if _, err := svc.UpdateApplicationTemplate(context.Background(), "app_1", "containers: []", "user_1"); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("update revision error should propagate, got %v", err)
	}

	idFailSvc := NewService(Options{Repository: NewMemoryRepository(), ManifestRepo: NewFakeManifestRepository(), IDGenerator: gitopsFailingIDs{err: errBoom}, Clock: fixedClock{now: time.Date(2026, 5, 30, 13, 0, 0, 0, time.UTC)}})
	if _, err := idFailSvc.EnsurePlatformTemplate(context.Background(), "java", "containers: []"); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("id failure should propagate, got %v", err)
	}
	appErrSvc := NewService(Options{Repository: NewMemoryRepository(), ManifestRepo: NewFakeManifestRepository(), Application: errAppQuery{err: errBoom}, IDGenerator: &staticIDs{ids: []shared.ID{"template_1", "revision_1"}}, Clock: fixedClock{now: time.Date(2026, 5, 30, 13, 0, 0, 0, time.UTC)}})
	if _, err := appErrSvc.CreateApplicationTemplate(context.Background(), "app_1", "java", "user_1"); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("app query error should propagate, got %v", err)
	}
}

func TestGitOpsServicePropagatesPromotionAndAgentUpdateErrors(t *testing.T) {
	errBoom := shared.NewError(shared.CodeInternal, "boom")
	repo := &gitopsErrRepo{MemoryRepository: NewMemoryRepository()}
	manifest := NewFakeManifestRepository()
	svc := NewService(Options{
		Repository: repo, ManifestRepo: manifest,
		Application: appQuery{"app_1": {ID: "app_1", TenantID: "tenant_1", ProjectID: "project_1", Name: "order-api"}},
		Environment: envQuery{envs: map[shared.ID]EnvironmentRef{"env_dev": {ID: "env_dev", TenantID: "tenant_1", ProjectID: "project_1", ApplicationID: "app_1", Name: "dev"}, "env_prod": {ID: "env_prod", TenantID: "tenant_1", ProjectID: "project_1", ApplicationID: "app_1", Name: "prod"}}, bindings: map[shared.ID]ClusterBindingRef{"env_dev": {ID: "binding_dev", EnvironmentID: "env_dev", Namespace: "order-dev", Active: true}, "env_prod": {ID: "binding_prod", EnvironmentID: "env_prod", Namespace: "order-prod", Active: true}}},
		IDGenerator: &staticIDs{ids: []shared.ID{"template_1", "revision_1", "template_2", "revision_2", "deployment_1", "manifest_1", "event_1", "deployment_2", "manifest_2", "event_2", "deployment_event_update"}},
		Clock:       fixedClock{now: time.Date(2026, 5, 30, 13, 0, 0, 0, time.UTC)},
	})
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers: []")
	_, _ = svc.CreateApplicationTemplate(context.Background(), "app_1", "java", "user_1")
	appErrSvc := NewService(Options{Repository: repo, ManifestRepo: manifest, Application: errAppQuery{err: errBoom}, Environment: svc.envs})
	if _, err := appErrSvc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{ApplicationID: "app_1"}); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("app query error should propagate, got %v", err)
	}
	envErrSvc := NewService(Options{Repository: repo, ManifestRepo: manifest, Application: svc.apps, Environment: errEnvQuery{err: errBoom}})
	if _, err := envErrSvc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{ApplicationID: "app_1", EnvironmentID: "env_dev"}); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("env query error should propagate, got %v", err)
	}
	commitErrSvc := NewService(Options{Repository: repo, ManifestRepo: gitopsErrManifest{err: errBoom}, Application: svc.apps, Environment: svc.envs, IDGenerator: &staticIDs{ids: []shared.ID{"deployment_bad", "manifest_bad", "event_bad"}}, Clock: fixedClock{now: time.Date(2026, 5, 30, 13, 0, 0, 0, time.UTC)}})
	if _, err := commitErrSvc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{PromotionID: "promotion_dev", FreightID: "freight_1", ApplicationID: "app_1", EnvironmentID: "env_dev", ImageURI: "repo:v1"}); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("manifest commit error should propagate, got %v", err)
	}
	mrErrSvc := NewService(Options{Repository: repo, ManifestRepo: gitopsErrManifest{err: errBoom}, Application: svc.apps, Environment: svc.envs, IDGenerator: &staticIDs{ids: []shared.ID{"deployment_bad2", "manifest_bad2", "event_bad2"}}, Clock: fixedClock{now: time.Date(2026, 5, 30, 13, 0, 0, 0, time.UTC)}})
	if _, err := mrErrSvc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{PromotionID: "promotion_prod", FreightID: "freight_1", ApplicationID: "app_1", EnvironmentID: "env_prod", ImageURI: "repo:v1"}); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("manifest MR error should propagate, got %v", err)
	}
	repo.createDeploymentErr = errBoom
	if _, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{PromotionID: "promotion_dev", FreightID: "freight_1", ApplicationID: "app_1", EnvironmentID: "env_dev", ImageURI: "repo:v1"}); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("create deployment error should propagate, got %v", err)
	}
	repo.createDeploymentErr = nil
	repo.createManifestRevisionErr = errBoom
	if _, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{PromotionID: "promotion_dev2", FreightID: "freight_1", ApplicationID: "app_1", EnvironmentID: "env_dev", ImageURI: "repo:v1"}); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("create manifest revision error should propagate, got %v", err)
	}
	repo.createManifestRevisionErr = nil
	deployment, err := repo.FindDeploymentByPromotion(context.Background(), "promotion_dev2")
	if err != nil {
		t.Fatalf("deployment should exist before manifest revision failure: %v", err)
	}
	repo.updateDeploymentErr = errBoom
	if err := svc.UpdateFromAgent(context.Background(), clusteragent.StatusReport{Applications: []clusteragent.ApplicationStatus{{DeploymentID: deployment.ID, SyncStatus: "Synced", HealthStatus: "Healthy"}}}); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("update deployment error should propagate, got %v", err)
	}
	repo.updateDeploymentErr = nil
	repo.createDeploymentEventErr = errBoom
	if err := svc.UpdateFromAgent(context.Background(), clusteragent.StatusReport{Applications: []clusteragent.ApplicationStatus{{DeploymentID: deployment.ID, SyncStatus: "Synced", HealthStatus: "Healthy", Message: "ok"}}}); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("create deployment event error should propagate, got %v", err)
	}
	if err := svc.UpdateFromAgent(context.Background(), clusteragent.StatusReport{Applications: []clusteragent.ApplicationStatus{{DeploymentID: ""}, {DeploymentID: "missing"}}}); err != nil {
		t.Fatalf("empty or missing deployment updates should be ignored: %v", err)
	}
}
