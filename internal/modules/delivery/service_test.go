package delivery

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
	"github.com/shareinto/paas/internal/platform/database"
	"github.com/shareinto/paas/internal/shared"
	"github.com/shareinto/paas/internal/shared/testutil"
	"github.com/shareinto/paas/internal/testsupport"
)

type fakeBuildQuery struct {
	runs      map[shared.ID]BuildRunRef
	artifacts map[shared.ID]BuildArtifactRef
}

func (q fakeBuildQuery) GetBuildRun(_ context.Context, id shared.ID) (BuildRunRef, error) {
	v, ok := q.runs[id]
	if !ok {
		return BuildRunRef{}, shared.NewError(shared.CodeNotFound, "build run not found")
	}
	return v, nil
}
func (q fakeBuildQuery) GetBuildArtifact(_ context.Context, id shared.ID) (BuildArtifactRef, error) {
	v, ok := q.artifacts[id]
	if !ok {
		return BuildArtifactRef{}, shared.NewError(shared.CodeNotFound, "build artifact not found")
	}
	return v, nil
}

func (q fakeBuildQuery) ListBuildArtifacts(_ context.Context, buildRunID shared.ID) ([]BuildArtifactRef, error) {
	out := make([]BuildArtifactRef, 0)
	for _, artifact := range q.artifacts {
		if artifact.BuildRunID == buildRunID {
			out = append(out, artifact)
		}
	}
	if len(out) == 0 {
		return nil, shared.NewError(shared.CodeNotFound, "build artifact not found")
	}
	return out, nil
}

type fakeAppQuery struct{ apps map[shared.ID]ApplicationRef }

func (q fakeAppQuery) GetApplication(_ context.Context, id shared.ID) (ApplicationRef, error) {
	v, ok := q.apps[id]
	if !ok {
		return ApplicationRef{}, shared.NewError(shared.CodeNotFound, "application not found")
	}
	return v, nil
}

type fakeWorkloadQuery struct{ workloads map[shared.ID][]WorkloadRef }

func (q fakeWorkloadQuery) ListEnabledWorkloads(_ context.Context, appID shared.ID) ([]WorkloadRef, error) {
	if workloads, ok := q.workloads[appID]; ok {
		return workloads, nil
	}
	return nil, shared.NewError(shared.CodeNotFound, "workloads not found")
}

type fakeEnvQuery struct{ envs map[shared.ID]EnvironmentRef }

func (q fakeEnvQuery) ListEnvironments(_ context.Context, appID shared.ID) ([]EnvironmentRef, error) {
	out := []EnvironmentRef{}
	for _, env := range q.envs {
		if env.ApplicationID == appID {
			out = append(out, env)
		}
	}
	return out, nil
}
func (q fakeEnvQuery) GetEnvironment(_ context.Context, id shared.ID) (EnvironmentRef, error) {
	v, ok := q.envs[id]
	if !ok {
		return EnvironmentRef{}, shared.NewError(shared.CodeNotFound, "environment not found")
	}
	return v, nil
}

type recordingGitOps struct {
	specs []GitOpsPromotionSpec
	err   error
}

func (g *recordingGitOps) ApplyPromotion(_ context.Context, spec GitOpsPromotionSpec) (GitOpsPromotionResult, error) {
	g.specs = append(g.specs, spec)
	if g.err != nil {
		return GitOpsPromotionResult{}, g.err
	}
	return GitOpsPromotionResult{ManifestRevision: "rev-1"}, nil
}

type recordingPermission struct {
	calls []identityaccess.Permission
	err   error
}

func (p *recordingPermission) Check(_ context.Context, _ identityaccess.Subject, _ identityaccess.ResourceScope, action identityaccess.Permission) error {
	p.calls = append(p.calls, action)
	if p.err != nil {
		return p.err
	}
	return nil
}

type recordingAudit struct{ events []AuditEvent }

func (a *recordingAudit) Log(_ context.Context, e AuditEvent) error {
	a.events = append(a.events, e)
	return nil
}

type recordingPublisher struct{ events []shared.DomainEvent }

func (p *recordingPublisher) Publish(_ context.Context, e shared.DomainEvent) error {
	p.events = append(p.events, e)
	return nil
}

type deliveryEnv struct {
	svc      *Service
	repo     Repository
	gitops   *recordingGitOps
	audit    *recordingAudit
	events   *recordingPublisher
	envQuery fakeEnvQuery
}

func newTestRepository(t *testing.T) Repository {
	t.Helper()
	repo, err := NewMySQLRepository(context.Background(), testsupport.MySQLDB(t, Migrations...))
	if err != nil {
		t.Fatalf("NewMySQLRepository() error = %v", err)
	}
	return repo
}

