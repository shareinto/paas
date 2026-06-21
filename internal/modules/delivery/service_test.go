package delivery

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
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

func (q fakeBuildQuery) ListBuildRuns(_ context.Context, applicationID shared.ID, _ shared.PageRequest) (shared.PageResult[BuildRunRef], error) {
	out := make([]BuildRunRef, 0)
	for _, run := range q.runs {
		if run.ApplicationID == applicationID {
			out = append(out, run)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return shared.PageResult[BuildRunRef]{Items: out, Total: int64(len(out)), Page: 1, PageSize: len(out)}, nil
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

type fakeRuntimeStateQuery struct {
	states map[string]StageRuntimeState
}

func (q fakeRuntimeStateQuery) ListStageRuntimeStates(_ context.Context, applicationID shared.ID) (map[string]StageRuntimeState, error) {
	out := map[string]StageRuntimeState{}
	for stageKey, state := range q.states {
		if state.ApplicationID == applicationID {
			out[stageKey] = state
		}
	}
	return out, nil
}

type fakeClusterQuery struct{ clusters map[shared.ID]ClusterRef }

func (q fakeClusterQuery) GetCluster(_ context.Context, id shared.ID) (ClusterRef, error) {
	v, ok := q.clusters[id]
	if !ok {
		return ClusterRef{}, shared.NewError(shared.CodeNotFound, "cluster not found")
	}
	return v, nil
}

type recordingStageSync struct {
	calls []SyncApplicationStagesInput
	err   error
}

func (s *recordingStageSync) SyncApplicationStages(_ context.Context, input SyncApplicationStagesInput) error {
	s.calls = append(s.calls, input)
	if s.err != nil {
		return s.err
	}
	return nil
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
	svc    *Service
	repo   Repository
	gitops *recordingGitOps
	audit  *recordingAudit
	events *recordingPublisher
	sync   *recordingStageSync
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
	sync := &recordingStageSync{}
	svc := NewService(Options{
		Repository: repo,
		BuildQuery: fakeBuildQuery{
			runs:      map[shared.ID]BuildRunRef{"build_1": {ID: "build_1", TenantID: "tenant_a", ProjectID: "project_payment", ApplicationID: "app_user", PipelineID: "pipeline_main", PipelineName: "main", PipelineDisplayName: "主流水线", CommitSHA: "abcdef1234567890"}},
			artifacts: map[shared.ID]BuildArtifactRef{"artifact_1": {ID: "artifact_1", BuildRunID: "build_1", ApplicationID: "app_user", URI: "registry.example/paas/user-api:abcdef", Digest: "sha256:abc", IsPrimary: true}},
		},
		ApplicationQuery:  fakeAppQuery{apps: map[shared.ID]ApplicationRef{"app_user": {ID: "app_user", TenantID: "tenant_a", ProjectID: "project_payment", ProjectName: "payment", Name: "user-api"}}},
		WorkloadQuery:     fakeWorkloadQuery{workloads: map[shared.ID][]WorkloadRef{"app_user": {{ID: "workload_api", TenantID: "tenant_a", ProjectID: "project_payment", ApplicationID: "app_user", Name: "api", DisplayName: "用户接口", Status: "enabled"}}}},
		GitOpsDeployment:  gitops,
		StageSync:         sync,
		PermissionChecker: &recordingPermission{},
		Audit:             audit,
		EventPublisher:    events,
		IDGenerator:       testutil.NewFakeIDGenerator(1),
		Clock:             testutil.NewFakeClock(time.Date(2026, 5, 30, 7, 0, 0, 0, time.UTC)),
	})
	for _, stageKey := range []string{"dev", "test", "staging", "prod", "qa"} {
		if err := repo.ReplaceStageClusterBindings(context.Background(), "tenant_a", stageKey, []StageClusterBinding{{
			ID:          shared.ID("binding_" + stageKey),
			TenantID:    "tenant_a",
			StageKey:    stageKey,
			ClusterID:   shared.ID("cluster_" + stageKey),
			ClusterName: stageKey + "-cluster",
			Status:      StageClusterBindingActive,
			CreatedAt:   time.Date(2026, 5, 30, 7, 0, 0, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 5, 30, 7, 0, 0, 0, time.UTC),
		}}); err != nil {
			t.Fatalf("seed stage cluster binding %s: %v", stageKey, err)
		}
	}
	return deliveryEnv{svc: svc, repo: repo, gitops: gitops, audit: audit, events: events, sync: sync}
}

func actor(id shared.ID) identityaccess.Subject {
	return identityaccess.Subject{Type: identityaccess.SubjectUser, ID: id}
}

func TestBuildSucceededFansOutToPipelineBoundWorkloads(t *testing.T) {
	env := newDeliveryEnv(t)
	ctx := context.Background()
	env.svc.workloads = fakeWorkloadQuery{workloads: map[shared.ID][]WorkloadRef{"app_user": {
		{ID: "workload_api", TenantID: "tenant_a", ProjectID: "project_payment", ApplicationID: "app_user", Name: "api", DisplayName: "用户接口", Status: "enabled"},
		{ID: "workload_worker", TenantID: "tenant_a", ProjectID: "project_payment", ApplicationID: "app_user", Name: "worker", DisplayName: "后台任务", Status: "enabled"},
	}}}
	payload := BuildSucceededPayload{BuildRunID: "build_1", ApplicationID: "app_user", WorkloadIDs: []shared.ID{"workload_api", "workload_worker"}, BuildArtifactID: "artifact_1"}

	release, err := env.svc.HandleBuildSucceeded(ctx, payload)
	if err != nil {
		t.Fatalf("HandleBuildSucceeded() error = %v", err)
	}
	if release.WorkloadID != "workload_api" {
		t.Fatalf("first release should be returned, got %+v", release)
	}
	apiRelease, err := env.repo.FindReleaseByBuildRunAndWorkload(ctx, "build_1", "workload_api")
	if err != nil {
		t.Fatalf("api release missing: %v", err)
	}
	workerRelease, err := env.repo.FindReleaseByBuildRunAndWorkload(ctx, "build_1", "workload_worker")
	if err != nil {
		t.Fatalf("worker release missing: %v", err)
	}
	if apiRelease.ID == workerRelease.ID || apiRelease.ImageBundleID == workerRelease.ImageBundleID {
		t.Fatalf("fan-out releases should have distinct release and bundle ids, api=%+v worker=%+v", apiRelease, workerRelease)
	}
	again, err := env.svc.HandleBuildSucceeded(ctx, payload)
	if err != nil {
		t.Fatalf("HandleBuildSucceeded(second) error = %v", err)
	}
	if again.ID != apiRelease.ID {
		t.Fatalf("fan-out should be idempotent for first workload, got %+v want %+v", again, apiRelease)
	}
}

func TestBuildSucceededCreatesReleaseWithoutDigestOrCommit(t *testing.T) {
	env := newDeliveryEnv(t)
	ctx := context.Background()
	env.svc.builds = fakeBuildQuery{
		runs: map[shared.ID]BuildRunRef{
			"build_1": {ID: "build_1", TenantID: "tenant_a", ProjectID: "project_payment", ApplicationID: "app_user", PipelineID: "pipeline_main", PipelineName: "main", PipelineDisplayName: "主流水线", Status: "succeeded"},
		},
		artifacts: map[shared.ID]BuildArtifactRef{
			"artifact_1": {ID: "artifact_1", BuildRunID: "build_1", ApplicationID: "app_user", URI: "registry.example/paas/user-api:abcdef", IsPrimary: true},
		},
	}

	release, err := env.svc.HandleBuildSucceeded(ctx, BuildSucceededPayload{BuildRunID: "build_1", ApplicationID: "app_user", WorkloadID: "workload_api", BuildArtifactID: "artifact_1"})
	if err != nil {
		t.Fatalf("HandleBuildSucceeded() should allow release without digest or commit: %v", err)
	}
	if release.Version != "build-build_1" || release.CommitSHA != "" || release.ImageDigest != "" || release.ImageURI != "registry.example/paas/user-api:abcdef" {
		t.Fatalf("release should use build id fallback and keep empty digest/commit, got %+v", release)
	}
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

func promoteFreightThrough(t *testing.T, env deliveryEnv, freight Freight, stageKeys ...string) {
	t.Helper()
	for _, stageKey := range stageKeys {
		promotion, err := env.svc.CreatePromotion(context.Background(), CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: stageKey})
		if err != nil {
			t.Fatalf("CreatePromotion(%s) error = %v", stageKey, err)
		}
		if promotion.Status != PromotionManifestUpdated {
			t.Fatalf("CreatePromotion(%s) status = %s, want %s", stageKey, promotion.Status, PromotionManifestUpdated)
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

func TestBuildSucceededCreatesImageBundleAndPromotionCarriesVariants(t *testing.T) {
	env := newDeliveryEnv(t)
	env.svc.builds = fakeBuildQuery{
		runs: map[shared.ID]BuildRunRef{
			"build_bundle": {ID: "build_bundle", TenantID: "tenant_a", ProjectID: "project_payment", ApplicationID: "app_user", PipelineID: "pipeline_main", PipelineName: "main", PipelineDisplayName: "主流水线", CommitSHA: "abcdef1234567890"},
		},
		artifacts: map[shared.ID]BuildArtifactRef{
			"artifact_aliyun": {ID: "artifact_aliyun", BuildRunID: "build_bundle", ApplicationID: "app_user", WorkloadID: "workload_api", URI: "registry.example/paas/user-api-aliyun:abcdef", Digest: "sha256:aliyun", IsPrimary: true, SelectorLabels: map[string]string{"cloud": "aliyun"}},
			"artifact_aws":    {ID: "artifact_aws", BuildRunID: "build_bundle", ApplicationID: "app_user", WorkloadID: "workload_api", URI: "registry.example/paas/user-api-aws:abcdef", Digest: "sha256:aws", SelectorLabels: map[string]string{"cloud": "aws"}},
		},
	}
	release, err := env.svc.HandleBuildSucceeded(context.Background(), BuildSucceededPayload{BuildRunID: "build_bundle", ApplicationID: "app_user", WorkloadID: "workload_api", BuildArtifactIDs: []shared.ID{"artifact_aliyun", "artifact_aws"}})
	if err != nil {
		t.Fatalf("HandleBuildSucceeded() error = %v", err)
	}
	if release.ImageBundleID.IsZero() {
		t.Fatalf("release should reference image bundle: %+v", release)
	}
	images, err := env.repo.ListImageBundleImages(context.Background(), release.ImageBundleID)
	if err != nil || len(images) != 2 {
		t.Fatalf("bundle should contain both image variants, got %+v, %v", images, err)
	}
	if images[0].SelectorLabels["cloud"] != "aliyun" || images[1].SelectorLabels["cloud"] != "aws" {
		t.Fatalf("bundle images should keep selector labels: %+v", images)
	}
	freight, err := env.svc.CreateFreight(context.Background(), CreateFreightInput{
		Actor:         actor("usr_dev"),
		ApplicationID: "app_user",
		Name:          "bundle-freight",
		Items:         []CreateFreightItemInput{{WorkloadID: "workload_api", SourceType: FreightItemPipelineArtifact, ReleaseID: release.ID}},
	})
	if err != nil {
		t.Fatalf("CreateFreight() error = %v", err)
	}
	promotion, err := env.svc.CreatePromotion(context.Background(), CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "dev"})
	if err != nil || promotion.Status != PromotionManifestUpdated {
		t.Fatalf("CreatePromotion() = %+v, %v", promotion, err)
	}
	if len(env.gitops.specs) != 1 || len(env.gitops.specs[0].Artifacts) != 1 {
		t.Fatalf("expected one GitOps artifact, got %+v", env.gitops.specs)
	}
	variants := env.gitops.specs[0].Artifacts[0].Variants
	if len(variants) != 2 || variants[0].SelectorLabels["cloud"] != "aliyun" || variants[1].SelectorLabels["cloud"] != "aws" {
		t.Fatalf("GitOps spec should carry bundle variants, got %+v", variants)
	}
}

func TestTenantDeliveryFlowTemplateStageLifecycleAndBindings(t *testing.T) {
	env := newDeliveryEnv(t)
	ctx := context.Background()

	template, err := env.svc.GetDeliveryFlowTemplate(ctx, "tenant_a")
	if err != nil {
		t.Fatalf("GetDeliveryFlowTemplate() error = %v", err)
	}
	if template.TenantID != "tenant_a" || len(template.Stages) != 4 {
		t.Fatalf("unexpected default template: %+v", template)
	}
	if template.Stages[0].StageKey != "dev" || template.Stages[3].StageKey != "prod" || !template.Stages[3].RequiresApproval {
		t.Fatalf("default stages should be dev/test/staging/prod with prod approval, got %+v", template.Stages)
	}
	if template.Stages[0].LayoutColumn != 0 || template.Stages[3].LayoutColumn != 3 || template.Stages[0].Color != "#ED204E" || template.Stages[3].Color != "#e78a00" {
		t.Fatalf("default stages should have fixed layout slots and automatic column colors, got %+v", template.Stages)
	}
	if got := edgePairs(template.Edges); strings.Join(got, ",") != "dev->test,staging->prod,test->staging" {
		t.Fatalf("default template should expose linear DAG edges, got %+v", got)
	}

	updated, err := env.svc.SaveDeliveryFlowTemplateStage(ctx, SaveDeliveryFlowTemplateStageInput{
		Actor:                actor("usr_admin"),
		TenantID:             "tenant_a",
		StageKey:             "test",
		DisplayName:          "集成测试",
		Color:                "#13c2c2",
		Order:                3,
		LayoutColumn:         1,
		LayoutRow:            -1,
		Status:               DeliveryFlowTemplateStageEnabled,
		RequiresApproval:     true,
		RequiresVerification: true,
		ApproveRoles:         []string{"tenant_admin", "operator"},
		VerifyRoles:          []string{"developer", "operator"},
	})
	if err != nil {
		t.Fatalf("SaveDeliveryFlowTemplateStage() error = %v", err)
	}
	if updated.StageKey != "test" || updated.DisplayName != "集成测试" || !updated.RequiresApproval || !updated.RequiresVerification {
		t.Fatalf("stage update should keep key and update editable fields: %+v", updated)
	}
	if updated.Color != "#FD5352" || updated.LayoutColumn != 1 || updated.LayoutRow != -1 {
		t.Fatalf("stage color should be derived from layout column, got %+v", updated)
	}
	if len(updated.ApproveRoles) != 2 || updated.ApproveRoles[0] != "tenant_admin" || len(updated.VerifyRoles) != 2 {
		t.Fatalf("roles should be persisted: %+v", updated)
	}

	if _, err := env.svc.SaveDeliveryFlowTemplateStage(ctx, SaveDeliveryFlowTemplateStageInput{Actor: actor("usr_admin"), TenantID: "tenant_a", StageKey: "qa", NewStageKey: "uat", DisplayName: "验收", Color: "#722ed1", Order: 5}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("changing stage key should fail, got %v", err)
	}

	bindings, err := env.svc.ReplaceStageClusterBindings(ctx, ReplaceStageClusterBindingsInput{
		Actor:    actor("usr_admin"),
		TenantID: "tenant_a",
		StageKey: "dev",
		Clusters: []StageClusterBindingInput{
			{ClusterID: "cluster_shanghai", ClusterName: "上海集群"},
		},
	})
	if err != nil {
		t.Fatalf("ReplaceStageClusterBindings() error = %v", err)
	}
	if len(bindings) != 1 || bindings[0].StageKey != "dev" || bindings[0].Status != StageClusterBindingActive {
		t.Fatalf("unexpected bindings: %+v", bindings)
	}
	if _, err := env.svc.ReplaceStageClusterBindings(ctx, ReplaceStageClusterBindingsInput{
		Actor:    actor("usr_admin"),
		TenantID: "tenant_a",
		StageKey: "dev",
		Clusters: []StageClusterBindingInput{
			{ClusterID: "cluster_shanghai", ClusterName: "上海集群"},
			{ClusterID: "cluster_beijing", ClusterName: "北京集群"},
		},
	}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("stage should allow at most one bound cluster, got %v", err)
	}
	prodBindings, err := env.svc.ReplaceStageClusterBindings(ctx, ReplaceStageClusterBindingsInput{
		Actor:    actor("usr_admin"),
		TenantID: "tenant_a",
		StageKey: "prod",
		Clusters: []StageClusterBindingInput{
			{ClusterID: "cluster_shanghai", ClusterName: "上海集群"},
		},
	})
	if err != nil || len(prodBindings) != 1 {
		t.Fatalf("same cluster can bind multiple stages, got %+v err=%v", prodBindings, err)
	}

	deleted, err := env.svc.DeleteDeliveryFlowTemplateStage(ctx, StageTemplateActionInput{Actor: actor("usr_admin"), TenantID: "tenant_a", StageKey: "staging"})
	if err != nil {
		t.Fatalf("DeleteDeliveryFlowTemplateStage() error = %v", err)
	}
	if deleted.StageKey != "staging" {
		t.Fatalf("delete should return deleted stage snapshot, got %+v", deleted)
	}
	if _, err := env.repo.FindDeliveryFlowTemplateStage(ctx, "tenant_a", "staging"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("stage should be physically deleted, got %v", err)
	}
	edges, err := env.repo.ListDeliveryFlowTemplateEdges(ctx, template.ID)
	if err != nil {
		t.Fatalf("ListDeliveryFlowTemplateEdges() error = %v", err)
	}
	for _, edge := range edges {
		if edge.FromStageKey == "staging" || edge.ToStageKey == "staging" {
			t.Fatalf("deleting stage should remove related DAG edges, got %+v", edges)
		}
	}

	appStages, err := env.svc.ListAppStages(ctx, "app_user")
	if err != nil {
		t.Fatalf("ListAppStages() error = %v", err)
	}
	if len(appStages) != 3 {
		t.Fatalf("expected deleted stage to be absent, got %+v", appStages)
	}
	if appStages[0].StageKey != "dev" || appStages[0].ClusterPoolSize != 1 || appStages[0].BoundClusterID != "cluster_shanghai" || appStages[0].BoundClusterName != "上海集群" || !appStages[0].DeliveryStageID.IsZero() {
		t.Fatalf("app stage should project tenant cluster pool: %+v", appStages[0])
	}
	if _, err := env.svc.ReplaceStageClusterBindings(ctx, ReplaceStageClusterBindingsInput{
		Actor:    actor("usr_admin"),
		TenantID: "tenant_a",
		StageKey: "staging",
		Clusters: []StageClusterBindingInput{
			{ClusterID: "cluster_hangzhou", ClusterName: "杭州集群"},
		},
	}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("deleted stage should reject cluster bindings, got %v", err)
	}
}

func TestListAppStagesIncludesRuntimeStatus(t *testing.T) {
	env := newDeliveryEnv(t)
	ctx := context.Background()
	env.svc.runtimeStates = fakeRuntimeStateQuery{states: map[string]StageRuntimeState{
		"dev": {
			ApplicationID:  "app_user",
			StageKey:       "dev",
			SyncStatus:     "OutOfSync",
			HealthStatus:   "Degraded",
			OperationState: "running",
			Message:        "Pod order-api-7d9 ImagePullBackOff",
		},
	}}

	stages, err := env.svc.ListAppStages(ctx, "app_user")
	if err != nil {
		t.Fatalf("ListAppStages() error = %v", err)
	}
	var dev AppStage
	for _, stage := range stages {
		if stage.StageKey == "dev" {
			dev = stage
			break
		}
	}
	if dev.SyncStatus != "OutOfSync" || dev.HealthStatus != "Degraded" || dev.OperationState != "running" || dev.RuntimeMessage != "Pod order-api-7d9 ImagePullBackOff" {
		t.Fatalf("dev stage should include runtime status, got %+v", dev)
	}
}

func TestReplaceDeliveryFlowTemplateGraphValidatesDAGAndSyncsStages(t *testing.T) {
	env := newDeliveryEnv(t)
	ctx := context.Background()

	template, err := env.svc.ReplaceDeliveryFlowTemplateGraph(ctx, ReplaceDeliveryFlowTemplateGraphInput{
		Actor:    actor("usr_admin"),
		TenantID: "tenant_a",
		Stages: []SaveDeliveryFlowTemplateStageInput{
			{StageKey: "dev", DisplayName: "开发", Color: "#ffffff", Order: 1, LayoutColumn: 0, LayoutRow: 0, Status: DeliveryFlowTemplateStageEnabled},
			{StageKey: "test", DisplayName: "测试", Color: "#ffffff", Order: 2, LayoutColumn: 1, LayoutRow: -1, Status: DeliveryFlowTemplateStageEnabled, RequiresVerification: true, VerifyRoles: []string{"operator"}},
			{StageKey: "qa", DisplayName: "验收", Color: "#ffffff", Order: 3, LayoutColumn: 1, LayoutRow: 1, Status: DeliveryFlowTemplateStageEnabled, RequiresApproval: true, ApproveRoles: []string{"operator"}},
			{StageKey: "prod", DisplayName: "生产", Color: "#ffffff", Order: 4, LayoutColumn: 2, LayoutRow: 0, Status: DeliveryFlowTemplateStageEnabled, RequiresApproval: true, ApproveRoles: []string{"prod_approver"}},
		},
		DeletedStageKeys: []string{"staging"},
		Edges: []DeliveryFlowTemplateEdgeInput{
			{FromStageKey: "dev", ToStageKey: "test"},
			{FromStageKey: "dev", ToStageKey: "qa"},
			{FromStageKey: "test", ToStageKey: "prod"},
			{FromStageKey: "qa", ToStageKey: "prod"},
		},
	})
	if err != nil {
		t.Fatalf("ReplaceDeliveryFlowTemplateGraph() error = %v", err)
	}
	if got := edgePairs(template.Edges); strings.Join(got, ",") != "dev->qa,dev->test,qa->prod,test->prod" {
		t.Fatalf("unexpected graph edges: %+v", got)
	}
	if template.Stages[1].Color != "#FD5352" || template.Stages[2].Color != "#FD5352" || template.Stages[3].Color != "#FE7537" || template.Stages[1].LayoutRow != -1 || template.Stages[2].LayoutRow != 1 {
		t.Fatalf("graph save should persist slots and derive column colors, got %+v", template.Stages)
	}
	if _, err := env.repo.FindDeliveryFlowTemplateStage(ctx, "tenant_a", "staging"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("graph save should delete explicitly deleted stages, got %v", err)
	}
	if len(env.sync.calls) != 1 || env.sync.calls[0].TenantID != "tenant_a" || strings.Join(env.sync.calls[0].StageKeys, ",") != "dev,test,qa,prod" {
		t.Fatalf("graph save should sync application stage projections, got %+v", env.sync.calls)
	}

	_, err = env.svc.ReplaceDeliveryFlowTemplateGraph(ctx, ReplaceDeliveryFlowTemplateGraphInput{
		Actor:    actor("usr_admin"),
		TenantID: "tenant_a",
		Stages: []SaveDeliveryFlowTemplateStageInput{
			{StageKey: "dev", DisplayName: "开发", Color: "#1677ff", Status: DeliveryFlowTemplateStageEnabled},
			{StageKey: "test", DisplayName: "测试", Color: "#52c41a", Status: DeliveryFlowTemplateStageEnabled},
		},
		Edges: []DeliveryFlowTemplateEdgeInput{
			{FromStageKey: "dev", ToStageKey: "test"},
			{FromStageKey: "test", ToStageKey: "dev"},
		},
	})
	if shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("cycle should be rejected as invalid argument, got %v", err)
	}
}

func TestReplaceDeliveryFlowTemplateGraphDoesNotDeleteOmittedStagesWithoutExplicitDelete(t *testing.T) {
	env := newDeliveryEnv(t)
	ctx := context.Background()

	template, err := env.svc.GetDeliveryFlowTemplate(ctx, "tenant_a")
	if err != nil {
		t.Fatalf("GetDeliveryFlowTemplate() error = %v", err)
	}
	if len(template.Stages) != 4 {
		t.Fatalf("expected default template stages, got %+v", template.Stages)
	}

	template, err = env.svc.ReplaceDeliveryFlowTemplateGraph(ctx, ReplaceDeliveryFlowTemplateGraphInput{
		Actor:    actor("usr_admin"),
		TenantID: "tenant_a",
		Stages: []SaveDeliveryFlowTemplateStageInput{
			{StageKey: "dev", DisplayName: "开发", Order: 1, LayoutColumn: 0, LayoutRow: 0, Status: DeliveryFlowTemplateStageEnabled},
		},
		Edges: nil,
	})
	if err != nil {
		t.Fatalf("ReplaceDeliveryFlowTemplateGraph() error = %v", err)
	}
	if len(template.Stages) != 4 {
		t.Fatalf("response should include full persisted template, got %+v", template.Stages)
	}
	for _, stageKey := range []string{"test", "staging", "prod"} {
		if _, err := env.repo.FindDeliveryFlowTemplateStage(ctx, "tenant_a", stageKey); err != nil {
			t.Fatalf("omitted stage %s should not be deleted without explicit delete list: %v", stageKey, err)
		}
	}
}

func TestCreatePromotionWithSingleBoundStageCluster(t *testing.T) {
	env := newDeliveryEnv(t)
	ctx := context.Background()
	freight := seedFreight(t, env)
	if _, err := env.svc.ReplaceStageClusterBindings(ctx, ReplaceStageClusterBindingsInput{
		Actor:    actor("usr_admin"),
		TenantID: "tenant_a",
		StageKey: "dev",
		Clusters: []StageClusterBindingInput{
			{ClusterID: "cluster_shanghai", ClusterName: "上海集群"},
		},
	}); err != nil {
		t.Fatalf("ReplaceStageClusterBindings() error = %v", err)
	}
	inferred, err := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "dev"})
	if err != nil {
		t.Fatalf("stage promotion should infer the single bound cluster: %v", err)
	}
	if inferred.Status != PromotionManifestUpdated || len(env.gitops.specs) != 1 || len(env.gitops.specs[0].TargetClusters) != 1 || env.gitops.specs[0].TargetClusters[0].ClusterID != "cluster_shanghai" {
		t.Fatalf("stage promotion should use the single bound cluster, promotion=%+v specs=%+v", inferred, env.gitops.specs)
	}
	if got := env.gitops.specs[0].TargetClusters[0].Namespace; got != "payment" {
		t.Fatalf("default namespace should be Kubernetes-compatible, got %q", got)
	}
	if _, err := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "dev", TargetClusterIDs: []shared.ID{"cluster_missing"}}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("cluster outside stage pool should fail, got %v", err)
	}
	env.gitops.specs = nil
	promotion, err := env.svc.CreatePromotion(ctx, CreatePromotionInput{
		Actor:             actor("usr_dev"),
		FreightID:         freight.ID,
		TargetStageKey:    "dev",
		TargetClusterIDs:  []shared.ID{"cluster_shanghai"},
		NamespaceOverride: "order-dev",
		Message:           "发布到上海",
	})
	if err != nil {
		t.Fatalf("CreatePromotion() error = %v", err)
	}
	if promotion.Status != PromotionManifestUpdated || promotion.TargetStageKey != "dev" || promotion.NamespaceOverride != "order-dev" {
		t.Fatalf("unexpected promotion: %+v", promotion)
	}
	if len(env.gitops.specs) != 1 || len(env.gitops.specs[0].TargetClusters) != 1 {
		t.Fatalf("gitops spec should include the single bound cluster, got %+v", env.gitops.specs)
	}
	target := env.gitops.specs[0].TargetClusters[0]
	if target.ClusterID != "cluster_shanghai" || target.Namespace != "order-dev" || target.ClusterName != "上海集群" {
		t.Fatalf("unexpected gitops target cluster: %+v", target)
	}
}

