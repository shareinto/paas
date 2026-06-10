package identityaccess

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/shareinto/paas/internal/platform/database"
	"github.com/shareinto/paas/internal/shared"
)

type MySQLRepository struct {
	db *sql.DB
}

func NewMySQLRepository(_ context.Context, db *sql.DB) (*MySQLRepository, error) {
	return &MySQLRepository{db: db}, nil
}

func (r *MySQLRepository) CreateUser(ctx context.Context, user User) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO users (id, username, display_name, email, avatar_url, disabled, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		user.ID, normalizeUsername(user.Username), user.DisplayName, user.Email, user.AvatarURL, user.Disabled, mysqlTime(user.CreatedAt), mysqlTime(user.UpdatedAt))
	return database.ConflictOrUnavailable(err, "username already exists", "create user failed")
}

func (r *MySQLRepository) UpdateUser(ctx context.Context, user User) error {
	result, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
UPDATE users SET username = ?, display_name = ?, email = ?, avatar_url = ?, disabled = ?, updated_at = ? WHERE id = ?`,
		normalizeUsername(user.Username), user.DisplayName, user.Email, user.AvatarURL, user.Disabled, mysqlTime(user.UpdatedAt), user.ID)
	if err != nil {
		return database.ConflictOrUnavailable(err, "username already exists", "update user failed")
	}
	return database.RequireAffected(result, "user not found")
}

func (r *MySQLRepository) GetUser(ctx context.Context, id shared.ID) (User, error) {
	user, err := scanUser(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, username, display_name, email, avatar_url, disabled, created_at, updated_at FROM users WHERE id = ?`, id))
	if err != nil {
		return User{}, database.NotFound(err, "user not found")
	}
	return user, nil
}

func (r *MySQLRepository) FindUserByUsername(ctx context.Context, username string) (User, error) {
	user, err := scanUser(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, username, display_name, email, avatar_url, disabled, created_at, updated_at FROM users WHERE username = ?`, normalizeUsername(username)))
	if err != nil {
		return User{}, database.NotFound(err, "user not found")
	}
	return user, nil
}

func (r *MySQLRepository) ListUsers(ctx context.Context, page shared.PageRequest) (shared.PageResult[User], error) {
	var total int64
	if err := database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&total); err != nil {
		return shared.PageResult[User]{}, database.WrapUnavailable(err, "count users failed")
	}
	page, limit, offset := database.LimitOffset(page)
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, `
SELECT id, username, display_name, email, avatar_url, disabled, created_at, updated_at
FROM users ORDER BY username ASC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return shared.PageResult[User]{}, database.WrapUnavailable(err, "list users failed")
	}
	defer rows.Close()
	items := []User{}
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return shared.PageResult[User]{}, err
		}
		items = append(items, user)
	}
	if err := rows.Err(); err != nil {
		return shared.PageResult[User]{}, database.WrapUnavailable(err, "list users failed")
	}
	return shared.NewPageResult(items, total, page), nil
}

func (r *MySQLRepository) SaveLocalCredential(ctx context.Context, credential LocalCredential) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO local_credentials (user_id, password_hash, updated_at)
VALUES (?, ?, ?)
ON DUPLICATE KEY UPDATE password_hash = VALUES(password_hash), updated_at = VALUES(updated_at)`,
		credential.UserID, credential.PasswordHash, mysqlTime(credential.UpdatedAt))
	return database.WrapUnavailable(err, "save local credential failed")
}

func (r *MySQLRepository) GetLocalCredential(ctx context.Context, userID shared.ID) (LocalCredential, error) {
	var credential LocalCredential
	err := database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT user_id, password_hash, updated_at FROM local_credentials WHERE user_id = ?`, userID).
		Scan(&credential.UserID, &credential.PasswordHash, &credential.UpdatedAt)
	if err != nil {
		return LocalCredential{}, database.NotFound(err, "local credential not found")
	}
	return credential, nil
}

func (r *MySQLRepository) CreateIdentity(ctx context.Context, identity Identity) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO identities (id, user_id, provider, issuer, subject, created_at)
VALUES (?, ?, ?, ?, ?, ?)`,
		identity.ID, identity.UserID, identity.Provider, strings.TrimSpace(identity.Issuer), strings.TrimSpace(identity.Subject), mysqlTime(identity.CreatedAt))
	return database.ConflictOrUnavailable(err, "identity already exists", "create identity failed")
}

func (r *MySQLRepository) FindIdentity(ctx context.Context, provider IdentityProvider, issuer string, subject string) (Identity, error) {
	identity, err := scanIdentity(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, user_id, provider, issuer, subject, created_at
FROM identities WHERE provider = ? AND issuer = ? AND subject = ?`,
		provider, strings.TrimSpace(issuer), strings.TrimSpace(subject)))
	if err != nil {
		return Identity{}, database.NotFound(err, "identity not found")
	}
	return identity, nil
}

