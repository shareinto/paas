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
	"github.com/shareinto/paas/internal/testsupport"
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
	Repository
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
	return r.Repository.CreateTemplate(ctx, template)
}

func (r *gitopsErrRepo) UpdateTemplate(ctx context.Context, template DeploymentTemplate) error {
	if r.updateTemplateErr != nil {
		return r.updateTemplateErr
	}
	return r.Repository.UpdateTemplate(ctx, template)
}

func (r *gitopsErrRepo) CreateTemplateRevision(ctx context.Context, revision DeploymentTemplateRevision) error {
	if r.createTemplateRevisionErr != nil {
		return r.createTemplateRevisionErr
	}
	return r.Repository.CreateTemplateRevision(ctx, revision)
}

func (r *gitopsErrRepo) CreateDeployment(ctx context.Context, deployment Deployment) error {
	if r.createDeploymentErr != nil {
		return r.createDeploymentErr
	}
	return r.Repository.CreateDeployment(ctx, deployment)
}

func (r *gitopsErrRepo) CreateManifestRevision(ctx context.Context, revision ManifestRevision) error {
	if r.createManifestRevisionErr != nil {
		return r.createManifestRevisionErr
	}
	return r.Repository.CreateManifestRevision(ctx, revision)
}

func (r *gitopsErrRepo) UpdateDeployment(ctx context.Context, deployment Deployment) error {
	if r.updateDeploymentErr != nil {
		return r.updateDeploymentErr
	}
	return r.Repository.UpdateDeployment(ctx, deployment)
}

func (r *gitopsErrRepo) CreateDeploymentEvent(ctx context.Context, event DeploymentEvent) error {
	if r.createDeploymentEventErr != nil {
		return r.createDeploymentEventErr
	}
	return r.Repository.CreateDeploymentEvent(ctx, event)
}

func newTestRepository(t *testing.T) Repository {
	t.Helper()
	repo, err := NewMySQLRepository(context.Background(), testsupport.MySQLDB(t, Migrations...))
	if err != nil {
		t.Fatalf("NewMySQLRepository() error = %v", err)
	}
	return repo
}

func listDeploymentEventsForTest(t *testing.T, repo Repository, deploymentID shared.ID) []DeploymentEvent {
	t.Helper()
	mysqlRepo, ok := repo.(*MySQLRepository)
	if !ok {
		t.Fatalf("test repository type = %T, want *MySQLRepository", repo)
	}
	rows, err := mysqlRepo.db.QueryContext(context.Background(), `
SELECT id, deployment_id, status, message, occurred_at
FROM deployment_events
WHERE deployment_id = ?
ORDER BY occurred_at, id`, deploymentID)
	if err != nil {
		t.Fatalf("query deployment events: %v", err)
	}
	defer rows.Close()
	events := []DeploymentEvent{}
	for rows.Next() {
		var event DeploymentEvent
		if err := rows.Scan(&event.ID, &event.DeploymentID, &event.Status, &event.Message, &event.OccurredAt); err != nil {
			t.Fatalf("scan deployment event: %v", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate deployment events: %v", err)
	}
	return events
}

type gitopsErrManifest struct{ err error }

func (m gitopsErrManifest) ReadFile(context.Context, string, string) (string, error) {
	return "", m.err
}
func (m gitopsErrManifest) CommitFiles(context.Context, CommitSpec) (CommitResult, error) {
	return CommitResult{}, m.err
}
func (m gitopsErrManifest) DeleteFiles(context.Context, DeleteFilesSpec) (CommitResult, error) {
	return CommitResult{}, m.err
}
func (m gitopsErrManifest) CreateMergeRequest(context.Context, MergeRequestSpec) (MergeRequestResult, error) {
	return MergeRequestResult{}, m.err
}
func (m gitopsErrManifest) GetMergeRequest(context.Context, string) (MergeRequest, error) {
	return MergeRequest{}, m.err
}
func (m gitopsErrManifest) CreateTag(context.Context, string, string) (TagResult, error) {
	return TagResult{}, m.err
}

type errAppQuery struct{ err error }

func (q errAppQuery) GetApplication(context.Context, shared.ID) (ApplicationRef, error) {
	return ApplicationRef{}, q.err
}

type appQuery map[shared.ID]ApplicationRef

func (q appQuery) GetApplication(_ context.Context, id shared.ID) (ApplicationRef, error) {
	return q[id], nil
}

func newTestService(t *testing.T, ids []shared.ID) (*Service, *FakeManifestRepository, *auditRecorder) {
	manifest := NewFakeManifestRepository()
	audit := &auditRecorder{}
	svc := NewService(Options{
		Repository: newTestRepository(t), ManifestRepo: manifest,
		Application: appQuery{"app_1": {ID: "app_1", TenantID: "tenant_1", ProjectID: "project_1", Name: "order-api"}},
		Audit:       audit, IDGenerator: &staticIDs{ids: ids}, Clock: fixedClock{now: time.Date(2026, 5, 30, 13, 0, 0, 0, time.UTC)},
	})
	return svc, manifest, audit
}

func targetClusters(stageKey string) []delivery.GitOpsPromotionTargetCluster {
	return []delivery.GitOpsPromotionTargetCluster{{
		ClusterID:   shared.ID("cluster_" + stageKey),
		ClusterName: stageKey + "-cluster",
		Namespace:   "order-" + stageKey,
	}}
}

func TestPlatformTemplateCanBeUpdated(t *testing.T) {
	svc, _, _ := newTestService(t, []shared.ID{"deployment_template_platform", "deployment_template_revision_platform", "deployment_template_revision_platform2"})
	_, err := svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app")
	if err != nil {
		t.Fatalf("platform template: %v", err)
	}
	revision, err := svc.UpdatePlatformTemplate(context.Background(), "initContainers:\n- name: init\n  command: ['mkdir','-p','/data']\nvolumeMounts: []\nsecurityContext: {}", "user_1")
	if err != nil {
		t.Fatalf("update platform template: %v", err)
	}
	if revision.Version != 2 {
		t.Fatalf("expected version 2, got %d", revision.Version)
	}
}

func TestTemplatePolicyValidationRejectsPrivilegedAndAllowsInitContainer(t *testing.T) {
	svc, _, _ := newTestService(t, nil)
	allowed := svc.ValidateTemplate(context.Background(), "initContainers:\n- name: init\nsecurityContext:\n  runAsNonRoot: true")
	if !allowed.Valid {
		t.Fatalf("expected initContainer template to be valid: %#v", allowed)
	}
	rejected := svc.ValidateTemplate(context.Background(), "securityContext:\n  privileged: true")
	if rejected.Valid {
		t.Fatalf("expected privileged template to be rejected")
	}
}

func TestApplyPromotionInitializesMissingDeploymentTemplate(t *testing.T) {
	svc, manifest, _ := newTestService(t, []shared.ID{
		"deployment_template_platform", "deployment_template_revision_platform",
		"deployment_1", "manifest_revision_1", "deployment_event_1",
	})

	result, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{
		PromotionID:   "promotion_dev",
		FreightID:     "freight_1",
		ApplicationID: "app_1",
		StageKey:      "dev", TargetClusters: targetClusters("dev"),
		ImageURI:    "registry/order-api:v1",
		ImageDigest: "sha256:old",
	})
	if err != nil {
		t.Fatalf("ApplyPromotion() should initialize missing deployment template: %v", err)
	}
	if result.ManifestRevision == "" || len(manifest.Commits) != 1 {
		t.Fatalf("promotion should commit manifest after template initialization, result=%+v commits=%+v", result, manifest.Commits)
	}
	platform, err := svc.repo.FindPlatformTemplate(context.Background(), "java")
	if err != nil || platform.Content == "" {
		t.Fatalf("default platform template should be initialized, got %+v err=%v", platform, err)
	}
}

func TestApplyPromotionWritesDirectorySourceArgoApplication(t *testing.T) {
	svc, manifest, _ := newTestService(t, []shared.ID{
		"deployment_template_platform", "deployment_template_revision_platform",
		"deployment_1", "manifest_revision_1", "deployment_event_1",
	})
	svc.manifestRepoURL = "ssh://git@gitlab.example/paas/manifests.git"
	svc.workloads = workloadQuery{
		workloads: map[shared.ID]WorkloadRef{
			"workload_api":    {ID: "workload_api", ApplicationID: "app_1", Name: "user-api", WorkloadType: "Deployment"},
			"workload_worker": {ID: "workload_worker", ApplicationID: "app_1", Name: "order-worker", WorkloadType: "StatefulSet"},
		},
	}

	_, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{
		PromotionID: "promotion_dev", FreightID: "freight_1", ApplicationID: "app_1", StageKey: "dev", TargetClusters: targetClusters("dev"),
		Artifacts: []delivery.GitOpsArtifactSpec{
			{WorkloadID: "workload_api", URI: "registry/user-api:v2", Repository: "registry/user-api", Tag: "v2", IsPrimary: true},
			{WorkloadID: "workload_worker", URI: "registry/order-worker:v5", Repository: "registry/order-worker", Tag: "v5"},
		},
	})
	if err != nil {
		t.Fatalf("ApplyPromotion() error = %v", err)
	}
	argo := manifest.Files["argocd/apps/dev/order-api-dev.yaml"]
	if _, ok := manifest.Files["argocd/apps/order-api-dev.yaml"]; ok {
		t.Fatalf("argo application should be written under stage directory, files=%+v", manifest.Files)
	}
	for _, expected := range []string{
		"name: order-api-dev",
		"finalizers:",
		"resources-finalizer.argocd.argoproj.io",
		"project: default",
		"server: https://kubernetes.default.svc",
		"repoURL: ssh://git@gitlab.example/paas/manifests.git",
		"targetRevision: main",
		"path: apps/order-api/dev",
		"syncPolicy:",
		"prune: true",
		"selfHeal: true",
		"CreateNamespace=true",
		"ServerSideApply=true",
	} {
		if !strings.Contains(argo, expected) {
			t.Fatalf("argo application should contain %q:\n%s", expected, argo)
		}
	}
	// Verify it does NOT use multi-source Helm approach
	if strings.Contains(argo, "sources:") || strings.Contains(argo, "ref: values") || strings.Contains(argo, "helm:") {
		t.Fatalf("argo application should use single directory source, not multi-source Helm:\n%s", argo)
	}
}

