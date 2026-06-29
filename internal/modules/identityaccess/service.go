package identityaccess

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/shareinto/paas/internal/shared"
)

const (
	defaultAccessTTL  = 2 * time.Hour
	defaultRefreshTTL = 30 * 24 * time.Hour
)

type Service struct {
	repo                     Repository
	audit                    AuditLogger
	ids                      shared.IDGenerator
	clock                    shared.Clock
	verifier                 OIDCVerifier
	roles                    map[RoleID]Role
	accessTTL                time.Duration
	refreshTTL               time.Duration
	localRegistrationEnabled bool

	stateMu sync.Mutex
	states  map[string]oidcLoginState
}

type oidcLoginState struct {
	ProviderID shared.ID
	Nonce      string
	ExpiresAt  time.Time
}

type Options struct {
	Repository               Repository
	Audit                    AuditLogger
	IDGenerator              shared.IDGenerator
	Clock                    shared.Clock
	Verifier                 OIDCVerifier
	AccessTTL                time.Duration
	RefreshTTL               time.Duration
	LocalRegistrationEnabled *bool
}

func NewService(opts Options) *Service {
	audit := opts.Audit
	if audit == nil {
		audit = NoopAuditLogger{}
	}
	ids := opts.IDGenerator
	if ids == nil {
		ids = shared.RandomIDGenerator{}
	}
	clock := opts.Clock
	if clock == nil {
		clock = shared.SystemClock{}
	}
	accessTTL := opts.AccessTTL
	if accessTTL == 0 {
		accessTTL = defaultAccessTTL
	}
	refreshTTL := opts.RefreshTTL
	if refreshTTL == 0 {
		refreshTTL = defaultRefreshTTL
	}
	localRegistrationEnabled := true
	if opts.LocalRegistrationEnabled != nil {
		localRegistrationEnabled = *opts.LocalRegistrationEnabled
	}
	return &Service{
		repo:                     opts.Repository,
		audit:                    audit,
		ids:                      ids,
		clock:                    clock,
		verifier:                 opts.Verifier,
		roles:                    BuiltInRoles(),
		accessTTL:                accessTTL,
		refreshTTL:               refreshTTL,
		localRegistrationEnabled: localRegistrationEnabled,
		states:                   map[string]oidcLoginState{},
	}
}

type CreateLocalUserInput struct {
	ActorID     shared.ID `json:"actor_id"`
	Username    string    `json:"username"`
	Password    string    `json:"password"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
}

type RegisterLocalUserInput struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}

type UserDTO struct {
	ID          shared.ID `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
	AvatarURL   string    `json:"avatar_url,omitempty"`
	Disabled    bool      `json:"disabled"`
}

type OIDCProviderDTO struct {
	ID          shared.ID `json:"id"`
	Name        string    `json:"name"`
	Issuer      string    `json:"issuer"`
	ClientID    string    `json:"client_id"`
	Scopes      []string  `json:"scopes"`
	RedirectURI string    `json:"redirect_uri"`
	Enabled     bool      `json:"enabled"`
}

type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type OIDCStartResult struct {
	State        string    `json:"state"`
	RedirectURL  string    `json:"redirect_url"`
	NonceExpires time.Time `json:"nonce_expires_at"`
}

type RoleDTO struct {
	ID              RoleID       `json:"id"`
	Name            string       `json:"name"`
	Description     string       `json:"description"`
	BuiltIn         bool         `json:"built_in"`
	Disabled        bool         `json:"disabled"`
	Permissions     []Permission `json:"permissions"`
	SuggestedScopes []ScopeKind  `json:"suggestedScopes"`
}

type UpdateRolePermissionsInput struct {
	Actor       Subject      `json:"actor"`
	Permissions []Permission `json:"permissions"`
}

type CreateRoleInput struct {
	Actor           Subject      `json:"actor"`
	ID              RoleID       `json:"id"`
	Name            string       `json:"name"`
	Description     string       `json:"description"`
	Permissions     []Permission `json:"permissions"`
	SuggestedScopes []ScopeKind  `json:"suggested_scopes"`
}

type UpdateRoleInput struct {
	Actor           Subject     `json:"actor"`
	Name            string      `json:"name"`
	Description     string      `json:"description"`
	Disabled        *bool       `json:"disabled"`
	SuggestedScopes []ScopeKind `json:"suggested_scopes"`
}

func (s *Service) CreateLocalUser(ctx context.Context, input CreateLocalUserInput) (UserDTO, error) {
	user, err := s.createLocalUser(ctx, RegisterLocalUserInput{Username: input.Username, Password: input.Password, DisplayName: input.DisplayName, Email: input.Email})
	if err != nil {
		return UserDTO{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.ActorID, Action: "user.create_local", ResourceType: "user", ResourceID: user.ID, Result: "succeeded", Summary: "创建本地用户", OccurredAt: user.CreatedAt})
	return ToUserDTO(user), nil
}

