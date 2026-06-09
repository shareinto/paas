package sourcerepository

import (
	"regexp"
	"strings"
	"time"

	"github.com/shareinto/paas/internal/shared"
)

var repositoryNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{1,62}$`)

type RepositoryStatus string

const (
	RepositoryStatusProvisioning RepositoryStatus = "provisioning"
	RepositoryStatusReady        RepositoryStatus = "ready"
	RepositoryStatusMigrating    RepositoryStatus = "migrating"
	RepositoryStatusFailed       RepositoryStatus = "failed"
	RepositoryStatusDisabled     RepositoryStatus = "disabled"
)

type SourceRepository struct {
	ID            shared.ID        `json:"id"`
	TenantID      shared.ID        `json:"tenant_id"`
	ProjectID     shared.ID        `json:"project_id"`
	Name          string           `json:"name"`
	DisplayName   string           `json:"display_name"`
	Description   string           `json:"description"`
	GitProvider   string           `json:"git_provider"`
	GitProjectID  string           `json:"git_project_id"`
	HTTPURL       string           `json:"http_url"`
	SSHURL        string           `json:"ssh_url"`
	DefaultBranch string           `json:"default_branch"`
	Status        RepositoryStatus `json:"status"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
}

type RepositoryMigrationStatus string

const (
	MigrationPending                    RepositoryMigrationStatus = "pending"
	MigrationCreatingTargetRepo         RepositoryMigrationStatus = "creating_target_repo"
	MigrationCloningSource              RepositoryMigrationStatus = "cloning_source"
	MigrationPushingTarget              RepositoryMigrationStatus = "pushing_target"
	MigrationVerifying                  RepositoryMigrationStatus = "verifying"
	MigrationAnalyzing                  RepositoryMigrationStatus = "analyzing"
	MigrationReadyForApplicationBinding RepositoryMigrationStatus = "ready_for_application_binding"
	MigrationSucceeded                  RepositoryMigrationStatus = "succeeded"
	MigrationFailed                     RepositoryMigrationStatus = "failed"
	MigrationCanceled                   RepositoryMigrationStatus = "canceled"
)

type RepositoryMigration struct {
	ID                 shared.ID                 `json:"id"`
	TenantID           shared.ID                 `json:"tenant_id"`
	ProjectID          shared.ID                 `json:"project_id"`
	SourceRepositoryID shared.ID                 `json:"source_repository_id"`
	SourceURL          string                    `json:"source_url"`
	Status             RepositoryMigrationStatus `json:"status"`
	ErrorMessage       string                    `json:"error_message"`
	RequestedBy        shared.ID                 `json:"requested_by"`
	CreatedAt          time.Time                 `json:"created_at"`
	UpdatedAt          time.Time                 `json:"updated_at"`
	CompletedAt        *time.Time                `json:"completed_at,omitempty"`
}

type PermissionSyncStatus string

const (
	PermissionSyncPending   PermissionSyncStatus = "pending"
	PermissionSyncRunning   PermissionSyncStatus = "running"
	PermissionSyncSucceeded PermissionSyncStatus = "succeeded"
	PermissionSyncFailed    PermissionSyncStatus = "failed"
)

type RepositoryPermissionSyncJob struct {
	ID                 shared.ID            `json:"id"`
	TenantID           shared.ID            `json:"tenant_id"`
	ProjectID          shared.ID            `json:"project_id"`
	SourceRepositoryID shared.ID            `json:"source_repository_id"`
	Status             PermissionSyncStatus `json:"status"`
	ErrorMessage       string               `json:"error_message"`
	RequestedBy        shared.ID            `json:"requested_by"`
	CreatedAt          time.Time            `json:"created_at"`
	UpdatedAt          time.Time            `json:"updated_at"`
}

type AssociatedApplication struct {
	ID          shared.ID `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name"`
}

type RepositoryTreeItem struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
}

type RepositoryBranch struct {
	Name    string `json:"name"`
	Default bool   `json:"default"`
}

type BuildSpecSuggestion struct {
	SourcePath          string   `json:"source_path"`
	BuildCommand        string   `json:"build_command"`
	ArtifactCopyCommand string   `json:"artifact_copy_command"`
	RuntimeBaseImage    string   `json:"runtime_base_image"`
	Evidence            []string `json:"evidence"`
}

type RepositoryCreatedPayload struct {
	SourceRepositoryID shared.ID `json:"source_repository_id"`
	TenantID           shared.ID `json:"tenant_id"`
	ProjectID          shared.ID `json:"project_id"`
	Name               string    `json:"name"`
}

type RepositoryDeletedPayload struct {
	SourceRepositoryID shared.ID `json:"source_repository_id"`
	TenantID           shared.ID `json:"tenant_id"`
	ProjectID          shared.ID `json:"project_id"`
	Name               string    `json:"name"`
}

type RepositoryMigrationCreatedPayload struct {
	MigrationID        shared.ID `json:"migration_id"`
	SourceRepositoryID shared.ID `json:"source_repository_id"`
	ProjectID          shared.ID `json:"project_id"`
}

func normalizeRepositoryName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func validateRepositoryName(name string) error {
	if !repositoryNamePattern.MatchString(name) {
		return shared.NewError(shared.CodeInvalidArgument, "repository name must start with a lowercase letter and contain lowercase letters, numbers or hyphens")
	}
	return nil
}

func normalizeDisplayName(name string, fallback string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return fallback
	}
	return name
}

func canTransitionMigration(from RepositoryMigrationStatus, to RepositoryMigrationStatus) bool {
	if from == to {
		return true
	}
	switch from {
	case MigrationPending:
		return to == MigrationCreatingTargetRepo || to == MigrationCanceled || to == MigrationFailed
	case MigrationCreatingTargetRepo:
		return to == MigrationCloningSource || to == MigrationFailed
	case MigrationCloningSource:
		return to == MigrationPushingTarget || to == MigrationFailed
	case MigrationPushingTarget:
		return to == MigrationVerifying || to == MigrationFailed
	case MigrationVerifying:
		return to == MigrationAnalyzing || to == MigrationFailed
	case MigrationAnalyzing:
		return to == MigrationReadyForApplicationBinding || to == MigrationFailed
	case MigrationReadyForApplicationBinding:
		return to == MigrationSucceeded || to == MigrationFailed
	case MigrationFailed:
		return to == MigrationPending
	default:
		return false
	}
}

func terminalMigrationStatus(status RepositoryMigrationStatus) bool {
	return status == MigrationSucceeded || status == MigrationCanceled
}