func newDeliveryEnv(t *testing.T) deliveryEnv {
	t.Helper()
	repo := newTestRepository(t)
	gitops := &recordingGitOps{}
	audit := &recordingAudit{}
	events := &recordingPublisher{}
	envs := fakeEnvQuery{envs: map[shared.ID]EnvironmentRef{
		"env_dev":     {ID: "env_dev", TenantID: "tenant_a", ProjectID: "project_payment", ApplicationID: "app_user", Name: "dev", Status: "deployable", BindingActive: true},
		"env_test":    {ID: "env_test", TenantID: "tenant_a", ProjectID: "project_payment", ApplicationID: "app_user", Name: "test", Status: "deployable", BindingActive: true},
		"env_staging": {ID: "env_staging", TenantID: "tenant_a", ProjectID: "project_payment", ApplicationID: "app_user", Name: "staging", Status: "deployable", BindingActive: true},
		"env_prod":    {ID: "env_prod", TenantID: "tenant_a", ProjectID: "project_payment", ApplicationID: "app_user", Name: "prod", Status: "deployable", BindingActive: true},
		"env_pending": {ID: "env_pending", TenantID: "tenant_a", ProjectID: "project_payment", ApplicationID: "app_user", Name: "qa", Status: "pending_cluster_binding", BindingActive: false},
	}}
	svc := NewService(Options{
		Repository: repo,
		BuildQuery: fakeBuildQuery{
			runs:      map[shared.ID]BuildRunRef{"build_1": {ID: "build_1", TenantID: "tenant_a", ProjectID: "project_payment", ApplicationID: "app_user", PipelineID: "pipeline_main", PipelineName: "main", PipelineDisplayName: "主流水线", CommitSHA: "abcdef1234567890"}},
			artifacts: map[shared.ID]BuildArtifactRef{"artifact_1": {ID: "artifact_1", BuildRunID: "build_1", ApplicationID: "app_user", WorkloadID: "workload_api", URI: "registry.example/paas/user-api:abcdef", Digest: "sha256:abc", IsPrimary: true}},
		},
		ApplicationQuery:  fakeAppQuery{apps: map[shared.ID]ApplicationRef{"app_user": {ID: "app_user", TenantID: "tenant_a", ProjectID: "project_payment", Name: "user-api"}}},
		WorkloadQuery:     fakeWorkloadQuery{workloads: map[shared.ID][]WorkloadRef{"app_user": {{ID: "workload_api", TenantID: "tenant_a", ProjectID: "project_payment", ApplicationID: "app_user", Name: "api", DisplayName: "用户接口", Status: "enabled"}}}},
		EnvironmentQuery:  envs,
		GitOpsDeployment:  gitops,
		PermissionChecker: &recordingPermission{},
		Audit:             audit,
		EventPublisher:    events,
		IDGenerator:       testutil.NewFakeIDGenerator(1),
		Clock:             testutil.NewFakeClock(time.Date(2026, 5, 30, 7, 0, 0, 0, time.UTC)),
	})
	return deliveryEnv{svc: svc, repo: repo, gitops: gitops, audit: audit, events: events, envQuery: envs}
}

func actor(id shared.ID) identityaccess.Subject {
	return identityaccess.Subject{Type: identityaccess.SubjectUser, ID: id}
}

func seedFreight(t *testing.T, env deliveryEnv) Freight {
	t.Helper()
	release, err := env.svc.HandleBuildSucceeded(context.Background(), BuildSucceededPayload{BuildRunID: "build_1", ApplicationID: "app_user", WorkloadID: "workload_api", BuildArtifactID: "artifact_1"})
	if err != nil {
		t.Fatalf("HandleBuildSucceeded() error = %v", err)
	}
	freight, err := env.svc.CreateFreight(context.Background(), CreateFreightInput{
		Actor:         actor("usr_dev"),
		ApplicationID: "app_user",
		Name:          "freight-main",
		Items: []CreateFreightItemInput{{
			WorkloadID: "workload_api",
			SourceType: FreightItemPipelineArtifact,
			ReleaseID:  release.ID,
		}},
	})
	if err != nil {
		t.Fatalf("CreateFreight() error = %v", err)
	}
	return freight
}

func promoteFreightThrough(t *testing.T, env deliveryEnv, freight Freight, environmentIDs ...shared.ID) {
	t.Helper()
	for _, environmentID := range environmentIDs {
		promotion, err := env.svc.CreatePromotion(context.Background(), CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetEnvironmentID: environmentID})
		if err != nil {
			t.Fatalf("CreatePromotion(%s) error = %v", environmentID, err)
		}
		if promotion.Status != PromotionManifestUpdated {
			t.Fatalf("CreatePromotion(%s) status = %s, want %s", environmentID, promotion.Status, PromotionManifestUpdated)
		}
	}
}

func TestBuildSucceededCreatesWorkloadReleaseCandidateOnly(t *testing.T) {
	env := newDeliveryEnv(t)
	release, err := env.svc.HandleBuildSucceeded(context.Background(), BuildSucceededPayload{BuildRunID: "build_1", ApplicationID: "app_user", WorkloadID: "workload_api", BuildArtifactID: "artifact_1"})
	if err != nil {
		t.Fatalf("HandleBuildSucceeded() error = %v", err)
	}
	if release.Status != ReleaseReady || release.Version != "abcdef123456" || release.WorkloadID != "workload_api" {
		t.Fatalf("unexpected release: %+v", release)
	}
	if release.PipelineID != "pipeline_main" || release.PipelineName != "main" || release.ImageRepository != "registry.example/paas/user-api" || release.ImageTag != "abcdef" {
		t.Fatalf("release should keep pipeline and image identity, got %+v", release)
	}
	freights, err := env.repo.ListFreightsByApplication(context.Background(), "app_user", shared.PageRequest{Page: 1, PageSize: 10})
	if err != nil || len(freights.Items) != 0 {
		t.Fatalf("BuildSucceeded must not auto-create freight, got %+v, %v", freights.Items, err)
	}
	flow, err := env.repo.FindDeliveryFlowByApplication(context.Background(), "app_user")
	if err != nil {
		t.Fatalf("default flow missing: %v", err)
	}
	stages, err := env.repo.ListDeliveryStages(context.Background(), flow.ID)
	if err != nil || len(stages) != 4 || stages[3].Name != "prod" || !stages[3].RequiresApproval {
		t.Fatalf("unexpected stages: %+v, %v", stages, err)
	}
	if len(env.events.events) != 0 {
		t.Fatalf("BuildSucceeded should not publish FreightCreated, got %+v", env.events.events)
	}
	againRelease, err := env.svc.HandleBuildSucceeded(context.Background(), BuildSucceededPayload{BuildRunID: "build_1", ApplicationID: "app_user", WorkloadID: "workload_api", BuildArtifactID: "artifact_1"})
	if err != nil || againRelease.ID != release.ID {
		t.Fatalf("idempotent build succeeded failed: %+v %v", againRelease, err)
	}
}