func (s *Service) RegisterLocal(ctx context.Context, input RegisterLocalUserInput) (TokenPair, UserDTO, error) {
	if !s.localRegistrationEnabled {
		return TokenPair{}, UserDTO{}, shared.NewError(shared.CodeFailedPrecondition, "注册功能未开启")
	}
	user, err := s.createLocalUser(ctx, input)
	if err != nil {
		return TokenPair{}, UserDTO{}, err
	}
	pair, err := s.issueTokenPair(ctx, user.ID)
	if err != nil {
		return TokenPair{}, UserDTO{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: user.ID, Action: "auth.register_local", ResourceType: "user", ResourceID: user.ID, Result: "succeeded", Summary: "自助注册本地用户", OccurredAt: s.clock.Now()})
	return pair, ToUserDTO(user), nil
}

func (s *Service) createLocalUser(ctx context.Context, input RegisterLocalUserInput) (User, error) {
	username := normalizeUsername(input.Username)
	if username == "" || strings.TrimSpace(input.Password) == "" {
		return User{}, shared.NewError(shared.CodeInvalidArgument, "username and password are required")
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, shared.WrapError(shared.CodeInternal, "failed to hash password", err)
	}
	now := s.clock.Now()
	userID, err := s.ids.NewID("usr")
	if err != nil {
		return User{}, err
	}
	identityID, err := s.ids.NewID("idn")
	if err != nil {
		return User{}, err
	}
	user := User{
		ID:          userID,
		Username:    username,
		DisplayName: strings.TrimSpace(input.DisplayName),
		Email:       strings.TrimSpace(input.Email),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.CreateUser(ctx, user); err != nil {
		return User{}, err
	}
	if err := s.repo.SaveLocalCredential(ctx, LocalCredential{UserID: userID, PasswordHash: passwordHash, UpdatedAt: now}); err != nil {
		return User{}, err
	}
	if err := s.repo.CreateIdentity(ctx, Identity{ID: identityID, UserID: userID, Provider: ProviderLocal, Issuer: "local", Subject: username, CreatedAt: now}); err != nil {
		return User{}, err
	}
	return user, nil
}

func (s *Service) DisableUser(ctx context.Context, actorID shared.ID, userID shared.ID) error {
	user, err := s.repo.GetUser(ctx, userID)
	if err != nil {
		return err
	}
	user.Disabled = true
	user.UpdatedAt = s.clock.Now()
	if err := s.repo.UpdateUser(ctx, user); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: actorID, Action: "user.disable", ResourceType: "user", ResourceID: userID, Result: "succeeded", Summary: "禁用用户", OccurredAt: user.UpdatedAt})
	return nil
}

func (s *Service) ResetPassword(ctx context.Context, actorID shared.ID, userID shared.ID, password string) error {
	if strings.TrimSpace(password) == "" {
		return shared.NewError(shared.CodeInvalidArgument, "password is required")
	}
	if _, err := s.repo.GetUser(ctx, userID); err != nil {
		return err
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return shared.WrapError(shared.CodeInternal, "failed to hash password", err)
	}
	now := s.clock.Now()
	if err := s.repo.SaveLocalCredential(ctx, LocalCredential{UserID: userID, PasswordHash: passwordHash, UpdatedAt: now}); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: actorID, Action: "user.reset_password", ResourceType: "user", ResourceID: userID, Result: "succeeded", Summary: "重置本地用户密码", OccurredAt: now})
	return nil
}

func (s *Service) LoginLocal(ctx context.Context, username string, password string) (TokenPair, UserDTO, error) {
	user, err := s.repo.FindUserByUsername(ctx, username)
	if err != nil {
		return TokenPair{}, UserDTO{}, shared.NewError(shared.CodeUnauthenticated, "invalid username or password")
	}
	if user.Disabled {
		return TokenPair{}, UserDTO{}, shared.NewError(shared.CodePermissionDenied, "user is disabled")
	}
	credential, err := s.repo.GetLocalCredential(ctx, user.ID)
	if err != nil {
		return TokenPair{}, UserDTO{}, shared.NewError(shared.CodeUnauthenticated, "invalid username or password")
	}
	if err := bcrypt.CompareHashAndPassword(credential.PasswordHash, []byte(password)); err != nil {
		return TokenPair{}, UserDTO{}, shared.NewError(shared.CodeUnauthenticated, "invalid username or password")
	}
	pair, err := s.issueTokenPair(ctx, user.ID)
	if err != nil {
		return TokenPair{}, UserDTO{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: user.ID, Action: "auth.login_local", ResourceType: "user", ResourceID: user.ID, Result: "succeeded", Summary: "本地账号密码登录", OccurredAt: s.clock.Now()})
	return pair, ToUserDTO(user), nil
}