func TestCreatePromotionWithDeletedStageClusterReturnsReadableMessage(t *testing.T) {
	env := newDeliveryEnv(t)
	env.svc.clusters = fakeClusterQuery{clusters: map[shared.ID]ClusterRef{}}
	ctx := context.Background()
	freight := seedFreight(t, env)
	if _, err := env.svc.ReplaceStageClusterBindings(ctx, ReplaceStageClusterBindingsInput{
		Actor:    actor("usr_admin"),
		TenantID: "tenant_a",
		StageKey: "dev",
		Clusters: []StageClusterBindingInput{
			{ClusterID: "cluster_shanghai", ClusterName: "上海集群"},
		},
	}); err != nil {
		t.Fatalf("ReplaceStageClusterBindings() error = %v", err)
	}

	mux := http.NewServeMux()
	NewHandler(env.svc).Register(mux)
	body := mustJSON(t, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetClusterIDs: []shared.ID{"cluster_shanghai"}})
	rec := serveJSON(mux, http.MethodPost, "/api/apps/app_user/delivery/stages/dev/promotions", body)

	assertStatus(t, rec, http.StatusNotFound)
	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if payload.Error.Code != string(shared.CodeNotFound) || payload.Error.Message != "目标集群不存在或已被删除，请重新绑定 Stage 集群" {
		t.Fatalf("unexpected error payload: %+v", payload.Error)
	}
}