func TestManualFreightValidatesWorkloadCoverageSourcesAndCustomImageRisk(t *testing.T) {
	env := newDeliveryEnv(t)
	env.svc.workloads = fakeWorkloadQuery{workloads: map[shared.ID][]WorkloadRef{"app_user": {
		{ID: "workload_api", TenantID: "tenant_a", ProjectID: "project_payment", ApplicationID: "app_user", Name: "api", DisplayName: "用户接口", Status: "enabled"},
		{ID: "workload_worker", TenantID: "tenant_a", ProjectID: "project_payment", ApplicationID: "app_user", Name: "worker", DisplayName: "后台任务", Status: "enabled"},
	}}}
	env.svc.builds = fakeBuildQuery{
		runs: map[shared.ID]BuildRunRef{
			"build_1": {ID: "build_1", TenantID: "tenant_a", ProjectID: "project_payment", ApplicationID: "app_user", PipelineID: "pipeline_api", PipelineName: "api", CommitSHA: "abcdef1234567890"},
		},
		artifacts: map[shared.ID]BuildArtifactRef{
			"artifact_1": {ID: "artifact_1", BuildRunID: "build_1", ApplicationID: "app_user", WorkloadID: "workload_api", URI: "registry.example/paas/user-api:abcdef", Digest: "sha256:abc", IsPrimary: true},
		},
	}
	release, err := env.svc.HandleBuildSucceeded(context.Background(), BuildSucceededPayload{BuildRunID: "build_1", ApplicationID: "app_user", WorkloadID: "workload_api", BuildArtifactID: "artifact_1"})
	if err != nil {
		t.Fatalf("HandleBuildSucceeded() error = %v", err)
	}
	if _, err := env.svc.CreateFreight(context.Background(), CreateFreightInput{Actor: actor("usr_dev"), ApplicationID: "app_user", Items: []CreateFreightItemInput{{WorkloadID: "workload_api", SourceType: FreightItemPipelineArtifact, ReleaseID: release.ID}}}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("missing workload should fail, got %v", err)
	}
	_, err = env.svc.CreateFreight(context.Background(), CreateFreightInput{Actor: actor("usr_dev"), ApplicationID: "app_user", Items: []CreateFreightItemInput{
		{WorkloadID: "workload_api", SourceType: FreightItemPipelineArtifact, ReleaseID: release.ID},
		{WorkloadID: "workload_api", SourceType: FreightItemCustomImage, ImageRef: "registry.example/paas/worker:1.0"},
	}})
	if shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate workload should fail, got %v", err)
	}
	_, err = env.svc.CreateFreight(context.Background(), CreateFreightInput{Actor: actor("usr_dev"), ApplicationID: "app_user", Items: []CreateFreightItemInput{
		{WorkloadID: "workload_worker", SourceType: FreightItemPipelineArtifact, ReleaseID: release.ID},
		{WorkloadID: "workload_api", SourceType: FreightItemCustomImage, ImageRef: "registry.example/paas/user-api:1.0"},
	}})
	if shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("pipeline artifact for wrong workload should fail, got %v", err)
	}
	_, err = env.svc.CreateFreight(context.Background(), CreateFreightInput{Actor: actor("usr_dev"), ApplicationID: "app_user", Items: []CreateFreightItemInput{
		{WorkloadID: "workload_api", SourceType: FreightItemPipelineArtifact, ReleaseID: release.ID},
		{WorkloadID: "workload_worker", SourceType: FreightItemCustomImage, ImageRef: "not a valid image"},
	}})
	if shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("invalid custom image should fail, got %v", err)
	}
	freight, err := env.svc.CreateFreight(context.Background(), CreateFreightInput{Actor: actor("usr_dev"), ApplicationID: "app_user", Name: "complete", Items: []CreateFreightItemInput{
		{WorkloadID: "workload_api", SourceType: FreightItemPipelineArtifact, ReleaseID: release.ID},
		{WorkloadID: "workload_worker", SourceType: FreightItemCustomImage, ImageRef: "registry.example/paas/worker:1.0"},
	}})
	if err != nil {
		t.Fatalf("CreateFreight() error = %v", err)
	}
	items, err := env.repo.ListFreightItems(context.Background(), freight.ID)
	if err != nil || len(items) != 2 || items[1].SourceType != FreightItemCustomImage || items[1].ImageTag != "1.0" {
		t.Fatalf("unexpected freight items: %+v, %v", items, err)
	}
	if len(env.audit.events) == 0 || env.audit.events[len(env.audit.events)-1].Action != "freight.custom_image_risk" {
		t.Fatalf("custom image tag risk should be audited, got %+v", env.audit.events)
	}
}

