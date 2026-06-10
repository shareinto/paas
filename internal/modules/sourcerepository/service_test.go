package sourcerepository

import (
	"bytes"
	"context"
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
	tenants  map[shared.ID]tenantproject.Tenant
}

func (q fakeProjectQuery) GetProject(_ context.Context, id shared.ID) (tenantproject.Project, error) {
	project, ok := q.projects[id]
	if !ok {
		return tenantproject.Project{}, shared.NewError(shared.CodeNotFound, "project not found")
	}
	return project, nil
}

func (q fakeProjectQuery) GetTenant(_ context.Context, id shared.ID) (tenantproject.Tenant, error) {
	tenant, ok := q.tenants[id]
	if !ok {
		return tenantproject.Tenant{}, shared.NewError(shared.CodeNotFound, "tenant not found")
	}
	return tenant, nil
}

type fakeMembershipQuery struct {
	members []tenantproject.TenantMember
	err     error
}

func (q fakeMembershipQuery) ListTenantMembers(context.Context, shared.ID) ([]tenantproject.TenantMember, error) {
	return q.members, q.err
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

type fakeGit struct {
	createCalls  []GitProjectSpec
	initCalls    []string
	protectCalls []string
	webhookCalls []string
	mirrorCalls  []GitMirrorSpec
	verifyCalls  []string
	syncCalls    [][]GitMemberAccess
	deleteCalls  []string
	files        []RepositoryFile
	tree         map[string][]RepositoryTreeItem
	createErr    error
	initErr      error
	protectErr   error
	webhookErr   error
	mirrorErr    error
	verifyErr    error
	listErr      error
	syncErr      error
}

type failingIDGenerator struct{}

func (failingIDGenerator) NewID(string) (shared.ID, error) {
	return "", errors.New("id generation failed")
}

type sequenceIDGenerator struct {
	ids []shared.ID
}

func (g *sequenceIDGenerator) NewID(string) (shared.ID, error) {
	if len(g.ids) == 0 {
		return "", errors.New("id generation failed")
	}
	id := g.ids[0]
	g.ids = g.ids[1:]
	return id, nil
}

func (g *fakeGit) CreateProject(_ context.Context, spec GitProjectSpec) (GitProject, error) {
	g.createCalls = append(g.createCalls, spec)
	if g.createErr != nil {
		return GitProject{}, g.createErr
	}
	id := "git-" + spec.RepositoryName
	return GitProject{ID: id, HTTPURL: "https://gitlab.example/" + spec.RepositoryName + ".git", SSHURL: "git@gitlab.example:" + spec.RepositoryName + ".git"}, nil
}

func (g *fakeGit) InitializeRepository(_ context.Context, gitProjectID string, _ string) error {
	g.initCalls = append(g.initCalls, gitProjectID)
	return g.initErr
}

func (g *fakeGit) DeleteProject(_ context.Context, gitProjectID string) error {
	g.deleteCalls = append(g.deleteCalls, gitProjectID)
	return nil
}

func (g *fakeGit) ProtectBranch(_ context.Context, gitProjectID string, _ string) error {
	g.protectCalls = append(g.protectCalls, gitProjectID)
	return g.protectErr
}

func (g *fakeGit) ConfigureWebhook(_ context.Context, gitProjectID string, _ string) error {
	g.webhookCalls = append(g.webhookCalls, gitProjectID)
	return g.webhookErr
}

func (g *fakeGit) SyncMembers(_ context.Context, _ string, members []GitMemberAccess) error {
	g.syncCalls = append(g.syncCalls, append([]GitMemberAccess(nil), members...))
	return g.syncErr
}

func (g *fakeGit) MirrorRepository(_ context.Context, spec GitMirrorSpec) error {
	g.mirrorCalls = append(g.mirrorCalls, spec)
	return g.mirrorErr
}

func (g *fakeGit) VerifyRepository(_ context.Context, gitProjectID string) error {
	g.verifyCalls = append(g.verifyCalls, gitProjectID)
	return g.verifyErr
}

func (g *fakeGit) ListFiles(_ context.Context, _ string, _ string) ([]RepositoryFile, error) {
	if g.listErr != nil {
		return nil, g.listErr
	}
	return append([]RepositoryFile(nil), g.files...), nil
}

func (g *fakeGit) ListTree(_ context.Context, _ string, _ string, path string) ([]RepositoryTreeItem, error) {
	if g.listErr != nil {
		return nil, g.listErr
	}
	return append([]RepositoryTreeItem(nil), g.tree[path]...), nil
}

func (g *fakeGit) ListBranches(context.Context, string) ([]RepositoryBranch, error) {
	if g.listErr != nil {
		return nil, g.listErr
	}
	return []RepositoryBranch{{Name: "feature/order"}, {Name: "main"}}, nil
}

type testEnv struct {
	svc        *Service
	repo       Repository
	git        *fakeGit
	permission *recordingPermission
	audit      *recordingAudit
	events     *recordingPublisher
}

func newTestRepository(t *testing.T) Repository {
	t.Helper()
	repo, err := NewMySQLRepository(context.Background(), testsupport.MySQLDB(t, Migrations...))
	if err != nil {
		t.Fatalf("NewMySQLRepository() error = %v", err)
	}
	return repo
}

func newTestEnv(t *testing.T) testEnv {
	t.Helper()
	repo := newTestRepository(t)
	git := &fakeGit{files: []RepositoryFile{
		{Path: "services/user-api/pom.xml"},
		{Path: "services/user-api/target/user-api.jar"},
		{Path: "apps/legacy-web/pom.xml"},
		{Path: "apps/legacy-web/target/legacy-web.war"},
	}, tree: map[string][]RepositoryTreeItem{
		"": {
			{Name: "README.md", Path: "README.md", Type: "blob"},
			{Name: "services", Path: "services", Type: "tree"},
			{Name: "apps", Path: "apps", Type: "tree"},
		},
		"services": {
			{Name: "user-api", Path: "services/user-api", Type: "tree"},
			{Name: "shared.md", Path: "services/shared.md", Type: "blob"},
		},
	}}
	permission := &recordingPermission{}
	audit := &recordingAudit{}
	events := &recordingPublisher{}
	svc := NewService(Options{
		Repository: repo,
		Git:        git,
		ProjectQuery: fakeProjectQuery{projects: map[shared.ID]tenantproject.Project{
			"project_payment": {ID: "project_payment", TenantID: "tenant_a", Name: "payment"},
		}, tenants: map[shared.ID]tenantproject.Tenant{
			"tenant_a": {ID: "tenant_a", Name: "rnd"},
		}},
		MembershipQuery: fakeMembershipQuery{members: []tenantproject.TenantMember{
			{TenantID: "tenant_a", UserID: "usr_owner", RoleID: identityaccess.RoleTenantOwner},
			{TenantID: "tenant_a", UserID: "usr_admin", RoleID: identityaccess.RoleProjectAdmin},
			{TenantID: "tenant_a", UserID: "usr_dev", RoleID: identityaccess.RoleDeveloper},
			{TenantID: "tenant_a", UserID: "usr_viewer", RoleID: identityaccess.RoleViewer},
			{TenantID: "tenant_a", UserID: "usr_auditor", RoleID: identityaccess.RoleSecurityAuditor},
		}},
		PermissionChecker:  permission,
		Audit:              audit,
		EventPublisher:     events,
		IDGenerator:        testutil.NewFakeIDGenerator(1),
		Clock:              testutil.NewFakeClock(time.Date(2026, 5, 30, 4, 0, 0, 0, time.UTC)),
		WebhookCallbackURL: "https://paas.example/api/gitlab/hooks",
	})
	return testEnv{svc: svc, repo: repo, git: git, permission: permission, audit: audit, events: events}
}

func actor() identityaccess.Subject {
	return identityaccess.Subject{Type: identityaccess.SubjectUser, ID: "usr_actor"}
}

func TestCreateSourceRepositoryCallsGitAndPublishesEvent(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()

	repository, err := env.svc.CreateSourceRepository(ctx, CreateSourceRepositoryInput{Actor: actor(), ProjectID: "project_payment", Name: "User-API", DisplayName: "用户接口", Description: " core "})
	if err != nil {
		t.Fatalf("CreateSourceRepository() error = %v", err)
	}
	if repository.ID != "repo_1" || repository.Name != "user-api" || repository.Status != RepositoryStatusReady || repository.DefaultBranch != "main" {
		t.Fatalf("unexpected repository: %+v", repository)
	}
	if repository.GitProjectID != "git-user-api" || !strings.Contains(repository.HTTPURL, "user-api.git") {
		t.Fatalf("git project should be persisted: %+v", repository)
	}
	if len(env.git.createCalls) != 1 || len(env.git.initCalls) != 0 || len(env.git.protectCalls) != 1 || len(env.git.webhookCalls) != 1 {
		t.Fatalf("expected create/protect/webhook calls without repository initialization, got %+v", env.git)
	}
	createSpec := env.git.createCalls[0]
	if createSpec.TenantID != "tenant_a" || createSpec.TenantName != "rnd" || createSpec.ProjectName != "payment" || createSpec.RepositoryName != "user-api" {
		t.Fatalf("git create spec should use tenant name for namespace path while preserving ids, got %+v", createSpec)
	}
	if len(env.permission.calls) != 1 || env.permission.calls[0].resource.ProjectID != "project_payment" || env.permission.calls[0].action != "project:update" {
		t.Fatalf("unexpected permission calls: %+v", env.permission.calls)
	}
	if len(env.events.events) != 1 || env.events.events[0].EventType != "SourceRepositoryCreated" {
		t.Fatalf("expected SourceRepositoryCreated, got %+v", env.events.events)
	}
	if len(env.audit.events) != 1 || env.audit.events[0].Action != "source_repository.create" {
		t.Fatalf("expected repository audit, got %+v", env.audit.events)
	}
	found, err := env.repo.FindSourceRepositoryByProjectAndName(ctx, "project_payment", "user-api")
	if err != nil || found.ID != repository.ID {
		t.Fatalf("FindSourceRepositoryByProjectAndName() = %+v, %v", found, err)
	}
}

func TestDeleteSourceRepositoryRemovesGitProjectAndBlocksAssociatedApplications(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	repository, err := env.svc.CreateSourceRepository(ctx, CreateSourceRepositoryInput{Actor: actor(), ProjectID: "project_payment", Name: "delete-api"})
	if err != nil {
		t.Fatalf("CreateSourceRepository() error = %v", err)
	}
	if err := env.repo.SetAssociatedApplications(ctx, repository.ID, []AssociatedApplication{{ID: "app_1", Name: "order-api"}}); err != nil {
		t.Fatalf("SetAssociatedApplications() error = %v", err)
	}
	if err := env.svc.DeleteSourceRepository(ctx, DeleteSourceRepositoryInput{Actor: actor(), SourceRepositoryID: repository.ID}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("associated applications should block delete, got %v", err)
	}
	if err := env.repo.SetAssociatedApplications(ctx, repository.ID, nil); err != nil {
		t.Fatalf("SetAssociatedApplications(nil) error = %v", err)
	}
	if err := env.svc.DeleteSourceRepository(ctx, DeleteSourceRepositoryInput{Actor: actor(), SourceRepositoryID: repository.ID}); err != nil {
		t.Fatalf("DeleteSourceRepository() error = %v", err)
	}
	if len(env.git.deleteCalls) != 1 || env.git.deleteCalls[0] != repository.GitProjectID {
		t.Fatalf("delete git calls = %#v", env.git.deleteCalls)
	}
	if _, err := env.repo.GetSourceRepository(ctx, repository.ID); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("repository should be removed, got %v", err)
	}
}