func (s *Service) CreateOIDCProvider(ctx context.Context, provider OIDCProvider) (OIDCProviderDTO, error) {
	if strings.TrimSpace(provider.Issuer) == "" || strings.TrimSpace(provider.ClientID) == "" || strings.TrimSpace(provider.RedirectURI) == "" {
		return OIDCProviderDTO{}, shared.NewError(shared.CodeInvalidArgument, "issuer, client_id and redirect_uri are required")
	}
	if provider.ID.IsZero() {
		id, err := s.ids.NewID("oidc")
		if err != nil {
			return OIDCProviderDTO{}, err
		}
		provider.ID = id
	}
	now := s.clock.Now()
	provider.CreatedAt = now
	provider.UpdatedAt = now
	if err := s.repo.CreateOIDCProvider(ctx, provider); err != nil {
		return OIDCProviderDTO{}, err
	}
	return ToOIDCProviderDTO(provider), nil
}

func (s *Service) ListOIDCProviders(ctx context.Context) ([]OIDCProviderDTO, error) {
	providers, err := s.repo.ListOIDCProviders(ctx, true)
	if err != nil {
		return nil, err
	}
	dtos := make([]OIDCProviderDTO, 0, len(providers))
	for _, provider := range providers {
		dtos = append(dtos, ToOIDCProviderDTO(provider))
	}
	return dtos, nil
}

func (s *Service) StartOIDC(ctx context.Context, providerID shared.ID) (OIDCStartResult, error) {
	provider, err := s.repo.GetOIDCProvider(ctx, providerID)
	if err != nil {
		return OIDCStartResult{}, err
	}
	if !provider.Enabled {
		return OIDCStartResult{}, shared.NewError(shared.CodeFailedPrecondition, "oidc provider is disabled")
	}
	state, err := randomToken()
	if err != nil {
		return OIDCStartResult{}, err
	}
	nonce, err := randomToken()
	if err != nil {
		return OIDCStartResult{}, err
	}
	expiresAt := s.clock.Now().Add(10 * time.Minute)
	s.stateMu.Lock()
	s.states[state] = oidcLoginState{ProviderID: provider.ID, Nonce: nonce, ExpiresAt: expiresAt}
	s.stateMu.Unlock()

	redirect, err := buildOIDCRedirect(provider, state, nonce)
	if err != nil {
		return OIDCStartResult{}, err
	}
	return OIDCStartResult{State: state, RedirectURL: redirect, NonceExpires: expiresAt}, nil
}

func (s *Service) CallbackOIDC(ctx context.Context, providerID shared.ID, state string, code string) (TokenPair, UserDTO, error) {
	if s.verifier == nil {
		return TokenPair{}, UserDTO{}, shared.NewError(shared.CodeFailedPrecondition, "oidc verifier is not configured")
	}
	loginState, err := s.consumeOIDCState(providerID, state)
	if err != nil {
		return TokenPair{}, UserDTO{}, err
	}
	if strings.TrimSpace(code) == "" {
		return TokenPair{}, UserDTO{}, shared.NewError(shared.CodeInvalidArgument, "code is required")
	}
	provider, err := s.repo.GetOIDCProvider(ctx, providerID)
	if err != nil {
		return TokenPair{}, UserDTO{}, err
	}
	claims, err := s.verifier.VerifyCallback(ctx, provider, code, loginState.Nonce)
	if err != nil {
		return TokenPair{}, UserDTO{}, shared.WrapError(shared.CodeUnauthenticated, "oidc callback verification failed", err)
	}
	if claims.Issuer != provider.Issuer || strings.TrimSpace(claims.Subject) == "" {
		return TokenPair{}, UserDTO{}, shared.NewError(shared.CodeUnauthenticated, "oidc identity is invalid")
	}
	user, err := s.findOrCreateOIDCUser(ctx, provider, claims)
	if err != nil {
		return TokenPair{}, UserDTO{}, err
	}
	if user.Disabled {
		return TokenPair{}, UserDTO{}, shared.NewError(shared.CodePermissionDenied, "user is disabled")
	}
	pair, err := s.issueTokenPair(ctx, user.ID)
	if err != nil {
		return TokenPair{}, UserDTO{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: user.ID, Action: "auth.login_oidc", ResourceType: "user", ResourceID: user.ID, Result: "succeeded", Summary: "企业身份登录", OccurredAt: s.clock.Now()})
	return pair, ToUserDTO(user), nil
}

func (s *Service) AuthenticateAccessToken(ctx context.Context, token string) (User, error) {
	stored, err := s.repo.FindAccessTokenByHash(ctx, hashToken(token))
	if err != nil {
		return User{}, shared.NewError(shared.CodeUnauthenticated, "invalid access token")
	}
	if stored.Kind != TokenKindAccess || stored.RevokedAt != nil || !s.clock.Now().Before(stored.ExpiresAt) {
		return User{}, shared.NewError(shared.CodeUnauthenticated, "invalid access token")
	}
	user, err := s.repo.GetUser(ctx, stored.UserID)
	if err != nil {
		return User{}, shared.NewError(shared.CodeUnauthenticated, "invalid access token")
	}
	if user.Disabled {
		return User{}, shared.NewError(shared.CodePermissionDenied, "user is disabled")
	}
	return user, nil
}