func (r *MySQLRepository) ListIdentitiesByUser(ctx context.Context, userID shared.ID) ([]Identity, error) {
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, `
SELECT id, user_id, provider, issuer, subject, created_at
FROM identities WHERE user_id = ? ORDER BY created_at ASC, id ASC`, userID)
	if err != nil {
		return nil, database.WrapUnavailable(err, "list identities failed")
	}
	defer rows.Close()
	items := []Identity{}
	for rows.Next() {
		identity, err := scanIdentity(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, identity)
	}
	if err := rows.Err(); err != nil {
		return nil, database.WrapUnavailable(err, "list identities failed")
	}
	return items, nil
}

func (r *MySQLRepository) CreateOIDCProvider(ctx context.Context, provider OIDCProvider) error {
	scopes, err := database.MarshalJSON(provider.Scopes)
	if err != nil {
		return err
	}
	_, err = database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO oidc_providers (id, name, issuer, client_id, client_secret_ref, scopes, redirect_uri, enabled, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		provider.ID, provider.Name, provider.Issuer, provider.ClientID, provider.ClientSecretRef, string(scopes), provider.RedirectURI, provider.Enabled, mysqlTime(provider.CreatedAt), mysqlTime(provider.UpdatedAt))
	return database.ConflictOrUnavailable(err, "oidc provider already exists", "create oidc provider failed")
}

func (r *MySQLRepository) GetOIDCProvider(ctx context.Context, id shared.ID) (OIDCProvider, error) {
	provider, err := scanOIDCProvider(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, name, issuer, client_id, client_secret_ref, scopes, redirect_uri, enabled, created_at, updated_at
FROM oidc_providers WHERE id = ?`, id))
	if err != nil {
		return OIDCProvider{}, database.NotFound(err, "oidc provider not found")
	}
	return provider, nil
}

func (r *MySQLRepository) ListOIDCProviders(ctx context.Context, enabledOnly bool) ([]OIDCProvider, error) {
	query := `
SELECT id, name, issuer, client_id, client_secret_ref, scopes, redirect_uri, enabled, created_at, updated_at
FROM oidc_providers`
	args := []any{}
	if enabledOnly {
		query += " WHERE enabled = 1"
	}
	query += " ORDER BY name ASC"
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, query, args...)
	if err != nil {
		return nil, database.WrapUnavailable(err, "list oidc providers failed")
	}
	defer rows.Close()
	items := []OIDCProvider{}
	for rows.Next() {
		provider, err := scanOIDCProvider(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, provider)
	}
	if err := rows.Err(); err != nil {
		return nil, database.WrapUnavailable(err, "list oidc providers failed")
	}
	return items, nil
}

func (r *MySQLRepository) CreateGroup(ctx context.Context, group Group) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO identity_groups (id, name, created_at) VALUES (?, ?, ?)`, group.ID, group.Name, mysqlTime(group.CreatedAt))
	return database.ConflictOrUnavailable(err, "group already exists", "create group failed")
}

func (r *MySQLRepository) AddGroupMember(ctx context.Context, member GroupMember) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT IGNORE INTO group_members (group_id, user_id) VALUES (?, ?)`, member.GroupID, member.UserID)
	return database.WrapUnavailable(err, "add group member failed")
}

func (r *MySQLRepository) ListGroupIDsByUser(ctx context.Context, userID shared.ID) ([]shared.ID, error) {
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, `
SELECT group_id FROM group_members WHERE user_id = ? ORDER BY group_id ASC`, userID)
	if err != nil {
		return nil, database.WrapUnavailable(err, "list group ids failed")
	}
	defer rows.Close()
	ids := []shared.ID{}
	for rows.Next() {
		var id shared.ID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, database.WrapUnavailable(err, "list group ids failed")
	}
	return ids, nil
}

func (r *MySQLRepository) CreateServiceAccount(ctx context.Context, account ServiceAccount) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO service_accounts (id, name, disabled, created_at) VALUES (?, ?, ?, ?)`,
		account.ID, account.Name, account.Disabled, mysqlTime(account.CreatedAt))
	return database.ConflictOrUnavailable(err, "service account already exists", "create service account failed")
}

