package sourcerepository

import (
	"context"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/modules/tenantproject"
	"github.com/shareinto/paas/internal/shared"
)

type Service struct {
	repo               Repository
	git                GitSourceRepositoryPort
	projects           ProjectQuery
	memberships        ProjectMembershipQuery
	permission         PermissionChecker
	audit              AuditLogger
	events             EventPublisher
	ids                shared.IDGenerator
	clock              shared.Clock
	webhookCallbackURL string
}

type Options struct {
	Repository         Repository
	Git                GitSourceRepositoryPort
	ProjectQuery       ProjectQuery
	MembershipQuery    ProjectMembershipQuery
	PermissionChecker  PermissionChecker
	Audit              AuditLogger
	EventPublisher     EventPublisher
	IDGenerator        shared.IDGenerator
	Clock              shared.Clock
	WebhookCallbackURL string
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
		repo:               opts.Repository,
		git:                opts.Git,
		projects:           opts.ProjectQuery,
		memberships:        opts.MembershipQuery,
		permission:         opts.PermissionChecker,
		audit:              audit,
		events:             events,
		ids:                ids,
		clock:              clock,
		webhookCallbackURL: strings.TrimSpace(opts.WebhookCallbackURL),
	}
}

type CreateSourceRepositoryInput struct {
	Actor         identityaccess.Subject `json:"actor"`
	ProjectID     shared.ID              `json:"project_id"`
	Name          string                 `json:"name"`
	DisplayName   string                 `json:"display_name"`
	Description   string                 `json:"description"`
	HTTPURL       string                 `json:"http_url"`
	DefaultBranch string                 `json:"default_branch"`
}

type CreateRepositoryMigrationInput struct {
	Actor         identityaccess.Subject `json:"actor"`
	ProjectID     shared.ID              `json:"project_id"`
	Name          string                 `json:"name"`
	DisplayName   string                 `json:"display_name"`
	Description   string                 `json:"description"`
	SourceURL     string                 `json:"source_url"`
	DefaultBranch string                 `json:"default_branch"`
}