func TestCreateSourceRepositoryValidationAndFailures(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	if _, err := env.svc.CreateSourceRepository(ctx, CreateSourceRepositoryInput{Actor: actor(), ProjectID: "", Name: "api"}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("missing project should fail, got %v", err)
	}
	if _, err := env.svc.CreateSourceRepository(ctx, CreateSourceRepositoryInput{Actor: actor(), ProjectID: "missing", Name: "api"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing project should fail, got %v", err)
	}
	if _, err := env.svc.CreateSourceRepository(ctx, CreateSourceRepositoryInput{Actor: identityaccess.Subject{}, ProjectID: "project_payment", Name: "api"}); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("missing actor should fail, got %v", err)
	}
	env.permission.err = shared.NewError(shared.CodePermissionDenied, "denied")
	if _, err := env.svc.CreateSourceRepository(ctx, CreateSourceRepositoryInput{Actor: actor(), ProjectID: "project_payment", Name: "api"}); shared.CodeOf(err) != shared.CodePermissionDenied {
		t.Fatalf("permission denial should fail, got %v", err)
	}
	env.permission.err = nil
	if _, err := env.svc.CreateSourceRepository(ctx, CreateSourceRepositoryInput{Actor: actor(), ProjectID: "project_payment", Name: "1bad"}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("invalid name should fail, got %v", err)
	}
	env.git.createErr = errors.New("gitlab unavailable")
	if _, err := env.svc.CreateSourceRepository(ctx, CreateSourceRepositoryInput{Actor: actor(), ProjectID: "project_payment", Name: "api"}); err == nil {
		t.Fatalf("git create failure should fail")
	}
	env.git.createErr = nil
	if _, err := env.svc.CreateSourceRepository(ctx, CreateSourceRepositoryInput{Actor: actor(), ProjectID: "project_payment", Name: "api-ok"}); err != nil {
		t.Fatalf("CreateSourceRepository() error = %v", err)
	}
	if _, err := env.svc.CreateSourceRepository(ctx, CreateSourceRepositoryInput{Actor: actor(), ProjectID: "project_payment", Name: "api-ok"}); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate repository should conflict, got %v", err)
	}
}

