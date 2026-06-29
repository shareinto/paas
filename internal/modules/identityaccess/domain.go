package identityaccess

import (
	"regexp"
	"strings"
	"time"

	"github.com/shareinto/paas/internal/shared"
)

type IdentityProvider string

const (
	ProviderLocal IdentityProvider = "local"
	ProviderOIDC  IdentityProvider = "oidc"
)

type User struct {
	ID          shared.ID
	Username    string
	DisplayName string
	Email       string
	AvatarURL   string
	Disabled    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

type Identity struct {
	ID        shared.ID
	UserID    shared.ID
	Provider  IdentityProvider
	Issuer    string
	Subject   string
	CreatedAt time.Time
}

type LocalCredential struct {
	UserID       shared.ID
	PasswordHash []byte
	UpdatedAt    time.Time
}

type OIDCProvider struct {
	ID              shared.ID
	Name            string
	Issuer          string
	ClientID        string
	ClientSecretRef string
	Scopes          []string
	RedirectURI     string
	Enabled         bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Group struct {
	ID        shared.ID
	Name      string
	CreatedAt time.Time
}

type GroupMember struct {
	GroupID shared.ID
	UserID  shared.ID
}

type ServiceAccount struct {
	ID        shared.ID
	Name      string
	Disabled  bool
	CreatedAt time.Time
}

type Permission string

var permissionPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*:[a-z][a-z0-9_-]*$`)

func ValidatePermission(permission Permission) error {
	if !permissionPattern.MatchString(string(permission)) {
		return shared.NewError(shared.CodeInvalidArgument, "permission must use resource:action format")
	}
	return nil
}

func ValidateGrantedPermission(permission Permission) error {
	if permission == "*:*" {
		return nil
	}
	if strings.HasSuffix(string(permission), ":*") && permissionPattern.MatchString(strings.TrimSuffix(string(permission), "*")+"read") {
		return nil
	}
	return ValidatePermission(permission)
}

type RoleID string

const (
	RolePlatformAdmin    RoleID = "platform_admin"
	RoleTenantOwner      RoleID = "tenant_owner"
	RoleTenantAdmin      RoleID = "tenant_admin"
	RoleProjectAdmin     RoleID = "project_admin"
	RoleApplicationAdmin RoleID = "application_admin"
	RoleDeveloper        RoleID = "developer"
	RoleViewer           RoleID = "viewer"
	RoleOperator         RoleID = "operator"
	RoleProdApprover     RoleID = "prod_approver"
	RoleSecurityAuditor  RoleID = "security_auditor"
)

type Role struct {
	ID              RoleID
	Name            string
	Description     string
	BuiltIn         bool
	Disabled        bool
	SuggestedScopes []ScopeKind
	Permissions     []Permission
}

func BuiltInRoles() map[RoleID]Role {
	return map[RoleID]Role{
		RolePlatformAdmin: {
			ID:              RolePlatformAdmin,
			Name:            "平台管理员",
			Description:     "平台全局最高权限，负责租户、项目、应用和平台配置。",
			BuiltIn:         true,
			SuggestedScopes: []ScopeKind{ScopePlatform},
			Permissions:     []Permission{"*:*"},
		},
		RoleTenantOwner: {
			ID:              RoleTenantOwner,
			Name:            "租户所有者",
			Description:     "租户内最高权限，负责租户配置、项目和应用管理。",
			BuiltIn:         true,
			SuggestedScopes: []ScopeKind{ScopeTenant},
			Permissions:     []Permission{"tenant:update", "project:update", "cluster:read", "cluster:manage", "application:create", "application:read", "build:create", "build:read", "build:cancel", "freight:create", "freight:delete", "deployment:create", "deployment:approve", "runtime:read", "runtime:restart", "audit:read"},
		},
		RoleTenantAdmin: {
			ID:              RoleTenantAdmin,
			Name:            "租户管理员",
			Description:     "租户管理角色，可管理项目、应用和集群配置。",
			BuiltIn:         true,
			SuggestedScopes: []ScopeKind{ScopeTenant},
			Permissions:     []Permission{"tenant:update", "project:update", "cluster:read", "cluster:manage", "application:create", "application:read", "build:create", "build:read", "build:cancel", "freight:create", "freight:delete", "deployment:create", "runtime:read", "runtime:restart"},
		},
		RoleProjectAdmin: {
			ID:              RoleProjectAdmin,
			Name:            "项目管理员",
			Description:     "项目内应用创建、应用配置和发布管理角色。",
			BuiltIn:         true,
			SuggestedScopes: []ScopeKind{ScopeProject},
			Permissions:     []Permission{"project:update", "application:create", "application:read", "application:update", "build:create", "build:read", "build:cancel", "freight:create", "freight:delete", "deployment:create", "runtime:read", "runtime:restart"},
		},
		RoleApplicationAdmin: {
			ID:              RoleApplicationAdmin,
			Name:            "应用管理员",
			Description:     "应用内最高权限，负责应用、构建、发布、准出和运行态管理。",
			BuiltIn:         true,
			SuggestedScopes: []ScopeKind{ScopeApplication},
			Permissions:     []Permission{"application:read", "application:update", "build:create", "build:read", "build:cancel", "freight:create", "freight:delete", "deployment:create", "deployment:approve", "runtime:read", "runtime:restart"},
		},
		RoleDeveloper: {
			ID:              RoleDeveloper,
			Name:            "开发者",
			Description:     "开发成员，可配置应用、触发构建并发起发布。",
			BuiltIn:         true,
			SuggestedScopes: []ScopeKind{ScopeProject, ScopeApplication},
			Permissions:     []Permission{"application:create", "application:read", "application:update", "build:create", "build:read", "build:cancel", "freight:create", "freight:delete", "deployment:create", "runtime:read"},
		},
		RoleViewer: {
			ID:              RoleViewer,
			Name:            "只读成员",
			Description:     "只读查看应用、构建、发布和运行态信息。",
			BuiltIn:         true,
			SuggestedScopes: []ScopeKind{ScopeTenant, ScopeProject, ScopeApplication},
			Permissions:     []Permission{"application:read", "stage:read", "build:read", "deployment:read", "runtime:read"},
		},
		RoleOperator: {
			ID:              RoleOperator,
			Name:            "运维人员",
			Description:     "应用运行态运维角色，可查看运行态、重启和进入终端。",
			BuiltIn:         true,
			SuggestedScopes: []ScopeKind{ScopeApplication, ScopeStage},
			Permissions:     []Permission{"stage:read", "stage:update", "deployment:create", "deployment:rollback", "build:read", "runtime:read", "runtime:restart", "runtime:terminal"},
		},
		RoleProdApprover: {
			ID:              RoleProdApprover,
			Name:            "生产审批人",
			Description:     "生产发布前审批或准出负责人。",
			BuiltIn:         true,
			SuggestedScopes: []ScopeKind{ScopeStage},
			Permissions:     []Permission{"deployment:approve", "deployment:read", "runtime:read"},
		},
		RoleSecurityAuditor: {
			ID:              RoleSecurityAuditor,
			Name:            "安全审计员",
			Description:     "安全和审计查看角色。",
			BuiltIn:         true,
			SuggestedScopes: []ScopeKind{ScopePlatform, ScopeTenant},
			Permissions:     []Permission{"audit:read", "application:read", "deployment:read", "runtime:read"},
		},
	}
}

type SubjectType string

const (
	SubjectUser           SubjectType = "user"
	SubjectGroup          SubjectType = "group"
	SubjectServiceAccount SubjectType = "service_account"
)

type Subject struct {
	Type SubjectType
	ID   shared.ID
}

type ScopeKind string

const (
	ScopePlatform    ScopeKind = "platform"
	ScopeTenant      ScopeKind = "tenant"
	ScopeProject     ScopeKind = "project"
	ScopeApplication ScopeKind = "application"
	ScopeStage       ScopeKind = "stage"
)

type ResourceScope struct {
	Kind          ScopeKind
	TenantID      shared.ID
	ProjectID     shared.ID
	ApplicationID shared.ID
	StageKey      shared.ID
}

type RoleBinding struct {
	ID          shared.ID   `json:"id"`
	SubjectType SubjectType `json:"subject_type"`
	SubjectID   shared.ID   `json:"subject_id"`
	RoleID      RoleID      `json:"role_id"`
	ScopeKind   ScopeKind   `json:"scope_kind"`
	ScopeID     shared.ID   `json:"scope_id"`
	CreatedAt   time.Time   `json:"created_at"`
}

type TokenKind string

const (
	TokenKindAccess  TokenKind = "access"
	TokenKindRefresh TokenKind = "refresh"
)

type AccessToken struct {
	ID        shared.ID
	UserID    shared.ID
	Kind      TokenKind
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	CreatedAt time.Time
}

func ScopeCovers(bindingKind ScopeKind, bindingID shared.ID, resource ResourceScope) bool {
	switch bindingKind {
	case ScopePlatform:
		return true
	case ScopeTenant:
		return !resource.TenantID.IsZero() && bindingID == resource.TenantID
	case ScopeProject:
		return !resource.ProjectID.IsZero() && bindingID == resource.ProjectID
	case ScopeApplication:
		return !resource.ApplicationID.IsZero() && bindingID == resource.ApplicationID
	case ScopeStage:
		return !resource.StageKey.IsZero() && bindingID == resource.StageKey
	default:
		return false
	}
}

func PermissionAllows(granted Permission, required Permission) bool {
	if granted == "*:*" {
		return true
	}
	if granted == required {
		return true
	}
	parts := strings.SplitN(string(granted), ":", 2)
	requiredParts := strings.SplitN(string(required), ":", 2)
	return len(parts) == 2 && len(requiredParts) == 2 && parts[0] == requiredParts[0] && parts[1] == "*"
}