func TestReadableNotFoundMessagesForPromotionFlow(t *testing.T) {
	cases := []struct {
		name    string
		message string
		want    string
	}{
		{name: "cluster", message: "cluster not found", want: "目标集群不存在或已被删除，请重新绑定 Stage 集群"},
		{name: "application", message: "application not found", want: "应用不存在或已被删除"},
		{name: "workload", message: "workload not found", want: "工作负载不存在或已被删除"},
		{name: "deployment template", message: "deployment template not found", want: "部署模板不存在，请先初始化应用部署模板"},
		{name: "application template", message: "application template not found", want: "部署模板不存在，请先初始化应用部署模板"},
		{name: "platform template", message: "platform template not found", want: "部署模板不存在，请先初始化应用部署模板"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := readableErrorMessage(shared.NewError(shared.CodeNotFound, tt.message))
			if got != tt.want {
				t.Fatalf("readableErrorMessage(%q) = %q, want %q", tt.message, got, tt.want)
			}
		})
	}
}

func TestCreatePromotionUsesTemplateApprovalRuleForAnyStage(t *testing.T) {
	env := newDeliveryEnv(t)
	ctx := context.Background()
	freight := seedFreight(t, env)
	if _, err := env.svc.SaveDeliveryFlowTemplateStage(ctx, SaveDeliveryFlowTemplateStageInput{
		Actor:            actor("usr_admin"),
		TenantID:         "tenant_a",
		StageKey:         "dev",
		DisplayName:      "开发",
		Order:            1,
		Status:           DeliveryFlowTemplateStageEnabled,
		RequiresApproval: true,
		ApproveRoles:     []string{"operator"},
	}); err != nil {
		t.Fatalf("SaveDeliveryFlowTemplateStage() error = %v", err)
	}
	promotion, err := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "dev"})
	if err != nil {
		t.Fatalf("CreatePromotion() error = %v", err)
	}
	if promotion.Status != PromotionPendingApproval {
		t.Fatalf("stage should use template approval rule, got %+v", promotion)
	}
	if len(env.gitops.specs) != 0 {
		t.Fatalf("pending approval promotion should not apply gitops yet, got %+v", env.gitops.specs)
	}
}