func TestManualFreightRejectsDirectPipelineArtifactWithoutDigestOrCommit(t *testing.T) {
	tests := []struct {
		name     string
		run      BuildRunRef
		artifact BuildArtifactRef
	}{
		{
			name:     "missing digest",
			run:      BuildRunRef{ID: "build_1", TenantID: "tenant_a", ProjectID: "project_payment", ApplicationID: "app_user", CommitSHA: "abcdef1234567890"},
			artifact: BuildArtifactRef{ID: "artifact_1", BuildRunID: "build_1", ApplicationID: "app_user", WorkloadID: "workload_api", URI: "registry.example/paas/user-api:abcdef"},
		},
		{
			name:     "missing commit",
			run:      BuildRunRef{ID: "build_1", TenantID: "tenant_a", ProjectID: "project_payment", ApplicationID: "app_user"},
			artifact: BuildArtifactRef{ID: "artifact_1", BuildRunID: "build_1", ApplicationID: "app_user", WorkloadID: "workload_api", URI: "registry.example/paas/user-api:abcdef", Digest: "sha256:abc"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newDeliveryEnv(t)
			env.svc.builds = fakeBuildQuery{
				runs:      map[shared.ID]BuildRunRef{tt.run.ID: tt.run},
				artifacts: map[shared.ID]BuildArtifactRef{tt.artifact.ID: tt.artifact},
			}
			_, err := env.svc.CreateFreight(context.Background(), CreateFreightInput{
				Actor:         actor("usr_dev"),
				ApplicationID: "app_user",
				Items: []CreateFreightItemInput{{
					WorkloadID:      "workload_api",
					SourceType:      FreightItemPipelineArtifact,
					BuildArtifactID: tt.artifact.ID,
				}},
			})
			if shared.CodeOf(err) != shared.CodeFailedPrecondition {
				t.Fatalf("CreateFreight() error = %v, want failed_precondition", err)
			}
		})
	}
}

func TestEligibleFreightsAndPromotionValidateStageOrder(t *testing.T) {
	env := newDeliveryEnv(t)
	freight := seedFreight(t, env)
	flow, err := env.repo.FindDeliveryFlowByApplication(context.Background(), "app_user")
	if err != nil {
		t.Fatalf("FindDeliveryFlowByApplication() error = %v", err)
	}
	stages, err := env.repo.ListDeliveryStages(context.Background(), flow.ID)
	if err != nil {
		t.Fatalf("ListDeliveryStages() error = %v", err)
	}
	if _, err := env.svc.CreatePromotion(context.Background(), CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetEnvironmentID: "env_test"}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("test stage should require dev first, got %v", err)
	}
	eligible, err := env.svc.ListEligibleFreights(context.Background(), "app_user", stages[0].ID)
	if err != nil || len(eligible) != 1 || eligible[0].ID != freight.ID {
		t.Fatalf("dev should have eligible freight, got %+v %v", eligible, err)
	}
	eligible, err = env.svc.ListEligibleFreights(context.Background(), "app_user", stages[1].ID)
	if err != nil || len(eligible) != 0 {
		t.Fatalf("test should not be eligible before dev, got %+v %v", eligible, err)
	}
	promoteFreightThrough(t, env, freight, "env_dev")
	eligible, err = env.svc.ListEligibleFreights(context.Background(), "app_user", stages[1].ID)
	if err != nil || len(eligible) != 1 || eligible[0].ID != freight.ID {
		t.Fatalf("test should be eligible after dev, got %+v %v", eligible, err)
	}
}

func TestPromotionDevAppliesGitOpsAndProdRequiresApproval(t *testing.T) {
	env := newDeliveryEnv(t)
	freight := seedFreight(t, env)
	dev, err := env.svc.CreatePromotion(context.Background(), CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetEnvironmentID: "env_dev"})
	if err != nil {
		t.Fatalf("CreatePromotion(dev) error = %v", err)
	}
	if dev.Status != PromotionManifestUpdated || dev.ManifestRevision != "rev-1" || len(env.gitops.specs) != 1 {
		t.Fatalf("dev promotion should apply gitops, got %+v specs=%+v", dev, env.gitops.specs)
	}
	if got := env.gitops.specs[0].Artifacts; len(got) != 1 || got[0].WorkloadID != "workload_api" || got[0].Repository != "registry.example/paas/user-api" || got[0].Tag != "abcdef" || got[0].Digest != "sha256:abc" {
		t.Fatalf("gitops artifact should include workload image fields, got %+v", got)
	}
	promoteFreightThrough(t, env, freight, "env_test", "env_staging")
	prod, err := env.svc.CreatePromotion(context.Background(), CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetEnvironmentID: "env_prod"})
	if err != nil {
		t.Fatalf("CreatePromotion(prod) error = %v", err)
	}
	if prod.Status != PromotionPendingApproval || len(env.gitops.specs) != 3 {
		t.Fatalf("prod should wait approval, got %+v specs=%+v", prod, env.gitops.specs)
	}
	if _, err := env.svc.ApprovePromotion(context.Background(), ApprovalInput{Actor: actor("usr_dev"), PromotionID: prod.ID}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("self approval should fail, got %v", err)
	}
	approved, err := env.svc.ApprovePromotion(context.Background(), ApprovalInput{Actor: actor("usr_ops"), PromotionID: prod.ID, Comment: "ok"})
	if err != nil {
		t.Fatalf("ApprovePromotion() error = %v", err)
	}
	if approved.Status != PromotionManifestUpdated || approved.ApprovedBy != "usr_ops" || len(env.gitops.specs) != 4 {
		t.Fatalf("approved prod should apply gitops, got %+v specs=%+v", approved, env.gitops.specs)
	}
}

