package identityaccess

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shareinto/paas/internal/shared"
)

type MemoryRepository struct {
	mu               sync.RWMutex
	users            map[shared.ID]User
	usersByUsername  map[string]shared.ID
	credentials      map[shared.ID]LocalCredential
	identities       map[string]Identity
	identitiesByUser map[shared.ID][]Identity
	oidcProviders    map[shared.ID]OIDCProvider
	groups           map[shared.ID]Group
	groupMembers     map[shared.ID]map[shared.ID]struct{}
	serviceAccounts  map[shared.ID]ServiceAccount
	roleBindings     map[shared.ID]RoleBinding
	tokens           map[string]AccessToken
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		users:            map[shared.ID]User{},
		usersByUsername:  map[string]shared.ID{},
		credentials:      map[shared.ID]LocalCredential{},
		identities:       map[string]Identity{},
		identitiesByUser: map[shared.ID][]Identity{},
		oidcProviders:    map[shared.ID]OIDCProvider{},
		groups:           map[shared.ID]Group{},
		groupMembers:     map[shared.ID]map[shared.ID]struct{}{},
		serviceAccounts:  map[shared.ID]ServiceAccount{},
		roleBindings:     map[shared.ID]RoleBinding{},
		tokens:           map[string]AccessToken{},
	}
}

func (r *MemoryRepository) CreateUser(_ context.Context, user User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	username := normalizeUsername(user.Username)
	if _, exists := r.usersByUsername[username]; exists {
		return shared.NewError(shared.CodeConflict, "username already exists")
	}
	r.users[user.ID] = user
	r.usersByUsername[username] = user.ID
	return nil
}

func (r *MemoryRepository) UpdateUser(_ context.Context, user User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.users[user.ID]; !ok {
		return shared.NewError(shared.CodeNotFound, "user not found")
	}
	r.users[user.ID] = user
	return nil
}

func (r *MemoryRepository) GetUser(_ context.Context, id shared.ID) (User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	user, ok := r.users[id]
	if !ok {
		return User{}, shared.NewError(shared.CodeNotFound, "user not found")
	}
	return user, nil
}

func (r *MemoryRepository) FindUserByUsername(_ context.Context, username string) (User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.usersByUsername[normalizeUsername(username)]
	if !ok {
		return User{}, shared.NewError(shared.CodeNotFound, "user not found")
	}
	return r.users[id], nil
}

func (r *MemoryRepository) ListUsers(_ context.Context, page shared.PageRequest) (shared.PageResult[User], error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	page = page.Normalize()
	users := make([]User, 0, len(r.users))
	for _, user := range r.users {
		users = append(users, user)
	}
	sort.Slice(users, func(i, j int) bool { return users[i].Username < users[j].Username })
	start := page.Offset()
	if start > len(users) {
		start = len(users)
	}
	end := start + page.PageSize
	if end > len(users) {
		end = len(users)
	}
	return shared.NewPageResult(users[start:end], int64(len(users)), page), nil
}

func (r *MemoryRepository) SaveLocalCredential(_ context.Context, credential LocalCredential) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.credentials[credential.UserID] = credential
	return nil
}

func (r *MemoryRepository) GetLocalCredential(_ context.Context, userID shared.ID) (LocalCredential, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	credential, ok := r.credentials[userID]
	if !ok {
		return LocalCredential{}, shared.NewError(shared.CodeNotFound, "local credential not found")
	}
	return credential, nil
}

func (r *MemoryRepository) CreateIdentity(_ context.Context, identity Identity) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := identityKey(identity.Provider, identity.Issuer, identity.Subject)
	if _, exists := r.identities[key]; exists {
		return shared.NewError(shared.CodeConflict, "identity already exists")
	}
	r.identities[key] = identity
	r.identitiesByUser[identity.UserID] = append(r.identitiesByUser[identity.UserID], identity)
	return nil
}

