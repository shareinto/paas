package tenantproject

import (
	"regexp"
	"strings"
	"time"

	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/shared"
)

var resourceNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{1,62}$`)

type Tenant struct {
	ID          shared.ID `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TenantMember struct {
	TenantID  shared.ID             `json:"tenant_id"`
	UserID    shared.ID             `json:"user_id"`
	RoleID    identityaccess.RoleID `json:"role_id"`
	CreatedAt time.Time             `json:"created_at"`
	UpdatedAt time.Time             `json:"updated_at"`
}

type Project struct {
	ID          shared.ID `json:"id"`
	TenantID    shared.ID `json:"tenant_id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type TenantCreatedPayload struct {
	TenantID shared.ID `json:"tenant_id"`
	Name     string    `json:"name"`
}

type ProjectCreatedPayload struct {
	ProjectID shared.ID `json:"project_id"`
	TenantID  shared.ID `json:"tenant_id"`
	Name      string    `json:"name"`
}

type ProjectDeletedPayload struct {
	ProjectID shared.ID `json:"project_id"`
	TenantID  shared.ID `json:"tenant_id"`
	Name      string    `json:"name"`
}

func normalizeResourceName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func validateResourceName(name string) error {
	if !resourceNamePattern.MatchString(name) {
		return shared.NewError(shared.CodeInvalidArgument, "name must start with a lowercase letter and contain lowercase letters, numbers or hyphens")
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