func TestFreightApprovalAndStageVerification(t *testing.T) {
	env := newDeliveryEnv(t)
	ctx := context.Background()
	freight := seedFreight(t, env)

	approval, err := env.svc.CompleteFreightApproval(ctx, FreightApprovalInput{
		Actor:          actor("usr_ops"),
		FreightID:      freight.ID,
		TargetStageKey: "prod",
		Decision:       FreightApprovalApproved,
		Comment:        "同意发布",
	})
	if err != nil {
		t.Fatalf("CompleteFreightApproval() error = %v", err)
	}
	if approval.Status != FreightApprovalApproved || approval.TargetStageKey != "prod" || approval.ApproverID != "usr_ops" {
		t.Fatalf("unexpected freight approval: %+v", approval)
	}

	if _, err := env.svc.CompleteStageVerification(ctx, StageVerificationInput{
		Actor:         actor("usr_ops"),
		ApplicationID: "app_user",
		StageKey:      "dev",
		FreightID:     freight.ID,
		Status:        StageVerificationPassed,
		Comment:       "验证通过",
		SyncStatus:    "OutOfSync",
		HealthStatus:  "Degraded",
		AgentStatus:   "ready",
	}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("verification without deployment record should fail, got %v", err)
	}

	promotion, err := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "dev"})
	if err != nil || promotion.Status != PromotionManifestUpdated {
		t.Fatalf("CreatePromotion() error = %v promotion=%+v", err, promotion)
	}
	verification, err := env.svc.CompleteStageVerification(ctx, StageVerificationInput{
		Actor:         actor("usr_ops"),
		ApplicationID: "app_user",
		StageKey:      "dev",
		FreightID:     freight.ID,
		Status:        StageVerificationFailed,
		Comment:       "健康异常，验证不通过",
		SyncStatus:    "OutOfSync",
		HealthStatus:  "Degraded",
		AgentStatus:   "ready",
	})
	if err != nil {
		t.Fatalf("CompleteStageVerification() error = %v", err)
	}
	if verification.Status != StageVerificationFailed || verification.HealthStatus != "Degraded" || verification.VerifierID != "usr_ops" {
		t.Fatalf("verification should persist evidence even when failed: %+v", verification)
	}
	verification, err = env.svc.CompleteStageVerification(ctx, StageVerificationInput{
		Actor:         actor("usr_ops"),
		ApplicationID: "app_user",
		StageKey:      "dev",
		FreightID:     freight.ID,
		Status:        StageVerificationPassed,
		Comment:       "复测通过",
		SyncStatus:    "Synced",
		HealthStatus:  "Healthy",
		AgentStatus:   "ready",
	})
	if err != nil {
		t.Fatalf("CompleteStageVerification(update) error = %v", err)
	}
	if verification.Status != StageVerificationPassed || verification.HealthStatus != "Healthy" || verification.Comment != "复测通过" {
		t.Fatalf("verification should be updatable after a failed result: %+v", verification)
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

func TestGetFreightDetailIncludesItems(t *testing.T) {
	env := newDeliveryEnv(t)
	freight := seedFreight(t, env)

	detail, err := env.svc.GetFreightDetail(context.Background(), freight.ID)
	if err != nil {
		t.Fatalf("GetFreightDetail() error = %v", err)
	}
	if detail.Freight.ID != freight.ID || len(detail.Items) != 1 {
		t.Fatalf("unexpected freight detail: %+v", detail)
	}
	item := detail.Items[0]
	if item.WorkloadID != "workload_api" || item.SourceType != FreightItemPipelineArtifact || item.ReleaseID == "" || item.BuildArtifactID == "" {
		t.Fatalf("detail item should keep workload/source/release/artifact identity: %+v", item)
	}
	if item.URI != "registry.example/paas/user-api:abcdef" || item.ImageRef != "registry.example/paas/user-api:abcdef" || item.ImageRepository != "registry.example/paas/user-api" || item.ImageTag != "abcdef" || item.Digest != "sha256:abc" {
		t.Fatalf("detail item should keep image fields: %+v", item)
	}
}

func TestArchiveFreightHidesFromPublishChoicesAndKeepsDetail(t *testing.T) {
	env := newDeliveryEnv(t)
	freight := seedFreight(t, env)

	archived, err := env.svc.ArchiveFreight(context.Background(), ArchiveFreightInput{Actor: actor("usr_dev"), FreightID: freight.ID})
	if err != nil {
		t.Fatalf("ArchiveFreight() error = %v", err)
	}
	if archived.Status != FreightArchived {
		t.Fatalf("archived freight status = %s, want %s", archived.Status, FreightArchived)
	}
	listed, err := env.svc.ListFreights(context.Background(), "app_user", shared.PageRequest{Page: 1, PageSize: 10})
	if err != nil || len(listed.Items) != 0 {
		t.Fatalf("archived freight should be hidden from list, got %+v, %v", listed.Items, err)
	}
	eligible, err := env.svc.ListEligibleFreights(context.Background(), "app_user", "dev")
	if err != nil || len(eligible) != 0 {
		t.Fatalf("archived freight should be hidden from eligible freights, got %+v, %v", eligible, err)
	}
	detail, err := env.svc.GetFreightDetail(context.Background(), freight.ID)
	if err != nil || detail.Freight.Status != FreightArchived || len(detail.Items) != 1 {
		t.Fatalf("archived freight detail should remain readable, got %+v, %v", detail, err)
	}
	if _, err := env.svc.CreatePromotion(context.Background(), CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "dev"}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("archived freight should not be promoted, got %v", err)
	}
	if _, err := env.svc.CreateRollbackPromotion(context.Background(), CreateRollbackPromotionInput{Actor: actor("usr_dev"), TargetFreightID: freight.ID, TargetStageKey: "dev"}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("archived freight should not be rollback target, got %v", err)
	}
	if len(env.audit.events) == 0 || env.audit.events[len(env.audit.events)-1].Action != "freight.archive" {
		t.Fatalf("archive should be audited, got %+v", env.audit.events)
	}
}