func (r *MySQLRepository) GetServiceAccount(ctx context.Context, id shared.ID) (ServiceAccount, error) {
	var account ServiceAccount
	err := database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, name, disabled, created_at FROM service_accounts WHERE id = ?`, id).
		Scan(&account.ID, &account.Name, &account.Disabled, &account.CreatedAt)
	if err != nil {
		return ServiceAccount{}, database.NotFound(err, "service account not found")
	}
	return account, nil
}

func (r *MySQLRepository) CreateRoleBinding(ctx context.Context, binding RoleBinding) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO role_bindings (id, subject_type, subject_id, role_id, scope_kind, scope_id, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
		binding.ID, binding.SubjectType, binding.SubjectID, binding.RoleID, binding.ScopeKind, binding.ScopeID, mysqlTime(binding.CreatedAt))
	return database.ConflictOrUnavailable(err, "role binding already exists", "create role binding failed")
}

func (r *MySQLRepository) ListRoleBindingsForSubject(ctx context.Context, subject Subject) ([]RoleBinding, error) {
	rows, err := database.ExecutorFromContext(ctx, r.db).QueryContext(ctx, `
SELECT id, subject_type, subject_id, role_id, scope_kind, scope_id, created_at
FROM role_bindings WHERE subject_type = ? AND subject_id = ? ORDER BY created_at ASC, id ASC`,
		subject.Type, subject.ID)
	if err != nil {
		return nil, database.WrapUnavailable(err, "list role bindings failed")
	}
	defer rows.Close()
	items := []RoleBinding{}
	for rows.Next() {
		binding, err := scanRoleBinding(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, binding)
	}
	if err := rows.Err(); err != nil {
		return nil, database.WrapUnavailable(err, "list role bindings failed")
	}
	return items, nil
}

func (r *MySQLRepository) CreateAccessToken(ctx context.Context, token AccessToken) error {
	_, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
INSERT INTO access_tokens (id, user_id, kind, token_hash, expires_at, revoked_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
		token.ID, token.UserID, token.Kind, token.TokenHash, mysqlTime(token.ExpiresAt), mysqlTimePtr(token.RevokedAt), mysqlTime(token.CreatedAt))
	return database.ConflictOrUnavailable(err, "token already exists", "create access token failed")
}

func (r *MySQLRepository) FindAccessTokenByHash(ctx context.Context, hash string) (AccessToken, error) {
	token, err := scanAccessToken(database.ExecutorFromContext(ctx, r.db).QueryRowContext(ctx, `
SELECT id, user_id, kind, token_hash, expires_at, revoked_at, created_at
FROM access_tokens WHERE token_hash = ?`, hash))
	if err != nil {
		return AccessToken{}, database.NotFound(err, "token not found")
	}
	return token, nil
}

func (r *MySQLRepository) RevokeAccessTokenByHash(ctx context.Context, hash string, revokedAt time.Time) error {
	result, err := database.ExecutorFromContext(ctx, r.db).ExecContext(ctx, `
UPDATE access_tokens SET revoked_at = ? WHERE token_hash = ?`, mysqlTime(revokedAt), hash)
	if err != nil {
		return database.WrapUnavailable(err, "revoke access token failed")
	}
	return database.RequireAffected(result, "token not found")
}

type identityScanner interface {
	Scan(dest ...any) error
}

func scanUser(scanner identityScanner) (User, error) {
	var user User
	err := scanner.Scan(&user.ID, &user.Username, &user.DisplayName, &user.Email, &user.AvatarURL, &user.Disabled, &user.CreatedAt, &user.UpdatedAt)
	return user, err
}

func scanIdentity(scanner identityScanner) (Identity, error) {
	var identity Identity
	err := scanner.Scan(&identity.ID, &identity.UserID, &identity.Provider, &identity.Issuer, &identity.Subject, &identity.CreatedAt)
	return identity, err
}

func scanOIDCProvider(scanner identityScanner) (OIDCProvider, error) {
	var provider OIDCProvider
	var scopes []byte
	if err := scanner.Scan(&provider.ID, &provider.Name, &provider.Issuer, &provider.ClientID, &provider.ClientSecretRef, &scopes, &provider.RedirectURI, &provider.Enabled, &provider.CreatedAt, &provider.UpdatedAt); err != nil {
		return OIDCProvider{}, err
	}
	if err := database.UnmarshalJSON(scopes, &provider.Scopes); err != nil {
		return OIDCProvider{}, err
	}
	return provider, nil
}

func scanRoleBinding(scanner identityScanner) (RoleBinding, error) {
	var binding RoleBinding
	err := scanner.Scan(&binding.ID, &binding.SubjectType, &binding.SubjectID, &binding.RoleID, &binding.ScopeKind, &binding.ScopeID, &binding.CreatedAt)
	return binding, err
}

func scanAccessToken(scanner identityScanner) (AccessToken, error) {
	var token AccessToken
	err := scanner.Scan(&token.ID, &token.UserID, &token.Kind, &token.TokenHash, &token.ExpiresAt, &token.RevokedAt, &token.CreatedAt)
	return token, err
}

func mysqlTime(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC()
	}
	return value
}

func mysqlTimePtr(value *time.Time) any {
	if value == nil || value.IsZero() {
		return nil
	}
	return *value
}