func TestPermissionMappingAndSync(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	repository, err := env.svc.CreateSourceRepository(ctx, CreateSourceRepositoryInput{Actor: actor(), ProjectID: "project_payment", Name: "api"})
	if err != nil {
		t.Fatalf("CreateSourceRepository() error = %v", err)
	}
	job, err := env.svc.SyncRepositoryPermissions(ctx, actor(), repository.ID)
	if err != nil {
		t.Fatalf("SyncRepositoryPermissions() error = %v", err)
	}
	if job.Status != PermissionSyncSucceeded {
		t.Fatalf("unexpected sync job: %+v", job)
	}
	if len(env.git.syncCalls) != 1 || len(env.git.syncCalls[0]) != 4 {
		t.Fatalf("expected four mapped members, got %+v", env.git.syncCalls)
	}
	expected := map[shared.ID]GitAccessLevel{"usr_owner": GitAccessOwner, "usr_admin": GitAccessMaintainer, "usr_dev": GitAccessDeveloper, "usr_viewer": GitAccessReporter}
	for _, access := range env.git.syncCalls[0] {
		if expected[access.UserID] != access.Access {
			t.Fatalf("unexpected access mapping for %+v", access)
		}
	}
	if _, ok := MapRoleToGitAccess(identityaccess.RoleOperator); ok {
		t.Fatalf("operator should not map to GitLab access by default")
	}

	env.git.syncErr = errors.New("sync failed")
	failed, err := env.svc.SyncRepositoryPermissions(ctx, actor(), repository.ID)
	if err == nil || failed.Status != PermissionSyncFailed || !strings.Contains(failed.ErrorMessage, "sync failed") {
		t.Fatalf("sync failure should persist failed job, got %+v, %v", failed, err)
	}
}

