package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/shared"
	"github.com/shareinto/paas/internal/shared/testutil"
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
	repo     *MemoryRepository
	gitops   *recordingGitOps
	audit    *recordingAudit
	events   *recordingPublisher
	envQuery fakeEnvQuery
}

func newDeliveryEnv() deliveryEnv {
	repo := NewMemoryRepository()
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
			artifacts: map[shared.ID]BuildArtifactRef{"artifact_1": {ID: "artifact_1", BuildRunID: "build_1", ApplicationID: "app_user", URI: "registry.example/paas/user-api:abcdef", Digest: "sha256:abc", IsPrimary: true}},
		},
		ApplicationQuery:  fakeAppQuery{apps: map[shared.ID]ApplicationRef{"app_user": {ID: "app_user", TenantID: "tenant_a", ProjectID: "project_payment", Name: "user-api"}}},
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
	_, freight, err := env.svc.HandleBuildSucceeded(context.Background(), BuildSucceededPayload{BuildRunID: "build_1", ApplicationID: "app_user", BuildArtifactID: "artifact_1"})
	if err != nil {
		t.Fatalf("HandleBuildSucceeded() error = %v", err)
	}
	return freight
}

func TestBuildSucceededCreatesReleaseFreightAndDefaultFlow(t *testing.T) {
	env := newDeliveryEnv()
	release, freight, err := env.svc.HandleBuildSucceeded(context.Background(), BuildSucceededPayload{BuildRunID: "build_1", ApplicationID: "app_user", BuildArtifactID: "artifact_1"})
	if err != nil {
		t.Fatalf("HandleBuildSucceeded() error = %v", err)
	}
	if release.Status != ReleaseReady || release.Version != "abcdef123456" || freight.Status != FreightAvailable {
		t.Fatalf("unexpected release/freight: %+v %+v", release, freight)
	}
	if release.PipelineID != "pipeline_main" || release.PipelineName != "main" || freight.PipelineID != "pipeline_main" || freight.PipelineDisplayName != "主流水线" {
		t.Fatalf("release and freight should keep pipeline identity, got %+v %+v", release, freight)
	}
	items, err := env.repo.ListFreightItems(context.Background(), freight.ID)
	if err != nil || len(items) != 1 || items[0].URI == "" {
		t.Fatalf("unexpected freight items: %+v, %v", items, err)
	}
	flow, err := env.repo.FindDeliveryFlowByApplication(context.Background(), "app_user")
	if err != nil {
		t.Fatalf("default flow missing: %v", err)
	}
	stages, err := env.repo.ListDeliveryStages(context.Background(), flow.ID)
	if err != nil || len(stages) != 4 || stages[3].Name != "prod" || !stages[3].RequiresApproval {
		t.Fatalf("unexpected stages: %+v, %v", stages, err)
	}
	if len(env.events.events) == 0 || env.events.events[0].EventType != "FreightCreated" {
		t.Fatalf("expected FreightCreated, got %+v", env.events.events)
	}
	againRelease, againFreight, err := env.svc.HandleBuildSucceeded(context.Background(), BuildSucceededPayload{BuildRunID: "build_1", ApplicationID: "app_user", BuildArtifactID: "artifact_1"})
	if err != nil || againRelease.ID != release.ID || againFreight.ID != freight.ID {
		t.Fatalf("idempotent build succeeded failed: %+v %+v %v", againRelease, againFreight, err)
	}
}

func TestPromotionDevAppliesGitOpsAndProdRequiresApproval(t *testing.T) {
	env := newDeliveryEnv()
	freight := seedFreight(t, env)
	dev, err := env.svc.CreatePromotion(context.Background(), CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetEnvironmentID: "env_dev"})
	if err != nil {
		t.Fatalf("CreatePromotion(dev) error = %v", err)
	}
	if dev.Status != PromotionManifestUpdated || dev.ManifestRevision != "rev-1" || len(env.gitops.specs) != 1 {
		t.Fatalf("dev promotion should apply gitops, got %+v specs=%+v", dev, env.gitops.specs)
	}
	prod, err := env.svc.CreatePromotion(context.Background(), CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetEnvironmentID: "env_prod"})
	if err != nil {
		t.Fatalf("CreatePromotion(prod) error = %v", err)
	}
	if prod.Status != PromotionPendingApproval || len(env.gitops.specs) != 1 {
		t.Fatalf("prod should wait approval, got %+v specs=%+v", prod, env.gitops.specs)
	}
	if _, err := env.svc.ApprovePromotion(context.Background(), ApprovalInput{Actor: actor("usr_dev"), PromotionID: prod.ID}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("self approval should fail, got %v", err)
	}
	approved, err := env.svc.ApprovePromotion(context.Background(), ApprovalInput{Actor: actor("usr_ops"), PromotionID: prod.ID, Comment: "ok"})
	if err != nil {
		t.Fatalf("ApprovePromotion() error = %v", err)
	}
	if approved.Status != PromotionManifestUpdated || approved.ApprovedBy != "usr_ops" || len(env.gitops.specs) != 2 {
		t.Fatalf("approved prod should apply gitops, got %+v specs=%+v", approved, env.gitops.specs)
	}
}