func TestRejectAbortPendingEnvironmentRollbackAndGitOpsFailure(t *testing.T) {
	env := newDeliveryEnv(t)
	freight := seedFreight(t, env)
	if _, err := env.svc.CreatePromotion(context.Background(), CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetEnvironmentID: "env_pending"}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("pending cluster binding should block promotion, got %v", err)
	}
	promoteFreightThrough(t, env, freight, "env_dev", "env_test", "env_staging")
	prod, _ := env.svc.CreatePromotion(context.Background(), CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetEnvironmentID: "env_prod"})
	rejected, err := env.svc.RejectPromotion(context.Background(), ApprovalInput{Actor: actor("usr_ops"), PromotionID: prod.ID, Comment: "no"})
	if err != nil || rejected.Status != PromotionRejected || rejected.CompletedAt == nil {
		t.Fatalf("reject failed: %+v %v", rejected, err)
	}
	dev, _ := env.svc.CreatePromotion(context.Background(), CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetEnvironmentID: "env_test"})
	aborted, err := env.svc.AbortPromotion(context.Background(), actor("usr_dev"), dev.ID)
	if err != nil || aborted.Status != PromotionAborted {
		t.Fatalf("abort should mark non-terminal promotion aborted, got %+v %v", aborted, err)
	}
	rollback, err := env.svc.CreateRollbackPromotion(context.Background(), CreateRollbackPromotionInput{Actor: actor("usr_dev"), TargetFreightID: freight.ID, CurrentFreightID: freight.ID, TargetEnvironmentID: "env_dev"})
	if err != nil || !rollback.IsRollback || rollback.Status != PromotionManifestUpdated {
		t.Fatalf("rollback failed: %+v %v", rollback, err)
	}
	env.gitops.err = errors.New("gitops failed")
	if _, err := env.svc.CreatePromotion(context.Background(), CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetEnvironmentID: "env_dev"}); err == nil {
		t.Fatalf("gitops failure should fail")
	}
}

