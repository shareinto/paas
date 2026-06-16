package tenantproject

import (
	"context"
	"strings"
	"time"

	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/shared"
)

type Service struct {
	repo       Repository
	permission PermissionChecker
	roles      RoleBindingManager
	deletion   ProjectDeletionGuard
	audit      AuditLogger
	events     EventPublisher
	ids        shared.IDGenerator
	clock      shared.Clock
}

type Options struct {
	Repository        Repository
	PermissionChecker PermissionChecker
	RoleBindings      RoleBindingManager
	ProjectDeletion   ProjectDeletionGuard
	Audit             AuditLogger
	EventPublisher    EventPublisher
	IDGenerator       shared.IDGenerator
	Clock             shared.Clock
}

func NewService(opts Options) *Service {
	audit := opts.Audit
	if audit == nil {
		audit = NoopAuditLogger{}
	}
	events := opts.EventPublisher
	if events == nil {
		events = NoopEventPublisher{}
	}
	ids := opts.IDGenerator
	if ids == nil {
		ids = shared.RandomIDGenerator{}
	}
	clock := opts.Clock
	if clock == nil {
		clock = shared.SystemClock{}
	}
	return &Service{
		repo:       opts.Repository,
		permission: opts.PermissionChecker,
		roles:      opts.RoleBindings,
		deletion:   opts.ProjectDeletion,
		audit:      audit,
		events:     events,
		ids:        ids,
		clock:      clock,
	}
}

func (s *Service) SetProjectDeletionGuard(guard ProjectDeletionGuard) {
	s.deletion = guard
}

type CreateTenantInput struct {
	Actor       identityaccess.Subject `json:"actor"`
	Name        string                 `json:"name"`
	DisplayName string                 `json:"display_name"`
	Description string                 `json:"description"`
}

type UpdateTenantInput struct {
	Actor       identityaccess.Subject `json:"actor"`
	TenantID    shared.ID              `json:"tenant_id"`
	DisplayName string                 `json:"display_name"`
	Description string                 `json:"description"`
}

type AddTenantMemberInput struct {
	Actor    identityaccess.Subject `json:"actor"`
	TenantID shared.ID              `json:"tenant_id"`
	UserID   shared.ID              `json:"user_id"`
	RoleID   identityaccess.RoleID  `json:"role_id"`
}

type RemoveTenantMemberInput struct {
	Actor    identityaccess.Subject `json:"actor"`
	TenantID shared.ID              `json:"tenant_id"`
	UserID   shared.ID              `json:"user_id"`
}

type UpsertProjectMemberInput struct {
	Actor     identityaccess.Subject `json:"actor"`
	ProjectID shared.ID              `json:"project_id"`
	UserID    shared.ID              `json:"user_id"`
	RoleID    identityaccess.RoleID  `json:"role_id"`
}

type RemoveProjectMemberInput struct {
	Actor     identityaccess.Subject `json:"actor"`
	ProjectID shared.ID              `json:"project_id"`
	UserID    shared.ID              `json:"user_id"`
}

type CreateProjectInput struct {
	Actor       identityaccess.Subject `json:"actor"`
	TenantID    shared.ID              `json:"tenant_id"`
	Name        string                 `json:"name"`
	DisplayName string                 `json:"display_name"`
	Description string                 `json:"description"`
}

type UpdateProjectInput struct {
	Actor       identityaccess.Subject `json:"actor"`
	ProjectID   shared.ID              `json:"project_id"`
	DisplayName string                 `json:"display_name"`
	Description string                 `json:"description"`
}

type DeleteProjectInput struct {
	Actor     identityaccess.Subject `json:"actor"`
	ProjectID shared.ID              `json:"project_id"`
}