func TestRejectAbortPendingEnvironmentRollbackAndGitOpsFailure(t *testing.T) {
	env := newDeliveryEnv()
	freight := seedFreight(t, env)
	if _, err := env.svc.CreatePromotion(context.Background(), CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetEnvironmentID: "env_pending"}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("pending cluster binding should block promotion, got %v", err)
	}
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
	env := newDeliveryEnv()
	mux := http.NewServeMux()
	NewHandler(env.svc).Register(mux)
	body, _ := json.Marshal(BuildSucceededPayload{BuildRunID: "build_1", ApplicationID: "app_user", BuildArtifactID: "artifact_1"})
	rec := serveJSON(mux, http.MethodPost, "/api/delivery/build-succeeded", body)
	assertStatus(t, rec, http.StatusCreated)
	var created struct {
		Freight Freight `json:"freight"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode freight: %v", err)
	}
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/apps/app_user/freights?page=1&page_size=5", nil), http.StatusOK)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/freights/"+created.Freight.ID.String(), nil), http.StatusOK)
	promoBody, _ := json.Marshal(CreatePromotionInput{Actor: actor("usr_dev"), FreightID: created.Freight.ID, TargetEnvironmentID: "env_dev"})
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
	rollbackBody, _ := json.Marshal(CreateRollbackPromotionInput{Actor: actor("usr_dev"), TargetFreightID: created.Freight.ID, TargetEnvironmentID: "env_dev"})
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/promotions/rollback", rollbackBody), http.StatusCreated)
	prodBody, _ := json.Marshal(CreatePromotionInput{Actor: actor("usr_dev"), FreightID: created.Freight.ID, TargetEnvironmentID: "env_prod"})
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
	env := newDeliveryEnv()
	if err := (NoopAuditLogger{}).Log(ctx, AuditEvent{}); err != nil {
		t.Fatalf("noop audit: %v", err)
	}
	if err := (NoopEventPublisher{}).Publish(ctx, shared.DomainEvent{}); err != nil {
		t.Fatalf("noop publisher: %v", err)
	}
	if _, _, err := env.svc.HandleBuildSucceeded(ctx, BuildSucceededPayload{}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("empty build payload should fail, got %v", err)
	}
	env.svc.builds = nil
	if _, _, err := env.svc.HandleBuildSucceeded(ctx, BuildSucceededPayload{BuildRunID: "build_1", ApplicationID: "app_user", BuildArtifactID: "artifact_1"}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("missing build query should fail, got %v", err)
	}
	env = newDeliveryEnv()
	env.svc.apps = nil
	if _, _, err := env.svc.HandleBuildSucceeded(ctx, BuildSucceededPayload{BuildRunID: "build_1", ApplicationID: "app_user", BuildArtifactID: "artifact_1"}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("missing app query should fail, got %v", err)
	}
	env = newDeliveryEnv()
	env.svc.envs = nil
	if _, _, err := env.svc.HandleBuildSucceeded(ctx, BuildSucceededPayload{BuildRunID: "build_1", ApplicationID: "app_user", BuildArtifactID: "artifact_1"}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("missing env query should fail, got %v", err)
	}
	env = newDeliveryEnv()
	if _, _, err := env.svc.HandleBuildSucceeded(ctx, BuildSucceededPayload{BuildRunID: "build_1", ApplicationID: "other", BuildArtifactID: "artifact_1"}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("ownership mismatch should fail, got %v", err)
	}
	env = newDeliveryEnv()
	freight := seedFreight(t, env)
	var release Release
	for _, value := range env.repo.releases {
		release = value
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
	env := newDeliveryEnv()
	freight := seedFreight(t, env)
	env.svc.permission = &recordingPermission{err: shared.NewError(shared.CodePermissionDenied, "denied")}
	if _, err := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetEnvironmentID: "env_dev"}); shared.CodeOf(err) != shared.CodePermissionDenied {
		t.Fatalf("promotion permission should fail, got %v", err)
	}
	env = newDeliveryEnv()
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
	env = newDeliveryEnv()
	freight = seedFreight(t, env)
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
	defaultSvc := NewService(Options{Repository: NewMemoryRepository()})
	if defaultSvc.audit == nil || defaultSvc.events == nil || defaultSvc.ids == nil || defaultSvc.clock == nil {
		t.Fatalf("default service dependencies should be set")
	}
	env := newDeliveryEnv()
	freight := seedFreight(t, env)
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
	env := newDeliveryEnv()
	env.svc.builds = fakeBuildQuery{
		runs:      map[shared.ID]BuildRunRef{"build_1": {ID: "build_1", TenantID: "tenant_a", ProjectID: "project_payment", ApplicationID: "app_user"}},
		artifacts: map[shared.ID]BuildArtifactRef{},
	}
	if _, _, err := env.svc.HandleBuildSucceeded(ctx, BuildSucceededPayload{BuildRunID: "build_1", ApplicationID: "app_user", BuildArtifactID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing build artifact should fail, got %v", err)
	}
	env = newDeliveryEnv()
	freight := seedFreight(t, env)
	if _, err := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: identityaccess.Subject{}, FreightID: freight.ID, TargetEnvironmentID: "env_dev"}); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("promotion missing actor should fail, got %v", err)
	}
	if _, err := env.svc.CreateRollbackPromotion(ctx, CreateRollbackPromotionInput{Actor: actor("usr_dev"), TargetFreightID: freight.ID, CurrentFreightID: "missing", TargetEnvironmentID: "env_dev"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("rollback missing current freight should fail, got %v", err)
	}
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