func TestArchiveFreightRejectsCurrentStageFreightAndUnfinishedPromotion(t *testing.T) {
	env := newDeliveryEnv(t)
	current := seedFreight(t, env)
	promoteFreightThrough(t, env, current, "dev")

	if _, err := env.svc.ArchiveFreight(context.Background(), ArchiveFreightInput{Actor: actor("usr_dev"), FreightID: current.ID}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("current stage freight should not be archived, got %v", err)
	}

	env = newDeliveryEnv(t)
	pending := seedFreight(t, env)
	flow, err := env.repo.FindDeliveryFlowByApplication(context.Background(), "app_user")
	if err != nil {
		t.Fatalf("FindDeliveryFlowByApplication() error = %v", err)
	}
	stages, err := env.repo.ListDeliveryStages(context.Background(), flow.ID)
	if err != nil {
		t.Fatalf("ListDeliveryStages() error = %v", err)
	}
	var prod DeliveryStage
	for _, stage := range stages {
		if stage.Name == "prod" {
			prod = stage
		}
	}
	if prod.ID.IsZero() {
		t.Fatalf("prod stage not found in %+v", stages)
	}
	now := time.Date(2026, 5, 30, 8, 0, 0, 0, time.UTC)
	if err := env.repo.CreatePromotion(context.Background(), Promotion{ID: "promotion_pending", TenantID: pending.TenantID, ProjectID: pending.ProjectID, ApplicationID: pending.ApplicationID, FreightID: pending.ID, TargetStageID: prod.ID, TargetStageKey: "prod", Status: PromotionPendingApproval, CreatedBy: "usr_dev", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("CreatePromotion() error = %v", err)
	}
	if _, err := env.svc.ArchiveFreight(context.Background(), ArchiveFreightInput{Actor: actor("usr_dev"), FreightID: pending.ID}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("freight with unfinished promotion should not be archived, got %v", err)
	}
}

func TestArchiveFreightHandler(t *testing.T) {
	env := newDeliveryEnv(t)
	freight := seedFreight(t, env)
	mux := http.NewServeMux()
	NewHandler(env.svc).Register(mux)

	rec := serveJSON(mux, http.MethodDelete, "/api/freights/"+freight.ID.String(), mustJSON(t, ArchiveFreightInput{Actor: actor("usr_dev")}))
	assertStatus(t, rec, http.StatusOK)
	var archived Freight
	if err := json.NewDecoder(rec.Body).Decode(&archived); err != nil {
		t.Fatalf("decode archived freight: %v", err)
	}
	if archived.ID != freight.ID || archived.Status != FreightArchived {
		t.Fatalf("DELETE freight should archive it, got %+v", archived)
	}
	listRec := serveJSON(mux, http.MethodGet, "/api/apps/app_user/freights?page=1&page_size=5", nil)
	assertStatus(t, listRec, http.StatusOK)
	var listed shared.PageResult[Freight]
	if err := json.NewDecoder(listRec.Body).Decode(&listed); err != nil {
		t.Fatalf("decode listed freights: %v", err)
	}
	if len(listed.Items) != 0 {
		t.Fatalf("archived freight should be hidden from API list, got %+v", listed.Items)
	}
	detailRec := serveJSON(mux, http.MethodGet, "/api/freights/"+freight.ID.String(), nil)
	assertStatus(t, detailRec, http.StatusOK)
	var detail FreightDetail
	if err := json.NewDecoder(detailRec.Body).Decode(&detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail.Freight.Status != FreightArchived || len(detail.Items) != 1 {
		t.Fatalf("archived freight detail should remain readable, got %+v", detail)
	}
}

func TestManualFreightAcceptsDirectPipelineArtifactWithoutDigest(t *testing.T) {
	env := newDeliveryEnv(t)
	env.svc.builds = fakeBuildQuery{
		runs: map[shared.ID]BuildRunRef{
			"build_1": {ID: "build_1", TenantID: "tenant_a", ProjectID: "project_payment", ApplicationID: "app_user", CommitSHA: "abcdef1234567890"},
		},
		artifacts: map[shared.ID]BuildArtifactRef{
			"artifact_1": {ID: "artifact_1", BuildRunID: "build_1", ApplicationID: "app_user", WorkloadID: "workload_api", URI: "registry.example/paas/user-api:abcdef"},
		},
	}
	freight, err := env.svc.CreateFreight(context.Background(), CreateFreightInput{
		Actor:         actor("usr_dev"),
		ApplicationID: "app_user",
		Items: []CreateFreightItemInput{{
			WorkloadID:      "workload_api",
			SourceType:      FreightItemPipelineArtifact,
			BuildArtifactID: "artifact_1",
		}},
	})
	if err != nil {
		t.Fatalf("CreateFreight() should allow pipeline artifact without digest: %v", err)
	}
	detail, err := env.svc.GetFreightDetail(context.Background(), freight.ID)
	if err != nil || len(detail.Items) != 1 || detail.Items[0].Digest != "" || detail.Items[0].ImageRef != "registry.example/paas/user-api:abcdef" {
		t.Fatalf("freight item should keep image without digest, got %+v, %v", detail, err)
	}
}

func TestManualFreightAcceptsDirectPipelineArtifactWithoutCommit(t *testing.T) {
	env := newDeliveryEnv(t)
	env.svc.builds = fakeBuildQuery{
		runs: map[shared.ID]BuildRunRef{
			"build_1": {ID: "build_1", TenantID: "tenant_a", ProjectID: "project_payment", ApplicationID: "app_user"},
		},
		artifacts: map[shared.ID]BuildArtifactRef{
			"artifact_1": {ID: "artifact_1", BuildRunID: "build_1", ApplicationID: "app_user", WorkloadID: "workload_api", URI: "registry.example/paas/user-api:abcdef", Digest: "sha256:abc"},
		},
	}
	freight, err := env.svc.CreateFreight(context.Background(), CreateFreightInput{
		Actor:         actor("usr_dev"),
		ApplicationID: "app_user",
		Items: []CreateFreightItemInput{{
			WorkloadID:      "workload_api",
			SourceType:      FreightItemPipelineArtifact,
			BuildArtifactID: "artifact_1",
		}},
	})
	if err != nil {
		t.Fatalf("CreateFreight() should allow pipeline artifact without commit: %v", err)
	}
	detail, err := env.svc.GetFreightDetail(context.Background(), freight.ID)
	if err != nil || len(detail.Items) != 1 || detail.Items[0].Digest != "sha256:abc" || detail.Items[0].ImageRef != "registry.example/paas/user-api:abcdef" {
		t.Fatalf("freight item should keep image without commit, got %+v, %v", detail, err)
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
	if _, err := env.svc.CreatePromotion(context.Background(), CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "test"}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
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
	promoteFreightThrough(t, env, freight, "dev")
	eligible, err = env.svc.ListEligibleFreights(context.Background(), "app_user", stages[1].ID)
	if err != nil || len(eligible) != 1 || eligible[0].ID != freight.ID {
		t.Fatalf("test should be eligible after dev, got %+v %v", eligible, err)
	}
}

func TestEligibleFreightsUseDAGParentsAndRequiredVerification(t *testing.T) {
	env := newDeliveryEnv(t)
	ctx := context.Background()
	freight := seedFreight(t, env)
	if _, err := env.svc.ReplaceDeliveryFlowTemplateGraph(ctx, ReplaceDeliveryFlowTemplateGraphInput{
		Actor:    actor("usr_admin"),
		TenantID: "tenant_a",
		Stages: []SaveDeliveryFlowTemplateStageInput{
			{StageKey: "dev", DisplayName: "开发", Color: "#1677ff", Order: 1, Status: DeliveryFlowTemplateStageEnabled},
			{StageKey: "test", DisplayName: "测试", Color: "#52c41a", Order: 2, Status: DeliveryFlowTemplateStageEnabled, RequiresVerification: true, VerifyRoles: []string{"operator"}},
			{StageKey: "qa", DisplayName: "验收", Color: "#13c2c2", Order: 3, Status: DeliveryFlowTemplateStageEnabled},
			{StageKey: "prod", DisplayName: "生产", Color: "#f5222d", Order: 4, Status: DeliveryFlowTemplateStageEnabled, RequiresApproval: true, ApproveRoles: []string{"prod_approver"}},
		},
		Edges: []DeliveryFlowTemplateEdgeInput{
			{FromStageKey: "dev", ToStageKey: "test"},
			{FromStageKey: "dev", ToStageKey: "qa"},
			{FromStageKey: "test", ToStageKey: "prod"},
			{FromStageKey: "qa", ToStageKey: "prod"},
		},
	}); err != nil {
		t.Fatalf("ReplaceDeliveryFlowTemplateGraph() error = %v", err)
	}
	if _, err := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "prod"}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("prod should require both DAG parents first, got %v", err)
	}
	promoteFreightThrough(t, env, freight, "dev", "test", "qa")
	if _, err := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "prod"}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("prod should wait for test verification, got %v", err)
	}
	if _, err := env.svc.CompleteStageVerification(ctx, StageVerificationInput{Actor: actor("usr_ops"), ApplicationID: "app_user", StageKey: "test", FreightID: freight.ID, Status: StageVerificationPassed, Comment: "通过", SyncStatus: "Synced", HealthStatus: "Healthy", AgentStatus: "ready"}); err != nil {
		t.Fatalf("CompleteStageVerification() error = %v", err)
	}
	prod, err := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "prod"})
	if err != nil {
		t.Fatalf("CreatePromotion(prod) should pass after both parents and verification: %v", err)
	}
	if prod.Status != PromotionPendingApproval {
		t.Fatalf("prod should use template approval rule, got %+v", prod)
	}
}