func (s *Service) CreateTenant(ctx context.Context, input CreateTenantInput) (Tenant, error) {
	if err := s.check(ctx, input.Actor, identityaccess.ResourceScope{Kind: identityaccess.ScopePlatform}, "tenant:update"); err != nil {
		return Tenant{}, err
	}
	name := normalizeResourceName(input.Name)
	if err := validateResourceName(name); err != nil {
		return Tenant{}, err
	}
	id, err := s.ids.NewID("tenant")
	if err != nil {
		return Tenant{}, err
	}
	now := s.clock.Now()
	tenant := Tenant{
		ID:          id,
		Name:        name,
		DisplayName: normalizeDisplayName(input.DisplayName, name),
		Description: strings.TrimSpace(input.Description),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.CreateTenant(ctx, tenant); err != nil {
		return Tenant{}, err
	}
	if err := s.publish(ctx, "TenantCreated", now, TenantCreatedPayload{TenantID: tenant.ID, Name: tenant.Name}); err != nil {
		return Tenant{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "tenant.create", ResourceType: "tenant", ResourceID: tenant.ID, Result: "succeeded", Summary: "创建租户", OccurredAt: now})
	return tenant, nil
}

func (s *Service) UpdateTenant(ctx context.Context, input UpdateTenantInput) (Tenant, error) {
	tenant, err := s.repo.GetTenant(ctx, input.TenantID)
	if err != nil {
		return Tenant{}, err
	}
	if err := s.check(ctx, input.Actor, identityaccess.ResourceScope{Kind: identityaccess.ScopeTenant, TenantID: tenant.ID}, "tenant:update"); err != nil {
		return Tenant{}, err
	}
	tenant.DisplayName = normalizeDisplayName(input.DisplayName, tenant.Name)
	tenant.Description = strings.TrimSpace(input.Description)
	tenant.UpdatedAt = s.clock.Now()
	if err := s.repo.UpdateTenant(ctx, tenant); err != nil {
		return Tenant{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "tenant.update", ResourceType: "tenant", ResourceID: tenant.ID, Result: "succeeded", Summary: "更新租户", OccurredAt: tenant.UpdatedAt})
	return tenant, nil
}

func (s *Service) AddTenantMember(ctx context.Context, input AddTenantMemberInput) (TenantMember, error) {
	if input.TenantID.IsZero() || input.UserID.IsZero() || input.RoleID == "" {
		return TenantMember{}, shared.NewError(shared.CodeInvalidArgument, "tenant_id, user_id and role_id are required")
	}
	if _, err := s.repo.GetTenant(ctx, input.TenantID); err != nil {
		return TenantMember{}, err
	}
	if err := s.check(ctx, input.Actor, identityaccess.ResourceScope{Kind: identityaccess.ScopeTenant, TenantID: input.TenantID}, "tenant:update"); err != nil {
		return TenantMember{}, err
	}
	now := s.clock.Now()
	member := TenantMember{TenantID: input.TenantID, UserID: input.UserID, RoleID: input.RoleID, CreatedAt: now, UpdatedAt: now}
	if existing, err := s.repo.GetTenantMember(ctx, input.TenantID, input.UserID); err == nil {
		member.CreatedAt = existing.CreatedAt
	}
	if err := s.repo.SaveTenantMember(ctx, member); err != nil {
		return TenantMember{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "tenant_member.upsert", ResourceType: "tenant", ResourceID: input.TenantID, Result: "succeeded", Summary: "修改租户成员权限", OccurredAt: now})
	return member, nil
}

func (s *Service) RemoveTenantMember(ctx context.Context, input RemoveTenantMemberInput) error {
	if input.TenantID.IsZero() || input.UserID.IsZero() {
		return shared.NewError(shared.CodeInvalidArgument, "tenant_id and user_id are required")
	}
	if err := s.check(ctx, input.Actor, identityaccess.ResourceScope{Kind: identityaccess.ScopeTenant, TenantID: input.TenantID}, "tenant:update"); err != nil {
		return err
	}
	if err := s.repo.DeleteTenantMember(ctx, input.TenantID, input.UserID); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "tenant_member.remove", ResourceType: "tenant", ResourceID: input.TenantID, Result: "succeeded", Summary: "移除租户成员", OccurredAt: s.clock.Now()})
	return nil
}

func (s *Service) ListProjectMembers(ctx context.Context, projectID shared.ID) ([]ProjectMember, error) {
	if projectID.IsZero() {
		return nil, shared.NewError(shared.CodeInvalidArgument, "project_id is required")
	}
	if _, err := s.repo.GetProject(ctx, projectID); err != nil {
		return nil, err
	}
	if s.roles == nil {
		return []ProjectMember{}, nil
	}
	bindings, err := s.roles.ListRoleBindingsByScope(ctx, identityaccess.ScopeProject, projectID)
	if err != nil {
		return nil, err
	}
	members := make([]ProjectMember, 0, len(bindings))
	for _, binding := range bindings {
		if binding.SubjectType != identityaccess.SubjectUser {
			continue
		}
		members = append(members, ProjectMember{ProjectID: projectID, UserID: binding.SubjectID, RoleID: binding.RoleID, CreatedAt: binding.CreatedAt, UpdatedAt: binding.CreatedAt})
	}
	return members, nil
}