func (s *Service) RefreshAccessToken(ctx context.Context, refreshToken string) (TokenPair, error) {
	stored, err := s.repo.FindAccessTokenByHash(ctx, hashToken(refreshToken))
	if err != nil {
		return TokenPair{}, shared.NewError(shared.CodeUnauthenticated, "invalid refresh token")
	}
	if stored.Kind != TokenKindRefresh || stored.RevokedAt != nil || !s.clock.Now().Before(stored.ExpiresAt) {
		return TokenPair{}, shared.NewError(shared.CodeUnauthenticated, "invalid refresh token")
	}
	user, err := s.repo.GetUser(ctx, stored.UserID)
	if err != nil || user.Disabled {
		return TokenPair{}, shared.NewError(shared.CodeUnauthenticated, "invalid refresh token")
	}
	return s.issueTokenPair(ctx, stored.UserID)
}

func (s *Service) Logout(ctx context.Context, token string) error {
	return s.repo.RevokeAccessTokenByHash(ctx, hashToken(token), s.clock.Now())
}

func (s *Service) Check(ctx context.Context, subject Subject, resource ResourceScope, action Permission) error {
	if err := ValidatePermission(action); err != nil {
		return err
	}
	roles, err := s.effectiveRoles(ctx)
	if err != nil {
		return err
	}
	subjects := []Subject{subject}
	if subject.Type == SubjectUser {
		groupIDs, err := s.repo.ListGroupIDsByUser(ctx, subject.ID)
		if err != nil {
			return err
		}
		for _, groupID := range groupIDs {
			subjects = append(subjects, Subject{Type: SubjectGroup, ID: groupID})
		}
	}
	for _, candidate := range subjects {
		bindings, err := s.repo.ListRoleBindingsForSubject(ctx, candidate)
		if err != nil {
			return err
		}
		for _, binding := range bindings {
			if !ScopeCovers(binding.ScopeKind, binding.ScopeID, resource) {
				continue
			}
			role, ok := roles[binding.RoleID]
			if !ok || role.Disabled {
				continue
			}
			for _, permission := range role.Permissions {
				if PermissionAllows(permission, action) {
					return nil
				}
			}
		}
	}
	return shared.NewError(shared.CodePermissionDenied, "permission denied")
}

func (s *Service) CreateRoleBinding(ctx context.Context, binding RoleBinding) (RoleBinding, error) {
	role, err := s.roleForBinding(ctx, binding.RoleID)
	if err != nil {
		return RoleBinding{}, err
	}
	if role.Disabled {
		return RoleBinding{}, shared.NewError(shared.CodeFailedPrecondition, "role is disabled")
	}
	if binding.SubjectID.IsZero() {
		return RoleBinding{}, shared.NewError(shared.CodeInvalidArgument, "subject_id is required")
	}
	if binding.ID.IsZero() {
		id, err := s.ids.NewID("rb")
		if err != nil {
			return RoleBinding{}, err
		}
		binding.ID = id
	}
	binding.CreatedAt = s.clock.Now()
	if err := s.repo.CreateRoleBinding(ctx, binding); err != nil {
		return RoleBinding{}, err
	}
	return binding, nil
}

func (s *Service) ReplaceRoleBindingForSubjectScope(ctx context.Context, binding RoleBinding) (RoleBinding, error) {
	if binding.SubjectType == "" {
		binding.SubjectType = SubjectUser
	}
	if binding.ScopeKind == "" {
		return RoleBinding{}, shared.NewError(shared.CodeInvalidArgument, "scope_kind is required")
	}
	role, err := s.roleForBinding(ctx, binding.RoleID)
	if err != nil {
		return RoleBinding{}, err
	}
	if role.Disabled {
		return RoleBinding{}, shared.NewError(shared.CodeFailedPrecondition, "role is disabled")
	}
	if binding.SubjectID.IsZero() {
		return RoleBinding{}, shared.NewError(shared.CodeInvalidArgument, "subject_id is required")
	}
	_ = s.repo.DeleteRoleBindingsForSubjectScope(ctx, Subject{Type: binding.SubjectType, ID: binding.SubjectID}, binding.ScopeKind, binding.ScopeID)
	return s.CreateRoleBinding(ctx, binding)
}

func (s *Service) DeleteRoleBindingsForSubjectScope(ctx context.Context, subject Subject, scopeKind ScopeKind, scopeID shared.ID) error {
	if subject.ID.IsZero() || subject.Type == "" || scopeKind == "" {
		return shared.NewError(shared.CodeInvalidArgument, "subject and scope are required")
	}
	return s.repo.DeleteRoleBindingsForSubjectScope(ctx, subject, scopeKind, scopeID)
}

func (s *Service) ListRoleBindingsByScope(ctx context.Context, scopeKind ScopeKind, scopeID shared.ID) ([]RoleBinding, error) {
	return s.repo.ListRoleBindingsByScope(ctx, scopeKind, scopeID)
}