func TestFreightCreationContextUsesStageProjectionKeys(t *testing.T) {
	env := newDeliveryEnv(t)
	freight := seedFreight(t, env)
	appStages, err := env.svc.ListAppStages(context.Background(), "app_user")
	if err != nil {
		t.Fatalf("ListAppStages() error = %v", err)
	}

	contextOut, err := env.svc.GetFreightCreationContext(context.Background(), "app_user")
	if err != nil {
		t.Fatalf("GetFreightCreationContext() error = %v", err)
	}
	if len(contextOut.Stages) != len(appStages) {
		t.Fatalf("context should include stage projections, got %+v want %+v", contextOut.Stages, appStages)
	}
	for i, stage := range appStages {
		got := contextOut.Stages[i]
		if got.ID != shared.ID(stage.StageKey) || got.StageKey != stage.StageKey || got.Name != stage.DisplayName || got.ApprovalRequired != stage.RequiresApproval {
			t.Fatalf("context stage[%d] = %+v, want %+v", i, got, stage)
		}
	}
	if got := contextOut.StageEligibility["dev"]; len(got) != 1 || got[0] != freight.ID {
		t.Fatalf("stage eligibility should use stage key, got %+v", contextOut.StageEligibility)
	}
}

func TestPromotionDevAppliesGitOpsAndProdRequiresApproval(t *testing.T) {
	env := newDeliveryEnv(t)
	freight := seedFreight(t, env)
	dev, err := env.svc.CreatePromotion(context.Background(), CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "dev"})
	if err != nil {
		t.Fatalf("CreatePromotion(dev) error = %v", err)
	}
	if dev.Status != PromotionManifestUpdated || dev.ManifestRevision != "rev-1" || len(env.gitops.specs) != 1 {
		t.Fatalf("dev promotion should apply gitops, got %+v specs=%+v", dev, env.gitops.specs)
	}
	if got := env.gitops.specs[0].Artifacts; len(got) != 1 || got[0].WorkloadID != "workload_api" || got[0].Repository != "registry.example/paas/user-api" || got[0].Tag != "abcdef" || got[0].Digest != "sha256:abc" {
		t.Fatalf("gitops artifact should include workload image fields, got %+v", got)
	}
	promoteFreightThrough(t, env, freight, "test", "staging")
	prod, err := env.svc.CreatePromotion(context.Background(), CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "prod"})
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

func edgePairs(edges []DeliveryFlowTemplateEdge) []string {
	out := make([]string, 0, len(edges))
	for _, edge := range edges {
		out = append(out, edge.FromStageKey+"->"+edge.ToStageKey)
	}
	sort.Strings(out)
	return out
}