func (s *Service) UpsertProjectMember(ctx context.Context, input UpsertProjectMemberInput) (ProjectMember, error) {
	if input.ProjectID.IsZero() || input.UserID.IsZero() || input.RoleID == "" {
		return ProjectMember{}, shared.NewError(shared.CodeInvalidArgument, "project_id, user_id and role_id are required")
	}
	project, err := s.repo.GetProject(ctx, input.ProjectID)
	if err != nil {
		return ProjectMember{}, err
	}
	if err := s.check(ctx, input.Actor, identityaccess.ResourceScope{Kind: identityaccess.ScopeProject, TenantID: project.TenantID, ProjectID: project.ID}, "project:update"); err != nil {
		return ProjectMember{}, err
	}
	if s.roles == nil {
		return ProjectMember{}, shared.NewError(shared.CodeFailedPrecondition, "role binding manager is required")
	}
	binding, err := s.roles.ReplaceRoleBindingForSubjectScope(ctx, identityaccess.RoleBinding{
		SubjectType: identityaccess.SubjectUser,
		SubjectID:   input.UserID,
		RoleID:      input.RoleID,
		ScopeKind:   identityaccess.ScopeProject,
		ScopeID:     project.ID,
	})
	if err != nil {
		return ProjectMember{}, err
	}
	now := s.clock.Now()
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "project_member.upsert", ResourceType: "project", ResourceID: project.ID, Result: "succeeded", Summary: "修改项目成员权限", OccurredAt: now})
	return ProjectMember{ProjectID: project.ID, UserID: input.UserID, RoleID: input.RoleID, CreatedAt: binding.CreatedAt, UpdatedAt: binding.CreatedAt}, nil
}

func (s *Service) RemoveProjectMember(ctx context.Context, input RemoveProjectMemberInput) error {
	if input.ProjectID.IsZero() || input.UserID.IsZero() {
		return shared.NewError(shared.CodeInvalidArgument, "project_id and user_id are required")
	}
	project, err := s.repo.GetProject(ctx, input.ProjectID)
	if err != nil {
		return err
	}
	if err := s.check(ctx, input.Actor, identityaccess.ResourceScope{Kind: identityaccess.ScopeProject, TenantID: project.TenantID, ProjectID: project.ID}, "project:update"); err != nil {
		return err
	}
	if s.roles == nil {
		return shared.NewError(shared.CodeFailedPrecondition, "role binding manager is required")
	}
	if err := s.roles.DeleteRoleBindingsForSubjectScope(ctx, identityaccess.Subject{Type: identityaccess.SubjectUser, ID: input.UserID}, identityaccess.ScopeProject, project.ID); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "project_member.remove", ResourceType: "project", ResourceID: project.ID, Result: "succeeded", Summary: "移除项目成员", OccurredAt: s.clock.Now()})
	return nil
}