func (s *Service) CreateSourceRepository(ctx context.Context, input CreateSourceRepositoryInput) (SourceRepository, error) {
	project, err := s.requireProject(ctx, input.ProjectID)
	if err != nil {
		return SourceRepository{}, err
	}
	if err := s.checkProject(ctx, input.Actor, project, "project:update"); err != nil {
		return SourceRepository{}, err
	}
	name := normalizeRepositoryName(input.Name)
	if err := validateRepositoryName(name); err != nil {
		return SourceRepository{}, err
	}
	httpURL, err := normalizeHTTPRepositoryURL(input.HTTPURL)
	if err != nil {
		return SourceRepository{}, err
	}
	defaultBranch := normalizeDefaultBranch(input.DefaultBranch)
	if s.git == nil {
		return SourceRepository{}, shared.NewError(shared.CodeFailedPrecondition, "git source repository port is required")
	}
	gitProject, err := s.git.ResolveProjectByHTTPURL(ctx, httpURL)
	if err != nil {
		return SourceRepository{}, mapGitIntegrationError(err)
	}
	if _, err := s.git.ListFiles(ctx, gitProject.ID, defaultBranch); err != nil {
		return SourceRepository{}, mapGitIntegrationError(err)
	}
	id, err := s.ids.NewID("repo")
	if err != nil {
		return SourceRepository{}, err
	}
	now := s.clock.Now()
	repository := SourceRepository{
		ID:            id,
		TenantID:      project.TenantID,
		ProjectID:     project.ID,
		Name:          name,
		DisplayName:   normalizeDisplayName(input.DisplayName, name),
		Description:   strings.TrimSpace(input.Description),
		GitProvider:   "gitlab",
		GitProjectID:  gitProject.ID,
		HTTPURL:       httpURL,
		SSHURL:        gitProject.SSHURL,
		DefaultBranch: defaultBranch,
		Status:        RepositoryStatusReady,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if repository.SSHURL == "" {
		repository.SSHURL = gitProject.SSHURL
	}
	if err := s.repo.CreateSourceRepository(ctx, repository); err != nil {
		return SourceRepository{}, err
	}
	if err := s.publish(ctx, "SourceRepositoryCreated", now, RepositoryCreatedPayload{SourceRepositoryID: repository.ID, TenantID: repository.TenantID, ProjectID: repository.ProjectID, Name: repository.Name}); err != nil {
		return SourceRepository{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "source_repository.create", ResourceType: "source_repository", ResourceID: repository.ID, Result: "succeeded", Summary: "登记源码仓库", OccurredAt: now})
	return repository, nil
}

func (s *Service) GetSourceRepository(ctx context.Context, id shared.ID) (SourceRepository, error) {
	return s.repo.GetSourceRepository(ctx, id)
}

func (s *Service) ListSourceRepositoriesByProject(ctx context.Context, projectID shared.ID, page shared.PageRequest) (shared.PageResult[SourceRepository], error) {
	if _, err := s.requireProject(ctx, projectID); err != nil {
		return shared.PageResult[SourceRepository]{}, err
	}
	return s.repo.ListSourceRepositoriesByProject(ctx, projectID, page)
}

func (s *Service) ListAssociatedApplications(ctx context.Context, sourceRepositoryID shared.ID) ([]AssociatedApplication, error) {
	return s.repo.ListAssociatedApplications(ctx, sourceRepositoryID)
}

type DeleteSourceRepositoryInput struct {
	Actor              identityaccess.Subject `json:"actor"`
	SourceRepositoryID shared.ID              `json:"source_repository_id"`
}

func (s *Service) DeleteSourceRepository(ctx context.Context, input DeleteSourceRepositoryInput) error {
	repository, err := s.repo.GetSourceRepository(ctx, input.SourceRepositoryID)
	if err != nil {
		return err
	}
	project := tenantproject.Project{ID: repository.ProjectID, TenantID: repository.TenantID}
	if err := s.checkProject(ctx, input.Actor, project, "project:update"); err != nil {
		return err
	}
	applications, err := s.repo.ListAssociatedApplications(ctx, repository.ID)
	if err != nil {
		return err
	}
	if len(applications) > 0 {
		return shared.NewError(shared.CodeFailedPrecondition, "source repository has associated applications")
	}
	if err := s.repo.DeleteSourceRepository(ctx, repository.ID); err != nil {
		return err
	}
	now := s.clock.Now()
	if err := s.publish(ctx, "SourceRepositoryDeleted", now, RepositoryDeletedPayload{SourceRepositoryID: repository.ID, TenantID: repository.TenantID, ProjectID: repository.ProjectID, Name: repository.Name}); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "source_repository.delete", ResourceType: "source_repository", ResourceID: repository.ID, Result: "succeeded", Summary: "删除源码仓库登记", OccurredAt: now})
	return nil
}