func TestRejectAbortPendingEnvironmentRollbackAndGitOpsFailure(t *testing.T) {
	env := newDeliveryEnv(t)
	freight := seedFreight(t, env)
	if err := env.repo.ReplaceStageClusterBindings(context.Background(), "tenant_a", "dev", nil); err != nil {
		t.Fatalf("clear dev stage cluster binding: %v", err)
	}
	if _, err := env.svc.CreatePromotion(context.Background(), CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "dev"}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("missing stage cluster binding should block promotion, got %v", err)
	}
	if err := env.repo.ReplaceStageClusterBindings(context.Background(), "tenant_a", "dev", []StageClusterBinding{{
		ID:          "binding_dev",
		TenantID:    "tenant_a",
		StageKey:    "dev",
		ClusterID:   "cluster_dev",
		ClusterName: "dev-cluster",
		Status:      StageClusterBindingActive,
		CreatedAt:   time.Date(2026, 5, 30, 7, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 5, 30, 7, 0, 0, 0, time.UTC),
	}}); err != nil {
		t.Fatalf("restore dev stage cluster binding: %v", err)
	}
	promoteFreightThrough(t, env, freight, "dev", "test", "staging")
	prod, _ := env.svc.CreatePromotion(context.Background(), CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "prod"})
	rejected, err := env.svc.RejectPromotion(context.Background(), ApprovalInput{Actor: actor("usr_ops"), PromotionID: prod.ID, Comment: "no"})
	if err != nil || rejected.Status != PromotionRejected || rejected.CompletedAt == nil {
		t.Fatalf("reject failed: %+v %v", rejected, err)
	}
	dev, _ := env.svc.CreatePromotion(context.Background(), CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "test"})
	aborted, err := env.svc.AbortPromotion(context.Background(), actor("usr_dev"), dev.ID)
	if err != nil || aborted.Status != PromotionAborted {
		t.Fatalf("abort should mark non-terminal promotion aborted, got %+v %v", aborted, err)
	}
	rollback, err := env.svc.CreateRollbackPromotion(context.Background(), CreateRollbackPromotionInput{Actor: actor("usr_dev"), TargetFreightID: freight.ID, CurrentFreightID: freight.ID, TargetStageKey: "dev"})
	if err != nil || !rollback.IsRollback || rollback.Status != PromotionManifestUpdated {
		t.Fatalf("rollback failed: %+v %v", rollback, err)
	}
	env.gitops.err = errors.New("gitops failed")
	if _, err := env.svc.CreatePromotion(context.Background(), CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "dev"}); err == nil {
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
	contextRec := serveJSON(mux, http.MethodGet, "/api/apps/app_user/freights/creation-context", nil)
	assertStatus(t, contextRec, http.StatusOK)
	var contextOut FreightCreationContext
	if err := json.NewDecoder(contextRec.Body).Decode(&contextOut); err != nil {
		t.Fatalf("decode freight creation context: %v", err)
	}
	firstStageEligible := []shared.ID(nil)
	if len(contextOut.Stages) > 0 {
		firstStageEligible = contextOut.StageEligibility[contextOut.Stages[0].ID]
	}
	if len(contextOut.Stages) != 4 || contextOut.Stages[0].ID != "dev" || contextOut.Stages[0].StageKey != "dev" || len(firstStageEligible) != 1 || firstStageEligible[0] != freight.ID {
		t.Fatalf("creation context should include stage projection keys and eligibility, got %+v", contextOut)
	}
	templateRec := serveJSON(mux, http.MethodGet, "/api/tenants/tenant_a/delivery-flow-template", nil)
	assertStatus(t, templateRec, http.StatusOK)
	var template DeliveryFlowTemplate
	if err := json.NewDecoder(templateRec.Body).Decode(&template); err != nil {
		t.Fatalf("decode delivery flow template: %v", err)
	}
	if len(template.Stages) != 4 || template.Stages[0].StageKey != "dev" {
		t.Fatalf("GET template should include default stages, got %+v", template)
	}
	stageBody := mustJSON(t, SaveDeliveryFlowTemplateStageInput{Actor: actor("usr_admin"), DisplayName: "开发环境", Color: "#1677ff", Order: 1, Status: DeliveryFlowTemplateStageEnabled, VerifyRoles: []string{"developer"}})
	stageRec := serveJSON(mux, http.MethodPatch, "/api/tenants/tenant_a/delivery-flow-template/stages/dev", stageBody)
	assertStatus(t, stageRec, http.StatusOK)
	bindingBody := mustJSON(t, ReplaceStageClusterBindingsInput{Actor: actor("usr_admin"), Clusters: []StageClusterBindingInput{{ClusterID: "cluster_shanghai", ClusterName: "上海集群"}}})
	bindingRec := serveJSON(mux, http.MethodPut, "/api/tenants/tenant_a/delivery-flow-template/stages/dev/cluster-bindings", bindingBody)
	assertStatus(t, bindingRec, http.StatusOK)
	var bindingOut struct {
		Items []StageClusterBinding `json:"items"`
	}
	if err := json.NewDecoder(bindingRec.Body).Decode(&bindingOut); err != nil {
		t.Fatalf("decode stage cluster bindings: %v", err)
	}
	if len(bindingOut.Items) != 1 || bindingOut.Items[0].ClusterID != "cluster_shanghai" {
		t.Fatalf("PUT cluster bindings should return bindings, got %+v", bindingOut)
	}
	bindingListRec := serveJSON(mux, http.MethodGet, "/api/tenants/tenant_a/delivery-flow-template/stages/dev/cluster-bindings", nil)
	assertStatus(t, bindingListRec, http.StatusOK)
	var bindingListOut struct {
		Items []StageClusterBinding `json:"items"`
	}
	if err := json.NewDecoder(bindingListRec.Body).Decode(&bindingListOut); err != nil {
		t.Fatalf("decode listed stage cluster bindings: %v", err)
	}
	if len(bindingListOut.Items) != 1 || bindingListOut.Items[0].ClusterName != "上海集群" {
		t.Fatalf("GET cluster bindings should return bindings, got %+v", bindingListOut)
	}
	appStagesRec := serveJSON(mux, http.MethodGet, "/api/apps/app_user/stages", nil)
	assertStatus(t, appStagesRec, http.StatusOK)
	var appStagesOut struct {
		Items []AppStage `json:"items"`
	}
	if err := json.NewDecoder(appStagesRec.Body).Decode(&appStagesOut); err != nil {
		t.Fatalf("decode app stages: %v", err)
	}
	if len(appStagesOut.Items) != 4 || appStagesOut.Items[0].ClusterPoolSize != 1 {
		t.Fatalf("GET app stages should include stage cluster pool, got %+v", appStagesOut)
	}
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/apps/app_user/freights?page=1&page_size=5", nil), http.StatusOK)
	detailRec := serveJSON(mux, http.MethodGet, "/api/freights/"+freight.ID.String(), nil)
	assertStatus(t, detailRec, http.StatusOK)
	var detail FreightDetail
	if err := json.NewDecoder(detailRec.Body).Decode(&detail); err != nil {
		t.Fatalf("decode freight detail: %v", err)
	}
	if detail.Freight.ID != freight.ID || len(detail.Items) != 1 || detail.Items[0].WorkloadID != "workload_api" || detail.Items[0].ImageRef == "" {
		t.Fatalf("GET freight detail should include items, got %+v", detail)
	}
	promoBody, _ := json.Marshal(CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "dev"})
	promoRec := serveJSON(mux, http.MethodPost, "/api/promotions", promoBody)
	assertStatus(t, promoRec, http.StatusCreated)
	var promotion Promotion
	_ = json.NewDecoder(promoRec.Body).Decode(&promotion)
	stagePromotionBody := mustJSON(t, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetClusterIDs: []shared.ID{"cluster_shanghai"}, NamespaceOverride: "order-dev"})
	stagePromotionRec := serveJSON(mux, http.MethodPost, "/api/apps/app_user/delivery/stages/"+appStagesOut.Items[0].StageKey+"/promotions", stagePromotionBody)
	assertStatus(t, stagePromotionRec, http.StatusCreated)
	var stagePromotion Promotion
	_ = json.NewDecoder(stagePromotionRec.Body).Decode(&stagePromotion)
	if stagePromotion.TargetStageKey != "dev" || stagePromotion.NamespaceOverride != "order-dev" {
		t.Fatalf("stage route should derive stage target from path, got %+v", stagePromotion)
	}
	stageKeyPromotionRec := serveJSON(mux, http.MethodPost, "/api/apps/app_user/delivery/stages/dev/promotions", stagePromotionBody)
	assertStatus(t, stageKeyPromotionRec, http.StatusCreated)
	var stageKeyPromotion Promotion
	_ = json.NewDecoder(stageKeyPromotionRec.Body).Decode(&stageKeyPromotion)
	if stageKeyPromotion.TargetStageKey != "dev" {
		t.Fatalf("stage route should accept stage key path value, got %+v", stageKeyPromotion)
	}
	freightApprovalRec := serveJSON(mux, http.MethodPost, "/api/freights/"+freight.ID.String()+"/approvals", mustJSON(t, FreightApprovalInput{Actor: actor("usr_ops"), TargetStageKey: "dev", Decision: FreightApprovalApproved, Comment: "同意"}))
	assertStatus(t, freightApprovalRec, http.StatusCreated)
	verificationRec := serveJSON(mux, http.MethodPost, "/api/apps/app_user/stages/dev/verification", mustJSON(t, StageVerificationInput{Actor: actor("usr_ops"), FreightID: freight.ID, Status: StageVerificationPassed, Comment: "验证通过", SyncStatus: "Synced", HealthStatus: "Healthy", AgentStatus: "ready"}))
	assertStatus(t, verificationRec, http.StatusCreated)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/promotions/"+promotion.ID.String(), nil), http.StatusOK)
	assertStatus(t, serveJSON(mux, http.MethodGet, "/api/apps/app_user/promotions", nil), http.StatusOK)
	abortBody, _ := json.Marshal(struct {
		Actor identityaccess.Subject `json:"actor"`
	}{Actor: actor("usr_dev")})
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/promotions/"+promotion.ID.String()+"/abort", abortBody), http.StatusOK)
	rollbackBody, _ := json.Marshal(CreateRollbackPromotionInput{Actor: actor("usr_dev"), TargetFreightID: freight.ID, TargetStageKey: "dev"})
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/promotions/rollback", rollbackBody), http.StatusCreated)
	prodBody, _ := json.Marshal(CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "prod"})
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/promotions", mustJSON(t, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "test"})), http.StatusCreated)
	assertStatus(t, serveJSON(mux, http.MethodPost, "/api/promotions", mustJSON(t, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "staging"})), http.StatusCreated)
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
	deleteStageRec := serveJSON(mux, http.MethodDelete, "/api/tenants/tenant_a/delivery-flow-template/stages/staging", mustJSON(t, StageTemplateActionInput{Actor: actor("usr_admin")}))
	assertStatus(t, deleteStageRec, http.StatusOK)
	templateAfterDeleteRec := serveJSON(mux, http.MethodGet, "/api/tenants/tenant_a/delivery-flow-template", nil)
	assertStatus(t, templateAfterDeleteRec, http.StatusOK)
	var templateAfterDelete DeliveryFlowTemplate
	if err := json.NewDecoder(templateAfterDeleteRec.Body).Decode(&templateAfterDelete); err != nil {
		t.Fatalf("decode template after delete: %v", err)
	}
	for _, stage := range templateAfterDelete.Stages {
		if stage.StageKey == "staging" {
			t.Fatalf("DELETE stage should physically remove staging, got %+v", templateAfterDelete.Stages)
		}
	}
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

