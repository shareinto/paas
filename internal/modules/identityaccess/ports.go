package identityaccess

import (
	"context"
	"time"

	"github.com/shareinto/paas/internal/shared"
)

type Repository interface {
	CreateUser(ctx context.Context, user User) error
	UpdateUser(ctx context.Context, user User) error
	GetUser(ctx context.Context, id shared.ID) (User, error)
	FindUserByUsername(ctx context.Context, username string) (User, error)
	ListUsers(ctx context.Context, page shared.PageRequest) (shared.PageResult[User], error)

	SaveLocalCredential(ctx context.Context, credential LocalCredential) error
	GetLocalCredential(ctx context.Context, userID shared.ID) (LocalCredential, error)

	CreateIdentity(ctx context.Context, identity Identity) error
	FindIdentity(ctx context.Context, provider IdentityProvider, issuer string, subject string) (Identity, error)
	ListIdentitiesByUser(ctx context.Context, userID shared.ID) ([]Identity, error)

	CreateOIDCProvider(ctx context.Context, provider OIDCProvider) error
	GetOIDCProvider(ctx context.Context, id shared.ID) (OIDCProvider, error)
	ListOIDCProviders(ctx context.Context, enabledOnly bool) ([]OIDCProvider, error)

	CreateGroup(ctx context.Context, group Group) error
	AddGroupMember(ctx context.Context, member GroupMember) error
	ListGroupIDsByUser(ctx context.Context, userID shared.ID) ([]shared.ID, error)

	CreateServiceAccount(ctx context.Context, account ServiceAccount) error
	GetServiceAccount(ctx context.Context, id shared.ID) (ServiceAccount, error)

	CreateRoleBinding(ctx context.Context, binding RoleBinding) error
	ListRoleBindingsForSubject(ctx context.Context, subject Subject) ([]RoleBinding, error)

	CreateAccessToken(ctx context.Context, token AccessToken) error
	FindAccessTokenByHash(ctx context.Context, hash string) (AccessToken, error)
	RevokeAccessTokenByHash(ctx context.Context, hash string, revokedAt time.Time) error
}

type AuditLogger interface {
	Log(ctx context.Context, event AuditEvent) error
}

type AuditEvent struct {
	ActorID      shared.ID
	Action       string
	ResourceType string
	ResourceID   shared.ID
	Result       string
	Summary      string
	OccurredAt   time.Time
}

type OIDCVerifier interface {
	VerifyCallback(ctx context.Context, provider OIDCProvider, code string, nonce string) (OIDCClaims, error)
}

type OIDCClaims struct {
	Issuer      string
	Subject     string
	Username    string
	DisplayName string
	Email       string
	AvatarURL   string
}

type PermissionChecker interface {
	Check(ctx context.Context, subject Subject, resource ResourceScope, action Permission) error
}

type AuthService interface {
	LoginLocal(ctx context.Context, username string, password string) (TokenPair, UserDTO, error)
	AuthenticateAccessToken(ctx context.Context, token string) (User, error)
}

type SubjectQuery interface {
	GetUser(ctx context.Context, id shared.ID) (User, error)
	ListIdentitiesByUser(ctx context.Context, userID shared.ID) ([]Identity, error)
}