func (s *Service) ListRoleBindingsForSubject(ctx context.Context, subject Subject) ([]RoleBinding, error) {
	if subject.ID.IsZero() || subject.Type == "" {
		return nil, shared.NewError(shared.CodeInvalidArgument, "subject is required")
	}
	return s.repo.ListRoleBindingsForSubject(ctx, subject)
}

func (s *Service) UpdateRolePermissions(ctx context.Context, roleID RoleID, input UpdateRolePermissionsInput) (RoleDTO, error) {
	if input.Actor.ID.IsZero() {
		return RoleDTO{}, shared.NewError(shared.CodeUnauthenticated, "actor is required")
	}
	if err := s.Check(ctx, input.Actor, ResourceScope{Kind: ScopePlatform}, "role:update"); err != nil {
		return RoleDTO{}, err
	}
	roles, err := s.effectiveRoles(ctx)
	if err != nil {
		return RoleDTO{}, err
	}
	role, ok := roles[roleID]
	if !ok {
		return RoleDTO{}, shared.NewError(shared.CodeInvalidArgument, "role is not supported")
	}
	permissions, err := normalizeGrantedPermissions(input.Permissions)
	if err != nil {
		return RoleDTO{}, err
	}
	if err := validateSystemPermissions(permissions); err != nil {
		return RoleDTO{}, err
	}
	if roleID == RolePlatformAdmin && !hasPermission(permissions, "*:*") {
		return RoleDTO{}, shared.NewError(shared.CodeInvalidArgument, "platform admin must keep full access")
	}
	role.Permissions = permissions
	if err := s.repo.UpsertRole(ctx, role); err != nil {
		return RoleDTO{}, err
	}
	if err := s.repo.ReplaceRolePermissions(ctx, role.ID, permissions); err != nil {
		return RoleDTO{}, err
	}
	s.roles[role.ID] = role
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "role.update_permissions", ResourceType: "role", ResourceID: shared.ID(role.ID), Result: "succeeded", Summary: "修改角色权限", OccurredAt: s.clock.Now()})
	return toRoleDTO(role), nil
}

func (s *Service) CreateRole(ctx context.Context, input CreateRoleInput) (RoleDTO, error) {
	if input.Actor.ID.IsZero() {
		return RoleDTO{}, shared.NewError(shared.CodeUnauthenticated, "actor is required")
	}
	if err := s.Check(ctx, input.Actor, ResourceScope{Kind: ScopePlatform}, "role:create"); err != nil {
		return RoleDTO{}, err
	}
	roleID, err := normalizeRoleID(input.ID)
	if err != nil {
		return RoleDTO{}, err
	}
	if _, ok := BuiltInRoles()[roleID]; ok {
		return RoleDTO{}, shared.NewError(shared.CodeInvalidArgument, "built-in role id cannot be reused")
	}
	roles, err := s.effectiveRoles(ctx)
	if err != nil {
		return RoleDTO{}, err
	}
	if _, exists := roles[roleID]; exists {
		return RoleDTO{}, shared.NewError(shared.CodeConflict, "role already exists")
	}
	permissions, err := normalizeGrantedPermissions(input.Permissions)
	if err != nil {
		return RoleDTO{}, err
	}
	if err := validateSystemPermissions(permissions); err != nil {
		return RoleDTO{}, err
	}
	role := Role{
		ID:              roleID,
		Name:            strings.TrimSpace(input.Name),
		Description:     strings.TrimSpace(input.Description),
		BuiltIn:         false,
		Disabled:        false,
		SuggestedScopes: normalizeScopeKinds(input.SuggestedScopes),
		Permissions:     permissions,
	}
	if role.Name == "" {
		return RoleDTO{}, shared.NewError(shared.CodeInvalidArgument, "name is required")
	}
	if err := s.repo.UpsertRole(ctx, role); err != nil {
		return RoleDTO{}, err
	}
	if err := s.repo.ReplaceRolePermissions(ctx, role.ID, permissions); err != nil {
		return RoleDTO{}, err
	}
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "role.create", ResourceType: "role", ResourceID: shared.ID(role.ID), Result: "succeeded", Summary: "创建角色", OccurredAt: s.clock.Now()})
	return toRoleDTO(role), nil
}