func TestRepositoryMigrationLifecycleAndSuggestions(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	migration, err := env.svc.CreateRepositoryMigration(ctx, CreateRepositoryMigrationInput{Actor: actor(), ProjectID: "project_payment", Name: "legacy-web", SourceURL: "https://example.com/legacy.git"})
	if err != nil {
		t.Fatalf("CreateRepositoryMigration() error = %v", err)
	}
	if migration.ID != "repo_migration_2" || migration.Status != MigrationPending {
		t.Fatalf("unexpected migration: %+v", migration)
	}
	processed, suggestions, err := env.svc.ProcessRepositoryMigration(ctx, migration.ID)
	if err != nil {
		t.Fatalf("ProcessRepositoryMigration() error = %v", err)
	}
	if processed.Status != MigrationSucceeded || processed.CompletedAt == nil {
		t.Fatalf("migration should succeed, got %+v", processed)
	}
	repository, err := env.svc.GetSourceRepository(ctx, processed.SourceRepositoryID)
	if err != nil {
		t.Fatalf("GetSourceRepository() error = %v", err)
	}
	if repository.Status != RepositoryStatusReady || repository.GitProjectID == "" {
		t.Fatalf("repository should be ready after migration, got %+v", repository)
	}
	if len(suggestions) != 2 {
		t.Fatalf("expected jar and war suggestions, got %+v", suggestions)
	}
	if suggestions[0].SourcePath != "apps/legacy-web" || suggestions[0].ArtifactCopyCommand != `cp -ar target/legacy-web.war "$PAAS_ARTIFACT_OUTPUT/app.war"` || !containsString(suggestions[0].Evidence, "apps/legacy-web/target/legacy-web.war") {
		t.Fatalf("unexpected first sorted suggestion: %+v", suggestions[0])
	}
	if len(env.git.mirrorCalls) != 1 || env.git.mirrorCalls[0].SourceURL != "https://example.com/legacy.git" {
		t.Fatalf("expected mirror call, got %+v", env.git.mirrorCalls)
	}
	page, err := env.repo.ListMigrationsByRepository(ctx, processed.SourceRepositoryID, shared.PageRequest{})
	if err != nil || page.Total != 1 {
		t.Fatalf("ListMigrationsByRepository() = %+v, %v", page, err)
	}
}

func TestMigrationCancelRetryAndFailure(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	migration, err := env.svc.CreateRepositoryMigration(ctx, CreateRepositoryMigrationInput{Actor: actor(), ProjectID: "project_payment", Name: "api", SourceURL: "https://example.com/api.git"})
	if err != nil {
		t.Fatalf("CreateRepositoryMigration() error = %v", err)
	}
	canceled, err := env.svc.CancelRepositoryMigration(ctx, actor(), migration.ID)
	if err != nil {
		t.Fatalf("CancelRepositoryMigration() error = %v", err)
	}
	if canceled.Status != MigrationCanceled {
		t.Fatalf("unexpected canceled migration: %+v", canceled)
	}
	if _, err := env.svc.CancelRepositoryMigration(ctx, actor(), canceled.ID); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("canceling terminal migration should fail, got %v", err)
	}

	failing := newTestEnv(t)
	failing.git.mirrorErr = errors.New("mirror rejected")
	failedMigration, err := failing.svc.CreateRepositoryMigration(ctx, CreateRepositoryMigrationInput{Actor: actor(), ProjectID: "project_payment", Name: "mirror-fail", SourceURL: "https://example.com/fail.git"})
	if err != nil {
		t.Fatalf("CreateRepositoryMigration() error = %v", err)
	}
	processed, _, err := failing.svc.ProcessRepositoryMigration(ctx, failedMigration.ID)
	if err == nil || processed.Status != MigrationFailed || !strings.Contains(processed.ErrorMessage, "mirror rejected") {
		t.Fatalf("process should fail and persist status, got %+v, %v", processed, err)
	}
	retried, err := failing.svc.RetryRepositoryMigration(ctx, actor(), failedMigration.ID)
	if err != nil {
		t.Fatalf("RetryRepositoryMigration() error = %v", err)
	}
	if retried.Status != MigrationPending {
		t.Fatalf("retry should reset status, got %+v", retried)
	}
	if _, err := failing.svc.RetryRepositoryMigration(ctx, actor(), retried.ID); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("retrying non-failed migration should fail, got %v", err)
	}
}

func TestGenerateBuildSpecSuggestions(t *testing.T) {
	files := []RepositoryFile{
		{Path: "pom.xml"},
		{Path: "target/root.jar"},
		{Path: "service/build.gradle"},
		{Path: "service/target/service.war"},
		{Path: "/absolute/pom.xml"},
		{Path: "../escape/pom.xml"},
	}
	suggestions := GenerateJavaBuildSpecSuggestions(files)
	if len(suggestions) != 2 {
		t.Fatalf("expected two suggestions, got %+v", suggestions)
	}
	if suggestions[0].SourcePath != "." || suggestions[0].BuildCommand != "mvn clean package -DskipTests" {
		t.Fatalf("unexpected root suggestion: %+v", suggestions[0])
	}
	if suggestions[1].SourcePath != "service" || suggestions[1].BuildCommand != "./gradlew clean build -x test" {
		t.Fatalf("unexpected gradle suggestion: %+v", suggestions[1])
	}
	if err := ValidateBuildSpecSuggestion(suggestions[0]); err != nil {
		t.Fatalf("valid suggestion should pass: %v", err)
	}
	bad := suggestions[0]
	bad.BuildCommand = " "
	if shared.CodeOf(ValidateBuildSpecSuggestion(bad)) != shared.CodeInvalidArgument {
		t.Fatalf("empty build command should fail")
	}
}

