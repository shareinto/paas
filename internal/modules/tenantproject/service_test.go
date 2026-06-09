package tenantproject

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/shared"
	"github.com/shareinto/paas/internal/shared/testutil"
)

type recordingPermission struct {
	calls []permissionCall
	err   error
}

type permissionCall struct {
	subject  identityaccess.Subject
	resource identityaccess.ResourceScope
	action   identityaccess.Permission
}

func (p *recordingPermission) Check(_ context.Context, subject identityaccess.Subject, resource identityaccess.ResourceScope, action identityaccess.Permission) error {
	p.calls = append(p.calls, permissionCall{subject: subject, resource: resource, action: action})
	if p.err != nil {
		return p.err
	}
	return nil
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

type recordingDeletionGuard struct {
	projects []Project
	actors   []identityaccess.Subject
	err      error
}

func (g *recordingDeletionGuard) PrepareProjectDeletion(_ context.Context, actor identityaccess.Subject, project Project) error {
	g.actors = append(g.actors, actor)
	g.projects = append(g.projects, project)
	return g.err
}

func newTestService() (*Service, *MemoryRepository, *recordingPermission, *recordingAudit, *recordingPublisher) {
	repo := NewMemoryRepository()
	permission := &recordingPermission{}
	audit := &recordingAudit{}
	events := &recordingPublisher{}
	svc := NewService(Options{
		Repository:        repo,
		PermissionChecker: permission,
		Audit:             audit,
		EventPublisher:    events,
		IDGenerator:       testutil.NewFakeIDGenerator(1),
		Clock:             testutil.NewFakeClock(time.Date(2026, 5, 30, 2, 0, 0, 0, time.UTC)),
	})
	return svc, repo, permission, audit, events
}

func testActor() identityaccess.Subject {
	return identityaccess.Subject{Type: identityaccess.SubjectUser, ID: "usr_admin"}
}

type failingIDGenerator struct{}

func (failingIDGenerator) NewID(string) (shared.ID, error) {
	return "", errors.New("id generator failed")
}

func TestCreateTenantPublishesEventAndAudit(t *testing.T) {
	svc, _, permission, audit, events := newTestService()
	ctx := context.Background()

	tenant, err := svc.CreateTenant(ctx, CreateTenantInput{Actor: testActor(), Name: "Payment", DisplayName: "支付团队", Description: " core "})
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	if tenant.ID != "tenant_1" || tenant.Name != "payment" || tenant.DisplayName != "支付团队" || tenant.Description != "core" {
		t.Fatalf("unexpected tenant: %+v", tenant)
	}
	if len(permission.calls) != 1 || permission.calls[0].resource.Kind != identityaccess.ScopePlatform || permission.calls[0].action != "tenant:update" {
		t.Fatalf("unexpected permission calls: %+v", permission.calls)
	}
	if len(events.events) != 1 || events.events[0].EventType != "TenantCreated" {
		t.Fatalf("expected TenantCreated event, got %+v", events.events)
	}
	if len(audit.events) != 1 || audit.events[0].Action != "tenant.create" || audit.events[0].ResourceID != tenant.ID {
		t.Fatalf("expected tenant create audit, got %+v", audit.events)
	}
}

func TestCreateTenantValidationAndConflicts(t *testing.T) {
	svc, _, permission, _, events := newTestService()
	ctx := context.Background()

	if _, err := svc.CreateTenant(ctx, CreateTenantInput{Actor: testActor(), Name: "1bad"}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("invalid tenant name should fail, got %v", err)
	}
	if _, err := svc.CreateTenant(ctx, CreateTenantInput{Actor: identityaccess.Subject{}, Name: "payment"}); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("missing actor should fail, got %v", err)
	}
	permission.err = shared.NewError(shared.CodePermissionDenied, "denied")
	if _, err := svc.CreateTenant(ctx, CreateTenantInput{Actor: testActor(), Name: "payment"}); shared.CodeOf(err) != shared.CodePermissionDenied {
		t.Fatalf("permission denial should fail, got %v", err)
	}
	permission.err = nil
	tenant, err := svc.CreateTenant(ctx, CreateTenantInput{Actor: testActor(), Name: "payment"})
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	if _, err := svc.CreateTenant(ctx, CreateTenantInput{Actor: testActor(), Name: "payment"}); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate tenant should conflict, got %v", err)
	}
	events.err = errors.New("event bus unavailable")
	if _, err := svc.CreateProject(ctx, CreateProjectInput{Actor: testActor(), TenantID: tenant.ID, Name: "checkout"}); err == nil {
		t.Fatalf("publish failure should fail project creation")
	}
}

