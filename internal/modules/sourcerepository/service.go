package sourcerepository

import (
	"context"
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
	tenant, err := s.projects.GetTenant(ctx, project.TenantID)
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
	defaultBranch := normalizeDefaultBranch(input.DefaultBranch)
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
		DefaultBranch: defaultBranch,
		Status:        RepositoryStatusProvisioning,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if s.git == nil {
		return SourceRepository{}, shared.NewError(shared.CodeFailedPrecondition, "git source repository port is required")
	}
	if err := s.repo.CreateSourceRepository(ctx, repository); err != nil {
		return SourceRepository{}, err
	}
	failCreate := func(cause error) (SourceRepository, error) {
		repository.Status = RepositoryStatusFailed
		repository.UpdatedAt = s.clock.Now()
		_ = s.repo.UpdateSourceRepository(ctx, repository)
		_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "source_repository.create", ResourceType: "source_repository", ResourceID: repository.ID, Result: "failed", Summary: "创建平台托管源码仓库失败", OccurredAt: repository.UpdatedAt})
		return SourceRepository{}, mapGitIntegrationError(cause)
	}
	gitProject, err := s.git.CreateProject(ctx, GitProjectSpec{
		TenantID:       project.TenantID,
		TenantName:     tenant.Name,
		ProjectID:      project.ID,
		ProjectName:    project.Name,
		RepositoryID:   repository.ID,
		RepositoryName: repository.Name,
		DefaultBranch:  defaultBranch,
	})
	if err != nil {
		return failCreate(err)
	}
	repository.GitProjectID = gitProject.ID
	repository.HTTPURL = gitProject.HTTPURL
	repository.SSHURL = gitProject.SSHURL
	if err := s.git.ProtectBranch(ctx, gitProject.ID, defaultBranch); err != nil {
		return failCreate(err)
	}
	if s.webhookCallbackURL != "" {
		if err := s.git.ConfigureWebhook(ctx, gitProject.ID, s.webhookCallbackURL); err != nil {
			return failCreate(err)
		}
	}
	repository.Status = RepositoryStatusReady
	repository.UpdatedAt = s.clock.Now()
	if err := s.repo.UpdateSourceRepository(ctx, repository); err != nil {
		return SourceRepository{}, err
	}
	if err := s.publish(ctx, "SourceRepositoryCreated", now, RepositoryCreatedPayload{SourceRepositoryID: repository.ID, TenantID: repository.TenantID, ProjectID: repository.ProjectID, Name: repository.Name}); err != nil {
		return SourceRepository{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "source_repository.create", ResourceType: "source_repository", ResourceID: repository.ID, Result: "succeeded", Summary: "创建平台托管源码仓库", OccurredAt: now})
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
	if repository.GitProjectID != "" {
		if s.git == nil {
			return shared.NewError(shared.CodeFailedPrecondition, "git source repository port is required")
		}
		if err := s.git.DeleteProject(ctx, repository.GitProjectID); err != nil && shared.CodeOf(err) != shared.CodeNotFound {
			return mapGitIntegrationError(err)
		}
	}
	if err := s.repo.DeleteSourceRepository(ctx, repository.ID); err != nil {
		return err
	}
	now := s.clock.Now()
	if err := s.publish(ctx, "SourceRepositoryDeleted", now, RepositoryDeletedPayload{SourceRepositoryID: repository.ID, TenantID: repository.TenantID, ProjectID: repository.ProjectID, Name: repository.Name}); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "source_repository.delete", ResourceType: "source_repository", ResourceID: repository.ID, Result: "succeeded", Summary: "删除平台托管源码仓库", OccurredAt: now})
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
	project, err := s.requireProject(ctx, input.ProjectID)
	if err != nil {
		return RepositoryMigration{}, err
	}
	if err := s.checkProject(ctx, input.Actor, project, "project:update"); err != nil {
		return RepositoryMigration{}, err
	}
	sourceURL := strings.TrimSpace(input.SourceURL)
	if sourceURL == "" {
		return RepositoryMigration{}, shared.NewError(shared.CodeInvalidArgument, "source_url is required")
	}
	name := normalizeRepositoryName(input.Name)
	if err := validateRepositoryName(name); err != nil {
		return RepositoryMigration{}, err
	}
	repoID, err := s.ids.NewID("repo")
	if err != nil {
		return RepositoryMigration{}, err
	}
	migrationID, err := s.ids.NewID("repo_migration")
	if err != nil {
		return RepositoryMigration{}, err
	}
	now := s.clock.Now()
	repository := SourceRepository{
		ID:            repoID,
		TenantID:      project.TenantID,
		ProjectID:     project.ID,
		Name:          name,
		DisplayName:   normalizeDisplayName(input.DisplayName, name),
		Description:   strings.TrimSpace(input.Description),
		GitProvider:   "gitlab",
		DefaultBranch: normalizeDefaultBranch(input.DefaultBranch),
		Status:        RepositoryStatusMigrating,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.repo.CreateSourceRepository(ctx, repository); err != nil {
		return RepositoryMigration{}, err
	}
	migration := RepositoryMigration{
		ID:                 migrationID,
		TenantID:           project.TenantID,
		ProjectID:          project.ID,
		SourceRepositoryID: repoID,
		SourceURL:          sourceURL,
		Status:             MigrationPending,
		RequestedBy:        input.Actor.ID,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := s.repo.CreateMigration(ctx, migration); err != nil {
		return RepositoryMigration{}, err
	}
	if err := s.publish(ctx, "RepositoryMigrationCreated", now, RepositoryMigrationCreatedPayload{MigrationID: migration.ID, SourceRepositoryID: repoID, ProjectID: project.ID}); err != nil {
		return RepositoryMigration{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "repository_migration.create", ResourceType: "repository_migration", ResourceID: migration.ID, Result: "succeeded", Summary: "创建源码仓库迁移任务", OccurredAt: now})
	return migration, nil
}

func (s *Service) GetRepositoryMigration(ctx context.Context, id shared.ID) (RepositoryMigration, error) {
	return s.repo.GetMigration(ctx, id)
}

func (s *Service) RetryRepositoryMigration(ctx context.Context, actor identityaccess.Subject, id shared.ID) (RepositoryMigration, error) {
	migration, err := s.repo.GetMigration(ctx, id)
	if err != nil {
		return RepositoryMigration{}, err
	}
	project := tenantproject.Project{ID: migration.ProjectID, TenantID: migration.TenantID}
	if err := s.checkProject(ctx, actor, project, "project:update"); err != nil {
		return RepositoryMigration{}, err
	}
	if migration.Status != MigrationFailed {
		return RepositoryMigration{}, shared.NewError(shared.CodeFailedPrecondition, "only failed migrations can be retried")
	}
	return s.transitionMigration(ctx, migration, MigrationPending, "")
}

func (s *Service) CancelRepositoryMigration(ctx context.Context, actor identityaccess.Subject, id shared.ID) (RepositoryMigration, error) {
	migration, err := s.repo.GetMigration(ctx, id)
	if err != nil {
		return RepositoryMigration{}, err
	}
	project := tenantproject.Project{ID: migration.ProjectID, TenantID: migration.TenantID}
	if err := s.checkProject(ctx, actor, project, "project:update"); err != nil {
		return RepositoryMigration{}, err
	}
	if terminalMigrationStatus(migration.Status) || migration.Status == MigrationFailed {
		return RepositoryMigration{}, shared.NewError(shared.CodeFailedPrecondition, "migration cannot be canceled in current status")
	}
	updated, err := s.transitionMigration(ctx, migration, MigrationCanceled, "")
	if err != nil {
		return RepositoryMigration{}, err
	}
	repository, err := s.repo.GetSourceRepository(ctx, updated.SourceRepositoryID)
	if err == nil {
		repository.Status = RepositoryStatusFailed
		repository.UpdatedAt = updated.UpdatedAt
		_ = s.repo.UpdateSourceRepository(ctx, repository)
	}
	return updated, nil
}

func (s *Service) ProcessRepositoryMigration(ctx context.Context, id shared.ID) (RepositoryMigration, []BuildSpecSuggestion, error) {
	if s.git == nil {
		return RepositoryMigration{}, nil, shared.NewError(shared.CodeFailedPrecondition, "git source repository port is required")
	}
	migration, err := s.repo.GetMigration(ctx, id)
	if err != nil {
		return RepositoryMigration{}, nil, err
	}
	if terminalMigrationStatus(migration.Status) {
		return migration, nil, nil
	}
	repository, err := s.repo.GetSourceRepository(ctx, migration.SourceRepositoryID)
	if err != nil {
		return RepositoryMigration{}, nil, err
	}

	var suggestions []BuildSpecSuggestion
	if migration.Status == MigrationPending {
		if migration, err = s.transitionMigration(ctx, migration, MigrationCreatingTargetRepo, ""); err != nil {
			return RepositoryMigration{}, nil, err
		}
		gitProject, createErr := s.git.CreateProject(ctx, GitProjectSpec{
			TenantID:       repository.TenantID,
			ProjectID:      repository.ProjectID,
			RepositoryID:   repository.ID,
			RepositoryName: repository.Name,
			DefaultBranch:  repository.DefaultBranch,
		})
		if createErr != nil {
			return s.failMigration(ctx, migration, repository, createErr)
		}
		repository.GitProjectID = gitProject.ID
		repository.HTTPURL = gitProject.HTTPURL
		repository.SSHURL = gitProject.SSHURL
		repository.UpdatedAt = s.clock.Now()
		if err := s.repo.UpdateSourceRepository(ctx, repository); err != nil {
			return RepositoryMigration{}, nil, err
		}
		if err := s.git.ProtectBranch(ctx, gitProject.ID, repository.DefaultBranch); err != nil {
			return s.failMigration(ctx, migration, repository, err)
		}
		if s.webhookCallbackURL != "" {
			if err := s.git.ConfigureWebhook(ctx, gitProject.ID, s.webhookCallbackURL); err != nil {
				return s.failMigration(ctx, migration, repository, err)
			}
		}
	}
	if migration.Status == MigrationCreatingTargetRepo {
		if migration, err = s.transitionMigration(ctx, migration, MigrationCloningSource, ""); err != nil {
			return RepositoryMigration{}, nil, err
		}
	}
	if migration.Status == MigrationCloningSource {
		if err := s.git.MirrorRepository(ctx, GitMirrorSpec{SourceURL: migration.SourceURL, GitProjectID: repository.GitProjectID}); err != nil {
			return s.failMigration(ctx, migration, repository, err)
		}
		if migration, err = s.transitionMigration(ctx, migration, MigrationPushingTarget, ""); err != nil {
			return RepositoryMigration{}, nil, err
		}
	}
	if migration.Status == MigrationPushingTarget {
		if migration, err = s.transitionMigration(ctx, migration, MigrationVerifying, ""); err != nil {
			return RepositoryMigration{}, nil, err
		}
	}
	if migration.Status == MigrationVerifying {
		if err := s.git.VerifyRepository(ctx, repository.GitProjectID); err != nil {
			return s.failMigration(ctx, migration, repository, err)
		}
		if migration, err = s.transitionMigration(ctx, migration, MigrationAnalyzing, ""); err != nil {
			return RepositoryMigration{}, nil, err
		}
	}
	if migration.Status == MigrationAnalyzing {
		files, err := s.git.ListFiles(ctx, repository.GitProjectID, repository.DefaultBranch)
		if err != nil {
			return s.failMigration(ctx, migration, repository, err)
		}
		suggestions = GenerateJavaBuildSpecSuggestions(files)
		if migration, err = s.transitionMigration(ctx, migration, MigrationReadyForApplicationBinding, ""); err != nil {
			return RepositoryMigration{}, nil, err
		}
	}
	if migration.Status == MigrationReadyForApplicationBinding {
		if migration, err = s.transitionMigration(ctx, migration, MigrationSucceeded, ""); err != nil {
			return RepositoryMigration{}, nil, err
		}
		repository.Status = RepositoryStatusReady
		repository.UpdatedAt = migration.UpdatedAt
		if err := s.repo.UpdateSourceRepository(ctx, repository); err != nil {
			return RepositoryMigration{}, nil, err
		}
		_ = s.audit.Log(ctx, AuditEvent{ActorID: migration.RequestedBy, Action: "repository_migration.succeed", ResourceType: "repository_migration", ResourceID: migration.ID, Result: "succeeded", Summary: "源码仓库迁移完成", OccurredAt: migration.UpdatedAt})
	}
	return migration, suggestions, nil
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

func (s *Service) failMigration(ctx context.Context, migration RepositoryMigration, repository SourceRepository, cause error) (RepositoryMigration, []BuildSpecSuggestion, error) {
	message := strings.TrimSpace(cause.Error())
	failed, err := s.transitionMigration(ctx, migration, MigrationFailed, message)
	if err != nil {
		return RepositoryMigration{}, nil, err
	}
	repository.Status = RepositoryStatusFailed
	repository.UpdatedAt = failed.UpdatedAt
	_ = s.repo.UpdateSourceRepository(ctx, repository)
	_ = s.audit.Log(ctx, AuditEvent{ActorID: migration.RequestedBy, Action: "repository_migration.fail", ResourceType: "repository_migration", ResourceID: migration.ID, Result: "failed", Summary: "源码仓库迁移失败", OccurredAt: failed.UpdatedAt})
	return failed, nil, cause
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