func TestRepositoryMemoryEdges(t *testing.T) {
	repo := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 5, 30, 5, 0, 0, 0, time.UTC)
	source := SourceRepository{ID: "repo_1", TenantID: "tenant_1", ProjectID: "project_1", Name: "api", Status: RepositoryStatusReady, CreatedAt: now, UpdatedAt: now}
	if err := repo.CreateSourceRepository(ctx, source); err != nil {
		t.Fatalf("CreateSourceRepository() error = %v", err)
	}
	if err := repo.CreateSourceRepository(ctx, source); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate create should conflict, got %v", err)
	}
	renamed := source
	renamed.Name = "api-v2"
	if err := repo.UpdateSourceRepository(ctx, renamed); err != nil {
		t.Fatalf("UpdateSourceRepository() error = %v", err)
	}
	renamed.ProjectID = "project_2"
	if shared.CodeOf(repo.UpdateSourceRepository(ctx, renamed)) != shared.CodeInvalidArgument {
		t.Fatalf("ownership change should fail")
	}
	if _, err := repo.GetSourceRepository(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing repository should fail, got %v", err)
	}
	if _, err := repo.FindSourceRepositoryByProjectAndName(ctx, source.ProjectID, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing repository name should fail, got %v", err)
	}
	page, err := repo.ListSourceRepositoriesByProject(ctx, source.ProjectID, shared.PageRequest{Page: 3, PageSize: 10})
	if err != nil || len(page.Items) != 0 || page.Total != 1 {
		t.Fatalf("out of range page should be empty with total, got %+v, %v", page, err)
	}
	if err := repo.SetAssociatedApplications(ctx, source.ID, []AssociatedApplication{{ID: "app_1", Name: "zeta"}, {ID: "app_2", Name: "alpha"}}); err != nil {
		t.Fatalf("SetAssociatedApplications() error = %v", err)
	}
	applications, err := repo.ListAssociatedApplications(ctx, source.ID)
	if err != nil || applications[0].Name != "alpha" {
		t.Fatalf("applications should be sorted, got %+v, %v", applications, err)
	}
	if _, err := repo.ListAssociatedApplications(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing associated apps should fail, got %v", err)
	}
	if err := repo.SetAssociatedApplications(ctx, "missing", nil); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("setting associated apps for missing repo should fail, got %v", err)
	}
	if err := repo.CreateMigration(ctx, RepositoryMigration{ID: "migration_1", SourceRepositoryID: source.ID, Status: MigrationPending, CreatedAt: now}); err != nil {
		t.Fatalf("CreateMigration() error = %v", err)
	}
	if err := repo.CreateMigration(ctx, RepositoryMigration{ID: "migration_2", SourceRepositoryID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("migration for missing repository should fail, got %v", err)
	}
	if err := repo.CreateMigration(ctx, RepositoryMigration{ID: "migration_1", SourceRepositoryID: source.ID}); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate migration should conflict, got %v", err)
	}
	if err := repo.UpdateMigration(ctx, RepositoryMigration{ID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing migration update should fail, got %v", err)
	}
	if _, err := repo.GetMigration(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing migration get should fail, got %v", err)
	}
	if _, err := repo.ListMigrationsByRepository(ctx, "missing", shared.PageRequest{}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("listing migrations for missing repo should fail, got %v", err)
	}
	if err := repo.CreatePermissionSyncJob(ctx, RepositoryPermissionSyncJob{ID: "job_1", SourceRepositoryID: source.ID, Status: PermissionSyncPending}); err != nil {
		t.Fatalf("CreatePermissionSyncJob() error = %v", err)
	}
	if err := repo.CreatePermissionSyncJob(ctx, RepositoryPermissionSyncJob{ID: "job_2", SourceRepositoryID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("permission sync for missing repository should fail, got %v", err)
	}
	if err := repo.CreatePermissionSyncJob(ctx, RepositoryPermissionSyncJob{ID: "job_1", SourceRepositoryID: source.ID}); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate permission sync job should conflict, got %v", err)
	}
	if got, err := repo.GetPermissionSyncJob(ctx, "job_1"); err != nil || got.ID != "job_1" {
		t.Fatalf("GetPermissionSyncJob() = %+v, %v", got, err)
	}
	if _, err := repo.GetPermissionSyncJob(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing permission job should fail, got %v", err)
	}
	if err := repo.UpdatePermissionSyncJob(ctx, RepositoryPermissionSyncJob{ID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing job update should fail, got %v", err)
	}
}

func TestHTTPHandler(t *testing.T) {
	env := newTestEnv(t)
	mux := http.NewServeMux()
	NewHandler(env.svc).Register(mux)
	body := bytes.NewBufferString(`{"actor":{"Type":"user","ID":"usr_actor"},"project_id":"project_payment","name":"api"}`)
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/source-repositories", body))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/projects/project_payment/source-repositories?page=1&page_size=10", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"total":1`) {
		t.Fatalf("list response = %d, %s", recorder.Code, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/source-repositories/repo_1/scan/java", bytes.NewBufferString(`{"ref":"main"}`)))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "target/user-api.jar") {
		t.Fatalf("scan response = %d, %s", recorder.Code, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/source-repositories/repo_1/tree?ref=main", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"path":"apps"`) || !strings.Contains(recorder.Body.String(), `"path":"services"`) {
		t.Fatalf("tree response = %d, %s", recorder.Code, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/source-repositories/repo_1/tree?ref=main&path=services", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"path":"services/user-api"`) {
		t.Fatalf("child tree response = %d, %s", recorder.Code, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/source-repositories/repo_1/branches", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"name":"main"`) || !strings.Contains(recorder.Body.String(), `"default":true`) {
		t.Fatalf("branches response = %d, %s", recorder.Code, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/source-repositories/repo_1/tree?path=../secret", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid tree path response = %d, %s", recorder.Code, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/source-repositories/repo_1/applications", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"items"`) {
		t.Fatalf("applications response = %d, %s", recorder.Code, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/source-repositories/repo_1/permission-sync", bytes.NewBufferString(`{"actor":{"Type":"user","ID":"usr_actor"}}`)))
	if recorder.Code != http.StatusCreated || !strings.Contains(recorder.Body.String(), "succeeded") {
		t.Fatalf("permission sync response = %d, %s", recorder.Code, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/repository-migrations", bytes.NewBufferString(`{"actor":{"Type":"user","ID":"usr_actor"},"project_id":"project_payment","name":"legacy","source_url":"https://example.com/legacy.git"}`)))
	if recorder.Code != http.StatusCreated || !strings.Contains(recorder.Body.String(), `"pending"`) {
		t.Fatalf("migration create response = %d, %s", recorder.Code, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/repository-migrations/repo_migration_5", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "repo_migration_5") {
		t.Fatalf("migration get response = %d, %s", recorder.Code, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/repository-migrations/repo_migration_5/process", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "build_spec_suggestions") {
		t.Fatalf("migration process response = %d, %s", recorder.Code, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/repository-migrations", bytes.NewBufferString(`{"actor":{"Type":"user","ID":"usr_actor"},"project_id":"project_payment","name":"cancel-me","source_url":"https://example.com/cancel.git"}`)))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("second migration create response = %d, %s", recorder.Code, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/repository-migrations/repo_migration_8/cancel", bytes.NewBufferString(`{"actor":{"Type":"user","ID":"usr_actor"}}`)))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "canceled") {
		t.Fatalf("migration cancel response = %d, %s", recorder.Code, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/source-repositories/missing", nil))
	if recorder.Code != http.StatusNotFound || !strings.Contains(recorder.Body.String(), "请求处理失败") {
		t.Fatalf("error response = %d, %s", recorder.Code, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/source-repositories", bytes.NewBufferString(`{bad json`)))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid json response = %d, %s", recorder.Code, recorder.Body.String())
	}
}

func TestHTTPRetryMigrationEndpoint(t *testing.T) {
	env := newTestEnv(t)
	mux := http.NewServeMux()
	NewHandler(env.svc).Register(mux)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/repository-migrations", bytes.NewBufferString(`{"actor":{"Type":"user","ID":"usr_actor"},"project_id":"project_payment","name":"retry-me","source_url":"https://example.com/retry.git"}`)))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("migration create response = %d, %s", recorder.Code, recorder.Body.String())
	}
	env.git.mirrorErr = errors.New("mirror failed")
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/repository-migrations/repo_migration_2/process", nil))
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("migration process failure response = %d, %s", recorder.Code, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/repository-migrations/repo_migration_2/retry", bytes.NewBufferString(`{"actor":{"Type":"user","ID":"usr_actor"}}`)))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"pending"`) {
		t.Fatalf("migration retry response = %d, %s", recorder.Code, recorder.Body.String())
	}
	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/repository-migrations/repo_migration_2/retry", bytes.NewBufferString(`{bad`)))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("bad retry json response = %d, %s", recorder.Code, recorder.Body.String())
	}
}

func TestHTTPErrorBranches(t *testing.T) {
	env := newTestEnv(t)
	mux := http.NewServeMux()
	NewHandler(env.svc).Register(mux)

	cases := []struct {
		method string
		path   string
		body   string
		status int
	}{
		{method: http.MethodGet, path: "/api/projects/missing/source-repositories", status: http.StatusNotFound},
		{method: http.MethodGet, path: "/api/source-repositories/missing/applications", status: http.StatusNotFound},
		{method: http.MethodGet, path: "/api/source-repositories/missing/tree", status: http.StatusNotFound},
		{method: http.MethodGet, path: "/api/source-repositories/missing/branches", status: http.StatusNotFound},
		{method: http.MethodPost, path: "/api/source-repositories/missing/scan/java", body: `{}`, status: http.StatusNotFound},
		{method: http.MethodPost, path: "/api/source-repositories/missing/permission-sync", body: `{bad`, status: http.StatusBadRequest},
		{method: http.MethodPost, path: "/api/repository-migrations", body: `{bad`, status: http.StatusBadRequest},
		{method: http.MethodGet, path: "/api/repository-migrations/missing", status: http.StatusNotFound},
		{method: http.MethodPost, path: "/api/repository-migrations/missing/cancel", body: `{bad`, status: http.StatusBadRequest},
		{method: http.MethodPost, path: "/api/repository-migrations/missing/cancel", body: `{"actor":{"Type":"user","ID":"usr_actor"}}`, status: http.StatusNotFound},
	}
	for _, tc := range cases {
		var body *bytes.Buffer
		if tc.body == "" {
			body = bytes.NewBuffer(nil)
		} else {
			body = bytes.NewBufferString(tc.body)
		}
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.path, body))
		if recorder.Code != tc.status {
			t.Fatalf("%s %s status = %d, want %d, body = %s", tc.method, tc.path, recorder.Code, tc.status, recorder.Body.String())
		}
	}
}

func TestNoopAndDefaultServiceFailures(t *testing.T) {
	ctx := context.Background()
	if err := (NoopAuditLogger{}).Log(ctx, AuditEvent{}); err != nil {
		t.Fatalf("noop audit should not fail: %v", err)
	}
	if err := (NoopEventPublisher{}).Publish(ctx, shared.DomainEvent{}); err != nil {
		t.Fatalf("noop publisher should not fail: %v", err)
	}
	svc := NewService(Options{Repository: newTestRepository(t)})
	if _, err := svc.CreateSourceRepository(ctx, CreateSourceRepositoryInput{Actor: actor(), ProjectID: "project_payment", Name: "api"}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("missing project query should fail, got %v", err)
	}
}

func TestServiceAdditionalErrorBranches(t *testing.T) {
	ctx := context.Background()
	env := newTestEnv(t)
	repository, err := env.svc.CreateSourceRepository(ctx, CreateSourceRepositoryInput{Actor: actor(), ProjectID: "project_payment", Name: "develop-repo", DefaultBranch: "develop"})
	if err != nil {
		t.Fatalf("CreateSourceRepository() error = %v", err)
	}
	if repository.DefaultBranch != "develop" {
		t.Fatalf("explicit default branch should be kept, got %+v", repository)
	}
	env = newTestEnv(t)
	env.git.protectErr = errors.New("protect failed")
	if _, err := env.svc.CreateSourceRepository(ctx, CreateSourceRepositoryInput{Actor: actor(), ProjectID: "project_payment", Name: "protect-fail"}); err == nil {
		t.Fatalf("protect failure should fail create")
	}
	env = newTestEnv(t)
	env.git.webhookErr = errors.New("webhook failed")
	if _, err := env.svc.CreateSourceRepository(ctx, CreateSourceRepositoryInput{Actor: actor(), ProjectID: "project_payment", Name: "webhook-fail"}); err == nil {
		t.Fatalf("webhook failure should fail create")
	}
	env = newTestEnv(t)
	env.events.err = errors.New("event bus down")
	if _, err := env.svc.CreateSourceRepository(ctx, CreateSourceRepositoryInput{Actor: actor(), ProjectID: "project_payment", Name: "event-fail"}); err == nil {
		t.Fatalf("event publish failure should fail create")
	}
	env = newTestEnv(t)
	env.svc.ids = failingIDGenerator{}
	if _, err := env.svc.CreateSourceRepository(ctx, CreateSourceRepositoryInput{Actor: actor(), ProjectID: "project_payment", Name: "id-fail"}); err == nil {
		t.Fatalf("id generation failure should fail create")
	}
	env = newTestEnv(t)
	if _, err := env.svc.CreateRepositoryMigration(ctx, CreateRepositoryMigrationInput{Actor: actor(), ProjectID: "project_payment", Name: "bad"}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("missing source_url should fail, got %v", err)
	}
	if _, err := env.svc.CreateRepositoryMigration(ctx, CreateRepositoryMigrationInput{Actor: actor(), ProjectID: "project_payment", Name: "1bad", SourceURL: "https://example.com/repo.git"}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("bad migration name should fail, got %v", err)
	}
	env.events.err = errors.New("event bus down")
	if _, err := env.svc.CreateRepositoryMigration(ctx, CreateRepositoryMigrationInput{Actor: actor(), ProjectID: "project_payment", Name: "event-migration", SourceURL: "https://example.com/repo.git"}); err == nil {
		t.Fatalf("event publish failure should fail migration create")
	}
	env.events.err = nil
	if _, _, err := NewService(Options{Repository: env.repo}).ProcessRepositoryMigration(ctx, "missing"); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("missing git should fail process, got %v", err)
	}
	repository, err = env.svc.CreateSourceRepository(ctx, CreateSourceRepositoryInput{Actor: actor(), ProjectID: "project_payment", Name: "scan"})
	if err != nil {
		t.Fatalf("CreateSourceRepository() error = %v", err)
	}
	env.git.listErr = errors.New("list failed")
	if _, err := env.svc.GenerateBuildSpecSuggestions(ctx, repository.ID, ""); err == nil {
		t.Fatalf("list files failure should fail suggestion generation")
	}
	if _, err := NewService(Options{Repository: env.repo}).GenerateBuildSpecSuggestions(ctx, repository.ID, ""); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("missing git should fail suggestion generation, got %v", err)
	}
}

func TestIDGenerationFailuresAfterPrimaryID(t *testing.T) {
	ctx := context.Background()
	env := newTestEnv(t)
	env.svc.ids = &sequenceIDGenerator{ids: []shared.ID{"repo_custom"}}
	if _, err := env.svc.CreateSourceRepository(ctx, CreateSourceRepositoryInput{Actor: actor(), ProjectID: "project_payment", Name: "event-id-fail"}); err == nil {
		t.Fatalf("event id failure should fail source repository create")
	}

	env = newTestEnv(t)
	env.svc.ids = &sequenceIDGenerator{ids: []shared.ID{"repo_custom"}}
	if _, err := env.svc.CreateRepositoryMigration(ctx, CreateRepositoryMigrationInput{Actor: actor(), ProjectID: "project_payment", Name: "migration-id-fail", SourceURL: "https://example.com/repo.git"}); err == nil {
		t.Fatalf("second migration id failure should fail migration create")
	}

	env = newTestEnv(t)
	env.svc.ids = &sequenceIDGenerator{ids: []shared.ID{"repo_custom", "migration_custom"}}
	if _, err := env.svc.CreateRepositoryMigration(ctx, CreateRepositoryMigrationInput{Actor: actor(), ProjectID: "project_payment", Name: "migration-event-id-fail", SourceURL: "https://example.com/repo.git"}); err == nil {
		t.Fatalf("event id failure should fail migration create")
	}
}

func TestProcessMigrationFailureBranches(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*fakeGit)
		message string
	}{
		{name: "protect-fail", mutate: func(g *fakeGit) { g.protectErr = errors.New("protect failed") }, message: "protect failed"},
		{name: "webhook-fail", mutate: func(g *fakeGit) { g.webhookErr = errors.New("webhook failed") }, message: "webhook failed"},
		{name: "verify-fail", mutate: func(g *fakeGit) { g.verifyErr = errors.New("verify failed") }, message: "verify failed"},
		{name: "list-fail", mutate: func(g *fakeGit) { g.listErr = errors.New("list failed") }, message: "list failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)
			tc.mutate(env.git)
			ctx := context.Background()
			migration, err := env.svc.CreateRepositoryMigration(ctx, CreateRepositoryMigrationInput{Actor: actor(), ProjectID: "project_payment", Name: tc.name, SourceURL: "https://example.com/" + tc.name + ".git"})
			if err != nil {
				t.Fatalf("CreateRepositoryMigration() error = %v", err)
			}
			processed, _, err := env.svc.ProcessRepositoryMigration(ctx, migration.ID)
			if err == nil || processed.Status != MigrationFailed || !strings.Contains(processed.ErrorMessage, tc.message) {
				t.Fatalf("expected failed migration containing %q, got %+v, %v", tc.message, processed, err)
			}
			repository, getErr := env.svc.GetSourceRepository(ctx, processed.SourceRepositoryID)
			if getErr != nil || repository.Status != RepositoryStatusFailed {
				t.Fatalf("repository should be marked failed, got %+v, %v", repository, getErr)
			}
		})
	}
}

func TestProcessMigrationTerminalAndInvalidTransition(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	migration, err := env.svc.CreateRepositoryMigration(ctx, CreateRepositoryMigrationInput{Actor: actor(), ProjectID: "project_payment", Name: "terminal", SourceURL: "https://example.com/terminal.git"})
	if err != nil {
		t.Fatalf("CreateRepositoryMigration() error = %v", err)
	}
	succeeded, _, err := env.svc.ProcessRepositoryMigration(ctx, migration.ID)
	if err != nil {
		t.Fatalf("ProcessRepositoryMigration() error = %v", err)
	}
	again, suggestions, err := env.svc.ProcessRepositoryMigration(ctx, migration.ID)
	if err != nil || again.Status != MigrationSucceeded || suggestions != nil {
		t.Fatalf("terminal migration should be a no-op, got %+v, %+v, %v", again, suggestions, err)
	}
	if _, err := env.svc.transitionMigration(ctx, succeeded, MigrationPending, ""); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("invalid transition should fail, got %v", err)
	}
	if _, _, err := env.svc.ProcessRepositoryMigration(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing migration should fail, got %v", err)
	}
}

func TestSyncRepositoryPermissionsPortFailures(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	repository, err := env.svc.CreateSourceRepository(ctx, CreateSourceRepositoryInput{Actor: actor(), ProjectID: "project_payment", Name: "sync-port"})
	if err != nil {
		t.Fatalf("CreateSourceRepository() error = %v", err)
	}
	noPorts := NewService(Options{Repository: env.repo, ProjectQuery: env.svc.projects, PermissionChecker: env.permission})
	if _, err := noPorts.SyncRepositoryPermissions(ctx, actor(), repository.ID); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("missing git/membership ports should fail, got %v", err)
	}
	membershipFail := NewService(Options{
		Repository:        env.repo,
		Git:               env.git,
		ProjectQuery:      env.svc.projects,
		MembershipQuery:   fakeMembershipQuery{err: errors.New("members unavailable")},
		PermissionChecker: env.permission,
		IDGenerator:       testutil.NewFakeIDGenerator(100),
		Clock:             testutil.NewFakeClock(time.Date(2026, 5, 30, 6, 0, 0, 0, time.UTC)),
	})
	job, err := membershipFail.SyncRepositoryPermissions(ctx, actor(), repository.ID)
	if err == nil || job.Status != PermissionSyncFailed || !strings.Contains(job.ErrorMessage, "members unavailable") {
		t.Fatalf("membership failure should persist failed job, got %+v, %v", job, err)
	}
}

func TestScannerAdditionalValidationBranches(t *testing.T) {
	suggestion := BuildSpecSuggestion{SourcePath: ".", BuildCommand: "mvn package", RuntimeBaseImage: "base"}
	suggestion.BuildCommand = ""
	if shared.CodeOf(ValidateBuildSpecSuggestion(suggestion)) != shared.CodeInvalidArgument {
		t.Fatalf("empty build command should fail")
	}
	suggestion.BuildCommand = "mvn package"
	suggestion.RuntimeBaseImage = ""
	if shared.CodeOf(ValidateBuildSpecSuggestion(suggestion)) != shared.CodeInvalidArgument {
		t.Fatalf("missing runtime image should fail")
	}
	if got := GenerateJavaBuildSpecSuggestions([]RepositoryFile{{Path: "lib/pom.xml"}}); len(got) != 1 || got[0].SourcePath != "lib" {
		t.Fatalf("build file without artifact should produce conservative suggestion, got %+v", got)
	}
	if got := GenerateJavaBuildSpecSuggestions([]RepositoryFile{{Path: "README.md"}}); len(got) != 0 {
		t.Fatalf("non-java repo should produce no suggestions, got %+v", got)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