func (r *MemoryRepository) FindIdentity(_ context.Context, provider IdentityProvider, issuer string, subject string) (Identity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	identity, ok := r.identities[identityKey(provider, issuer, subject)]
	if !ok {
		return Identity{}, shared.NewError(shared.CodeNotFound, "identity not found")
	}
	return identity, nil
}

func (r *MemoryRepository) ListIdentitiesByUser(_ context.Context, userID shared.ID) ([]Identity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]Identity(nil), r.identitiesByUser[userID]...), nil
}

func (r *MemoryRepository) CreateOIDCProvider(_ context.Context, provider OIDCProvider) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.oidcProviders[provider.ID] = provider
	return nil
}

func (r *MemoryRepository) GetOIDCProvider(_ context.Context, id shared.ID) (OIDCProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	provider, ok := r.oidcProviders[id]
	if !ok {
		return OIDCProvider{}, shared.NewError(shared.CodeNotFound, "oidc provider not found")
	}
	return provider, nil
}

func (r *MemoryRepository) ListOIDCProviders(_ context.Context, enabledOnly bool) ([]OIDCProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	providers := make([]OIDCProvider, 0, len(r.oidcProviders))
	for _, provider := range r.oidcProviders {
		if enabledOnly && !provider.Enabled {
			continue
		}
		providers = append(providers, provider)
	}
	sort.Slice(providers, func(i, j int) bool { return providers[i].Name < providers[j].Name })
	return providers, nil
}

func (r *MemoryRepository) CreateGroup(_ context.Context, group Group) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.groups[group.ID] = group
	return nil
}

func (r *MemoryRepository) AddGroupMember(_ context.Context, member GroupMember) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.groupMembers[member.UserID] == nil {
		r.groupMembers[member.UserID] = map[shared.ID]struct{}{}
	}
	r.groupMembers[member.UserID][member.GroupID] = struct{}{}
	return nil
}

func (r *MemoryRepository) ListGroupIDsByUser(_ context.Context, userID shared.ID) ([]shared.ID, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]shared.ID, 0, len(r.groupMembers[userID]))
	for id := range r.groupMembers[userID] {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

func (r *MemoryRepository) CreateServiceAccount(_ context.Context, account ServiceAccount) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.serviceAccounts[account.ID] = account
	return nil
}

func (r *MemoryRepository) GetServiceAccount(_ context.Context, id shared.ID) (ServiceAccount, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	account, ok := r.serviceAccounts[id]
	if !ok {
		return ServiceAccount{}, shared.NewError(shared.CodeNotFound, "service account not found")
	}
	return account, nil
}

func (r *MemoryRepository) CreateRoleBinding(_ context.Context, binding RoleBinding) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.roleBindings[binding.ID] = binding
	return nil
}

func (r *MemoryRepository) ListRoleBindingsForSubject(_ context.Context, subject Subject) ([]RoleBinding, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	bindings := make([]RoleBinding, 0)
	for _, binding := range r.roleBindings {
		if binding.SubjectType == subject.Type && binding.SubjectID == subject.ID {
			bindings = append(bindings, binding)
		}
	}
	return bindings, nil
}

func (r *MemoryRepository) CreateAccessToken(_ context.Context, token AccessToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokens[token.TokenHash] = token
	return nil
}

func (r *MemoryRepository) FindAccessTokenByHash(_ context.Context, hash string) (AccessToken, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	token, ok := r.tokens[hash]
	if !ok {
		return AccessToken{}, shared.NewError(shared.CodeNotFound, "token not found")
	}
	return token, nil
}

func (r *MemoryRepository) RevokeAccessTokenByHash(_ context.Context, hash string, revokedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	token, ok := r.tokens[hash]
	if !ok {
		return shared.NewError(shared.CodeNotFound, "token not found")
	}
	token.RevokedAt = &revokedAt
	r.tokens[hash] = token
	return nil
}

func identityKey(provider IdentityProvider, issuer string, subject string) string {
	return string(provider) + "|" + strings.TrimSpace(issuer) + "|" + strings.TrimSpace(subject)
}

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}