func (s *Service) SyncRepositoryPermissions(ctx context.Context, actor identityaccess.Subject, sourceRepositoryID shared.ID) (RepositoryPermissionSyncJob, error) {
	repository, err := s.repo.GetSourceRepository(ctx, sourceRepositoryID)
	if err != nil {
		return RepositoryPermissionSyncJob{}, err
	}
	project := tenantproject.Project{ID: repository.ProjectID, TenantID: repository.TenantID}
	if err := s.checkProject(ctx, actor, project, "project:update"); err != nil {
		return RepositoryPermissionSyncJob{}, err
	}
	if s.git == nil || s.memberships == nil {
		return RepositoryPermissionSyncJob{}, shared.NewError(shared.CodeFailedPrecondition, "git and membership ports are required")
	}
	id, err := s.ids.NewID("repo_perm_sync")
	if err != nil {
		return RepositoryPermissionSyncJob{}, err
	}
	now := s.clock.Now()
	job := RepositoryPermissionSyncJob{
		ID:                 id,
		TenantID:           repository.TenantID,
		ProjectID:          repository.ProjectID,
		SourceRepositoryID: repository.ID,
		Status:             PermissionSyncRunning,
		RequestedBy:        actor.ID,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := s.repo.CreatePermissionSyncJob(ctx, job); err != nil {
		return RepositoryPermissionSyncJob{}, err
	}
	members, err := s.memberships.ListTenantMembers(ctx, repository.TenantID)
	if err != nil {
		return s.failPermissionJob(ctx, job, err)
	}
	accesses := make([]GitMemberAccess, 0, len(members))
	for _, member := range members {
		access, ok := MapRoleToGitAccess(member.RoleID)
		if !ok {
			continue
		}
		accesses = append(accesses, GitMemberAccess{UserID: member.UserID, RoleID: member.RoleID, Access: access})
	}
	if err := s.git.SyncMembers(ctx, repository.GitProjectID, accesses); err != nil {
		return s.failPermissionJob(ctx, job, err)
	}
	job.Status = PermissionSyncSucceeded
	job.UpdatedAt = s.clock.Now()
	if err := s.repo.UpdatePermissionSyncJob(ctx, job); err != nil {
		return RepositoryPermissionSyncJob{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: actor.ID, Action: "source_repository.permission_sync", ResourceType: "source_repository", ResourceID: repository.ID, Result: "succeeded", Summary: "同步源码仓库成员权限", OccurredAt: job.UpdatedAt})
	return job, nil
}

func (s *Service) CreateRepositoryMigration(ctx context.Context, input CreateRepositoryMigrationInput) (RepositoryMigration, error) {
	return RepositoryMigration{}, shared.NewError(shared.CodeFailedPrecondition, "repository migration is no longer supported; create the repository in GitLab and register its http_url")
}

func (s *Service) GetRepositoryMigration(ctx context.Context, id shared.ID) (RepositoryMigration, error) {
	return RepositoryMigration{}, shared.NewError(shared.CodeFailedPrecondition, "repository migration is no longer supported; create the repository in GitLab and register its http_url")
}

func (s *Service) RetryRepositoryMigration(ctx context.Context, actor identityaccess.Subject, id shared.ID) (RepositoryMigration, error) {
	return RepositoryMigration{}, shared.NewError(shared.CodeFailedPrecondition, "repository migration is no longer supported; create the repository in GitLab and register its http_url")
}

func (s *Service) CancelRepositoryMigration(ctx context.Context, actor identityaccess.Subject, id shared.ID) (RepositoryMigration, error) {
	return RepositoryMigration{}, shared.NewError(shared.CodeFailedPrecondition, "repository migration is no longer supported; create the repository in GitLab and register its http_url")
}

func (s *Service) ProcessRepositoryMigration(ctx context.Context, id shared.ID) (RepositoryMigration, []BuildSpecSuggestion, error) {
	return RepositoryMigration{}, nil, shared.NewError(shared.CodeFailedPrecondition, "repository migration is no longer supported; create the repository in GitLab and register its http_url")
}

func (s *Service) GenerateBuildSpecSuggestions(ctx context.Context, sourceRepositoryID shared.ID, ref string) ([]BuildSpecSuggestion, error) {
	repository, err := s.repo.GetSourceRepository(ctx, sourceRepositoryID)
	if err != nil {
		return nil, err
	}
	if s.git == nil {
		return nil, shared.NewError(shared.CodeFailedPrecondition, "git source repository port is required")
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		ref = repository.DefaultBranch
	}
	files, err := s.git.ListFiles(ctx, repository.GitProjectID, ref)
	if err != nil {
		return nil, err
	}
	return GenerateJavaBuildSpecSuggestions(files), nil
}

func (s *Service) ListRepositoryTree(ctx context.Context, sourceRepositoryID shared.ID, ref string, treePath string) ([]RepositoryTreeItem, error) {
	repository, err := s.repo.GetSourceRepository(ctx, sourceRepositoryID)
	if err != nil {
		return nil, err
	}
	if repository.Status != RepositoryStatusReady {
		return nil, shared.NewError(shared.CodeFailedPrecondition, "source repository is not ready")
	}
	if s.git == nil {
		return nil, shared.NewError(shared.CodeFailedPrecondition, "git source repository port is required")
	}
	ref = strings.TrimSpace(ref)
	if ref == "" {
		ref = repository.DefaultBranch
	}
	normalizedPath, err := normalizeTreePath(treePath)
	if err != nil {
		return nil, err
	}
	items, err := s.git.ListTree(ctx, repository.GitProjectID, ref, normalizedPath)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Type != items[j].Type {
			return items[i].Type == "tree"
		}
		return items[i].Path < items[j].Path
	})
	return items, nil
}

func (s *Service) ListRepositoryBranches(ctx context.Context, sourceRepositoryID shared.ID) ([]RepositoryBranch, error) {
	repository, err := s.repo.GetSourceRepository(ctx, sourceRepositoryID)
	if err != nil {
		return nil, err
	}
	if repository.Status != RepositoryStatusReady {
		return nil, shared.NewError(shared.CodeFailedPrecondition, "source repository is not ready")
	}
	if s.git == nil {
		return nil, shared.NewError(shared.CodeFailedPrecondition, "git source repository port is required")
	}
	branches, err := s.git.ListBranches(ctx, repository.GitProjectID)
	if err != nil {
		return nil, err
	}
	for i := range branches {
		if branches[i].Name == repository.DefaultBranch {
			branches[i].Default = true
		}
	}
	sort.SliceStable(branches, func(i, j int) bool {
		if branches[i].Default != branches[j].Default {
			return branches[i].Default
		}
		return branches[i].Name < branches[j].Name
	})
	return branches, nil
}