func TestApplyPromotionWritesRenderedManifests(t *testing.T) {
	svc, manifest, _ := newTestService(t, []shared.ID{
		"deployment_template_platform", "deployment_template_revision_platform",
		"deployment_1", "manifest_revision_1", "deployment_event_1",
	})
	svc.workloads = workloadQuery{
		workloads: map[shared.ID]WorkloadRef{
			"workload_api": {ID: "workload_api", ApplicationID: "app_1", Name: "user-api", WorkloadType: "Deployment"},
		},
		configs: map[string]WorkloadStageConfigRef{
			"workload_api|dev": {
				ServicePorts: []WorkloadServicePortRef{{Name: "http", Port: 80, TargetPort: 8080, Protocol: "TCP"}},
			},
		},
	}

	_, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{
		PromotionID: "promotion_dev", FreightID: "freight_1", ApplicationID: "app_1", StageKey: "dev", TargetClusters: targetClusters("dev"),
		Artifacts: []delivery.GitOpsArtifactSpec{{WorkloadID: "workload_api", URI: "registry/user-api:v2", Repository: "registry/user-api", Tag: "v2", Digest: "sha256:api", IsPrimary: true}},
	})
	if err != nil {
		t.Fatalf("ApplyPromotion() error = %v", err)
	}

	manifests := manifest.Files["apps/order-api/dev/manifests.yaml"]
	if manifests == "" {
		t.Fatalf("manifests.yaml should be written, files=%+v", manifest.Files)
	}
	// Should contain rendered K8s manifests, not Helm values
	for _, want := range []string{
		"kind: Deployment",
		"kind: Service",
		"user-api",
	} {
		if !strings.Contains(manifests, want) {
			t.Fatalf("manifests missing %q:\n%s", want, manifests)
		}
	}
}

func TestPreviewPromotionManifestRendersYamlDiffInputsWithoutCommitting(t *testing.T) {
	svc, manifest, _ := newTestService(t, []shared.ID{
		"deployment_template_platform", "deployment_template_revision_platform",
		"deployment_1", "manifest_revision_1", "deployment_event_1",
	})
	svc.workloads = workloadQuery{
		workloads: map[shared.ID]WorkloadRef{
			"workload_api": {ID: "workload_api", ApplicationID: "app_1", Name: "user-api", WorkloadType: "Deployment"},
		},
		configs: map[string]WorkloadStageConfigRef{
			"workload_api|dev": {
				ServicePorts: []WorkloadServicePortRef{{Name: "http", Port: 80, TargetPort: 8080, Protocol: "TCP"}},
			},
		},
	}

	if _, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{
		PromotionID: "promotion_dev", FreightID: "freight_1", ApplicationID: "app_1", StageKey: "dev", TargetClusters: targetClusters("dev"),
		Artifacts: []delivery.GitOpsArtifactSpec{{WorkloadID: "workload_api", URI: "registry/user-api:v1", Repository: "registry/user-api", Tag: "v1", Digest: "sha256:old", IsPrimary: true}},
	}); err != nil {
		t.Fatalf("ApplyPromotion() error = %v", err)
	}

	expected, current, err := svc.PreviewPromotionManifest(context.Background(), delivery.GitOpsPromotionSpec{
		PromotionID: "promotion_preview", FreightID: "freight_2", ApplicationID: "app_1", StageKey: "dev", TargetClusters: targetClusters("dev"),
		Artifacts: []delivery.GitOpsArtifactSpec{{WorkloadID: "workload_api", URI: "registry/user-api:v2", Repository: "registry/user-api", Tag: "v2", Digest: "sha256:new", IsPrimary: true}},
	})
	if err != nil {
		t.Fatalf("PreviewPromotionManifest() error = %v", err)
	}
	if !strings.Contains(current, "image: registry/user-api:v1@sha256:old") || !strings.Contains(expected, "image: registry/user-api:v2@sha256:new") {
		t.Fatalf("preview should compare rendered manifests, current:\n%s\nexpected:\n%s", current, expected)
	}
	if len(manifest.Commits) != 1 {
		t.Fatalf("preview should not commit manifests, commits=%+v", manifest.Commits)
	}
}

func TestRenderExpectedManifestUsesProjectStageNamespace(t *testing.T) {
	svc, manifest, _ := newTestService(t, []shared.ID{
		"deployment_template_platform", "deployment_template_revision_platform",
		"deployment_1", "manifest_revision_1", "deployment_event_1",
	})
	svc.apps = appQuery{"app_1": {ID: "app_1", TenantID: "tenant_1", ProjectID: "project_1", ProjectName: "macc", Name: "order-api"}}
	svc.workloads = workloadQuery{
		workloads: map[shared.ID]WorkloadRef{
			"workload_api": {ID: "workload_api", ApplicationID: "app_1", Name: "user-api", WorkloadType: "Deployment"},
		},
		configs: map[string]WorkloadStageConfigRef{
			"workload_api|dev": {
				ServicePorts: []WorkloadServicePortRef{{Name: "http", Port: 80, TargetPort: 8080, Protocol: "TCP"}},
			},
		},
	}

	if _, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{
		PromotionID: "promotion_dev", FreightID: "freight_1", ApplicationID: "app_1", StageKey: "dev", TargetClusters: targetClusters("dev"),
		Artifacts: []delivery.GitOpsArtifactSpec{{WorkloadID: "workload_api", URI: "registry/user-api:v1", Repository: "registry/user-api", Tag: "v1", Digest: "sha256:old", IsPrimary: true}},
	}); err != nil {
		t.Fatalf("ApplyPromotion() error = %v", err)
	}
	path := "apps/order-api/dev/manifests.yaml"
	manifest.Files[path] = strings.ReplaceAll(manifest.Files[path], "namespace: order-dev", "namespace: macc")

	expected, current, err := svc.RenderExpectedManifest(context.Background(), "app_1", "dev", []delivery.FreightItem{{
		WorkloadID: "workload_api", ContainerName: "app", URI: "registry/user-api:v1", ImageRepository: "registry/user-api", ImageTag: "v1", Digest: "sha256:old",
	}})
	if err != nil {
		t.Fatalf("RenderExpectedManifest() error = %v", err)
	}
	if !strings.Contains(current, "namespace: macc") {
		t.Fatalf("test manifest should use macc namespace:\n%s", current)
	}
	if strings.Contains(expected, "namespace: default") || !strings.Contains(expected, "namespace: macc-dev") {
		t.Fatalf("expected manifest should use project-stage namespace:\n%s", expected)
	}
}

func TestRenderExpectedManifestFallsBackToProjectNamespace(t *testing.T) {
	svc, manifest, _ := newTestService(t, []shared.ID{
		"deployment_template_platform", "deployment_template_revision_platform",
		"deployment_1", "manifest_revision_1", "deployment_event_1",
	})
	svc.apps = appQuery{"app_1": {ID: "app_1", TenantID: "tenant_1", ProjectID: "project_1", ProjectName: "macc", Name: "frontend"}}
	svc.workloads = workloadQuery{
		workloads: map[shared.ID]WorkloadRef{
			"workload_frontend": {ID: "workload_frontend", ApplicationID: "app_1", Name: "macc-frontend", WorkloadType: "Deployment"},
		},
		configs: map[string]WorkloadStageConfigRef{
			"workload_frontend|dev": {
				ServicePorts: []WorkloadServicePortRef{{Name: "http", Port: 80, TargetPort: 8080, Protocol: "TCP"}},
			},
		},
	}

	if _, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{
		PromotionID: "promotion_dev", FreightID: "freight_1", ApplicationID: "app_1", StageKey: "dev", TargetClusters: targetClusters("dev"),
		Artifacts: []delivery.GitOpsArtifactSpec{{WorkloadID: "workload_frontend", URI: "registry/frontend:v1", Repository: "registry/frontend", Tag: "v1", Digest: "sha256:old", IsPrimary: true}},
	}); err != nil {
		t.Fatalf("ApplyPromotion() error = %v", err)
	}
	delete(manifest.Files, "apps/frontend/dev/manifests.yaml")

	expected, _, err := svc.RenderExpectedManifest(context.Background(), "app_1", "dev", []delivery.FreightItem{{
		WorkloadID: "workload_frontend", ContainerName: "app", URI: "registry/frontend:v1", ImageRepository: "registry/frontend", ImageTag: "v1", Digest: "sha256:old",
	}})
	if err != nil {
		t.Fatalf("RenderExpectedManifest() error = %v", err)
	}
	if strings.Contains(expected, "namespace: default") || !strings.Contains(expected, "namespace: macc-dev") {
		t.Fatalf("expected manifest should use project-stage namespace fallback:\n%s", expected)
	}
}