func TestHandlerFlowAndRepositoryBranches(t *testing.T) {
	env := newDeliveryEnv(t)
	mux := http.NewServeMux()
	NewHandler(env.svc).Register(mux)
	body, _ := json.Marshal(BuildSucceededPayload{BuildRunID: "build_1", ApplicationID: "app_user", WorkloadID: "workload_api", BuildArtifactID: "artifact_1"})
	rec := serveJSON(mux, http.MethodPost, "/api/delivery/build-succeeded", body)
	assertStatus(t, rec, http.StatusCreated)
	var created struct {
		Release Release `json:"release"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode release: %v", err)
	}
	freightBody, _ := json.Marshal(CreateFreightInput{Actor: actor("usr_dev"), Name: "freight-main", Items: []CreateFreightItemInput{{WorkloadID: "workload_api", SourceType: FreightItemPipelineArtifact, ReleaseID: created.Release.ID}}})
	freightRec := serveJSON(mux, http.MethodPost, "/api/apps/app_user/freights", freightBody)
	assertStatus(t, freightRec, http.StatusCreated)
	var freight Freight
	_ = json.NewDecoder(freightRec.Body).Decode(&freight)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/apps/app_user/freights/creation-context", nil), http.StatusOK)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/apps/app_user/freights?page=1&page_size=5", nil), http.StatusOK)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/freights/"+freight.ID.String(), nil), http.StatusOK)
	promoBody, _ := json.Marshal(CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetEnvironmentID: "env_dev"})
	promoRec := serveJSON(mux, http.MethodPost, "/api/promotions", promoBody)
	assertStatus(t, promoRec, http.StatusCreated)
	var promotion Promotion
	_ = json.NewDecoder(promoRec.Body).Decode(&promotion)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/promotions/"+promotion.ID.String(), nil), http.StatusOK)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/apps/app_user/promotions", nil), http.StatusOK)
	abortBody, _ := json.Marshal(struct {
		Actor identityaccess.Subject `json:"actor"`
	}{Actor: actor("usr_dev")})
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/promotions/"+promotion.ID.String()+"/abort", abortBody), http.StatusOK)
	rollbackBody, _ := json.Marshal(CreateRollbackPromotionInput{Actor: actor("usr_dev"), TargetFreightID: freight.ID, TargetEnvironmentID: "env_dev"})
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/promotions/rollback", rollbackBody), http.StatusCreated)
	prodBody, _ := json.Marshal(CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetEnvironmentID: "env_prod"})
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/promotions", mustJSON(t, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetEnvironmentID: "env_test"})), http.StatusCreated)
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/promotions", mustJSON(t, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetEnvironmentID: "env_staging"})), http.StatusCreated)
	prodRec := serveJSON(mux, http.MethodPost, "/api/promotions", prodBody)
	assertStatus(t, prodRec, http.StatusCreated)
	var prod Promotion
	_ = json.NewDecoder(prodRec.Body).Decode(&prod)
	approveBody, _ := json.Marshal(ApprovalInput{Actor: actor("usr_ops"), Comment: "ok"})
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/promotions/"+prod.ID.String()+"/approve", approveBody), http.StatusOK)
	prodRec = serveJSON(mux, http.MethodPost, "/api/promotions", prodBody)
	assertStatus(t, prodRec, http.StatusCreated)
	_ = json.NewDecoder(prodRec.Body).Decode(&prod)
	rejectBody, _ := json.Marshal(ApprovalInput{Actor: actor("usr_ops"), Comment: "no"})
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/promotions/"+prod.ID.String()+"/reject", rejectBody), http.StatusOK)
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/promotions", []byte("{")), http.StatusBadRequest)
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/delivery/build-succeeded", []byte("{")), http.StatusBadRequest)
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/delivery/build-succeeded", []byte(`{}`)), http.StatusBadRequest)
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/promotions/rollback", []byte("{")), http.StatusBadRequest)
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/promotions/rollback", []byte(`{}`)), http.StatusNotFound)
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/promotions/"+promotion.ID.String()+"/abort", []byte("{")), http.StatusBadRequest)
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/promotions/missing/abort", abortBody), http.StatusNotFound)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/freights/missing", nil), http.StatusNotFound)
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/promotions/"+prod.ID.String()+"/approve", []byte("{")), http.StatusBadRequest)
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/promotions/missing/approve", approveBody), http.StatusNotFound)
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/promotions/"+prod.ID.String()+"/reject", []byte("{")), http.StatusBadRequest)
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/promotions/missing/reject", rejectBody), http.StatusNotFound)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/promotions/missing", nil), http.StatusNotFound)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/apps/app_user/freights?page=x", nil), http.StatusOK)
}

func TestFailureBranchesAndRepositoryContracts(t *testing.T) {
	ctx := context.Background()
	env := newDeliveryEnv(t)
	if err := (NoopAuditLogger{}).Log(ctx, AuditEvent{}); err != nil {
		t.Fatalf("noop audit: %v", err)
	}
	if err := (NoopEventPublisher{}).Publish(ctx, shared.DomainEvent{}); err != nil {
		t.Fatalf("noop publisher: %v", err)
	}
	if _, err := env.svc.HandleBuildSucceeded(ctx, BuildSucceededPayload{}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("empty build payload should fail, got %v", err)
	}
	env.svc.builds = nil
	if _, err := env.svc.HandleBuildSucceeded(ctx, BuildSucceededPayload{BuildRunID: "build_1", ApplicationID: "app_user", WorkloadID: "workload_api", BuildArtifactID: "artifact_1"}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("missing build query should fail, got %v", err)
	}
	env = newDeliveryEnv(t)
	env.svc.apps = nil
	if _, err := env.svc.HandleBuildSucceeded(ctx, BuildSucceededPayload{BuildRunID: "build_1", ApplicationID: "app_user", WorkloadID: "workload_api", BuildArtifactID: "artifact_1"}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("missing app query should fail while ensuring default flow, got %v", err)
	}
	env = newDeliveryEnv(t)
	env.svc.envs = nil
	if _, err := env.svc.HandleBuildSucceeded(ctx, BuildSucceededPayload{BuildRunID: "build_1", ApplicationID: "app_user", WorkloadID: "workload_api", BuildArtifactID: "artifact_1"}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("missing env query should fail while ensuring default flow, got %v", err)
	}
	env = newDeliveryEnv(t)
	if _, err := env.svc.HandleBuildSucceeded(ctx, BuildSucceededPayload{BuildRunID: "build_1", ApplicationID: "other", WorkloadID: "workload_api", BuildArtifactID: "artifact_1"}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("ownership mismatch should fail, got %v", err)
	}
	env = newDeliveryEnv(t)
	freight := seedFreight(t, env)
	release, err := env.repo.FindReleaseByBuildRun(ctx, "build_1")
	if err != nil {
		t.Fatalf("FindReleaseByBuildRun() error = %v", err)
	}
	if _, err := env.svc.GetRelease(ctx, release.ID); err != nil {
		t.Fatalf("GetRelease() error = %v", err)
	}
	if _, err := env.repo.GetRelease(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing release: %v", err)
	}
	if err := env.repo.CreateRelease(ctx, release); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate release: %v", err)
	}
	if _, err := env.repo.FindReleaseByBuildRun(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing release by build: %v", err)
	}
	if err := env.repo.CreateFreight(ctx, freight); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate freight: %v", err)
	}
	if err := env.repo.CreateFreightItem(ctx, FreightItem{ID: "bad", FreightID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("bad freight item: %v", err)
	}
	items, _ := env.repo.ListFreightItems(ctx, freight.ID)
	if err := env.repo.CreateFreightItem(ctx, items[0]); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate freight item: %v", err)
	}
	if _, err := env.repo.ListFreightItems(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing freight items: %v", err)
	}
	flow, _ := env.repo.FindDeliveryFlowByApplication(ctx, "app_user")
	if _, err := env.repo.GetDeliveryFlow(ctx, flow.ID); err != nil {
		t.Fatalf("GetDeliveryFlow() error = %v", err)
	}
	if _, err := env.repo.GetDeliveryFlow(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing flow: %v", err)
	}
	if err := env.repo.CreateDeliveryFlow(ctx, flow); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate flow: %v", err)
	}
	if _, err := env.repo.FindDeliveryFlowByApplication(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing flow by app: %v", err)
	}
	stages, _ := env.repo.ListDeliveryStages(ctx, flow.ID)
	if _, err := env.repo.GetDeliveryStage(ctx, stages[0].ID); err != nil {
		t.Fatalf("GetDeliveryStage() error = %v", err)
	}
	if _, err := env.repo.GetDeliveryStage(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing stage: %v", err)
	}
	if err := env.repo.CreateDeliveryStage(ctx, stages[0]); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate stage: %v", err)
	}
	if err := env.repo.CreateDeliveryStage(ctx, DeliveryStage{ID: "bad", DeliveryFlowID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("bad stage flow: %v", err)
	}
	if _, err := env.repo.FindDeliveryStageByEnvironment(ctx, "other", stages[0].EnvironmentID); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("wrong app stage: %v", err)
	}
	if _, err := env.repo.ListDeliveryStages(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing stages: %v", err)
	}
}

func TestPromotionFailureBranches(t *testing.T) {
	ctx := context.Background()
	env := newDeliveryEnv(t)
	freight := seedFreight(t, env)
	env.svc.permission = &recordingPermission{err: shared.NewError(shared.CodePermissionDenied, "denied")}
	if _, err := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetEnvironmentID: "env_dev"}); shared.CodeOf(err) != shared.CodePermissionDenied {
		t.Fatalf("promotion permission should fail, got %v", err)
	}
	env = newDeliveryEnv(t)
	freight = seedFreight(t, env)
	if _, err := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetEnvironmentID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing env should fail, got %v", err)
	}
	env.gitops.err = errors.New("gitops failed")
	if _, err := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetEnvironmentID: "env_dev"}); err == nil {
		t.Fatalf("gitops failure should fail")
	}
	env.svc.gitops = nil
	if _, err := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetEnvironmentID: "env_dev"}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("missing gitops should fail, got %v", err)
	}
	env = newDeliveryEnv(t)
	freight = seedFreight(t, env)
	promoteFreightThrough(t, env, freight, "env_dev", "env_test", "env_staging")
	prod, _ := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetEnvironmentID: "env_prod"})
	if _, err := env.svc.RejectPromotion(ctx, ApprovalInput{Actor: actor("usr_dev"), PromotionID: prod.ID}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("self reject should fail, got %v", err)
	}
	if _, err := env.svc.ApprovePromotion(ctx, ApprovalInput{Actor: actor("usr_ops"), PromotionID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing approve should fail, got %v", err)
	}
	if _, err := env.svc.RejectPromotion(ctx, ApprovalInput{Actor: actor("usr_ops"), PromotionID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing reject should fail, got %v", err)
	}
	dev, _ := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetEnvironmentID: "env_dev"})
	if _, err := env.svc.ApprovePromotion(ctx, ApprovalInput{Actor: actor("usr_ops"), PromotionID: dev.ID}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("approve non-pending should fail, got %v", err)
	}
	if _, err := env.svc.RejectPromotion(ctx, ApprovalInput{Actor: actor("usr_ops"), PromotionID: dev.ID}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("reject non-pending should fail, got %v", err)
	}
	if _, err := env.svc.AbortPromotion(ctx, identityaccess.Subject{}, prod.ID); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("abort missing actor should fail, got %v", err)
	}
	if _, err := env.svc.CreateRollbackPromotion(ctx, CreateRollbackPromotionInput{Actor: actor("usr_dev"), TargetFreightID: "missing", TargetEnvironmentID: "env_dev"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("rollback missing freight should fail, got %v", err)
	}
}

func TestMoreRepositoryAndDefaultBranches(t *testing.T) {
	ctx := context.Background()
	defaultSvc := NewService(Options{Repository: newTestRepository(t)})
	if defaultSvc.audit == nil || defaultSvc.events == nil || defaultSvc.ids == nil || defaultSvc.clock == nil {
		t.Fatalf("default service dependencies should be set")
	}
	env := newDeliveryEnv(t)
	freight := seedFreight(t, env)
	promoteFreightThrough(t, env, freight, "env_dev", "env_test", "env_staging")
	promo, _ := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetEnvironmentID: "env_prod"})
	if err := env.repo.CreatePromotion(ctx, promo); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate promotion should conflict, got %v", err)
	}
	changed := promo
	changed.FreightID = "other"
	if err := env.repo.UpdatePromotion(ctx, changed); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("changing promotion ownership should fail, got %v", err)
	}
	if err := env.repo.UpdatePromotion(ctx, Promotion{ID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing promotion update should fail, got %v", err)
	}
	if err := env.repo.CreatePromotionApproval(ctx, PromotionApproval{ID: "bad", PromotionID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("approval for missing promotion should fail, got %v", err)
	}
	approval, _ := env.repo.GetPromotionApproval(ctx, promo.ID)
	if err := env.repo.CreatePromotionApproval(ctx, approval); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate approval should conflict, got %v", err)
	}
	if err := env.repo.UpdatePromotionApproval(ctx, PromotionApproval{PromotionID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing approval update should fail, got %v", err)
	}
	if _, err := env.repo.GetPromotionApproval(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing approval get should fail, got %v", err)
	}
	if _, err := env.repo.ListFreightsByApplication(ctx, "app_user", shared.PageRequest{Page: 99, PageSize: 10}); err != nil {
		t.Fatalf("out of range freights should succeed: %v", err)
	}
	if _, err := env.repo.ListPromotionsByApplication(ctx, "app_user", shared.PageRequest{Page: 99, PageSize: 10}); err != nil {
		t.Fatalf("out of range promotions should succeed: %v", err)
	}
	if _, err := (failingBuildQuery{}).GetBuildArtifact(ctx, "x"); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("failing build artifact query should fail, got %v", err)
	}
	if _, err := (failingEnvQuery{}).GetEnvironment(ctx, "x"); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("failing env get should fail, got %v", err)
	}
	if releaseVersion("", "run_1") != "build-run_1" {
		t.Fatalf("releaseVersion fallback mismatch")
	}
	if firstNonEmpty("", " a ") != "a" || firstNonEmpty("", "") != "" {
		t.Fatalf("firstNonEmpty mismatch")
	}
}

func TestRemainingServiceBranches(t *testing.T) {
	ctx := context.Background()
	env := newDeliveryEnv(t)
	env.svc.builds = fakeBuildQuery{
		runs:      map[shared.ID]BuildRunRef{"build_1": {ID: "build_1", TenantID: "tenant_a", ProjectID: "project_payment", ApplicationID: "app_user"}},
		artifacts: map[shared.ID]BuildArtifactRef{},
	}
	if _, err := env.svc.HandleBuildSucceeded(ctx, BuildSucceededPayload{BuildRunID: "build_1", ApplicationID: "app_user", WorkloadID: "workload_api", BuildArtifactID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing build artifact should fail, got %v", err)
	}
	env = newDeliveryEnv(t)
	freight := seedFreight(t, env)
	if _, err := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: identityaccess.Subject{}, FreightID: freight.ID, TargetEnvironmentID: "env_dev"}); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("promotion missing actor should fail, got %v", err)
	}
	if _, err := env.svc.CreateRollbackPromotion(ctx, CreateRollbackPromotionInput{Actor: actor("usr_dev"), TargetFreightID: freight.ID, CurrentFreightID: "missing", TargetEnvironmentID: "env_dev"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("rollback missing current freight should fail, got %v", err)
	}
	promoteFreightThrough(t, env, freight, "env_dev", "env_test", "env_staging")
	prod, _ := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetEnvironmentID: "env_prod"})
	env.svc.permission = &recordingPermission{err: shared.NewError(shared.CodePermissionDenied, "denied")}
	if _, err := env.svc.ApprovePromotion(ctx, ApprovalInput{Actor: actor("usr_ops"), PromotionID: prod.ID}); shared.CodeOf(err) != shared.CodePermissionDenied {
		t.Fatalf("approve permission denied should fail, got %v", err)
	}
	if _, err := env.svc.RejectPromotion(ctx, ApprovalInput{Actor: actor("usr_ops"), PromotionID: prod.ID}); shared.CodeOf(err) != shared.CodePermissionDenied {
		t.Fatalf("reject permission denied should fail, got %v", err)
	}
	env.svc.permission = nil
	rejected, _ := env.svc.RejectPromotion(ctx, ApprovalInput{Actor: actor("usr_ops"), PromotionID: prod.ID})
	again, err := env.svc.AbortPromotion(ctx, actor("usr_ops"), rejected.ID)
	if err != nil || again.Status != PromotionRejected {
		t.Fatalf("abort terminal should be no-op, got %+v %v", again, err)
	}
}

func TestMigrationsAddWorkloadColumnsWithoutBlockingLegacyMultiItemFreight(t *testing.T) {
	ctx := context.Background()
	db := testsupport.MySQLDB(t)
	migrator := database.NewMigrator(db)

	oldCore := Migrations[0]
	replacements := []string{
		"  workload_id VARCHAR(64) NOT NULL DEFAULT '',\n",
		"  image_repository VARCHAR(1024) NOT NULL DEFAULT '',\n",
		"  image_tag VARCHAR(255) NOT NULL DEFAULT '',\n",
		"  source_type VARCHAR(64) NOT NULL DEFAULT 'pipeline_artifact',\n",
		"  workload_id VARCHAR(64) NOT NULL DEFAULT '',\n",
		"  source_type VARCHAR(64) NOT NULL DEFAULT 'pipeline_artifact',\n",
		"  image_ref VARCHAR(1024) NOT NULL DEFAULT '',\n",
		"  image_repository VARCHAR(1024) NOT NULL DEFAULT '',\n",
		"  image_tag VARCHAR(255) NOT NULL DEFAULT '',\n",
		"  UNIQUE KEY uk_freight_items_workload (freight_id, workload_id),\n",
	}
	for _, replacement := range replacements {
		oldCore.Up = strings.Replace(oldCore.Up, replacement, "", 1)
	}
	oldCore.Up = strings.Replace(oldCore.Up, "  KEY idx_releases_application (application_id),\n  KEY idx_releases_workload_created (application_id, workload_id, created_at)\n", "  KEY idx_releases_application (application_id)\n", 1)
	if oldCore.Up == Migrations[0].Up {
		t.Fatalf("test setup did not remove workload v2 columns from old core migration")
	}
	if err := migrator.Up(ctx, []database.Migration{oldCore}); err != nil {
		t.Fatalf("apply old core migration: %v", err)
	}

	now := time.Date(2026, 5, 30, 7, 0, 0, 0, time.UTC)
	if _, err := db.ExecContext(ctx, `INSERT INTO freights (id, tenant_id, project_id, application_id, pipeline_id, pipeline_name, pipeline_display_name, name, status, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, "freight_legacy", "tenant_a", "project_payment", "app_user", "pipeline_main", "main", "主流水线", "legacy", "ready", now); err != nil {
		t.Fatalf("insert legacy freight: %v", err)
	}
	for _, itemID := range []shared.ID{"item_1", "item_2"} {
		if _, err := db.ExecContext(ctx, `INSERT INTO freight_items (id, tenant_id, project_id, freight_id, application_id, release_id, build_artifact_id, source_key, type, name, uri, digest, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, itemID, "tenant_a", "project_payment", "freight_legacy", "app_user", "release_"+string(itemID), "artifact_"+string(itemID), "main", string(FreightItemApplicationRelease), "main", "registry.example/paas/app:legacy", "sha256:abc", now); err != nil {
			t.Fatalf("insert legacy freight item %s: %v", itemID, err)
		}
	}

	if err := migrator.Up(ctx, Migrations[1:]); err != nil {
		t.Fatalf("apply workload v2 follow-up migrations: %v", err)
	}

	repo, err := NewMySQLRepository(ctx, db)
	if err != nil {
		t.Fatalf("NewMySQLRepository() error = %v", err)
	}
	freight, err := repo.GetFreight(ctx, "freight_legacy")
	if err != nil || freight.ID != "freight_legacy" {
		t.Fatalf("GetFreight() = %+v, %v", freight, err)
	}
	items, err := repo.ListFreightItems(ctx, "freight_legacy")
	if err != nil || len(items) != 2 {
		t.Fatalf("ListFreightItems() = %+v, %v", items, err)
	}
}

func serveJSON(mux *http.ServeMux, method, target string, body []byte) *httptest.ResponseRecorder {
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
		t.Fatalf("status=%d want=%d body=%s", rec.Code, status, rec.Body.String())
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return data
}