func MapRoleToGitAccess(roleID identityaccess.RoleID) (GitAccessLevel, bool) {
	switch roleID {
	case identityaccess.RoleTenantOwner:
		return GitAccessOwner, true
	case identityaccess.RoleProjectAdmin:
		return GitAccessMaintainer, true
	case identityaccess.RoleDeveloper:
		return GitAccessDeveloper, true
	case identityaccess.RoleViewer:
		return GitAccessReporter, true
	default:
		return "", false
	}
}

func (s *Service) requireProject(ctx context.Context, projectID shared.ID) (tenantproject.Project, error) {
	if projectID.IsZero() {
		return tenantproject.Project{}, shared.NewError(shared.CodeInvalidArgument, "project_id is required")
	}
	if s.projects == nil {
		return tenantproject.Project{}, shared.NewError(shared.CodeFailedPrecondition, "project query port is required")
	}
	return s.projects.GetProject(ctx, projectID)
}

func normalizeTreePath(value string) (string, error) {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" || value == "." {
		return "", nil
	}
	if strings.HasPrefix(value, "/") || path.IsAbs(value) {
		return "", shared.NewError(shared.CodeInvalidArgument, "path must be relative")
	}
	cleaned := path.Clean(value)
	if cleaned == "." {
		return "", nil
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, "/../") {
		return "", shared.NewError(shared.CodeInvalidArgument, "path cannot contain ..")
	}
	return cleaned, nil
}

func (s *Service) checkProject(ctx context.Context, actor identityaccess.Subject, project tenantproject.Project, action identityaccess.Permission) error {
	if actor.ID.IsZero() {
		return shared.NewError(shared.CodeUnauthenticated, "actor is required")
	}
	if s.permission == nil {
		return nil
	}
	return s.permission.Check(ctx, actor, identityaccess.ResourceScope{Kind: identityaccess.ScopeProject, TenantID: project.TenantID, ProjectID: project.ID}, action)
}

func (s *Service) transitionMigration(ctx context.Context, migration RepositoryMigration, next RepositoryMigrationStatus, message string) (RepositoryMigration, error) {
	if !canTransitionMigration(migration.Status, next) {
		return RepositoryMigration{}, shared.NewError(shared.CodeFailedPrecondition, "invalid repository migration status transition")
	}
	now := s.clock.Now()
	migration.Status = next
	migration.ErrorMessage = strings.TrimSpace(message)
	migration.UpdatedAt = now
	if terminalMigrationStatus(next) {
		migration.CompletedAt = &now
	}
	if err := s.repo.UpdateMigration(ctx, migration); err != nil {
		return RepositoryMigration{}, err
	}
	return migration, nil
}

func (s *Service) failPermissionJob(ctx context.Context, job RepositoryPermissionSyncJob, cause error) (RepositoryPermissionSyncJob, error) {
	job.Status = PermissionSyncFailed
	job.ErrorMessage = strings.TrimSpace(cause.Error())
	job.UpdatedAt = s.clock.Now()
	if err := s.repo.UpdatePermissionSyncJob(ctx, job); err != nil {
		return RepositoryPermissionSyncJob{}, err
	}
	return job, cause
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

func mapGitIntegrationError(err error) error {
	if shared.CodeOf(err) == shared.CodeUnauthenticated {
		return shared.WrapError(shared.CodeUnavailable, "gitlab integration authentication failed", err)
	}
	return err
}

func normalizeDefaultBranch(branch string) string {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return "main"
	}
	return branch
}

func normalizeHTTPRepositoryURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", shared.NewError(shared.CodeInvalidArgument, "http_url is required")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", shared.NewError(shared.CodeInvalidArgument, "http_url must be a valid http or https repository url")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", shared.NewError(shared.CodeInvalidArgument, "http_url must use http or https")
	}
	if parsed.User != nil {
		return "", shared.NewError(shared.CodeInvalidArgument, "http_url must not contain credentials")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", shared.NewError(shared.CodeInvalidArgument, "http_url must not contain query or fragment")
	}
	return value, nil
}