func TestApplyPromotionCommitsAllStagesAndUpdatesDeploymentFromAgent(t *testing.T) {
	ids := []shared.ID{
		"deployment_template_platform", "deployment_template_revision_platform",
		"deployment_1", "manifest_revision_1", "deployment_event_1",
		"deployment_2", "manifest_revision_2", "deployment_event_2",
		"deployment_3", "manifest_revision_3", "deployment_event_3",
		"deployment_4", "manifest_revision_4", "deployment_event_4",
		"deployment_event_5",
	}
	svc, manifest, audit := newTestService(t, ids)
	svc.workloads = workloadQuery{
		workloads: map[shared.ID]WorkloadRef{
			"workload_api": {ID: "workload_api", ApplicationID: "app_1", Name: "order-api", WorkloadType: "Deployment"},
		},
	}
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app")
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app")
	dev, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{PromotionID: "promotion_dev", FreightID: "freight_1", ApplicationID: "app_1", StageKey: "dev", TargetClusters: targetClusters("dev"), Artifacts: []delivery.GitOpsArtifactSpec{{WorkloadID: "workload_api", URI: "registry/order-api:v1", Repository: "registry/order-api", Tag: "v1", Digest: "sha256:old", IsPrimary: true}}})
	if err != nil {
		t.Fatalf("apply dev: %v", err)
	}
	testEnv, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{PromotionID: "promotion_test", FreightID: "freight_1", ApplicationID: "app_1", StageKey: "test", TargetClusters: targetClusters("test"), Artifacts: []delivery.GitOpsArtifactSpec{{WorkloadID: "workload_api", URI: "registry/order-api:v1-test", Repository: "registry/order-api", Tag: "v1-test", Digest: "sha256:test", IsPrimary: true}}})
	if err != nil {
		t.Fatalf("apply test: %v", err)
	}
	staging, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{PromotionID: "promotion_staging", FreightID: "freight_2", ApplicationID: "app_1", StageKey: "staging", TargetClusters: targetClusters("staging"), Artifacts: []delivery.GitOpsArtifactSpec{{WorkloadID: "workload_api", URI: "registry/order-api:v1-staging", Repository: "registry/order-api", Tag: "v1-staging", Digest: "sha256:staging", IsPrimary: true}}})
	if err != nil {
		t.Fatalf("apply staging: %v", err)
	}
	prod, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{PromotionID: "promotion_prod", FreightID: "freight_2", ApplicationID: "app_1", StageKey: "prod", TargetClusters: targetClusters("prod"), Artifacts: []delivery.GitOpsArtifactSpec{{WorkloadID: "workload_api", URI: "registry/order-api:v0", Repository: "registry/order-api", Tag: "v0", Digest: "sha256:rollback", IsPrimary: true}}, IsRollback: true})
	if err != nil {
		t.Fatalf("apply prod: %v", err)
	}
	if dev.ManifestRevision == "" || testEnv.ManifestRevision == "" || staging.ManifestRevision == "" || prod.ManifestRevision == "" || len(manifest.Commits) != 4 || len(manifest.MRs) != 0 {
		t.Fatalf("unexpected manifest operations: commits=%d mrs=%d", len(manifest.Commits), len(manifest.MRs))
	}
	if !strings.Contains(manifest.Files["apps/order-api/dev/manifests.yaml"], "sha256:old") || !strings.Contains(manifest.Files["apps/order-api/test/manifests.yaml"], "sha256:test") || !strings.Contains(manifest.Files["apps/order-api/staging/manifests.yaml"], "sha256:staging") || !strings.Contains(manifest.Files["apps/order-api/prod/manifests.yaml"], "sha256:rollback") {
		t.Fatalf("manifest files did not contain expected digests: %#v", manifest.Files)
	}
	for _, path := range []string{
		"argocd/apps/dev/order-api-dev.yaml",
		"argocd/apps/test/order-api-test.yaml",
		"argocd/apps/staging/order-api-staging.yaml",
		"argocd/apps/prod/order-api-prod.yaml",
	} {
		if manifest.Files[path] == "" {
			t.Fatalf("argo application missing stage-scoped path %q: %#v", path, manifest.Files)
		}
	}
	if _, ok := manifest.Files["argocd/apps/dev/order-api-dev-binding_dev.yaml"]; ok {
		t.Fatalf("argo application name should not include cluster binding id: %#v", manifest.Files)
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

func TestDeleteApplicationManifestsRemovesValuesAndArgoApplicationsForDeployedStages(t *testing.T) {
	ids := []shared.ID{
		"deployment_template_platform", "deployment_template_revision_platform",
		"deployment_dev", "manifest_dev", "event_dev",
		"deployment_prod", "manifest_prod", "event_prod",
	}
	svc, manifest, _ := newTestService(t, ids)
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app")
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app")
	if _, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{PromotionID: "promotion_dev", FreightID: "freight_1", ApplicationID: "app_1", StageKey: "dev", TargetClusters: targetClusters("dev"), ImageURI: "registry/order-api:v1"}); err != nil {
		t.Fatalf("apply dev: %v", err)
	}
	if _, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{PromotionID: "promotion_prod", FreightID: "freight_1", ApplicationID: "app_1", StageKey: "prod", TargetClusters: targetClusters("prod"), ImageURI: "registry/order-api:v1"}); err != nil {
		t.Fatalf("apply prod: %v", err)
	}
	manifest.Files["apps/order-api/staging/manifests.yaml"] = "should remain because no deployment record"

	if err := svc.DeleteApplicationManifests(context.Background(), "app_1"); err != nil {
		t.Fatalf("DeleteApplicationManifests() error = %v", err)
	}

	for _, path := range []string{
		"apps/order-api/dev/manifests.yaml",
		"argocd/apps/dev/order-api-dev.yaml",
		"apps/order-api/prod/manifests.yaml",
		"argocd/apps/prod/order-api-prod.yaml",
	} {
		if _, ok := manifest.Files[path]; ok {
			t.Fatalf("manifest cleanup should delete %s, files=%+v", path, manifest.Files)
		}
	}
	if manifest.Files["apps/order-api/staging/manifests.yaml"] == "" {
		t.Fatalf("cleanup should not delete stages without deployment records, files=%+v", manifest.Files)
	}
	for _, path := range []string{
		"argocd/apps/dev/.gitkeep",
		"argocd/apps/prod/.gitkeep",
	} {
		if _, ok := manifest.Files[path]; !ok {
			t.Fatalf("cleanup should keep stage directory placeholder %s, files=%+v", path, manifest.Files)
		}
	}
	if len(manifest.Deletes) != 1 {
		t.Fatalf("cleanup should submit one delete commit, deletes=%+v", manifest.Deletes)
	}
	if manifest.Deletes[0].Branch != "main" || !strings.Contains(manifest.Deletes[0].Message, "paas: delete order-api manifests") {
		t.Fatalf("unexpected delete commit spec: %+v", manifest.Deletes[0])
	}
}

func TestDeleteApplicationManifestsIgnoresMissingFilesAndFailsOnRepositoryError(t *testing.T) {
	svc, manifest, _ := newTestService(t, []shared.ID{
		"deployment_template_platform", "deployment_template_revision_platform",
		"deployment_dev", "manifest_dev", "event_dev",
	})
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app")
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app")
	if _, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{PromotionID: "promotion_dev", FreightID: "freight_1", ApplicationID: "app_1", StageKey: "dev", TargetClusters: targetClusters("dev"), ImageURI: "registry/order-api:v1"}); err != nil {
		t.Fatalf("apply dev: %v", err)
	}
	delete(manifest.Files, "argocd/apps/dev/order-api-dev.yaml")
	if err := svc.DeleteApplicationManifests(context.Background(), "app_1"); err != nil {
		t.Fatalf("missing manifest files should be ignored, got %v", err)
	}
	if _, ok := manifest.Files["argocd/apps/dev/.gitkeep"]; !ok {
		t.Fatalf("cleanup retry should restore stage directory placeholder, files=%+v", manifest.Files)
	}

	errBoom := shared.NewError(shared.CodeInternal, "gitlab delete failed")
	failing := NewService(Options{
		Repository:   svc.repo,
		ManifestRepo: gitopsErrManifest{err: errBoom},
		Application:  appQuery{"app_1": {ID: "app_1", TenantID: "tenant_1", ProjectID: "project_1", Name: "order-api"}},
	})
	if err := failing.DeleteApplicationManifests(context.Background(), "app_1"); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("repository delete failure should be returned, got %v", err)
	}
}

