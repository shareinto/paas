package identityaccess

import (
	"context"
	"database/sql"
	"time"

	"github.com/shareinto/paas/internal/platform/database"
	"github.com/shareinto/paas/internal/shared"
)

type MySQLRepository struct {
	inner *MemoryRepository
	store *database.SnapshotStore
}

type identitySnapshot struct {
	Users           []User
	Credentials     []LocalCredential
	Identities      []Identity
	OIDCProviders   []OIDCProvider
	Groups          []Group
	GroupMembers    []GroupMember
	ServiceAccounts []ServiceAccount
	RoleBindings    []RoleBinding
	Tokens          []AccessToken
}

func NewMySQLRepository(ctx context.Context, db *sql.DB) (*MySQLRepository, error) {
	repo := &MySQLRepository{inner: NewMemoryRepository(), store: database.NewSnapshotStore(db, "identity-access")}
	var snapshot identitySnapshot
	if err := repo.store.Load(ctx, &snapshot); err != nil {
		return nil, err
	}
	repo.restore(snapshot)
	return repo, nil
}

func (r *MySQLRepository) restore(snapshot identitySnapshot) {
	r.inner.mu.Lock()
	defer r.inner.mu.Unlock()
	for _, user := range snapshot.Users {
		r.inner.users[user.ID] = user
		r.inner.usersByUsername[normalizeUsername(user.Username)] = user.ID
	}
	for _, credential := range snapshot.Credentials {
		r.inner.credentials[credential.UserID] = credential
	}
	for _, identity := range snapshot.Identities {
		r.inner.identities[identityKey(identity.Provider, identity.Issuer, identity.Subject)] = identity
		r.inner.identitiesByUser[identity.UserID] = append(r.inner.identitiesByUser[identity.UserID], identity)
	}
	for _, provider := range snapshot.OIDCProviders {
		r.inner.oidcProviders[provider.ID] = provider
	}
	for _, group := range snapshot.Groups {
		r.inner.groups[group.ID] = group
	}
	for _, member := range snapshot.GroupMembers {
		if r.inner.groupMembers[member.GroupID] == nil {
			r.inner.groupMembers[member.GroupID] = map[shared.ID]struct{}{}
		}
		r.inner.groupMembers[member.GroupID][member.UserID] = struct{}{}
	}
	for _, account := range snapshot.ServiceAccounts {
		r.inner.serviceAccounts[account.ID] = account
	}
	for _, binding := range snapshot.RoleBindings {
		r.inner.roleBindings[binding.ID] = binding
	}
	for _, token := range snapshot.Tokens {
		r.inner.tokens[token.TokenHash] = token
	}
}

func (r *MySQLRepository) snapshot() identitySnapshot {
	r.inner.mu.RLock()
	defer r.inner.mu.RUnlock()
	out := identitySnapshot{
		Users:           make([]User, 0, len(r.inner.users)),
		Credentials:     make([]LocalCredential, 0, len(r.inner.credentials)),
		Identities:      make([]Identity, 0, len(r.inner.identities)),
		OIDCProviders:   make([]OIDCProvider, 0, len(r.inner.oidcProviders)),
		Groups:          make([]Group, 0, len(r.inner.groups)),
		ServiceAccounts: make([]ServiceAccount, 0, len(r.inner.serviceAccounts)),
		RoleBindings:    make([]RoleBinding, 0, len(r.inner.roleBindings)),
		Tokens:          make([]AccessToken, 0, len(r.inner.tokens)),
	}
	for _, value := range r.inner.users {
		out.Users = append(out.Users, value)
	}
	for _, value := range r.inner.credentials {
		out.Credentials = append(out.Credentials, value)
	}
	for _, value := range r.inner.identities {
		out.Identities = append(out.Identities, value)
	}
	for _, value := range r.inner.oidcProviders {
		out.OIDCProviders = append(out.OIDCProviders, value)
	}
	for _, value := range r.inner.groups {
		out.Groups = append(out.Groups, value)
	}
	for groupID, members := range r.inner.groupMembers {
		for userID := range members {
			out.GroupMembers = append(out.GroupMembers, GroupMember{GroupID: groupID, UserID: userID})
		}
	}
	for _, value := range r.inner.serviceAccounts {
		out.ServiceAccounts = append(out.ServiceAccounts, value)
	}
	for _, value := range r.inner.roleBindings {
		out.RoleBindings = append(out.RoleBindings, value)
	}
	for _, value := range r.inner.tokens {
		out.Tokens = append(out.Tokens, value)
	}
	return out
}