func (s *Service) UpdateRole(ctx context.Context, roleID RoleID, input UpdateRoleInput) (RoleDTO, error) {
	if input.Actor.ID.IsZero() {
		return RoleDTO{}, shared.NewError(shared.CodeUnauthenticated, "actor is required")
	}
	if err := s.Check(ctx, input.Actor, ResourceScope{Kind: ScopePlatform}, "role:update"); err != nil {
		return RoleDTO{}, err
	}
	roles, err := s.effectiveRoles(ctx)
	if err != nil {
		return RoleDTO{}, err
	}
	role, ok := roles[roleID]
	if !ok {
		return RoleDTO{}, shared.NewError(shared.CodeNotFound, "role not found")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return RoleDTO{}, shared.NewError(shared.CodeInvalidArgument, "name is required")
	}
	if role.BuiltIn && input.Disabled != nil && *input.Disabled {
		return RoleDTO{}, shared.NewError(shared.CodeFailedPrecondition, "built-in role cannot be disabled")
	}
	role.Name = name
	role.Description = strings.TrimSpace(input.Description)
	role.SuggestedScopes = normalizeScopeKinds(input.SuggestedScopes)
	if input.Disabled != nil {
		role.Disabled = *input.Disabled
	}
	if role.BuiltIn {
		role.Disabled = false
	}
	if err := s.repo.UpsertRole(ctx, role); err != nil {
		return RoleDTO{}, err
	}
	s.roles[role.ID] = role
	_ = s.audit.Log(ctx, AuditEvent{ActorID: input.Actor.ID, Action: "role.update", ResourceType: "role", ResourceID: shared.ID(role.ID), Result: "succeeded", Summary: "修改角色", OccurredAt: s.clock.Now()})
	return toRoleDTO(role), nil
}

func (s *Service) DeleteRole(ctx context.Context, roleID RoleID, actor Subject) error {
	if actor.ID.IsZero() {
		return shared.NewError(shared.CodeUnauthenticated, "actor is required")
	}
	if err := s.Check(ctx, actor, ResourceScope{Kind: ScopePlatform}, "role:delete"); err != nil {
		return err
	}
	roles, err := s.effectiveRoles(ctx)
	if err != nil {
		return err
	}
	role, ok := roles[roleID]
	if !ok {
		return shared.NewError(shared.CodeNotFound, "role not found")
	}
	if role.BuiltIn {
		return shared.NewError(shared.CodeFailedPrecondition, "built-in role cannot be deleted")
	}
	count, err := s.repo.RoleBindingCountByRole(ctx, roleID)
	if err != nil {
		return err
	}
	if count > 0 {
		return shared.NewError(shared.CodeFailedPrecondition, "role is still bound to subjects")
	}
	if err := s.repo.DeleteRole(ctx, roleID); err != nil {
		return err
	}
	delete(s.roles, roleID)
	_ = s.audit.Log(ctx, AuditEvent{ActorID: actor.ID, Action: "role.delete", ResourceType: "role", ResourceID: shared.ID(roleID), Result: "succeeded", Summary: "删除角色", OccurredAt: s.clock.Now()})
	return nil
}

func (s *Service) ListRoles(ctx context.Context) ([]RoleDTO, error) {
	effective, err := s.effectiveRoles(ctx)
	if err != nil {
		return nil, err
	}
	roles := make([]RoleDTO, 0, len(effective))
	for _, roleID := range orderedRoleIDs(effective) {
		roles = append(roles, toRoleDTO(effective[roleID]))
	}
	return roles, nil
}

func (s *Service) ListPermissions(ctx context.Context) ([]Permission, error) {
	seen := map[Permission]bool{}
	for _, permission := range SystemPermissions() {
		seen[permission] = true
	}
	items := make([]Permission, 0, len(seen))
	for permission := range seen {
		items = append(items, permission)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i] < items[j]
	})
	return items, nil
}

func (s *Service) GetUser(ctx context.Context, id shared.ID) (User, error) {
	return s.repo.GetUser(ctx, id)
}

func (s *Service) ListUsers(ctx context.Context, page shared.PageRequest) (shared.PageResult[UserDTO], error) {
	result, err := s.repo.ListUsers(ctx, page)
	if err != nil {
		return shared.PageResult[UserDTO]{}, err
	}
	items := make([]UserDTO, 0, len(result.Items))
	for _, user := range result.Items {
		items = append(items, ToUserDTO(user))
	}
	return shared.NewPageResult(items, result.Total, page), nil
}

func (s *Service) ListIdentitiesByUser(ctx context.Context, userID shared.ID) ([]Identity, error) {
	return s.repo.ListIdentitiesByUser(ctx, userID)
}

func (s *Service) issueTokenPair(ctx context.Context, userID shared.ID) (TokenPair, error) {
	access, err := randomToken()
	if err != nil {
		return TokenPair{}, err
	}
	refresh, err := randomToken()
	if err != nil {
		return TokenPair{}, err
	}
	now := s.clock.Now()
	accessID, err := s.ids.NewID("tok")
	if err != nil {
		return TokenPair{}, err
	}
	refreshID, err := s.ids.NewID("tok")
	if err != nil {
		return TokenPair{}, err
	}
	accessExpiresAt := now.Add(s.accessTTL)
	if err := s.repo.CreateAccessToken(ctx, AccessToken{ID: accessID, UserID: userID, Kind: TokenKindAccess, TokenHash: hashToken(access), ExpiresAt: accessExpiresAt, CreatedAt: now}); err != nil {
		return TokenPair{}, err
	}
	if err := s.repo.CreateAccessToken(ctx, AccessToken{ID: refreshID, UserID: userID, Kind: TokenKindRefresh, TokenHash: hashToken(refresh), ExpiresAt: now.Add(s.refreshTTL), CreatedAt: now}); err != nil {
		return TokenPair{}, err
	}
	return TokenPair{AccessToken: access, RefreshToken: refresh, ExpiresAt: accessExpiresAt}, nil
}