func TestHandlerWritesReadablePromotionPreconditionError(t *testing.T) {
	env := newDeliveryEnv(t)
	mux := http.NewServeMux()
	NewHandler(env.svc).Register(mux)
	freight := seedFreight(t, env)
	if err := env.repo.ReplaceStageClusterBindings(context.Background(), "tenant_a", "dev", nil); err != nil {
		t.Fatalf("clear dev stage cluster binding: %v", err)
	}

	body := mustJSON(t, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID})
	rec := serveJSON(mux, http.MethodPost, "/api/apps/app_user/delivery/stages/dev/promotions", body)
	assertStatus(t, rec, http.StatusPreconditionFailed)

	var out struct {
		Error struct {
			Code    shared.ErrorCode `json:"code"`
			Message string           `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if out.Error.Code != shared.CodeFailedPrecondition || out.Error.Message != "该 Stage 未绑定集群，请先在交付流模板中绑定集群" {
		t.Fatalf("unexpected error response: %+v", out.Error)
	}
}

func TestFailedPreconditionMessagePreservesSafeChineseReason(t *testing.T) {
	if got := failedPreconditionMessage("当前版本不适用于目标环境，请联系平台管理员检查运行时镜像配置"); got != "当前版本不适用于目标环境，请联系平台管理员检查运行时镜像配置" {
		t.Fatalf("safe Chinese reason should be preserved, got %q", got)
	}
	if got := failedPreconditionMessage("当前 token 配置无效"); got != "发布前置条件不满足" {
		t.Fatalf("sensitive Chinese reason should be hidden, got %q", got)
	}
}

func TestFreightCreationContextBackfillsMissingReleaseFromSucceededBuild(t *testing.T) {
	env := newDeliveryEnv(t)
	ctx := context.Background()
	env.svc.builds = fakeBuildQuery{
		runs: map[shared.ID]BuildRunRef{
			"build_2": {ID: "build_2", TenantID: "tenant_a", ProjectID: "project_payment", ApplicationID: "app_user", PipelineID: "pipeline_main", PipelineName: "main", PipelineDisplayName: "主流水线", Status: "succeeded"},
		},
		artifacts: map[shared.ID]BuildArtifactRef{
			"artifact_2": {ID: "artifact_2", BuildRunID: "build_2", ApplicationID: "app_user", WorkloadID: "workload_api", URI: "registry.example/paas/user-api:legacy", IsPrimary: true},
		},
	}

	contextOut, err := env.svc.GetFreightCreationContext(ctx, "app_user")
	if err != nil {
		t.Fatalf("GetFreightCreationContext() error = %v", err)
	}
	release := contextOut.LatestReleasesByWorkload["workload_api"]
	if release.ID.IsZero() || release.BuildRunID != "build_2" || release.Version != "build-build_2" || release.CommitSHA != "" || release.ImageDigest != "" || release.ImageURI != "registry.example/paas/user-api:legacy" {
		t.Fatalf("creation context should backfill release from successful build without commit, got %+v", release)
	}
	if artifact := contextOut.LatestArtifactsByWorkload["workload_api"]; artifact.ID != "artifact_2" {
		t.Fatalf("creation context should expose backfilled artifact, got %+v", contextOut.LatestArtifactsByWorkload)
	}
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
	if _, err := env.repo.FindDeliveryStageByName(ctx, "other", stages[0].Name); shared.CodeOf(err) != shared.CodeNotFound {
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
	if _, err := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "dev"}); shared.CodeOf(err) != shared.CodePermissionDenied {
		t.Fatalf("promotion permission should fail, got %v", err)
	}
	env = newDeliveryEnv(t)
	freight = seedFreight(t, env)
	if _, err := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing env should fail, got %v", err)
	}
	env.gitops.err = errors.New("gitops failed")
	if _, err := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "dev"}); err == nil {
		t.Fatalf("gitops failure should fail")
	}
	env.svc.gitops = nil
	if _, err := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "dev"}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("missing gitops should fail, got %v", err)
	}
	env = newDeliveryEnv(t)
	freight = seedFreight(t, env)
	promoteFreightThrough(t, env, freight, "dev", "test", "staging")
	prod, _ := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "prod"})
	if _, err := env.svc.RejectPromotion(ctx, ApprovalInput{Actor: actor("usr_dev"), PromotionID: prod.ID}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("self reject should fail, got %v", err)
	}
	if _, err := env.svc.ApprovePromotion(ctx, ApprovalInput{Actor: actor("usr_ops"), PromotionID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing approve should fail, got %v", err)
	}
	if _, err := env.svc.RejectPromotion(ctx, ApprovalInput{Actor: actor("usr_ops"), PromotionID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing reject should fail, got %v", err)
	}
	dev, _ := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "dev"})
	if _, err := env.svc.ApprovePromotion(ctx, ApprovalInput{Actor: actor("usr_ops"), PromotionID: dev.ID}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("approve non-pending should fail, got %v", err)
	}
	if _, err := env.svc.RejectPromotion(ctx, ApprovalInput{Actor: actor("usr_ops"), PromotionID: dev.ID}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("reject non-pending should fail, got %v", err)
	}
	if _, err := env.svc.AbortPromotion(ctx, identityaccess.Subject{}, prod.ID); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("abort missing actor should fail, got %v", err)
	}
	if _, err := env.svc.CreateRollbackPromotion(ctx, CreateRollbackPromotionInput{Actor: actor("usr_dev"), TargetFreightID: "missing", TargetStageKey: "dev"}); shared.CodeOf(err) != shared.CodeNotFound {
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
	promoteFreightThrough(t, env, freight, "dev", "test", "staging")
	promo, _ := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "prod"})
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
	if _, err := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: identityaccess.Subject{}, FreightID: freight.ID, TargetStageKey: "dev"}); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("promotion missing actor should fail, got %v", err)
	}
	if _, err := env.svc.CreateRollbackPromotion(ctx, CreateRollbackPromotionInput{Actor: actor("usr_dev"), TargetFreightID: freight.ID, CurrentFreightID: "missing", TargetStageKey: "dev"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("rollback missing current freight should fail, got %v", err)
	}
	promoteFreightThrough(t, env, freight, "dev", "test", "staging")
	prod, _ := env.svc.CreatePromotion(ctx, CreatePromotionInput{Actor: actor("usr_dev"), FreightID: freight.ID, TargetStageKey: "prod"})
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
		"  UNIQUE KEY uk_releases_build_run_workload (build_run_id, workload_id),\n",
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