func TestApplyPromotionCreatesDeploymentPerSelectedStageCluster(t *testing.T) {
	ids := []shared.ID{
		"deployment_template_platform", "deployment_template_revision_platform",
		"deployment_shanghai", "manifest_shanghai", "event_shanghai",
	}
	svc, manifest, _ := newTestService(t, ids)
	svc.workloads = workloadQuery{
		workloads: map[shared.ID]WorkloadRef{
			"workload_api": {ID: "workload_api", ApplicationID: "app_1", Name: "order-api", WorkloadType: "Deployment"},
		},
	}
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app")
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app")

	result, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{
		PromotionID:   "promotion_dev_multi",
		FreightID:     "freight_1",
		ApplicationID: "app_1",
		StageKey:      "dev",
		Artifacts: []delivery.GitOpsArtifactSpec{
			{WorkloadID: "workload_api", URI: "registry/order-api:v2", Repository: "registry/order-api", Tag: "v2", Digest: "sha256:multi", IsPrimary: true},
		},
		TargetClusters: []delivery.GitOpsPromotionTargetCluster{
			{ClusterID: "cluster_shanghai", ClusterName: "上海集群", Namespace: "order-dev"},
		},
	})
	if err != nil {
		t.Fatalf("ApplyPromotion() error = %v", err)
	}
	if result.ManifestRevision == "" || len(manifest.Commits) != 1 {
		t.Fatalf("stage cluster promotion should commit once, result=%+v commits=%d", result, len(manifest.Commits))
	}
	if !strings.Contains(manifest.Files["apps/order-api/dev/manifests.yaml"], "sha256:multi") {
		t.Fatalf("stage values file missing digest: %#v", manifest.Files)
	}
	values := manifest.Files["apps/order-api/dev/manifests.yaml"]
	for _, expected := range []string{
		"paas.shareinto.com/stage-key: \"dev\"",
		"paas.shareinto.com/application-id: \"app_1\"",
		"paas.shareinto.com/deployment-id: \"deployment_shanghai\"",
	} {
		if !strings.Contains(values, expected) {
			t.Fatalf("stage manifests should contain %q:\n%s", expected, values)
		}
	}
	if _, ok := manifest.Files["apps/order-api/dev/cluster_shanghai/manifests.yaml"]; ok {
		t.Fatalf("stage bound to one cluster should not write cluster-specific manifests path: %#v", manifest.Files)
	}
	argo := manifest.Files["argocd/apps/dev/order-api-dev.yaml"]
	for _, expected := range []string{
		"paas.shareinto.com/application-id: app_1",
		"paas.shareinto.com/stage-key: dev",
		"paas.shareinto.com/deployment-id: deployment_shanghai",
	} {
		if !strings.Contains(argo, expected) {
			t.Fatalf("argo application should contain %q:\n%s", expected, argo)
		}
	}
	page, err := svc.ListDeployments(context.Background(), "app_1", shared.PageRequest{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListDeployments() error = %v", err)
	}
	if page.Total != 1 {
		t.Fatalf("expected one deployment for selected stage cluster, got %+v", page)
	}
	if page.Items[0].StageKey != "dev" {
		t.Fatalf("deployment should be associated by stage key, got %+v", page.Items[0])
	}
	revision, err := svc.repo.GetManifestRevision(context.Background(), "manifest_shanghai")
	if err != nil {
		t.Fatalf("GetManifestRevision() error = %v", err)
	}
	if revision.StageKey != "dev" {
		t.Fatalf("manifest revision should be associated by stage key, got %+v", revision)
	}
}

func TestApplyPromotionRejectsMultipleTargetClusters(t *testing.T) {
	ids := []shared.ID{
		"deployment_template_platform", "deployment_template_revision_platform",
	}
	svc, _, _ := newTestService(t, ids)
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app")
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app")

	_, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{
		PromotionID:   "promotion_dev_multi",
		FreightID:     "freight_1",
		ApplicationID: "app_1",
		StageKey:      "dev",
		ImageURI:      "registry/order-api:v2",
		TargetClusters: []delivery.GitOpsPromotionTargetCluster{
			{ClusterID: "cluster_shanghai", ClusterName: "上海集群", Namespace: "order-dev"},
			{ClusterID: "cluster_beijing", ClusterName: "北京集群", Namespace: "order-dev"},
		},
	})
	if shared.CodeOf(err) != shared.CodeInvalidArgument || !strings.Contains(err.Error(), "一个环境只能绑定一个集群") {
		t.Fatalf("expected multiple target clusters to be rejected, got %v", err)
	}
}

func TestApplyPromotionSelectsImageBundleVariantByTargetClusterLabels(t *testing.T) {
	ids := []shared.ID{
		"deployment_template_platform", "deployment_template_revision_platform",
		"deployment_aliyun", "manifest_aliyun", "event_aliyun",
		"deployment_aws", "manifest_aws", "event_aws",
	}
	svc, manifest, _ := newTestService(t, ids)
	svc.workloads = workloadQuery{workloads: map[shared.ID]WorkloadRef{
		"workload_api": {ID: "workload_api", ApplicationID: "app_1", Name: "order-api", WorkloadType: "Deployment"},
	}, configs: map[string]WorkloadStageConfigRef{}}
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app")
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app")

	_, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{
		PromotionID:    "promotion_bundle",
		FreightID:      "freight_bundle",
		ApplicationID:  "app_1",
		StageKey:       "dev",
		TargetClusters: []delivery.GitOpsPromotionTargetCluster{{ClusterID: "cluster_aliyun", ClusterName: "阿里云集群", Namespace: "order-dev", Labels: map[string]string{"cloud": "aliyun"}}},
		Artifacts: []delivery.GitOpsArtifactSpec{{
			WorkloadID: "workload_api",
			Name:       "订单 API",
			Variants: []delivery.GitOpsImageVariant{
				{URI: "registry/order-api:aliyun", Repository: "registry/order-api", Tag: "aliyun", Digest: "sha256:aliyun", SelectorLabels: map[string]string{"cloud": "aliyun"}, IsPrimary: true},
				{URI: "registry/order-api:aws", Repository: "registry/order-api", Tag: "aws", Digest: "sha256:aws", SelectorLabels: map[string]string{"cloud": "aws"}},
			},
			IsPrimary: true,
		}},
	})
	if err != nil {
		t.Fatalf("ApplyPromotion() error = %v", err)
	}
	values := manifest.Files["apps/order-api/dev/manifests.yaml"]
	if !strings.Contains(values, "registry/order-api:aliyun@sha256:aliyun") {
		t.Fatalf("selected cluster should use matching image:\n%s", values)
	}
}

func TestApplyPromotionRejectsImageBundleWithoutUniqueClusterMatch(t *testing.T) {
	svc, _, _ := newTestService(t, []shared.ID{
		"deployment_template_platform", "deployment_template_revision_platform",
		"deployment_missing", "manifest_missing", "event_missing",
	})
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app")
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app")

	_, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{
		PromotionID:   "promotion_bundle_missing",
		FreightID:     "freight_bundle",
		ApplicationID: "app_1",
		StageKey:      "dev",
		TargetClusters: []delivery.GitOpsPromotionTargetCluster{
			{ClusterID: "cluster_unknown", ClusterName: "未知集群", Namespace: "order-dev", Labels: map[string]string{"cloud": "tencent"}},
		},
		Artifacts: []delivery.GitOpsArtifactSpec{{
			WorkloadID: "workload_api",
			Variants: []delivery.GitOpsImageVariant{
				{URI: "registry/order-api:aliyun", Repository: "registry/order-api", Tag: "aliyun", Digest: "sha256:aliyun", SelectorLabels: map[string]string{"cloud": "aliyun"}},
				{URI: "registry/order-api:aws", Repository: "registry/order-api", Tag: "aws", Digest: "sha256:aws", SelectorLabels: map[string]string{"cloud": "aws"}},
			},
		}},
	})
	if shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("missing variant match should fail with failed_precondition, got %v", err)
	}
}

type workloadQuery struct {
	workloads      map[shared.ID]WorkloadRef
	configs        map[string]WorkloadStageConfigRef
	defaultConfigs map[shared.ID]WorkloadStageConfigRef
}

func (q workloadQuery) GetWorkload(_ context.Context, applicationID shared.ID, workloadID shared.ID) (WorkloadRef, error) {
	workload, ok := q.workloads[workloadID]
	if !ok || workload.ApplicationID != applicationID {
		return WorkloadRef{}, shared.NewError(shared.CodeNotFound, "workload not found")
	}
	return workload, nil
}

func (q workloadQuery) GetWorkloadStageConfig(_ context.Context, workloadID shared.ID, stageKey string) (WorkloadStageConfigRef, error) {
	config, ok := q.configs[string(workloadID)+"|"+stageKey]
	if !ok {
		return WorkloadStageConfigRef{}, shared.NewError(shared.CodeNotFound, "workload stage config not found")
	}
	return config, nil
}

func (q workloadQuery) GetWorkloadDefaultConfig(_ context.Context, workloadID shared.ID) (WorkloadStageConfigRef, error) {
	config, ok := q.defaultConfigs[workloadID]
	if !ok {
		return WorkloadStageConfigRef{}, shared.NewError(shared.CodeNotFound, "workload default config not found")
	}
	return config, nil
}

func TestApplyPromotionFallsBackToWorkloadDefaultConfig(t *testing.T) {
	ids := []shared.ID{
		"deployment_template_platform", "deployment_template_revision_platform",
		"deployment_1", "manifest_revision_1", "deployment_event_1",
	}
	svc, manifest, _ := newTestService(t, ids)
	svc.workloads = workloadQuery{
		workloads: map[shared.ID]WorkloadRef{
			"workload_api": {ID: "workload_api", ApplicationID: "app_1", Name: "user-api", DisplayName: "用户 API", WorkloadType: "Deployment"},
		},
		configs: map[string]WorkloadStageConfigRef{},
		defaultConfigs: map[shared.ID]WorkloadStageConfigRef{
			"workload_api": {
				Replicas:     2,
				ServicePorts: []WorkloadServicePortRef{{Name: "http", Port: 80, TargetPort: 8080, Protocol: "TCP"}},
				EnvVars:      []WorkloadEnvVarRef{{Name: "LOG_LEVEL", Value: "info"}},
				ConfigFiles:  []WorkloadConfigFileRef{{MountPath: "/etc/app/application.yaml", Content: "bG9nOiBpbmZv", Base64Encoded: true}},
				WritableDirs: []WorkloadWritableDirRef{{MountPath: "/data", OwnerGroup: "app:app", Mode: "0775"}},
			},
		},
	}
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app")
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app")

	_, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{
		PromotionID: "promotion_dev", FreightID: "freight_1", ApplicationID: "app_1", StageKey: "dev", TargetClusters: targetClusters("dev"),
		Artifacts: []delivery.GitOpsArtifactSpec{{WorkloadID: "workload_api", URI: "registry/user-api:v2", Repository: "registry/user-api", Tag: "v2", Digest: "sha256:api", IsPrimary: true}},
	})
	if err != nil {
		t.Fatalf("ApplyPromotion() error = %v", err)
	}
	values := manifest.Files["apps/order-api/dev/manifests.yaml"]
	for _, expected := range []string{
		"replicas: 2",
		"name: LOG_LEVEL",
		"value: \"info\"",
		"mountPath: /etc/app",
		"log: info",
		"mountPath: /data",
	} {
		if !strings.Contains(values, expected) {
			t.Fatalf("values should contain %q:\n%s", expected, values)
		}
	}
}