func TestTenantMemberLifecycleAndQueries(t *testing.T) {
	svc, _, _, audit, _ := newTestService()
	ctx := context.Background()
	tenant, err := svc.CreateTenant(ctx, CreateTenantInput{Actor: testActor(), Name: "platform"})
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}

	member, err := svc.AddTenantMember(ctx, AddTenantMemberInput{Actor: testActor(), TenantID: tenant.ID, UserID: "usr_dev", RoleID: identityaccess.RoleDeveloper})
	if err != nil {
		t.Fatalf("AddTenantMember() error = %v", err)
	}
	if member.RoleID != identityaccess.RoleDeveloper {
		t.Fatalf("unexpected member: %+v", member)
	}
	ok, err := svc.IsTenantMember(ctx, tenant.ID, "usr_dev")
	if err != nil || !ok {
		t.Fatalf("IsTenantMember() = %v, %v", ok, err)
	}
	members, err := svc.ListTenantMembers(ctx, tenant.ID)
	if err != nil {
		t.Fatalf("ListTenantMembers() error = %v", err)
	}
	if len(members) != 1 || members[0].UserID != "usr_dev" {
		t.Fatalf("unexpected members: %+v", members)
	}
	if err := svc.RemoveTenantMember(ctx, RemoveTenantMemberInput{Actor: testActor(), TenantID: tenant.ID, UserID: "usr_dev"}); err != nil {
		t.Fatalf("RemoveTenantMember() error = %v", err)
	}
	ok, err = svc.IsTenantMember(ctx, tenant.ID, "usr_dev")
	if err != nil || ok {
		t.Fatalf("removed member should be absent, got %v, %v", ok, err)
	}
	if len(audit.events) < 3 || audit.events[len(audit.events)-1].Action != "tenant_member.remove" {
		t.Fatalf("member changes should be audited, got %+v", audit.events)
	}
}