func (s *Service) consumeOIDCState(providerID shared.ID, state string) (oidcLoginState, error) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	loginState, ok := s.states[state]
	if !ok {
		return oidcLoginState{}, shared.NewError(shared.CodeUnauthenticated, "oidc state is invalid")
	}
	delete(s.states, state)
	if loginState.ProviderID != providerID || !s.clock.Now().Before(loginState.ExpiresAt) {
		return oidcLoginState{}, shared.NewError(shared.CodeUnauthenticated, "oidc state is invalid")
	}
	return loginState, nil
}

func (s *Service) findOrCreateOIDCUser(ctx context.Context, provider OIDCProvider, claims OIDCClaims) (User, error) {
	identity, err := s.repo.FindIdentity(ctx, ProviderOIDC, claims.Issuer, claims.Subject)
	if err == nil {
		return s.repo.GetUser(ctx, identity.UserID)
	}
	if shared.CodeOf(err) != shared.CodeNotFound {
		return User{}, err
	}
	now := s.clock.Now()
	userID, err := s.ids.NewID("usr")
	if err != nil {
		return User{}, err
	}
	identityID, err := s.ids.NewID("idn")
	if err != nil {
		return User{}, err
	}
	username := normalizeUsername(claims.Username)
	if username == "" {
		username = normalizeUsername(claims.Email)
	}
	if username == "" {
		username = normalizeUsername(provider.ID.String() + "_" + claims.Subject)
	}
	user := User{
		ID:          userID,
		Username:    username,
		DisplayName: strings.TrimSpace(claims.DisplayName),
		Email:       strings.TrimSpace(claims.Email),
		AvatarURL:   strings.TrimSpace(claims.AvatarURL),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.CreateUser(ctx, user); err != nil {
		return User{}, err
	}
	if err := s.repo.CreateIdentity(ctx, Identity{ID: identityID, UserID: userID, Provider: ProviderOIDC, Issuer: claims.Issuer, Subject: claims.Subject, CreatedAt: now}); err != nil {
		return User{}, err
	}
	return user, nil
}

func ToUserDTO(user User) UserDTO {
	return UserDTO{ID: user.ID, Username: user.Username, DisplayName: user.DisplayName, Email: user.Email, AvatarURL: user.AvatarURL, Disabled: user.Disabled}
}

func ToOIDCProviderDTO(provider OIDCProvider) OIDCProviderDTO {
	return OIDCProviderDTO{ID: provider.ID, Name: provider.Name, Issuer: provider.Issuer, ClientID: provider.ClientID, Scopes: append([]string(nil), provider.Scopes...), RedirectURI: provider.RedirectURI, Enabled: provider.Enabled}
}

func (s *Service) effectiveRoles(ctx context.Context) (map[RoleID]Role, error) {
	roles := cloneRoles(s.roles)
	if len(roles) == 0 {
		roles = cloneRoles(BuiltInRoles())
	}
	stored, err := s.repo.ListRoles(ctx)
	if err != nil {
		return nil, err
	}
	for _, role := range stored {
		if existing, ok := roles[role.ID]; ok {
			if role.Name == "" {
				role.Name = existing.Name
			}
			if role.Description == "" {
				role.Description = existing.Description
			}
			if len(role.SuggestedScopes) == 0 {
				role.SuggestedScopes = append([]ScopeKind(nil), existing.SuggestedScopes...)
			}
			if existing.BuiltIn {
				role.BuiltIn = true
				role.Disabled = false
			}
		}
		role.Permissions = append([]Permission(nil), role.Permissions...)
		roles[role.ID] = role
	}
	return roles, nil
}

func cloneRoles(input map[RoleID]Role) map[RoleID]Role {
	roles := make(map[RoleID]Role, len(input))
	for id, role := range input {
		role.Permissions = append([]Permission(nil), role.Permissions...)
		role.SuggestedScopes = append([]ScopeKind(nil), role.SuggestedScopes...)
		roles[id] = role
	}
	return roles
}

func orderedRoleIDs(roles map[RoleID]Role) []RoleID {
	order := []RoleID{RolePlatformAdmin, RoleTenantOwner, RoleTenantAdmin, RoleProjectAdmin, RoleDeveloper, RoleViewer, RoleOperator, RoleProdApprover, RoleSecurityAuditor}
	seen := map[RoleID]bool{}
	result := make([]RoleID, 0, len(roles))
	for _, roleID := range order {
		if _, ok := roles[roleID]; ok {
			result = append(result, roleID)
			seen[roleID] = true
		}
	}
	for roleID := range roles {
		if !seen[roleID] {
			result = append(result, roleID)
		}
	}
	prefix := orderedBuiltInPrefix(result)
	offset := prefix
	sort.SliceStable(result[prefix:], func(i, j int) bool {
		return result[offset+i] < result[offset+j]
	})
	return result
}

func orderedBuiltInPrefix(ids []RoleID) int {
	count := 0
	for _, roleID := range ids {
		switch roleID {
		case RolePlatformAdmin, RoleTenantOwner, RoleTenantAdmin, RoleProjectAdmin, RoleDeveloper, RoleViewer, RoleOperator, RoleProdApprover, RoleSecurityAuditor:
			count++
		default:
			return count
		}
	}
	return count
}

func (s *Service) roleForBinding(ctx context.Context, roleID RoleID) (Role, error) {
	roles, err := s.effectiveRoles(ctx)
	if err != nil {
		return Role{}, err
	}
	role, ok := roles[roleID]
	if !ok {
		return Role{}, shared.NewError(shared.CodeInvalidArgument, "role is not supported")
	}
	return role, nil
}

func normalizeRoleID(roleID RoleID) (RoleID, error) {
	value := strings.TrimSpace(string(roleID))
	if value == "" {
		return "", shared.NewError(shared.CodeInvalidArgument, "role id is required")
	}
	if len(value) > 64 {
		return "", shared.NewError(shared.CodeInvalidArgument, "role id is too long")
	}
	for i, r := range value {
		valid := r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '-'
		if !valid || i == 0 && !(r >= 'a' && r <= 'z') {
			return "", shared.NewError(shared.CodeInvalidArgument, "role id must use lowercase letters, numbers, underscore or dash")
		}
	}
	return RoleID(value), nil
}

func normalizeScopeKinds(input []ScopeKind) []ScopeKind {
	allowed := map[ScopeKind]bool{
		ScopePlatform:    true,
		ScopeTenant:      true,
		ScopeProject:     true,
		ScopeApplication: true,
		ScopeStage:       true,
	}
	seen := map[ScopeKind]bool{}
	result := make([]ScopeKind, 0, len(input))
	for _, scope := range input {
		if !allowed[scope] || seen[scope] {
			continue
		}
		seen[scope] = true
		result = append(result, scope)
	}
	return result
}

func normalizeGrantedPermissions(input []Permission) ([]Permission, error) {
	seen := map[Permission]bool{}
	permissions := make([]Permission, 0, len(input))
	for _, permission := range input {
		permission = Permission(strings.TrimSpace(string(permission)))
		if permission == "" || seen[permission] {
			continue
		}
		if err := ValidateGrantedPermission(permission); err != nil {
			return nil, err
		}
		seen[permission] = true
		permissions = append(permissions, permission)
	}
	sort.Slice(permissions, func(i, j int) bool {
		return permissions[i] < permissions[j]
	})
	return permissions, nil
}

func validateSystemPermissions(input []Permission) error {
	allowed := map[Permission]bool{}
	for _, permission := range SystemPermissions() {
		allowed[permission] = true
	}
	for _, permission := range input {
		if !allowed[permission] {
			return shared.NewError(shared.CodeInvalidArgument, "permission is not supported")
		}
	}
	return nil
}

func SystemPermissions() []Permission {
	seen := map[Permission]bool{}
	for _, role := range BuiltInRoles() {
		for _, permission := range role.Permissions {
			if permission != "*:*" {
				seen[permission] = true
			}
		}
	}
	for _, permission := range []Permission{
		"role:create", "role:read", "role:update", "role:delete",
		"user:create", "user:read", "user:update",
		"deployment:verify",
		"runtime:logs", "runtime:terminal", "runtime:restart", "runtime:read",
		"workload:read", "workload:update", "workload:configure",
	} {
		seen[permission] = true
	}
	items := make([]Permission, 0, len(seen)+1)
	items = append(items, "*:*")
	for permission := range seen {
		items = append(items, permission)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i] < items[j]
	})
	return items
}

func hasPermission(permissions []Permission, required Permission) bool {
	for _, permission := range permissions {
		if permission == required {
			return true
		}
	}
	return false
}

func toRoleDTO(role Role) RoleDTO {
	return RoleDTO{
		ID:              role.ID,
		Name:            role.Name,
		Description:     role.Description,
		BuiltIn:         role.BuiltIn,
		Disabled:        role.Disabled,
		Permissions:     append([]Permission(nil), role.Permissions...),
		SuggestedScopes: append([]ScopeKind(nil), role.SuggestedScopes...),
	}
}

func buildOIDCRedirect(provider OIDCProvider, state string, nonce string) (string, error) {
	baseURL, err := url.Parse(strings.TrimRight(provider.Issuer, "/") + "/authorize")
	if err != nil {
		return "", err
	}
	values := baseURL.Query()
	values.Set("response_type", "code")
	values.Set("client_id", provider.ClientID)
	values.Set("redirect_uri", provider.RedirectURI)
	values.Set("scope", strings.Join(provider.Scopes, " "))
	values.Set("state", state)
	values.Set("nonce", nonce)
	baseURL.RawQuery = values.Encode()
	return baseURL.String(), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func randomToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}