func TestApplyPromotionMergesStageConfigAndRendersNamedContainers(t *testing.T) {
	ids := []shared.ID{"deployment_template_platform", "deployment_template_revision_platform", "deployment_1", "manifest_revision_1", "deployment_event_1"}
	svc, manifest, _ := newTestService(t, ids)
	svc.workloads = workloadQuery{
		workloads: map[shared.ID]WorkloadRef{
			"workload_frontend": {ID: "workload_frontend", ApplicationID: "app_1", Name: "macc-frontend", WorkloadType: "Deployment"},
		},
		defaultConfigs: map[shared.ID]WorkloadStageConfigRef{
			"workload_frontend": {
				Replicas:     2,
				NetworkMode:  "host",
				ServicePorts: []WorkloadServicePortRef{{Name: "http", Port: 80, TargetPort: 80, Protocol: "TCP"}},
				IngressHosts: []WorkloadIngressHostRef{{Host: "cloud-ltt.rj.link", Path: "/", ServicePort: "80", TLS: true}},
				InitContainers: []WorkloadInitContainerRef{{
					Name:    "fix-log-permissions",
					Image:   "cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg/busybox:1.34.1",
					Command: []string{"sh", "-c", "mkdir -p /legacy-ignored"},
				}},
				ValuesOverride: map[string]any{
					"containers": []any{
						map[string]any{
							"name": "frontend",
							"config_files": []any{
								map[string]any{"mount_path": "/etc/nginx/nginx.conf", "content": "worker_processes 4;"},
							},
						},
						map[string]any{
							"name": "backend",
							"env_vars": []any{
								map[string]any{"name": "CATALINA_OPTS", "value": "-Xms512m"},
							},
							"liveness_probe": map[string]any{
								"name":                  "liveness",
								"type":                  "http",
								"path":                  "/bizprocessor/ruok",
								"port":                  8080,
								"initial_delay_seconds": 120,
								"period_seconds":        10,
								"failure_threshold":     3,
							},
							"readiness_probe": map[string]any{
								"name":                  "readiness",
								"type":                  "http",
								"path":                  "/bizprocessor/ruok",
								"port":                  8080,
								"initial_delay_seconds": 60,
								"period_seconds":        5,
								"failure-threshold":     5,
							},
							"startup_probe": map[string]any{
								"enabled":           false,
								"type":              "http",
								"path":              "/bizprocessor/ruok",
								"port":              8080,
								"failure_threshold": 30,
							},
							"config_files": []any{
								map[string]any{"mount_path": "/usr/local/tomcat/conf/catalina.properties", "content": "common.loader=/opt/app"},
								map[string]any{"mount_path": "/usr/local/tomcat/macc_conf/logback.xml", "content": "<configuration/>"},
							},
							"writable_dirs": []any{
								map[string]any{"mount_path": "/macc", "owner_group": "10001:0", "mode": "775", "size_limit": "2Gi"},
							},
						},
					},
				},
			},
		},
		configs: map[string]WorkloadStageConfigRef{
			"workload_frontend|dev": {
				Replicas: 1,
				ValuesOverride: map[string]any{
					"containers": []any{
						map[string]any{
							"name": "backend",
							"config_files": []any{
								map[string]any{"mount_path": "/macc/macc_conf/app.properties", "content": "env=dev"},
							},
						},
					},
					"k8sCompat": map[string]any{
						"volumes": []any{
							map[string]any{
								"name": "catalina-config",
								"configMap": map[string]any{
									"name":        "macc-frontend-config",
									"defaultMode": 420,
									"items": []any{
										map[string]any{"key": "catalina.properties", "path": "catalina.properties"},
									},
								},
							},
							map[string]any{"name": "macc-conf", "configMap": map[string]any{"name": "macc-frontend-config"}},
							map[string]any{"name": "app-logs", "emptyDir": map[string]any{}},
							map[string]any{"name": "app-home", "emptyDir": map[string]any{}},
						},
						"containers": []any{
							map[string]any{
								"name": "backend",
								"env": []any{
									map[string]any{"name": "CATALINA_OPTS", "value": "-Xms512m"},
									map[string]any{"name": "POD_NAME", "valueFrom": map[string]any{"fieldRef": map[string]any{"fieldPath": "metadata.name"}}},
								},
								"volumeMounts": []any{
									map[string]any{"name": "catalina-config", "mountPath": "/usr/local/tomcat/conf/catalina.properties", "subPath": "catalina.properties", "readOnly": true},
									map[string]any{"name": "macc-conf", "mountPath": "/usr/local/tomcat/macc_conf"},
									map[string]any{"name": "macc-data", "mountPath": "/macc"},
									map[string]any{"name": "app-logs", "mountPath": "/logs", "subPathExpr": "macc/$(APP_NAME)/$(POD_NAME)"},
									map[string]any{"name": "app-home", "mountPath": "/etc/config"},
								},
							},
						},
					},
				},
			},
		},
	}
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app")
	_, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{
		PromotionID: "promotion_dev", FreightID: "freight_1", ApplicationID: "app_1", StageKey: "dev", TargetClusters: targetClusters("dev"),
		Artifacts: []delivery.GitOpsArtifactSpec{
			{WorkloadID: "workload_frontend", ContainerName: "frontend", URI: "registry/frontend:v1", Repository: "registry/frontend", Tag: "v1", IsPrimary: true},
			{WorkloadID: "workload_frontend", ContainerName: "backend", URI: "registry/macc-webbase:v1", Repository: "registry/macc-webbase", Tag: "v1"},
		},
	})
	if err != nil {
		t.Fatalf("ApplyPromotion() error = %v", err)
	}
	values := manifest.Files["apps/order-api/dev/manifests.yaml"]
	for _, expected := range []string{
		"replicas: 1",
		"hostNetwork: true",
		"kind: Service",
		"kind: Ingress",
		"name: macc-frontend",
		"name: macc-frontend-config",
		"secretName: macc-frontend-tls",
		"name: macc-frontend\n            port:",
		"cert-manager.io/cluster-issuer: \"letsencrypt-prod\"",
		"- name: frontend",
		"image: registry/frontend:v1",
		"- name: backend",
		"image: registry/macc-webbase:v1",
		"name: CATALINA_OPTS",
		"subPath: frontend-nginx.conf",
		"subPath: backend-logback.xml",
		"subPath: backend-app.properties",
		"name: writable-backend-0",
		"sizeLimit: 2Gi",
		"name: app-logs",
		"hostPath:",
		"path: /cloud",
		"type: DirectoryOrCreate",
		"mountPath: /logs",
		"subPathExpr: macc/$(APP_NAME)/$(POD_NAME)",
		"name: \"app-home\"",
		"mountPath: /etc/config",
		"name: APP_NAME",
		"metadata.labels['app']",
		"name: POD_NAME",
		"metadata.name",
		"dnsConfig:",
		"- name: ndots",
		"value: \"1\"",
		"initContainers:",
		"- name: init",
		"image: cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg/busybox:1.34.1",
		"mkdir -p '/etc/config' '/logs' '/logs/nginx' '/macc'",
		"chown '10001:0' '/logs'",
		"chmod '775' '/logs'",
		"chown '10001:0' '/logs/nginx'",
		"chmod '775' '/logs/nginx'",
		"chown '10001:0' '/macc'",
		"chmod '775' '/macc'",
		"runAsUser: 0",
		"livenessProbe:",
		"readinessProbe:",
		"failureThreshold: 3",
		"failureThreshold: 5",
		"path: /bizprocessor/ruok",
		"initialDelaySeconds: 120",
		"initialDelaySeconds: 60",
	} {
		if !strings.Contains(values, expected) {
			t.Fatalf("manifest should contain %q:\n%s", expected, values)
		}
	}
	if strings.Contains(values, "name: app\n") {
		t.Fatalf("named multi-container workload should not render default app container:\n%s", values)
	}
	if strings.Contains(values, "fix-log-permissions") || strings.Contains(values, "/legacy-ignored") {
		t.Fatalf("manifest should use template-managed init container, not user-provided initContainers:\n%s", values)
	}
	if strings.Contains(values, "failure-threshold") || strings.Contains(values, "livenessProbe:\n          name:") || strings.Contains(values, "readinessProbe:\n          name:") {
		t.Fatalf("manifest should render Kubernetes probe fields, not platform probe metadata:\n%s", values)
	}
	if strings.Contains(values, "startupProbe:") {
		t.Fatalf("manifest should not render disabled startup probe:\n%s", values)
	}
	if strings.Contains(values, "\n          mountPath: /usr/local/tomcat/macc_conf\n") {
		t.Fatalf("manifest should not render raw macc_conf directory mount when platform config files mount individual files:\n%s", values)
	}
	if strings.Contains(values, "name: \"macc-conf\"") {
		t.Fatalf("manifest should not render unused raw macc_conf volume after filtering its directory mount:\n%s", values)
	}
	if strings.Contains(values, "name: \"app-logs\"\n        emptyDir") || strings.Contains(values, "name: app-logs\n        emptyDir") {
		t.Fatalf("manifest should render app-logs as hostPath, not emptyDir:\n%s", values)
	}
	if count := strings.Count(values, "mountPath: /logs"); count != 3 {
		t.Fatalf("manifest should mount /logs for frontend, backend, and init container, got %d:\n%s", count, values)
	}
	if !strings.Contains(values, "mountPath: /usr/local/tomcat/macc_conf/logback.xml") || !strings.Contains(values, "mountPath: /macc/macc_conf/app.properties") {
		t.Fatalf("manifest should keep platform single-file config mounts:\n%s", values)
	}
	if count := strings.Count(values, "name: CATALINA_OPTS"); count != 1 {
		t.Fatalf("manifest should render CATALINA_OPTS once, got %d:\n%s", count, values)
	}
	if count := strings.Count(values, "mountPath: /usr/local/tomcat/conf/catalina.properties"); count != 1 {
		t.Fatalf("manifest should render catalina.properties mount once, got %d:\n%s", count, values)
	}
	if strings.Contains(values, "\n      - configMap:\n        defaultMode:") {
		t.Fatalf("raw configMap volume fields should be nested under configMap:\n%s", values)
	}
	if !strings.Contains(values, "\n      - configMap:\n          defaultMode:") {
		t.Fatalf("raw configMap volume should render nested defaultMode:\n%s", values)
	}
	latestDeploy, err := svc.GetLatestDeploymentForStage(context.Background(), "app_1", "dev")
	if err != nil {
		t.Fatalf("GetLatestDeploymentForStage() error = %v", err)
	}
	_, templateRevision, err := svc.GetPlatformTemplateRevision(context.Background())
	if err != nil {
		t.Fatalf("GetPlatformTemplateRevision() error = %v", err)
	}
	currentHash := svc.ComputeStageConfigHashForFreightItems(context.Background(), templateRevision.ID, "dev", []delivery.FreightItem{
		{WorkloadID: "workload_frontend", ContainerName: "frontend", URI: "registry/frontend:v1", ImageRepository: "registry/frontend", ImageTag: "v1"},
		{WorkloadID: "workload_frontend", ContainerName: "backend", URI: "registry/macc-webbase:v1", ImageRepository: "registry/macc-webbase", ImageTag: "v1"},
	})
	if latestDeploy.ConfigHash != currentHash {
		t.Fatalf("workspace config hash should match deployed hash when rendered manifests are unchanged, deployed=%s current=%s", latestDeploy.ConfigHash, currentHash)
	}
}