func TestTenantMemberValidationAndRepositoryErrors(t *testing.T) {
	svc, repo, _, _, _ := newTestService()
	ctx := context.Background()
	tenant, err := svc.CreateTenant(ctx, CreateTenantInput{Actor: testActor(), Name: "platform"})
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	if _, err := svc.AddTenantMember(ctx, AddTenantMemberInput{Actor: testActor(), TenantID: tenant.ID}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("missing member fields should fail, got %v", err)
	}
	if _, err := svc.AddTenantMember(ctx, AddTenantMemberInput{Actor: testActor(), TenantID: "missing", UserID: "usr_dev", RoleID: identityaccess.RoleDeveloper}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing tenant should fail, got %v", err)
	}
	if err := svc.RemoveTenantMember(ctx, RemoveTenantMemberInput{Actor: testActor(), TenantID: tenant.ID}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("missing remove fields should fail, got %v", err)
	}
	if err := svc.RemoveTenantMember(ctx, RemoveTenantMemberInput{Actor: testActor(), TenantID: tenant.ID, UserID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("remove missing member should fail, got %v", err)
	}
	if _, err := repo.ListTenantMembers(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("listing members for missing tenant should fail, got %v", err)
	}
	if err := repo.SaveTenantMember(ctx, TenantMember{TenantID: "missing", UserID: "usr_dev"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("saving member for missing tenant should fail, got %v", err)
	}
	if err := repo.DeleteTenantMember(ctx, "missing", "usr_dev"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("deleting member for missing tenant should fail, got %v", err)
	}
}

func TestUpdateTenantAndTenantQueries(t *testing.T) {
	svc, repo, permission, audit, _ := newTestService()
	ctx := context.Background()
	tenant, err := svc.CreateTenant(ctx, CreateTenantInput{Actor: testActor(), Name: "payment"})
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	updated, err := svc.UpdateTenant(ctx, UpdateTenantInput{Actor: testActor(), TenantID: tenant.ID, DisplayName: "支付平台", Description: " owner "})
	if err != nil {
		t.Fatalf("UpdateTenant() error = %v", err)
	}
	if updated.DisplayName != "支付平台" || updated.Description != "owner" {
		t.Fatalf("unexpected tenant update: %+v", updated)
	}
	lastCall := permission.calls[len(permission.calls)-1]
	if lastCall.resource.Kind != identityaccess.ScopeTenant || lastCall.resource.TenantID != tenant.ID || lastCall.action != "tenant:update" {
		t.Fatalf("tenant update should check tenant scope, got %+v", lastCall)
	}
	if audit.events[len(audit.events)-1].Action != "tenant.update" {
		t.Fatalf("tenant update should be audited, got %+v", audit.events)
	}
	got, err := svc.GetTenant(ctx, tenant.ID)
	if err != nil || got.ID != tenant.ID {
		t.Fatalf("GetTenant() = %+v, %v", got, err)
	}
	found, err := repo.FindTenantByName(ctx, "payment")
	if err != nil || found.ID != tenant.ID {
		t.Fatalf("FindTenantByName() = %+v, %v", found, err)
	}
	if _, err := repo.FindTenantByName(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing tenant name should be not found, got %v", err)
	}
	renamed := updated
	renamed.Name = "billing"
	if err := repo.UpdateTenant(ctx, renamed); err != nil {
		t.Fatalf("repository tenant rename error = %v", err)
	}
	second := Tenant{ID: "tenant_second", Name: "second", CreatedAt: updated.CreatedAt, UpdatedAt: updated.UpdatedAt}
	if err := repo.CreateTenant(ctx, second); err != nil {
		t.Fatalf("CreateTenant second error = %v", err)
	}
	second.Name = "billing"
	if err := repo.UpdateTenant(ctx, second); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate tenant rename should conflict, got %v", err)
	}
	if err := repo.UpdateTenant(ctx, Tenant{ID: "missing", Name: "none"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("updating missing tenant should fail, got %v", err)
	}
}

func TestCreateProjectRequiresTenantAndPublishesEvent(t *testing.T) {
	svc, _, permission, audit, events := newTestService()
	ctx := context.Background()
	tenant, err := svc.CreateTenant(ctx, CreateTenantInput{Actor: testActor(), Name: "payment"})
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}

	project, err := svc.CreateProject(ctx, CreateProjectInput{Actor: testActor(), TenantID: tenant.ID, Name: "User-API", DisplayName: "用户接口"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if project.ID != "project_3" || project.TenantID != tenant.ID || project.Name != "user-api" {
		t.Fatalf("unexpected project: %+v", project)
	}
	lastCall := permission.calls[len(permission.calls)-1]
	if lastCall.resource.Kind != identityaccess.ScopeTenant || lastCall.resource.TenantID != tenant.ID || lastCall.action != "project:update" {
		t.Fatalf("unexpected project permission call: %+v", lastCall)
	}
	if events.events[len(events.events)-1].EventType != "ProjectCreated" {
		t.Fatalf("expected ProjectCreated event, got %+v", events.events)
	}
	if audit.events[len(audit.events)-1].Action != "project.create" {
		t.Fatalf("expected project create audit, got %+v", audit.events)
	}
	tenantID, err := svc.GetProjectTenantID(ctx, project.ID)
	if err != nil || tenantID != tenant.ID {
		t.Fatalf("GetProjectTenantID() = %s, %v", tenantID, err)
	}
}

func TestProjectValidationAndUpdate(t *testing.T) {
	svc, repo, permission, _, _ := newTestService()
	ctx := context.Background()
	tenant, err := svc.CreateTenant(ctx, CreateTenantInput{Actor: testActor(), Name: "payment"})
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	if _, err := svc.CreateProject(ctx, CreateProjectInput{Actor: testActor(), TenantID: "missing", Name: "api"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("project must belong to existing tenant, got %v", err)
	}
	if _, err := svc.CreateProject(ctx, CreateProjectInput{Actor: testActor(), TenantID: tenant.ID, Name: "1bad"}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("invalid project name should fail, got %v", err)
	}
	project, err := svc.CreateProject(ctx, CreateProjectInput{Actor: testActor(), TenantID: tenant.ID, Name: "api"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	if _, err := svc.CreateProject(ctx, CreateProjectInput{Actor: testActor(), TenantID: tenant.ID, Name: "api"}); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate project name in tenant should conflict, got %v", err)
	}
	updated, err := svc.UpdateProject(ctx, UpdateProjectInput{Actor: testActor(), ProjectID: project.ID, DisplayName: "接口服务", Description: " handles traffic "})
	if err != nil {
		t.Fatalf("UpdateProject() error = %v", err)
	}
	if updated.DisplayName != "接口服务" || updated.Description != "handles traffic" {
		t.Fatalf("unexpected updated project: %+v", updated)
	}
	lastCall := permission.calls[len(permission.calls)-1]
	if lastCall.resource.Kind != identityaccess.ScopeProject || lastCall.resource.ProjectID != project.ID || lastCall.resource.TenantID != tenant.ID {
		t.Fatalf("update should check project scope, got %+v", lastCall)
	}
	if got, err := svc.GetProject(ctx, project.ID); err != nil || got.ID != project.ID {
		t.Fatalf("GetProject() = %+v, %v", got, err)
	}
	found, err := repo.FindProjectByTenantAndName(ctx, tenant.ID, "api")
	if err != nil || found.ID != project.ID {
		t.Fatalf("FindProjectByTenantAndName() = %+v, %v", found, err)
	}
	if _, err := repo.FindProjectByTenantAndName(ctx, tenant.ID, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing project name should be not found, got %v", err)
	}
	otherTenant, err := svc.CreateTenant(ctx, CreateTenantInput{Actor: testActor(), Name: "other"})
	if err != nil {
		t.Fatalf("CreateTenant other error = %v", err)
	}
	tenantChanged := updated
	tenantChanged.TenantID = otherTenant.ID
	if err := repo.UpdateProject(ctx, tenantChanged); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("repository should reject project tenant change, got %v", err)
	}
	updated.TenantID = "tenant_other"
	if err := repo.UpdateProject(ctx, updated); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("repository should reject tenant changes to missing tenant, got %v", err)
	}
	if _, err := repo.GetProject(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing project should be not found, got %v", err)
	}
	if _, err := svc.GetProjectTenantID(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing project tenant query should be not found, got %v", err)
	}
	if _, err := svc.UpdateProject(ctx, UpdateProjectInput{Actor: testActor(), ProjectID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("updating missing project should be not found, got %v", err)
	}
	if err := repo.UpdateProject(ctx, Project{ID: "missing", TenantID: tenant.ID, Name: "none"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("repository updating missing project should be not found, got %v", err)
	}
}

func TestDeleteProjectChecksPermissionGuardPublishesEventAndAudit(t *testing.T) {
	svc, repo, permission, audit, events := newTestService()
	guard := &recordingDeletionGuard{}
	svc.deletion = guard
	ctx := context.Background()
	tenant, err := svc.CreateTenant(ctx, CreateTenantInput{Actor: testActor(), Name: "payment"})
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	project, err := svc.CreateProject(ctx, CreateProjectInput{Actor: testActor(), TenantID: tenant.ID, Name: "api"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	if err := svc.DeleteProject(ctx, DeleteProjectInput{Actor: testActor(), ProjectID: project.ID}); err != nil {
		t.Fatalf("DeleteProject() error = %v", err)
	}
	if _, err := svc.GetProject(ctx, project.ID); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("deleted project should be absent, got %v", err)
	}
	lastCall := permission.calls[len(permission.calls)-1]
	if lastCall.resource.Kind != identityaccess.ScopeProject || lastCall.resource.ProjectID != project.ID || lastCall.action != "project:update" {
		t.Fatalf("delete should check project scope, got %+v", lastCall)
	}
	if len(guard.projects) != 1 || guard.projects[0].ID != project.ID || guard.actors[0].ID != testActor().ID {
		t.Fatalf("deletion preparation not called: projects=%+v actors=%+v", guard.projects, guard.actors)
	}
	if events.events[len(events.events)-1].EventType != "ProjectDeleted" {
		t.Fatalf("expected ProjectDeleted event, got %+v", events.events)
	}
	if audit.events[len(audit.events)-1].Action != "project.delete" {
		t.Fatalf("expected delete audit, got %+v", audit.events)
	}
	if _, err := repo.FindProjectByTenantAndName(ctx, tenant.ID, "api"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("deleted project name index should be removed, got %v", err)
	}
}

func TestDeleteProjectValidationErrors(t *testing.T) {
	svc, repo, _, _, events := newTestService()
	ctx := context.Background()
	tenant, err := svc.CreateTenant(ctx, CreateTenantInput{Actor: testActor(), Name: "payment"})
	if err != nil {
		t.Fatalf("CreateTenant() error = %v", err)
	}
	project, err := svc.CreateProject(ctx, CreateProjectInput{Actor: testActor(), TenantID: tenant.ID, Name: "api"})
	if err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}
	svc.deletion = &recordingDeletionGuard{err: shared.NewError(shared.CodeFailedPrecondition, "project has resources")}
	if err := svc.DeleteProject(ctx, DeleteProjectInput{Actor: testActor(), ProjectID: project.ID}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("guard failure should block delete, got %v", err)
	}
	if _, err := svc.GetProject(ctx, project.ID); err != nil {
		t.Fatalf("blocked delete should keep project: %v", err)
	}
	if err := svc.DeleteProject(ctx, DeleteProjectInput{Actor: testActor(), ProjectID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing project delete should fail, got %v", err)
	}
	svc.deletion = nil
	events.err = errors.New("event bus unavailable")
	if err := svc.DeleteProject(ctx, DeleteProjectInput{Actor: testActor(), ProjectID: project.ID}); err == nil {
		t.Fatalf("publish failure should fail delete")
	}
	if err := repo.DeleteProject(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("repository deleting missing project should fail, got %v", err)
	}
}

func TestListQueriesArePagedAndSorted(t *testing.T) {
	svc, _, _, _, _ := newTestService()
	ctx := context.Background()
	alpha, err := svc.CreateTenant(ctx, CreateTenantInput{Actor: testActor(), Name: "alpha"})
	if err != nil {
		t.Fatalf("CreateTenant alpha error = %v", err)
	}
	beta, err := svc.CreateTenant(ctx, CreateTenantInput{Actor: testActor(), Name: "beta"})
	if err != nil {
		t.Fatalf("CreateTenant beta error = %v", err)
	}
	if _, err := svc.CreateProject(ctx, CreateProjectInput{Actor: testActor(), TenantID: beta.ID, Name: "zeta"}); err != nil {
		t.Fatalf("CreateProject zeta error = %v", err)
	}
	if _, err := svc.CreateProject(ctx, CreateProjectInput{Actor: testActor(), TenantID: beta.ID, Name: "alpha"}); err != nil {
		t.Fatalf("CreateProject alpha error = %v", err)
	}
	tenants, err := svc.ListTenants(ctx, shared.PageRequest{Page: 1, PageSize: 1})
	if err != nil {
		t.Fatalf("ListTenants() error = %v", err)
	}
	if tenants.Total != 2 || len(tenants.Items) != 1 || tenants.Items[0].ID != alpha.ID {
		t.Fatalf("unexpected tenant page: %+v", tenants)
	}
	projects, err := svc.ListProjectsByTenant(ctx, beta.ID, shared.PageRequest{})
	if err != nil {
		t.Fatalf("ListProjectsByTenant() error = %v", err)
	}
	if projects.Total != 2 || projects.Items[0].Name != "alpha" || projects.Items[1].Name != "zeta" {
		t.Fatalf("unexpected project page: %+v", projects)
	}
	if _, err := svc.ListProjectsByTenant(ctx, "missing", shared.PageRequest{}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("list projects for missing tenant should fail, got %v", err)
	}
}

func TestDefaultServiceNoopsAndIDFailure(t *testing.T) {
	ctx := context.Background()
	if err := (NoopAuditLogger{}).Log(ctx, AuditEvent{}); err != nil {
		t.Fatalf("NoopAuditLogger should not fail: %v", err)
	}
	if err := (NoopEventPublisher{}).Publish(ctx, shared.DomainEvent{}); err != nil {
		t.Fatalf("NoopEventPublisher should not fail: %v", err)
	}

	repo := NewMemoryRepository()
	svc := NewService(Options{Repository: repo})
	tenant, err := svc.CreateTenant(ctx, CreateTenantInput{Actor: testActor(), Name: "default"})
	if err != nil {
		t.Fatalf("default service CreateTenant() error = %v", err)
	}
	if tenant.ID.IsZero() || tenant.CreatedAt.IsZero() {
		t.Fatalf("default service should set id and time: %+v", tenant)
	}

	failing := NewService(Options{
		Repository:        NewMemoryRepository(),
		PermissionChecker: &recordingPermission{},
		IDGenerator:       failingIDGenerator{},
		Clock:             testutil.NewFakeClock(time.Date(2026, 5, 30, 3, 0, 0, 0, time.UTC)),
	})
	if _, err := failing.CreateTenant(ctx, CreateTenantInput{Actor: testActor(), Name: "fail"}); err == nil {
		t.Fatalf("id generator failure should fail tenant creation")
	}
	repo2 := NewMemoryRepository()
	failingPublish := NewService(Options{
		Repository:        repo2,
		PermissionChecker: &recordingPermission{},
		IDGenerator:       testutil.NewFakeIDGenerator(1),
		Clock:             testutil.NewFakeClock(time.Date(2026, 5, 30, 3, 0, 0, 0, time.UTC)),
	})
	failingPublish.ids = failingIDGenerator{}
	if err := repo2.CreateTenant(ctx, Tenant{ID: "tenant_seed", Name: "seed"}); err != nil {
		t.Fatalf("seed tenant error = %v", err)
	}
	if _, err := failingPublish.CreateProject(ctx, CreateProjectInput{Actor: testActor(), TenantID: "tenant_seed", Name: "api"}); err == nil {
		t.Fatalf("id generator failure should fail project creation")
	}
}