func (r *MySQLRepository) persist(ctx context.Context) error { return r.store.Save(ctx, r.snapshot()) }

func (r *MySQLRepository) CreateUser(ctx context.Context, user User) error {
	if err := r.inner.CreateUser(ctx, user); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) UpdateUser(ctx context.Context, user User) error {
	if err := r.inner.UpdateUser(ctx, user); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) GetUser(ctx context.Context, id shared.ID) (User, error) {
	return r.inner.GetUser(ctx, id)
}
func (r *MySQLRepository) FindUserByUsername(ctx context.Context, username string) (User, error) {
	return r.inner.FindUserByUsername(ctx, username)
}
func (r *MySQLRepository) ListUsers(ctx context.Context, page shared.PageRequest) (shared.PageResult[User], error) {
	return r.inner.ListUsers(ctx, page)
}
func (r *MySQLRepository) SaveLocalCredential(ctx context.Context, credential LocalCredential) error {
	if err := r.inner.SaveLocalCredential(ctx, credential); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) GetLocalCredential(ctx context.Context, userID shared.ID) (LocalCredential, error) {
	return r.inner.GetLocalCredential(ctx, userID)
}
func (r *MySQLRepository) CreateIdentity(ctx context.Context, identity Identity) error {
	if err := r.inner.CreateIdentity(ctx, identity); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) FindIdentity(ctx context.Context, provider IdentityProvider, issuer string, subject string) (Identity, error) {
	return r.inner.FindIdentity(ctx, provider, issuer, subject)
}
func (r *MySQLRepository) ListIdentitiesByUser(ctx context.Context, userID shared.ID) ([]Identity, error) {
	return r.inner.ListIdentitiesByUser(ctx, userID)
}
func (r *MySQLRepository) CreateOIDCProvider(ctx context.Context, provider OIDCProvider) error {
	if err := r.inner.CreateOIDCProvider(ctx, provider); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) GetOIDCProvider(ctx context.Context, id shared.ID) (OIDCProvider, error) {
	return r.inner.GetOIDCProvider(ctx, id)
}
func (r *MySQLRepository) ListOIDCProviders(ctx context.Context, enabledOnly bool) ([]OIDCProvider, error) {
	return r.inner.ListOIDCProviders(ctx, enabledOnly)
}
func (r *MySQLRepository) CreateGroup(ctx context.Context, group Group) error {
	if err := r.inner.CreateGroup(ctx, group); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) AddGroupMember(ctx context.Context, member GroupMember) error {
	if err := r.inner.AddGroupMember(ctx, member); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) ListGroupIDsByUser(ctx context.Context, userID shared.ID) ([]shared.ID, error) {
	return r.inner.ListGroupIDsByUser(ctx, userID)
}
func (r *MySQLRepository) CreateServiceAccount(ctx context.Context, account ServiceAccount) error {
	if err := r.inner.CreateServiceAccount(ctx, account); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) GetServiceAccount(ctx context.Context, id shared.ID) (ServiceAccount, error) {
	return r.inner.GetServiceAccount(ctx, id)
}
func (r *MySQLRepository) CreateRoleBinding(ctx context.Context, binding RoleBinding) error {
	if err := r.inner.CreateRoleBinding(ctx, binding); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) ListRoleBindingsForSubject(ctx context.Context, subject Subject) ([]RoleBinding, error) {
	return r.inner.ListRoleBindingsForSubject(ctx, subject)
}
func (r *MySQLRepository) CreateAccessToken(ctx context.Context, token AccessToken) error {
	if err := r.inner.CreateAccessToken(ctx, token); err != nil {
		return err
	}
	return r.persist(ctx)
}
func (r *MySQLRepository) FindAccessTokenByHash(ctx context.Context, hash string) (AccessToken, error) {
	return r.inner.FindAccessTokenByHash(ctx, hash)
}
func (r *MySQLRepository) RevokeAccessTokenByHash(ctx context.Context, hash string, revokedAt time.Time) error {
	if err := r.inner.RevokeAccessTokenByHash(ctx, hash, revokedAt); err != nil {
		return err
	}
	return r.persist(ctx)
}