func TestRenderConfigMapUsesExplicitBlockIndent(t *testing.T) {
	content := "\n which cache used\naliyun.enabled=true\n"
	values := renderConfigMap("macc-frontend-config", "macc", map[string]string{"app": "frontend"}, []WorkloadConfigFileRef{
		{MountPath: "/macc/macc_conf/aliyun.properties", Content: content},
	})
	if !strings.Contains(values, "backend-aliyun.properties: |2") && !strings.Contains(values, "aliyun.properties: |2") {
		t.Fatalf("configmap should render explicit block indent indicator:\n%s", values)
	}
	if strings.Contains(values, "\n  aliyun.properties: |\n") {
		t.Fatalf("configmap should not use auto-detected block indentation:\n%s", values)
	}
	if !strings.Contains(values, "\n     which cache used\n    aliyun.enabled=true\n") {
		t.Fatalf("configmap content indentation should preserve leading space without breaking following lines:\n%s", values)
	}
}

func TestApplyPromotionUpdatesMultipleWorkloadValuesAndRollbackImages(t *testing.T) {
	ids := []shared.ID{
		"deployment_template_platform", "deployment_template_revision_platform",
		"deployment_1", "manifest_revision_1", "deployment_event_1",
		"deployment_2", "manifest_revision_2", "deployment_event_2",
	}
	svc, manifest, _ := newTestService(t, ids)
	svc.workloads = workloadQuery{
		workloads: map[shared.ID]WorkloadRef{
			"workload_api":    {ID: "workload_api", ApplicationID: "app_1", Name: "user-api", DisplayName: "用户 API", WorkloadType: "Deployment"},
			"workload_worker": {ID: "workload_worker", ApplicationID: "app_1", Name: "order-worker", DisplayName: "订单 Worker", WorkloadType: "StatefulSet"},
		},
		configs: map[string]WorkloadStageConfigRef{
			"workload_api|dev": {
				Replicas:         2,
				ServicePorts:     []WorkloadServicePortRef{{Name: "http", Port: 8080, TargetPort: 8080, Protocol: "TCP"}},
				ResourceRequests: WorkloadResourceListRef{CPU: "100m", Memory: "128Mi"},
				ResourceLimits:   WorkloadResourceListRef{CPU: "500m", Memory: "512Mi"},
				EnvVars:          []WorkloadEnvVarRef{{Name: "LOG_LEVEL", Value: "debug"}},
				Probes:           []WorkloadProbeRef{{Name: "readiness", Type: "http", Path: "/ready", Port: 8080, InitialDelaySeconds: 5, PeriodSeconds: 10}},
				IngressHosts:     []WorkloadIngressHostRef{{Host: "api.dev.example.com", Path: "/"}},
				SecretRefs:       []WorkloadSecretRef{{Name: "DB_PASSWORD", SecretRef: "secret/db-password"}},
				ConfigFiles:      []WorkloadConfigFileRef{{MountPath: "/etc/app/config.yaml", Content: "feature: true"}},
				WritableDirs:     []WorkloadWritableDirRef{{MountPath: "/data", SizeLimit: "1Gi"}},
				VolumeMounts:     []WorkloadVolumeMountRef{{Name: "config", MountPath: "/etc/app"}},
				InitContainers:   []WorkloadInitContainerRef{{Name: "init-permission", Image: "cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg/busybox:1.34.1", Command: []string{"sh", "-c", "mkdir -p /legacy-ignored"}}},
				ValuesOverride: map[string]any{
					"k8sCompat": map[string]any{
						"ingress": map[string]any{
							"annotations": map[string]any{
								"cert-manager.io/cluster-issuer": "letsencrypt-prod",
								"higress.io/session-cookie-name": "MACC_FRONTEND_ROUTE",
								"higress.io/session-cookie-path": "/",
								"higress.io/affinity":            "cookie",
								"higress.io/affinity-mode":       "balanced",
							},
						},
					},
				},
			},
		},
	}
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app")
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app")

	_, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{
		PromotionID: "promotion_dev", FreightID: "freight_1", ApplicationID: "app_1", StageKey: "dev", TargetClusters: targetClusters("dev"),
		Artifacts: []delivery.GitOpsArtifactSpec{
			{WorkloadID: "workload_api", URI: "registry/user-api:v2", Repository: "registry/user-api", Tag: "v2", Digest: "sha256:api", IsPrimary: true},
			{WorkloadID: "workload_worker", URI: "registry/order-worker:v5", Repository: "registry/order-worker", Tag: "v5", Digest: "sha256:worker"},
		},
	})
	if err != nil {
		t.Fatalf("apply multi-workload promotion: %v", err)
	}
	values := manifest.Files["apps/order-api/dev/manifests.yaml"]
	for _, want := range []string{
		"kind: Deployment",
		"replicas: 2",
		"image: registry/user-api:v2@sha256:api",
		"cpu: \"100m\"",
		"memory: \"128Mi\"",
		"cpu: \"500m\"",
		"memory: \"512Mi\"",
		"path: /ready",
		"host: api.dev.example.com",
		"ingressClassName: higress",
		"name: DB_PASSWORD",
		"name: secret/db-password",
		"mountPath: /etc/app",
		"mountPath: /data",
		"sizeLimit: 1Gi",
		"name: config",
		"name: init",
		"securityContext:",
		"runAsUser: 0",
		"mkdir -p '/data' '/logs' '/logs/nginx'",
		"chown '10001:0' '/logs'",
		"chmod '775' '/logs'",
		"cert-manager.io/cluster-issuer: \"letsencrypt-prod\"",
		"higress.io/affinity: \"cookie\"",
		"higress.io/affinity-mode: \"balanced\"",
		"higress.io/session-cookie-name: \"MACC_FRONTEND_ROUTE\"",
		"higress.io/session-cookie-path: \"/\"",
		"kind: StatefulSet",
		"image: registry/order-worker:v5@sha256:worker",
	} {
		if !strings.Contains(values, want) {
			t.Fatalf("values missing %q:\n%s", want, values)
		}
	}
	if strings.Contains(values, "ingressClassName: nginx") {
		t.Fatalf("values should not contain nginx ingress class:\n%s", values)
	}
	if strings.Contains(values, "init-permission") || strings.Contains(values, "/legacy-ignored") {
		t.Fatalf("values should ignore configured initContainers and render template-managed init:\n%s", values)
	}
	deployment, err := svc.GetDeployment(context.Background(), "deployment_1")
	if err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	if !strings.Contains(deployment.WorkloadSummary, "user-api/app=registry/user-api:v2@sha256:api") || !strings.Contains(deployment.WorkloadSummary, "order-worker/app=registry/order-worker:v5@sha256:worker") {
		t.Fatalf("deployment summary missing workload images: %#v", deployment)
	}

	_, err = svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{
		PromotionID: "promotion_rollback", FreightID: "freight_history", ApplicationID: "app_1", StageKey: "dev", TargetClusters: targetClusters("dev"), IsRollback: true,
		Artifacts: []delivery.GitOpsArtifactSpec{
			{WorkloadID: "workload_api", URI: "registry/user-api:v1", Repository: "registry/user-api", Tag: "v1", Digest: "sha256:api-old", IsPrimary: true},
			{WorkloadID: "workload_worker", URI: "registry/order-worker:v4", Repository: "registry/order-worker", Tag: "v4", Digest: "sha256:worker-old"},
		},
	})
	if err != nil {
		t.Fatalf("rollback promotion: %v", err)
	}
	rollbackValues := manifest.Files["apps/order-api/dev/manifests.yaml"]
	if !strings.Contains(rollbackValues, "registry/user-api:v1@sha256:api-old") || !strings.Contains(rollbackValues, "registry/order-worker:v4@sha256:worker-old") {
		t.Fatalf("rollback values did not use historical freight images:\n%s", rollbackValues)
	}
	revision, err := svc.repo.GetManifestRevision(context.Background(), "manifest_revision_2")
	if err != nil || revision.ChangeType != "rollback" {
		t.Fatalf("rollback manifest revision = %#v err=%v", revision, err)
	}
}