func (s *Service) CreateProject(ctx context.Context, input CreateProjectInput) (Project, error) {
	if input.TenantID.IsZero() {
		return Project{}, shared.NewError(shared.CodeInvalidArgument, "tenant_id is required")
	}
	if _, err := s.repo.GetTenant(ctx, input.TenantID); err != nil {
		return Project{}, err
	}
	if err := s.check(ctx, input.Actor, identityaccess.ResourceScope{Kind: identityaccess.ScopeTenant, TenantID: input.TenantID}, "project:update"); err != nil {
		return Project{}, err
	}
	name := normalizeResourceName(input.Name)
	if err := validateResourceName(name); err != nil {
		return Project{}, err
	}
	id, err := s.ids.NewID("project")
	if err != nil {
		return Project{}, err
	}
	now := s.clock.Now()
	project := Project{
		ID:          id,
		TenantID:    input.TenantID,
		Name:        name,
		DisplayName: normalizeDisplayName(input.DisplayName, name),
		Description: strings.TrimSpace(input.Description),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.CreateProject(ctx, project); err != nil {
		return Project{}, err
	}
	if err := s.publish(ctx, "ProjectCreated", now, ProjectCreatedPayload{ProjectID: project.ID, TenantID: project.TenantID, Name: project.Name}); err != nil {
		return Project{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "project.create", ResourceType: "project", ResourceID: project.ID, Result: "succeeded", Summary: "创建项目", OccurredAt: now})
	return project, nil
}

func (s *Service) UpdateProject(ctx context.Context, input UpdateProjectInput) (Project, error) {
	project, err := s.repo.GetProject(ctx, input.ProjectID)
	if err != nil {
		return Project{}, err
	}
	if err := s.check(ctx, input.Actor, identityaccess.ResourceScope{Kind: identityaccess.ScopeProject, TenantID: project.TenantID, ProjectID: project.ID}, "project:update"); err != nil {
		return Project{}, err
	}
	project.DisplayName = normalizeDisplayName(input.DisplayName, project.Name)
	project.Description = strings.TrimSpace(input.Description)
	project.UpdatedAt = s.clock.Now()
	if err := s.repo.UpdateProject(ctx, project); err != nil {
		return Project{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "project.update", ResourceType: "project", ResourceID: project.ID, Result: "succeeded", Summary: "更新项目", OccurredAt: project.UpdatedAt})
	return project, nil
}

func (s *Service) DeleteProject(ctx context.Context, input DeleteProjectInput) error {
	project, err := s.repo.GetProject(ctx, input.ProjectID)
	if err != nil {
		return err
	}
	if err := s.check(ctx, input.Actor, identityaccess.ResourceScope{Kind: identityaccess.ScopeProject, TenantID: project.TenantID, ProjectID: project.ID}, "project:update"); err != nil {
		return err
	}
	if s.deletion != nil {
		if err := s.deletion.PrepareProjectDeletion(ctx, input.Actor, project); err != nil {
			return err
		}
	}
	if err := s.repo.DeleteProject(ctx, project.ID); err != nil {
		return err
	}
	now := s.clock.Now()
	if err := s.publish(ctx, "ProjectDeleted", now, ProjectDeletedPayload{ProjectID: project.ID, TenantID: project.TenantID, Name: project.Name}); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "project.delete", ResourceType: "project", ResourceID: project.ID, Result: "succeeded", Summary: "删除项目", OccurredAt: now})
	return nil
}

func (s *Service) GetTenant(ctx context.Context, id shared.ID) (Tenant, error) {
	return s.repo.GetTenant(ctx, id)
}

func (s *Service) ListTenants(ctx context.Context, page shared.PageRequest) (shared.PageResult[Tenant], error) {
	return s.repo.ListTenants(ctx, page)
}

func (s *Service) GetTenantMember(ctx context.Context, tenantID shared.ID, userID shared.ID) (TenantMember, error) {
	return s.repo.GetTenantMember(ctx, tenantID, userID)
}

func (s *Service) IsTenantMember(ctx context.Context, tenantID shared.ID, userID shared.ID) (bool, error) {
	_, err := s.repo.GetTenantMember(ctx, tenantID, userID)
	if err == nil {
		return true, nil
	}
	if shared.CodeOf(err) == shared.CodeNotFound {
		return false, nil
	}
	return false, err
}

func (s *Service) ListTenantMembers(ctx context.Context, tenantID shared.ID) ([]TenantMember, error) {
	return s.repo.ListTenantMembers(ctx, tenantID)
}

func (s *Service) GetProject(ctx context.Context, id shared.ID) (Project, error) {
	return s.repo.GetProject(ctx, id)
}

func (s *Service) GetProjectTenantID(ctx context.Context, projectID shared.ID) (shared.ID, error) {
	project, err := s.repo.GetProject(ctx, projectID)
	if err != nil {
		return "", err
	}
	return project.TenantID, nil
}

func (s *Service) ListProjectsByTenant(ctx context.Context, tenantID shared.ID, page shared.PageRequest) (shared.PageResult[Project], error) {
	return s.repo.ListProjectsByTenant(ctx, tenantID, page)
}

func (s *Service) check(ctx context.Context, actor identityaccess.Subject, resource identityaccess.ResourceScope, action identityaccess.Permission) error {
	if actor.ID.IsZero() {
		return shared.NewError(shared.CodeUnauthenticated, "actor is required")
	}
	if s.permission == nil {
		return nil
	}
	return s.permission.Check(ctx, actor, resource, action)
}

func (s *Service) publish(ctx context.Context, eventType string, occurredAt time.Time, payload any) error {
	eventID, err := s.ids.NewID("evt")
	if err != nil {
		return err
	}
	event, err := shared.NewDomainEvent(eventID, eventType, occurredAt, payload)
	if err != nil {
		return err
	}
	return s.events.Publish(ctx, event)
}