func TestRenderIngressAddsDefaultCertManagerAnnotationForTLS(t *testing.T) {
	ingress := renderIngress(
		"macc-frontend",
		"macc",
		map[string]string{"app.kubernetes.io/name": "macc-frontend"},
		[]WorkloadIngressHostRef{{Host: "cloud-ltt.rj.link", Path: "/", TLS: true}},
		[]WorkloadServicePortRef{{Name: "http", Port: 80, TargetPort: 80, Protocol: "TCP"}},
		nil,
	)
	for _, want := range []string{
		"annotations:",
		"cert-manager.io/cluster-issuer: \"letsencrypt-prod\"",
		"ingressClassName: higress",
		"tls:",
		"secretName: macc-frontend-tls",
	} {
		if !strings.Contains(ingress, want) {
			t.Fatalf("ingress missing %q:\n%s", want, ingress)
		}
	}

	withoutTLS := renderIngress(
		"macc-frontend",
		"macc",
		map[string]string{"app.kubernetes.io/name": "macc-frontend"},
		[]WorkloadIngressHostRef{{Host: "cloud-ltt.rj.link", Path: "/"}},
		[]WorkloadServicePortRef{{Name: "http", Port: 80, TargetPort: 80, Protocol: "TCP"}},
		nil,
	)
	if strings.Contains(withoutTLS, "cert-manager.io/cluster-issuer") {
		t.Fatalf("non-TLS ingress should not include cert-manager annotation:\n%s", withoutTLS)
	}

	customIssuer := renderIngress(
		"macc-frontend",
		"macc",
		map[string]string{"app.kubernetes.io/name": "macc-frontend"},
		[]WorkloadIngressHostRef{{Host: "cloud-ltt.rj.link", Path: "/", TLS: true}},
		[]WorkloadServicePortRef{{Name: "http", Port: 80, TargetPort: 80, Protocol: "TCP"}},
		map[string]any{"ingressAnnotations": map[string]any{"cert-manager.io/cluster-issuer": "corp-ca"}},
	)
	if !strings.Contains(customIssuer, "cert-manager.io/cluster-issuer: \"corp-ca\"") {
		t.Fatalf("custom issuer should be preserved:\n%s", customIssuer)
	}
	if strings.Contains(customIssuer, "cert-manager.io/cluster-issuer: \"letsencrypt-prod\"") {
		t.Fatalf("default issuer should not override custom issuer:\n%s", customIssuer)
	}
}

func TestRenderWorkloadAddsLegacyAppLabel(t *testing.T) {
	labels := buildLabels("macc-frontend", "macc-frontend", "frontend", "app_1", "dev", "deployment_1")
	workload := renderWorkload(
		WorkloadRef{Name: "macc-frontend", WorkloadType: "Deployment"},
		WorkloadStageConfigRef{},
		map[string]containerImage{"frontend": {Repository: "registry/frontend", Tag: "v1"}},
		"macc-frontend",
		"macc",
		labels,
	)
	if count := strings.Count(workload, "app: \"macc-frontend\""); count != 2 {
		t.Fatalf("workload should include legacy app label on resource and pod template, got %d:\n%s", count, workload)
	}
	if !strings.Contains(workload, "app.kubernetes.io/name: \"macc-frontend\"") {
		t.Fatalf("workload should retain app.kubernetes labels:\n%s", workload)
	}
	for _, want := range []string{
		"podAntiAffinity:",
		"preferredDuringSchedulingIgnoredDuringExecution:",
		"weight: 100",
		"app.kubernetes.io/name: \"macc-frontend\"",
		"app.kubernetes.io/instance: \"macc-frontend\"",
		"topologyKey: \"kubernetes.io/hostname\"",
	} {
		if !strings.Contains(workload, want) {
			t.Fatalf("workload should include soft pod anti-affinity %q:\n%s", want, workload)
		}
	}
	if strings.Contains(workload, "requiredDuringSchedulingIgnoredDuringExecution") {
		t.Fatalf("workload should use soft pod anti-affinity, not required anti-affinity:\n%s", workload)
	}
}

func TestApplyPromotionRecordsFailedDeploymentWhenProdCommitFails(t *testing.T) {
	errBoom := shared.NewError(shared.CodeInternal, "gitlab commit failed")
	repo := newTestRepository(t)
	svc := NewService(Options{
		Repository: repo, ManifestRepo: gitopsErrManifest{err: errBoom},
		Application: appQuery{"app_1": {ID: "app_1", TenantID: "tenant_1", ProjectID: "project_1", Name: "order-api"}},
		IDGenerator: &staticIDs{ids: []shared.ID{"template_1", "revision_1", "template_2", "revision_2", "deployment_failed", "manifest_failed", "event_failed"}},
		Clock:       fixedClock{now: time.Date(2026, 5, 30, 13, 0, 0, 0, time.UTC)},
	})
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers: []")
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app")
	if _, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{PromotionID: "promotion_prod", FreightID: "freight_1", ApplicationID: "app_1", StageKey: "prod", TargetClusters: targetClusters("prod"), ImageURI: "repo:v1"}); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("expected commit error, got %v", err)
	}
	deployment, err := repo.FindDeploymentByPromotion(context.Background(), "promotion_prod")
	if err != nil {
		t.Fatalf("failed deployment should be recorded: %v", err)
	}
	if deployment.Status != DeploymentFailed || !strings.Contains(deployment.Message, "提交部署清单失败") {
		t.Fatalf("failed deployment should contain Chinese commit failure reason: %#v", deployment)
	}
	if !deployment.ManifestRevisionID.IsZero() {
		t.Fatalf("failed deployment should not reference unsaved manifest revision: %#v", deployment)
	}
	events := listDeploymentEventsForTest(t, repo, deployment.ID)
	if len(events) != 1 || events[0].Status != DeploymentFailed || !strings.Contains(events[0].Message, "提交部署清单失败") {
		t.Fatalf("failed deployment event should contain Chinese commit failure reason: %#v", events)
	}
}

func TestApplyPromotionRecordsFailedDeploymentWhenCommitFails(t *testing.T) {
	errBoom := shared.NewError(shared.CodeInternal, "gitlab commit failed")
	repo := newTestRepository(t)
	svc := NewService(Options{
		Repository: repo, ManifestRepo: gitopsErrManifest{err: errBoom},
		Application: appQuery{"app_1": {ID: "app_1", TenantID: "tenant_1", ProjectID: "project_1", Name: "order-api"}},
		IDGenerator: &staticIDs{ids: []shared.ID{"template_1", "revision_1", "template_2", "revision_2", "deployment_failed", "manifest_failed", "event_failed"}},
		Clock:       fixedClock{now: time.Date(2026, 5, 30, 13, 0, 0, 0, time.UTC)},
	})
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers: []")
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app")
	if _, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{PromotionID: "promotion_dev", FreightID: "freight_1", ApplicationID: "app_1", StageKey: "dev", TargetClusters: targetClusters("dev"), ImageURI: "repo:v1"}); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("expected commit error, got %v", err)
	}
	deployment, err := repo.FindDeploymentByPromotion(context.Background(), "promotion_dev")
	if err != nil {
		t.Fatalf("failed deployment should be recorded: %v", err)
	}
	if deployment.Status != DeploymentFailed || !strings.Contains(deployment.Message, "提交部署清单失败") {
		t.Fatalf("failed deployment should contain Chinese commit failure reason: %#v", deployment)
	}
	if !deployment.ManifestRevisionID.IsZero() {
		t.Fatalf("failed deployment should not reference unsaved manifest revision: %#v", deployment)
	}
	events := listDeploymentEventsForTest(t, repo, deployment.ID)
	if len(events) != 1 || events[0].Status != DeploymentFailed || !strings.Contains(events[0].Message, "提交部署清单失败") {
		t.Fatalf("failed deployment event should contain Chinese commit failure reason: %#v", events)
	}
}

func TestGitOpsHTTPHandlerCoversTemplateAndDeploymentAPIs(t *testing.T) {
	ids := []shared.ID{
		"deployment_template_platform", "deployment_template_revision_platform",
		"deployment_1", "manifest_revision_1", "deployment_event_1",
	}
	svc, manifest, _ := newTestService(t, ids)
	mux := http.NewServeMux()
	NewHandler(svc).Register(mux)

	createPlatform := httptest.NewRecorder()
	mux.ServeHTTP(createPlatform, httptest.NewRequest(http.MethodPost, "/api/deployment-templates/platform", bytes.NewBufferString(`{"name":"java","content":"containers:\n- name: app"}`)))
	if createPlatform.Code != http.StatusCreated {
		t.Fatalf("create platform status=%d body=%s", createPlatform.Code, createPlatform.Body.String())
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

	if _, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{PromotionID: "promotion_dev", FreightID: "freight_1", ApplicationID: "app_1", StageKey: "dev", TargetClusters: targetClusters("dev"), ImageURI: "registry/order-api:v1", ImageDigest: "sha256:1"}); err != nil {
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
	svc, _, _ := newTestService(t, []shared.ID{"deployment_template_platform", "deployment_template_revision_platform"})
	if _, err := svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app"); err != nil {
		t.Fatalf("platform template: %v", err)
	}
	if _, err := svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app"); err != nil {
		t.Fatalf("idempotent platform template should return existing: %v", err)
	}
	if _, err := svc.UpdatePlatformTemplate(context.Background(), "", "user_1"); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("empty template content should fail, got %v", err)
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
	repo := newTestRepository(t)
	svcNoRevision := NewService(Options{Repository: repo})
	if err := repo.CreateTemplate(context.Background(), DeploymentTemplate{ID: "template_without_revision", Name: "test-no-revision"}); err != nil {
		t.Fatalf("create template without revision: %v", err)
	}
	if _, err := repo.GetCurrentTemplateRevision(context.Background(), "template_without_revision"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing current revision should fail, got %v", err)
	}
	_ = svcNoRevision
}

func TestGitOpsRepositoryConflictAndMissingBranches(t *testing.T) {
	repo := newTestRepository(t)
	ctx := context.Background()
	template := DeploymentTemplate{ID: "template_1", Name: "java"}
	if err := repo.CreateTemplate(ctx, template); err != nil {
		t.Fatalf("create template: %v", err)
	}
	if err := repo.CreateTemplate(ctx, template); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate template should conflict, got %v", err)
	}
	if err := repo.CreateTemplate(ctx, DeploymentTemplate{ID: "template_2", Name: "java"}); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate name template should conflict, got %v", err)
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
	first := Deployment{ID: "deployment_history_1", ApplicationID: "app_1", StageKey: "dev", PromotionID: "promotion_history_1", FreightID: "freight_1", ManifestRevisionID: "manifest_history_1", CreatedAt: time.Date(2026, 5, 30, 13, 1, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 5, 30, 13, 1, 0, 0, time.UTC)}
	second := Deployment{ID: "deployment_history_2", ApplicationID: "app_1", StageKey: "dev", PromotionID: "promotion_history_2", FreightID: "freight_2", ManifestRevisionID: "manifest_history_2", CreatedAt: time.Date(2026, 5, 30, 13, 2, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 5, 30, 13, 2, 0, 0, time.UTC)}
	withoutCommit := Deployment{ID: "deployment_history_pending", ApplicationID: "app_1", StageKey: "dev", PromotionID: "promotion_history_pending", FreightID: "freight_pending", ManifestRevisionID: "manifest_history_pending", CreatedAt: time.Date(2026, 5, 30, 13, 3, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 5, 30, 13, 3, 0, 0, time.UTC)}
	for _, item := range []Deployment{first, second, withoutCommit} {
		if err := repo.CreateDeployment(ctx, item); err != nil {
			t.Fatalf("create history deployment %s: %v", item.ID, err)
		}
	}
	revisions := []ManifestRevision{
		{ID: "manifest_history_1", DeploymentID: first.ID, PromotionID: first.PromotionID, ApplicationID: first.ApplicationID, StageKey: first.StageKey, Path: "apps/order/dev/manifests.yaml", CommitSHA: "commit_11111111", ChangeType: "commit", CreatedAt: first.CreatedAt},
		{ID: "manifest_history_2", DeploymentID: second.ID, PromotionID: second.PromotionID, ApplicationID: second.ApplicationID, StageKey: second.StageKey, Path: "apps/order/dev/manifests.yaml", CommitSHA: "commit_22222222", ChangeType: "commit", CreatedAt: second.CreatedAt},
		{ID: "manifest_history_pending", DeploymentID: withoutCommit.ID, PromotionID: withoutCommit.PromotionID, ApplicationID: withoutCommit.ApplicationID, StageKey: withoutCommit.StageKey, Path: "apps/order/dev/manifests.yaml", ChangeType: "commit", CreatedAt: withoutCommit.CreatedAt},
	}
	for _, revision := range revisions {
		if err := repo.CreateManifestRevision(ctx, revision); err != nil {
			t.Fatalf("create history revision %s: %v", revision.ID, err)
		}
	}
	history, err := repo.ListDeploymentsByStage(ctx, "app_1", "dev", shared.PageRequest{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListDeploymentsByStage() error = %v", err)
	}
	if history.Total != 2 || len(history.Items) != 2 || history.Items[0].ID != second.ID || history.Items[1].ID != first.ID {
		t.Fatalf("unexpected history page: total=%d items=%#v", history.Total, history.Items)
	}
	previous, err := repo.GetPreviousCommittedDeploymentForStage(ctx, "app_1", "dev", second.CreatedAt, second.ID)
	if err != nil {
		t.Fatalf("GetPreviousCommittedDeploymentForStage() error = %v", err)
	}
	if previous.ID != first.ID {
		t.Fatalf("previous deployment = %s, want %s", previous.ID, first.ID)
	}
}

func TestGitOpsServicePropagatesTemplateAndQueryErrors(t *testing.T) {
	errBoom := shared.NewError(shared.CodeInternal, "boom")
	repo := &gitopsErrRepo{Repository: newTestRepository(t)}
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
	repo.updateTemplateErr = errBoom
	if _, err := svc.UpdatePlatformTemplate(context.Background(), "containers: []", "user_1"); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("update template error should propagate, got %v", err)
	}
	repo.updateTemplateErr = nil
	repo.createTemplateRevisionErr = errBoom
	if _, err := svc.UpdatePlatformTemplate(context.Background(), "containers: []", "user_1"); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("update revision error should propagate, got %v", err)
	}

	idFailSvc := NewService(Options{Repository: newTestRepository(t), ManifestRepo: NewFakeManifestRepository(), IDGenerator: gitopsFailingIDs{err: errBoom}, Clock: fixedClock{now: time.Date(2026, 5, 30, 13, 0, 0, 0, time.UTC)}})
	if _, err := idFailSvc.EnsurePlatformTemplate(context.Background(), "java", "containers: []"); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("id failure should propagate, got %v", err)
	}
	appErrSvc := NewService(Options{Repository: newTestRepository(t), ManifestRepo: NewFakeManifestRepository(), Application: errAppQuery{err: errBoom}, IDGenerator: &staticIDs{ids: []shared.ID{"template_1", "revision_1"}}, Clock: fixedClock{now: time.Date(2026, 5, 30, 13, 0, 0, 0, time.UTC)}})
	if _, err := appErrSvc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{ApplicationID: "app_1"}); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("app query error should propagate, got %v", err)
	}
}

func TestGitOpsServicePropagatesPromotionAndAgentUpdateErrors(t *testing.T) {
	errBoom := shared.NewError(shared.CodeInternal, "boom")
	repo := &gitopsErrRepo{Repository: newTestRepository(t)}
	manifest := NewFakeManifestRepository()
	svc := NewService(Options{
		Repository: repo, ManifestRepo: manifest,
		Application: appQuery{"app_1": {ID: "app_1", TenantID: "tenant_1", ProjectID: "project_1", Name: "order-api"}},
		IDGenerator: &staticIDs{ids: []shared.ID{"template_1", "revision_1", "template_2", "revision_2", "deployment_1", "manifest_1", "event_1", "deployment_2", "manifest_2", "event_2", "deployment_event_update"}},
		Clock:       fixedClock{now: time.Date(2026, 5, 30, 13, 0, 0, 0, time.UTC)},
	})
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers: []")
	_, _ = svc.EnsurePlatformTemplate(context.Background(), "java", "containers:\n- name: app")
	appErrSvc := NewService(Options{Repository: repo, ManifestRepo: manifest, Application: errAppQuery{err: errBoom}})
	if _, err := appErrSvc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{ApplicationID: "app_1"}); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("app query error should propagate, got %v", err)
	}
	commitErrSvc := NewService(Options{Repository: repo, ManifestRepo: gitopsErrManifest{err: errBoom}, Application: svc.apps, IDGenerator: &staticIDs{ids: []shared.ID{"deployment_bad", "manifest_bad", "event_bad"}}, Clock: fixedClock{now: time.Date(2026, 5, 30, 13, 0, 0, 0, time.UTC)}})
	if _, err := commitErrSvc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{PromotionID: "promotion_dev", FreightID: "freight_1", ApplicationID: "app_1", StageKey: "dev", TargetClusters: targetClusters("dev"), ImageURI: "repo:v1"}); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("manifest commit error should propagate, got %v", err)
	}
	prodCommitErrSvc := NewService(Options{Repository: repo, ManifestRepo: gitopsErrManifest{err: errBoom}, Application: svc.apps, IDGenerator: &staticIDs{ids: []shared.ID{"deployment_bad2", "manifest_bad2", "event_bad2"}}, Clock: fixedClock{now: time.Date(2026, 5, 30, 13, 0, 0, 0, time.UTC)}})
	if _, err := prodCommitErrSvc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{PromotionID: "promotion_prod", FreightID: "freight_1", ApplicationID: "app_1", StageKey: "prod", TargetClusters: targetClusters("prod"), ImageURI: "repo:v1"}); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("manifest prod commit error should propagate, got %v", err)
	}
	repo.createDeploymentErr = errBoom
	if _, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{PromotionID: "promotion_dev", FreightID: "freight_1", ApplicationID: "app_1", StageKey: "dev", TargetClusters: targetClusters("dev"), ImageURI: "repo:v1"}); shared.CodeOf(err) != shared.CodeInternal {
		t.Fatalf("create deployment error should propagate, got %v", err)
	}
	repo.createDeploymentErr = nil
	repo.createManifestRevisionErr = errBoom
	if _, err := svc.ApplyPromotion(context.Background(), delivery.GitOpsPromotionSpec{PromotionID: "promotion_dev2", FreightID: "freight_1", ApplicationID: "app_1", StageKey: "dev", TargetClusters: targetClusters("dev"), ImageURI: "repo:v1"}); shared.CodeOf(err) != shared.CodeInternal {
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
