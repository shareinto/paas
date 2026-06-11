package identityaccess

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/shareinto/paas/internal/shared"
	"github.com/shareinto/paas/internal/shared/testutil"
	"github.com/shareinto/paas/internal/testsupport"
)

type fakeVerifier struct {
	wantNonce string
	claims    OIDCClaims
	err       error
}

func (v *fakeVerifier) VerifyCallback(_ context.Context, _ OIDCProvider, code string, nonce string) (OIDCClaims, error) {
	if code == "bad" {
		return OIDCClaims{}, errors.New("bad code")
	}
	if v.wantNonce != "" && nonce != v.wantNonce {
		return OIDCClaims{}, errors.New("bad nonce")
	}
	if v.err != nil {
		return OIDCClaims{}, v.err
	}
	return v.claims, nil
}

type recordingAudit struct {
	events []AuditEvent
}

func (a *recordingAudit) Log(_ context.Context, event AuditEvent) error {
	a.events = append(a.events, event)
	return nil
}

type failingIDGenerator struct {
	remaining int
}

func (g *failingIDGenerator) NewID(prefix string) (shared.ID, error) {
	if g.remaining <= 0 {
		return "", errors.New("id generator failed")
	}
	g.remaining--
	return shared.ID(prefix + "_ok"), nil
}

func newTestRepository(t *testing.T) Repository {
	t.Helper()
	repo, err := NewMySQLRepository(context.Background(), testsupport.MySQLDB(t, Migrations...))
	if err != nil {
		t.Fatalf("NewMySQLRepository() error = %v", err)
	}
	return repo
}

func newTestService(t *testing.T, verifier OIDCVerifier) (*Service, Repository, *recordingAudit) {
	t.Helper()
	repo := newTestRepository(t)
	audit := &recordingAudit{}
	svc := NewService(Options{
		Repository:  repo,
		Audit:       audit,
		IDGenerator: testutil.NewFakeIDGenerator(1),
		Clock:       testutil.NewFakeClock(time.Date(2026, 5, 30, 1, 0, 0, 0, time.UTC)),
		Verifier:    verifier,
		AccessTTL:   time.Hour,
		RefreshTTL:  24 * time.Hour,
	})
	return svc, repo, audit
}

func TestValidatePermission(t *testing.T) {
	if err := ValidatePermission("application:create"); err != nil {
		t.Fatalf("permission should be valid: %v", err)
	}
	for _, permission := range []Permission{"bad", "Application:create", "app:"} {
		if err := ValidatePermission(permission); shared.CodeOf(err) != shared.CodeInvalidArgument {
			t.Fatalf("%q should be invalid_argument, got %v", permission, err)
		}
	}
}

func TestCreateLocalUserStoresPasswordHashAndLoginIssuesHashedTokens(t *testing.T) {
	svc, repo, audit := newTestService(t, nil)
	ctx := context.Background()

	user, err := svc.CreateLocalUser(ctx, CreateLocalUserInput{ActorID: "usr_admin", Username: "Alice", Password: "secret", DisplayName: "Alice", Email: "a@example.com"})
	if err != nil {
		t.Fatalf("CreateLocalUser() error = %v", err)
	}
	if user.Username != "alice" {
		t.Fatalf("username should be normalized, got %q", user.Username)
	}

	credential, err := repo.GetLocalCredential(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetLocalCredential() error = %v", err)
	}
	if string(credential.PasswordHash) == "secret" {
		t.Fatalf("password must not be stored in plaintext")
	}
	if err := bcrypt.CompareHashAndPassword(credential.PasswordHash, []byte("secret")); err != nil {
		t.Fatalf("stored password hash should verify: %v", err)
	}

	pair, loggedIn, err := svc.LoginLocal(ctx, "alice", "secret")
	if err != nil {
		t.Fatalf("LoginLocal() error = %v", err)
	}
	if loggedIn.ID != user.ID || pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Fatalf("unexpected login result: %+v %+v", pair, loggedIn)
	}
	if _, err := repo.FindAccessTokenByHash(ctx, pair.AccessToken); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("raw access token must not be stored")
	}
	if _, err := repo.FindAccessTokenByHash(ctx, hashToken(pair.AccessToken)); err != nil {
		t.Fatalf("hashed access token should be stored: %v", err)
	}
	if len(audit.events) < 2 {
		t.Fatalf("create and login should write audit events")
	}
}

func TestLocalLoginRejectsBadPasswordAndDisabledUser(t *testing.T) {
	svc, _, _ := newTestService(t, nil)
	ctx := context.Background()
	user, err := svc.CreateLocalUser(ctx, CreateLocalUserInput{Username: "bob", Password: "secret"})
	if err != nil {
		t.Fatalf("CreateLocalUser() error = %v", err)
	}
	if _, _, err := svc.LoginLocal(ctx, "bob", "wrong"); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("wrong password should be unauthenticated, got %v", err)
	}
	if err := svc.DisableUser(ctx, "usr_admin", user.ID); err != nil {
		t.Fatalf("DisableUser() error = %v", err)
	}
	if _, _, err := svc.LoginLocal(ctx, "bob", "secret"); shared.CodeOf(err) != shared.CodePermissionDenied {
		t.Fatalf("disabled user should be denied, got %v", err)
	}
}

func TestLocalUserServiceErrorBranches(t *testing.T) {
	svc, repo, _ := newTestService(t, nil)
	ctx := context.Background()
	if _, _, err := svc.LoginLocal(ctx, "missing", "secret"); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("missing local user should be unauthenticated, got %v", err)
	}
	orphan := User{ID: "usr_orphan", Username: "orphan", CreatedAt: svc.clock.Now(), UpdatedAt: svc.clock.Now()}
	if err := repo.CreateUser(ctx, orphan); err != nil {
		t.Fatalf("CreateUser orphan error = %v", err)
	}
	if _, _, err := svc.LoginLocal(ctx, "orphan", "secret"); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("missing credential should be unauthenticated, got %v", err)
	}
	if err := svc.DisableUser(ctx, "usr_admin", "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("disable missing user should be not_found, got %v", err)
	}
	if err := svc.ResetPassword(ctx, "usr_admin", "missing", "new"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("reset missing user should be not_found, got %v", err)
	}
	if err := svc.ResetPassword(ctx, "usr_admin", orphan.ID, ""); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("empty reset password should be invalid_argument, got %v", err)
	}
	svc.ids = &failingIDGenerator{remaining: 0}
	if _, err := svc.CreateLocalUser(ctx, CreateLocalUserInput{Username: "idfail", Password: "secret"}); err == nil {
		t.Fatalf("id generator failure should fail CreateLocalUser")
	}
}

func TestResetPassword(t *testing.T) {
	svc, _, _ := newTestService(t, nil)
	ctx := context.Background()
	user, err := svc.CreateLocalUser(ctx, CreateLocalUserInput{Username: "reset", Password: "old"})
	if err != nil {
		t.Fatalf("CreateLocalUser() error = %v", err)
	}
	if err := svc.ResetPassword(ctx, "usr_admin", user.ID, "new"); err != nil {
		t.Fatalf("ResetPassword() error = %v", err)
	}
	if _, _, err := svc.LoginLocal(ctx, "reset", "old"); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("old password should fail, got %v", err)
	}
	if _, _, err := svc.LoginLocal(ctx, "reset", "new"); err != nil {
		t.Fatalf("new password should work: %v", err)
	}
}

func TestOIDCStartAndCallbackCreatesAndReusesIdentity(t *testing.T) {
	verifier := &fakeVerifier{claims: OIDCClaims{Issuer: "https://idp.example.com", Subject: "sub-1", Username: "oidc-user", DisplayName: "企业用户", Email: "oidc@example.com"}}
	svc, repo, _ := newTestService(t, verifier)
	ctx := context.Background()
	provider, err := svc.CreateOIDCProvider(ctx, OIDCProvider{ID: "oidc_main", Name: "企业身份", Issuer: "https://idp.example.com", ClientID: "paas", ClientSecretRef: "secret/oidc", Scopes: []string{"openid", "profile"}, RedirectURI: "https://paas.example.com/callback", Enabled: true})
	if err != nil {
		t.Fatalf("CreateOIDCProvider() error = %v", err)
	}

	start, err := svc.StartOIDC(ctx, provider.ID)
	if err != nil {
		t.Fatalf("StartOIDC() error = %v", err)
	}
	verifier.wantNonce = svc.states[start.State].Nonce
	if !strings.Contains(start.RedirectURL, "state=") || !strings.Contains(start.RedirectURL, "nonce=") {
		t.Fatalf("redirect url should contain state and nonce: %s", start.RedirectURL)
	}
	pair, user, err := svc.CallbackOIDC(ctx, provider.ID, start.State, "code")
	if err != nil {
		t.Fatalf("CallbackOIDC() error = %v", err)
	}
	if pair.AccessToken == "" || user.Username != "oidc-user" {
		t.Fatalf("unexpected callback result: %+v %+v", pair, user)
	}
	identities, err := repo.ListIdentitiesByUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListIdentitiesByUser() error = %v", err)
	}
	if len(identities) != 1 || identities[0].Issuer != "https://idp.example.com" || identities[0].Subject != "sub-1" {
		t.Fatalf("unexpected identities: %+v", identities)
	}

	start2, err := svc.StartOIDC(ctx, provider.ID)
	if err != nil {
		t.Fatalf("StartOIDC() second error = %v", err)
	}
	verifier.wantNonce = svc.states[start2.State].Nonce
	_, user2, err := svc.CallbackOIDC(ctx, provider.ID, start2.State, "code")
	if err != nil {
		t.Fatalf("CallbackOIDC() second error = %v", err)
	}
	if user2.ID != user.ID {
		t.Fatalf("same issuer+subject should map to same user: %s != %s", user2.ID, user.ID)
	}
}

func TestOIDCStateValidation(t *testing.T) {
	svc, _, _ := newTestService(t, &fakeVerifier{claims: OIDCClaims{Issuer: "https://idp", Subject: "s"}})
	ctx := context.Background()
	provider, err := svc.CreateOIDCProvider(ctx, OIDCProvider{Name: "企业身份", Issuer: "https://idp", ClientID: "paas", ClientSecretRef: "secret", RedirectURI: "https://paas/callback", Enabled: true})
	if err != nil {
		t.Fatalf("CreateOIDCProvider() error = %v", err)
	}
	start, err := svc.StartOIDC(ctx, provider.ID)
	if err != nil {
		t.Fatalf("StartOIDC() error = %v", err)
	}
	if _, _, err := svc.CallbackOIDC(ctx, provider.ID, "wrong", "code"); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("wrong state should be unauthenticated, got %v", err)
	}
	if _, _, err := svc.CallbackOIDC(ctx, provider.ID, start.State, "bad"); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("bad code should be unauthenticated, got %v", err)
	}
	if _, _, err := svc.CallbackOIDC(ctx, provider.ID, start.State, "code"); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("state reuse should be rejected, got %v", err)
	}
}

func TestPermissionCheckerDirectGroupAndServiceAccount(t *testing.T) {
	svc, repo, _ := newTestService(t, nil)
	ctx := context.Background()

	user, err := svc.CreateLocalUser(ctx, CreateLocalUserInput{Username: "dev", Password: "secret"})
	if err != nil {
		t.Fatalf("CreateLocalUser() error = %v", err)
	}
	if _, err := svc.CreateRoleBinding(ctx, RoleBinding{SubjectType: SubjectUser, SubjectID: user.ID, RoleID: RoleDeveloper, ScopeKind: ScopeTenant, ScopeID: "tenant_1"}); err != nil {
		t.Fatalf("CreateRoleBinding() error = %v", err)
	}
	if err := svc.Check(ctx, Subject{Type: SubjectUser, ID: user.ID}, ResourceScope{Kind: ScopeProject, TenantID: "tenant_1", ProjectID: "project_1"}, "build:create"); err != nil {
		t.Fatalf("tenant scoped developer should create builds: %v", err)
	}
	if err := svc.Check(ctx, Subject{Type: SubjectUser, ID: user.ID}, ResourceScope{Kind: ScopeProject, TenantID: "tenant_1", ProjectID: "project_1"}, "freight:create"); err != nil {
		t.Fatalf("tenant scoped developer should create freights: %v", err)
	}
	if err := svc.Check(ctx, Subject{Type: SubjectUser, ID: user.ID}, ResourceScope{Kind: ScopeProject, TenantID: "tenant_2", ProjectID: "project_2"}, "build:create"); shared.CodeOf(err) != shared.CodePermissionDenied {
		t.Fatalf("different tenant should be denied, got %v", err)
	}

	if err := repo.CreateGroup(ctx, Group{ID: "group_viewer", Name: "viewer"}); err != nil {
		t.Fatalf("CreateGroup() error = %v", err)
	}
	if err := repo.AddGroupMember(ctx, GroupMember{GroupID: "group_viewer", UserID: user.ID}); err != nil {
		t.Fatalf("AddGroupMember() error = %v", err)
	}
	if _, err := svc.CreateRoleBinding(ctx, RoleBinding{SubjectType: SubjectGroup, SubjectID: "group_viewer", RoleID: RoleViewer, ScopeKind: ScopeProject, ScopeID: "project_2"}); err != nil {
		t.Fatalf("CreateRoleBinding() group error = %v", err)
	}
	if err := svc.Check(ctx, Subject{Type: SubjectUser, ID: user.ID}, ResourceScope{Kind: ScopeApplication, TenantID: "tenant_2", ProjectID: "project_2", ApplicationID: "app_1"}, "application:read"); err != nil {
		t.Fatalf("group viewer should read application: %v", err)
	}

	if err := repo.CreateServiceAccount(ctx, ServiceAccount{ID: "sa_1", Name: "deploy-bot"}); err != nil {
		t.Fatalf("CreateServiceAccount() error = %v", err)
	}
	if _, err := svc.CreateRoleBinding(ctx, RoleBinding{SubjectType: SubjectServiceAccount, SubjectID: "sa_1", RoleID: RoleOperator, ScopeKind: ScopeEnvironment, ScopeID: "env_prod"}); err != nil {
		t.Fatalf("CreateRoleBinding() service account error = %v", err)
	}
	if err := svc.Check(ctx, Subject{Type: SubjectServiceAccount, ID: "sa_1"}, ResourceScope{Kind: ScopeEnvironment, TenantID: "tenant_1", ProjectID: "project_1", EnvironmentID: "env_prod"}, "deployment:rollback"); err != nil {
		t.Fatalf("service account operator should rollback deployment: %v", err)
	}
	if err := svc.Check(ctx, Subject{Type: SubjectServiceAccount, ID: "sa_1"}, ResourceScope{Kind: ScopeEnvironment, TenantID: "tenant_1", ProjectID: "project_1", EnvironmentID: "env_prod"}, "freight:create"); shared.CodeOf(err) != shared.CodePermissionDenied {
		t.Fatalf("operator should not create freight by default, got %v", err)
	}
	account, err := repo.GetServiceAccount(ctx, "sa_1")
	if err != nil || account.Name != "deploy-bot" {
		t.Fatalf("GetServiceAccount() = %+v, %v", account, err)
	}
}

func TestBuiltInProjectRolesAllowFreightCreate(t *testing.T) {
	for _, roleID := range []RoleID{RoleTenantOwner, RoleTenantAdmin, RoleProjectAdmin, RoleDeveloper} {
		t.Run(string(roleID), func(t *testing.T) {
			svc, _, _ := newTestService(t, nil)
			ctx := context.Background()
			user, err := svc.CreateLocalUser(ctx, CreateLocalUserInput{Username: "user_" + string(roleID), Password: "secret"})
			if err != nil {
				t.Fatalf("CreateLocalUser() error = %v", err)
			}
			if _, err := svc.CreateRoleBinding(ctx, RoleBinding{SubjectType: SubjectUser, SubjectID: user.ID, RoleID: roleID, ScopeKind: ScopeProject, ScopeID: "project_1"}); err != nil {
				t.Fatalf("CreateRoleBinding() error = %v", err)
			}
			if err := svc.Check(ctx, Subject{Type: SubjectUser, ID: user.ID}, ResourceScope{Kind: ScopeProject, TenantID: "tenant_1", ProjectID: "project_1"}, "freight:create"); err != nil {
				t.Fatalf("%s should create freight: %v", roleID, err)
			}
		})
	}
}

func TestTenantAdminRolesManageTenantClusters(t *testing.T) {
	for _, roleID := range []RoleID{RoleTenantOwner, RoleTenantAdmin} {
		t.Run(string(roleID), func(t *testing.T) {
			svc, _, _ := newTestService(t, nil)
			ctx := context.Background()
			user, err := svc.CreateLocalUser(ctx, CreateLocalUserInput{Username: "cluster_" + string(roleID), Password: "secret"})
			if err != nil {
				t.Fatalf("CreateLocalUser() error = %v", err)
			}
			if _, err := svc.CreateRoleBinding(ctx, RoleBinding{SubjectType: SubjectUser, SubjectID: user.ID, RoleID: roleID, ScopeKind: ScopeTenant, ScopeID: "tenant_1"}); err != nil {
				t.Fatalf("CreateRoleBinding() error = %v", err)
			}
			subject := Subject{Type: SubjectUser, ID: user.ID}
			resource := ResourceScope{Kind: ScopeTenant, TenantID: "tenant_1"}
			if err := svc.Check(ctx, subject, resource, "cluster:manage"); err != nil {
				t.Fatalf("%s should manage tenant clusters: %v", roleID, err)
			}
			if err := svc.Check(ctx, subject, resource, "cluster:read"); err != nil {
				t.Fatalf("%s should read tenant clusters: %v", roleID, err)
			}
			if err := svc.Check(ctx, subject, ResourceScope{Kind: ScopeTenant, TenantID: "tenant_2"}, "cluster:manage"); shared.CodeOf(err) != shared.CodePermissionDenied {
				t.Fatalf("%s should not manage another tenant cluster, got %v", roleID, err)
			}
		})
	}
}

func TestTokenAuthenticateRefreshAndLogout(t *testing.T) {
	svc, _, _ := newTestService(t, nil)
	ctx := context.Background()
	user, err := svc.CreateLocalUser(ctx, CreateLocalUserInput{Username: "token", Password: "secret"})
	if err != nil {
		t.Fatalf("CreateLocalUser() error = %v", err)
	}
	pair, _, err := svc.LoginLocal(ctx, "token", "secret")
	if err != nil {
		t.Fatalf("LoginLocal() error = %v", err)
	}
	authenticated, err := svc.AuthenticateAccessToken(ctx, pair.AccessToken)
	if err != nil {
		t.Fatalf("AuthenticateAccessToken() error = %v", err)
	}
	if authenticated.ID != user.ID {
		t.Fatalf("authenticated user = %s, want %s", authenticated.ID, user.ID)
	}
	refreshed, err := svc.RefreshAccessToken(ctx, pair.RefreshToken)
	if err != nil {
		t.Fatalf("RefreshAccessToken() error = %v", err)
	}
	if refreshed.AccessToken == "" || refreshed.AccessToken == pair.AccessToken {
		t.Fatalf("unexpected refreshed token: %+v", refreshed)
	}
	if err := svc.Logout(ctx, pair.AccessToken); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if _, err := svc.AuthenticateAccessToken(ctx, pair.AccessToken); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("revoked token should be rejected, got %v", err)
	}
}

func TestTokenFailureBranches(t *testing.T) {
	repo := newTestRepository(t)
	clock := testutil.NewFakeClock(time.Date(2026, 5, 30, 1, 0, 0, 0, time.UTC))
	svc := NewService(Options{Repository: repo, IDGenerator: testutil.NewFakeIDGenerator(1), Clock: clock, AccessTTL: time.Minute, RefreshTTL: 2 * time.Minute})
	ctx := context.Background()
	user, err := svc.CreateLocalUser(ctx, CreateLocalUserInput{Username: "token-fail", Password: "secret"})
	if err != nil {
		t.Fatalf("CreateLocalUser() error = %v", err)
	}
	pair, _, err := svc.LoginLocal(ctx, "token-fail", "secret")
	if err != nil {
		t.Fatalf("LoginLocal() error = %v", err)
	}
	if _, err := svc.AuthenticateAccessToken(ctx, "wrong"); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("wrong token should be unauthenticated, got %v", err)
	}
	if _, err := svc.AuthenticateAccessToken(ctx, pair.RefreshToken); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("refresh token used as access should be unauthenticated, got %v", err)
	}
	if _, err := svc.RefreshAccessToken(ctx, pair.AccessToken); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("access token used as refresh should be unauthenticated, got %v", err)
	}
	clock.Advance(3 * time.Minute)
	if _, err := svc.AuthenticateAccessToken(ctx, pair.AccessToken); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("expired access token should be unauthenticated, got %v", err)
	}
	if _, err := svc.RefreshAccessToken(ctx, pair.RefreshToken); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("expired refresh token should be unauthenticated, got %v", err)
	}

	pair, _, err = svc.LoginLocal(ctx, "token-fail", "secret")
	if err != nil {
		t.Fatalf("LoginLocal() after clock advance error = %v", err)
	}
	if err := svc.DisableUser(ctx, "usr_admin", user.ID); err != nil {
		t.Fatalf("DisableUser() error = %v", err)
	}
	if _, err := svc.AuthenticateAccessToken(ctx, pair.AccessToken); shared.CodeOf(err) != shared.CodePermissionDenied {
		t.Fatalf("disabled user access token should be denied, got %v", err)
	}
	if _, err := svc.RefreshAccessToken(ctx, pair.RefreshToken); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("disabled user refresh should be unauthenticated, got %v", err)
	}
}

func TestIssueTokenPairIDFailure(t *testing.T) {
	svc, _, _ := newTestService(t, nil)
	ctx := context.Background()
	if _, err := svc.CreateLocalUser(ctx, CreateLocalUserInput{Username: "idtoken", Password: "secret"}); err != nil {
		t.Fatalf("CreateLocalUser() error = %v", err)
	}
	svc.ids = &failingIDGenerator{remaining: 0}
	if _, _, err := svc.LoginLocal(ctx, "idtoken", "secret"); err == nil {
		t.Fatalf("id generator failure should fail login token issuing")
	}
	svc.ids = &failingIDGenerator{remaining: 1}
	if _, _, err := svc.LoginLocal(ctx, "idtoken", "secret"); err == nil {
		t.Fatalf("second id generator failure should fail login token issuing")
	}
}

func TestOIDCErrorBranches(t *testing.T) {
	ctx := context.Background()
	svc, repo, _ := newTestService(t, &fakeVerifier{claims: OIDCClaims{Issuer: "https://idp", Subject: "sub", Username: "oidc-err"}})
	disabled, err := svc.CreateOIDCProvider(ctx, OIDCProvider{Name: "停用身份源", Issuer: "https://disabled", ClientID: "client", ClientSecretRef: "secret", RedirectURI: "https://paas/callback", Enabled: false})
	if err != nil {
		t.Fatalf("CreateOIDCProvider disabled error = %v", err)
	}
	if _, err := svc.StartOIDC(ctx, disabled.ID); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("disabled provider should be failed_precondition, got %v", err)
	}

	provider, err := svc.CreateOIDCProvider(ctx, OIDCProvider{Name: "企业身份", Issuer: "https://idp", ClientID: "client", ClientSecretRef: "secret", RedirectURI: "https://paas/callback", Enabled: true})
	if err != nil {
		t.Fatalf("CreateOIDCProvider error = %v", err)
	}
	start, err := svc.StartOIDC(ctx, provider.ID)
	if err != nil {
		t.Fatalf("StartOIDC() error = %v", err)
	}
	if _, _, err := svc.CallbackOIDC(ctx, provider.ID, start.State, ""); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("empty code should be invalid_argument, got %v", err)
	}

	svc.verifier = &fakeVerifier{claims: OIDCClaims{Issuer: "https://other-idp", Subject: "sub"}}
	start, err = svc.StartOIDC(ctx, provider.ID)
	if err != nil {
		t.Fatalf("StartOIDC() second error = %v", err)
	}
	if _, _, err := svc.CallbackOIDC(ctx, provider.ID, start.State, "code"); shared.CodeOf(err) != shared.CodeUnauthenticated {
		t.Fatalf("issuer mismatch should be unauthenticated, got %v", err)
	}

	svc.verifier = &fakeVerifier{claims: OIDCClaims{Issuer: "https://idp", Subject: "sub", Username: "oidc-err"}}
	start, err = svc.StartOIDC(ctx, provider.ID)
	if err != nil {
		t.Fatalf("StartOIDC() third error = %v", err)
	}
	_, user, err := svc.CallbackOIDC(ctx, provider.ID, start.State, "code")
	if err != nil {
		t.Fatalf("CallbackOIDC() create user error = %v", err)
	}
	if err := svc.DisableUser(ctx, "usr_admin", user.ID); err != nil {
		t.Fatalf("DisableUser() error = %v", err)
	}
	start, err = svc.StartOIDC(ctx, provider.ID)
	if err != nil {
		t.Fatalf("StartOIDC() disabled user error = %v", err)
	}
	if _, _, err := svc.CallbackOIDC(ctx, provider.ID, start.State, "code"); shared.CodeOf(err) != shared.CodePermissionDenied {
		t.Fatalf("disabled mapped OIDC user should be denied, got %v", err)
	}

	if _, err := repo.FindIdentity(ctx, ProviderOIDC, "https://idp", "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing identity should be not_found, got %v", err)
	}
}

func TestOIDCCreateUserFallbacksAndConflicts(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newTestService(t, &fakeVerifier{claims: OIDCClaims{Issuer: "https://idp", Subject: "sub-email", Email: "mail@example.com"}})
	provider, err := svc.CreateOIDCProvider(ctx, OIDCProvider{Name: "企业身份", Issuer: "https://idp", ClientID: "client", ClientSecretRef: "secret", RedirectURI: "https://paas/callback", Enabled: true})
	if err != nil {
		t.Fatalf("CreateOIDCProvider() error = %v", err)
	}
	start, err := svc.StartOIDC(ctx, provider.ID)
	if err != nil {
		t.Fatalf("StartOIDC() error = %v", err)
	}
	_, user, err := svc.CallbackOIDC(ctx, provider.ID, start.State, "code")
	if err != nil {
		t.Fatalf("CallbackOIDC() email fallback error = %v", err)
	}
	if user.Username != "mail@example.com" {
		t.Fatalf("email fallback username = %q", user.Username)
	}

	if _, err := svc.CreateLocalUser(ctx, CreateLocalUserInput{Username: "taken", Password: "secret"}); err != nil {
		t.Fatalf("CreateLocalUser taken error = %v", err)
	}
	svc.verifier = &fakeVerifier{claims: OIDCClaims{Issuer: "https://idp", Subject: "sub-conflict", Username: "taken"}}
	start, err = svc.StartOIDC(ctx, provider.ID)
	if err != nil {
		t.Fatalf("StartOIDC() conflict error = %v", err)
	}
	if _, _, err := svc.CallbackOIDC(ctx, provider.ID, start.State, "code"); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("username conflict should be conflict, got %v", err)
	}
}

func TestDTOsDoNotExposeSecrets(t *testing.T) {
	userJSON, err := json.Marshal(ToUserDTO(User{ID: "usr_1", Username: "u", Email: "u@example.com"}))
	if err != nil {
		t.Fatalf("marshal user dto: %v", err)
	}
	providerJSON, err := json.Marshal(ToOIDCProviderDTO(OIDCProvider{ID: "oidc_1", Name: "idp", Issuer: "https://idp", ClientID: "client", ClientSecretRef: "secret/ref", Scopes: []string{"openid"}, RedirectURI: "https://paas/callback", Enabled: true}))
	if err != nil {
		t.Fatalf("marshal provider dto: %v", err)
	}
	combined := string(userJSON) + string(providerJSON)
	for _, forbidden := range []string{"password", "password_hash", "client_secret", "secret/ref", "token_hash"} {
		if strings.Contains(combined, forbidden) {
			t.Fatalf("DTO JSON leaked %q: %s", forbidden, combined)
		}
	}
}

func TestScopeCoversAndPermissionAllows(t *testing.T) {
	resource := ResourceScope{Kind: ScopeEnvironment, TenantID: "tenant", ProjectID: "project", ApplicationID: "app", EnvironmentID: "env"}
	tests := []struct {
		kind ScopeKind
		id   shared.ID
		want bool
	}{
		{ScopePlatform, "", true},
		{ScopeTenant, "tenant", true},
		{ScopeProject, "project", true},
		{ScopeApplication, "app", true},
		{ScopeEnvironment, "env", true},
		{ScopeTenant, "other", false},
		{"unknown", "env", false},
	}
	for _, tt := range tests {
		if got := ScopeCovers(tt.kind, tt.id, resource); got != tt.want {
			t.Fatalf("ScopeCovers(%s,%s) = %v, want %v", tt.kind, tt.id, got, tt.want)
		}
	}
	if !PermissionAllows("*:*", "build:create") || !PermissionAllows("build:*", "build:create") || !PermissionAllows("build:create", "build:create") {
		t.Fatalf("wildcard and exact permissions should allow")
	}
	if PermissionAllows("build:read", "build:create") {
		t.Fatalf("different action should not allow")
	}
}

func TestRepositoryListUsersAndConflictPaths(t *testing.T) {
	svc, repo, _ := newTestService(t, nil)
	ctx := context.Background()
	for _, username := range []string{"c", "a", "b"} {
		if _, err := svc.CreateLocalUser(ctx, CreateLocalUserInput{Username: username, Password: "secret"}); err != nil {
			t.Fatalf("CreateLocalUser(%s) error = %v", username, err)
		}
	}
	if _, err := svc.CreateLocalUser(ctx, CreateLocalUserInput{Username: "a", Password: "secret"}); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate username should conflict, got %v", err)
	}
	page, err := repo.ListUsers(ctx, shared.PageRequest{Page: 1, PageSize: 2})
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if page.Total != 3 || len(page.Items) != 2 || page.Items[0].Username != "a" {
		t.Fatalf("unexpected page: %+v", page)
	}
	empty, err := repo.ListUsers(ctx, shared.PageRequest{Page: 9, PageSize: 2})
	if err != nil {
		t.Fatalf("ListUsers empty page error = %v", err)
	}
	if len(empty.Items) != 0 {
		t.Fatalf("expected empty page, got %+v", empty.Items)
	}
	if _, err := repo.GetUser(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing user should be not_found, got %v", err)
	}
	if _, err := repo.GetServiceAccount(ctx, "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing service account should be not_found, got %v", err)
	}
}

func TestServiceQueryPorts(t *testing.T) {
	svc, _, _ := newTestService(t, nil)
	ctx := context.Background()
	user, err := svc.CreateLocalUser(ctx, CreateLocalUserInput{Username: "query", Password: "secret"})
	if err != nil {
		t.Fatalf("CreateLocalUser() error = %v", err)
	}
	identities, err := svc.ListIdentitiesByUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("ListIdentitiesByUser() error = %v", err)
	}
	if len(identities) != 1 || identities[0].Provider != ProviderLocal {
		t.Fatalf("unexpected identities: %+v", identities)
	}
	roles := svc.ListRoles()
	if len(roles) != len(BuiltInRoles()) {
		t.Fatalf("roles length = %d", len(roles))
	}
}

func TestServiceDefaultDependenciesAndValidation(t *testing.T) {
	svc := NewService(Options{Repository: newTestRepository(t)})
	if svc.audit == nil || svc.ids == nil || svc.clock == nil || svc.accessTTL != defaultAccessTTL || svc.refreshTTL != defaultRefreshTTL {
		t.Fatalf("default dependencies were not initialized")
	}
	if err := (NoopAuditLogger{}).Log(context.Background(), AuditEvent{}); err != nil {
		t.Fatalf("NoopAuditLogger should not fail: %v", err)
	}
	if _, err := svc.CreateLocalUser(context.Background(), CreateLocalUserInput{Username: "", Password: ""}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("empty local user input should be invalid_argument, got %v", err)
	}
	if _, err := svc.CreateOIDCProvider(context.Background(), OIDCProvider{}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("empty oidc provider should be invalid_argument, got %v", err)
	}
	svc.ids = &failingIDGenerator{remaining: 0}
	if _, err := svc.CreateOIDCProvider(context.Background(), OIDCProvider{Name: "企业身份", Issuer: "https://idp", ClientID: "client", ClientSecretRef: "secret", RedirectURI: "https://paas/callback"}); err == nil {
		t.Fatalf("oidc provider id generator failure should fail")
	}
	svc.ids = testutil.NewFakeIDGenerator(100)
	if _, err := svc.StartOIDC(context.Background(), "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing oidc provider should be not_found, got %v", err)
	}
	if _, _, err := svc.CallbackOIDC(context.Background(), "missing", "state", "code"); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("missing verifier should be failed_precondition, got %v", err)
	}
	if _, err := svc.CreateRoleBinding(context.Background(), RoleBinding{SubjectType: SubjectUser, SubjectID: "usr_1", RoleID: "unknown", ScopeKind: ScopePlatform}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("unknown role should be invalid_argument, got %v", err)
	}
	if _, err := svc.CreateRoleBinding(context.Background(), RoleBinding{SubjectType: SubjectUser, RoleID: RoleViewer, ScopeKind: ScopePlatform}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("missing subject should be invalid_argument, got %v", err)
	}
	if err := svc.Check(context.Background(), Subject{Type: SubjectUser, ID: "usr_1"}, ResourceScope{Kind: ScopePlatform}, "bad"); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("bad permission should be invalid_argument, got %v", err)
	}
}

func TestHTTPHandlersLoginMeAndOIDCProviderRedaction(t *testing.T) {
	svc, _, _ := newTestService(t, nil)
	ctx := context.Background()
	_, err := svc.CreateLocalUser(ctx, CreateLocalUserInput{Username: "api", Password: "secret"})
	if err != nil {
		t.Fatalf("CreateLocalUser() error = %v", err)
	}
	if _, err := svc.CreateOIDCProvider(ctx, OIDCProvider{Name: "企业身份", Issuer: "https://idp", ClientID: "client", ClientSecretRef: "secret/ref", Scopes: []string{"openid"}, RedirectURI: "https://paas/callback", Enabled: true}); err != nil {
		t.Fatalf("CreateOIDCProvider() error = %v", err)
	}
	mux := http.NewServeMux()
	NewHandler(svc).Register(mux)

	loginBody := strings.NewReader(`{"username":"api","password":"secret"}`)
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", loginBody)
	loginRec := httptest.NewRecorder()
	mux.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d body = %s", loginRec.Code, loginRec.Body.String())
	}
	var loginResp struct {
		Token TokenPair `json:"token"`
		User  UserDTO   `json:"user"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("unmarshal login response: %v", err)
	}
	if loginResp.Token.AccessToken == "" || loginResp.User.Username != "api" {
		t.Fatalf("unexpected login response: %+v", loginResp)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+loginResp.Token.AccessToken)
	meRec := httptest.NewRecorder()
	mux.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("me status = %d body = %s", meRec.Code, meRec.Body.String())
	}

	providerReq := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/providers", nil)
	providerRec := httptest.NewRecorder()
	mux.ServeHTTP(providerRec, providerReq)
	if providerRec.Code != http.StatusOK {
		t.Fatalf("providers status = %d body = %s", providerRec.Code, providerRec.Body.String())
	}
	forbiddenBody := loginRec.Body.String() + meRec.Body.String() + providerRec.Body.String()
	for _, forbidden := range []string{"password_hash", "token_hash", "client_secret", "secret/ref"} {
		if strings.Contains(forbiddenBody, forbidden) {
			t.Fatalf("API response leaked %q: %s", forbidden, forbiddenBody)
		}
	}
}

func TestHTTPHandlersCoverUserRoleTokenAndOIDCFlows(t *testing.T) {
	verifier := &fakeVerifier{claims: OIDCClaims{Issuer: "https://idp", Subject: "sub", Username: "oidc-api"}}
	svc, _, _ := newTestService(t, verifier)
	mux := http.NewServeMux()
	NewHandler(svc).Register(mux)

	createProviderRec := doJSON(mux, http.MethodPost, "/api/auth/oidc/providers", `{"name":"企业身份","issuer":"https://idp","client_id":"client","client_secret_ref":"secret/ref","scopes":["openid"],"redirect_uri":"https://paas/callback","enabled":true}`, "")
	if createProviderRec.Code != http.StatusCreated {
		t.Fatalf("create oidc provider status = %d body = %s", createProviderRec.Code, createProviderRec.Body.String())
	}
	if strings.Contains(createProviderRec.Body.String(), "client_secret") || strings.Contains(createProviderRec.Body.String(), "secret/ref") {
		t.Fatalf("create oidc provider response leaked secret ref: %s", createProviderRec.Body.String())
	}

	createRec := doJSON(mux, http.MethodPost, "/api/users", `{"actor_id":"usr_admin","username":"http","password":"secret","display_name":"HTTP"}`, "")
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create user status = %d body = %s", createRec.Code, createRec.Body.String())
	}
	var created UserDTO
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal created user: %v", err)
	}

	getRec := doJSON(mux, http.MethodGet, "/api/users/"+created.ID.String(), ``, "")
	if getRec.Code != http.StatusOK {
		t.Fatalf("get user status = %d body = %s", getRec.Code, getRec.Body.String())
	}

	resetRec := doJSON(mux, http.MethodPost, "/api/users/"+created.ID.String()+"/reset-password", `{"actor_id":"usr_admin","password":"changed"}`, "")
	if resetRec.Code != http.StatusNoContent {
		t.Fatalf("reset status = %d body = %s", resetRec.Code, resetRec.Body.String())
	}

	loginRec := doJSON(mux, http.MethodPost, "/api/auth/login", `{"username":"http","password":"changed"}`, "")
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login status = %d body = %s", loginRec.Code, loginRec.Body.String())
	}
	var loginResp struct {
		Token TokenPair `json:"token"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("unmarshal login: %v", err)
	}

	refreshRec := doJSON(mux, http.MethodPost, "/api/auth/refresh", `{"refresh_token":"`+loginResp.Token.RefreshToken+`"}`, "")
	if refreshRec.Code != http.StatusOK {
		t.Fatalf("refresh status = %d body = %s", refreshRec.Code, refreshRec.Body.String())
	}

	rolesRec := doJSON(mux, http.MethodGet, "/api/roles", ``, "")
	if rolesRec.Code != http.StatusOK {
		t.Fatalf("roles status = %d body = %s", rolesRec.Code, rolesRec.Body.String())
	}

	roleBindingBody := `{"subject_type":"user","subject_id":"` + created.ID.String() + `","role_id":"viewer","scope_kind":"platform","scope_id":""}`
	roleBindingRec := doJSON(mux, http.MethodPost, "/api/role-bindings", roleBindingBody, "")
	if roleBindingRec.Code != http.StatusCreated {
		t.Fatalf("role binding status = %d body = %s", roleBindingRec.Code, roleBindingRec.Body.String())
	}

	logoutRec := doJSON(mux, http.MethodPost, "/api/auth/logout", ``, loginResp.Token.AccessToken)
	if logoutRec.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d body = %s", logoutRec.Code, logoutRec.Body.String())
	}
	meAfterLogout := doJSON(mux, http.MethodGet, "/api/auth/me", ``, loginResp.Token.AccessToken)
	if meAfterLogout.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout status = %d body = %s", meAfterLogout.Code, meAfterLogout.Body.String())
	}

	var provider OIDCProviderDTO
	if err := json.Unmarshal(createProviderRec.Body.Bytes(), &provider); err != nil {
		t.Fatalf("unmarshal created oidc provider: %v", err)
	}
	startRec := doJSON(mux, http.MethodGet, "/api/auth/oidc/"+provider.ID.String()+"/start", ``, "")
	if startRec.Code != http.StatusOK {
		t.Fatalf("oidc start status = %d body = %s", startRec.Code, startRec.Body.String())
	}
	var start OIDCStartResult
	if err := json.Unmarshal(startRec.Body.Bytes(), &start); err != nil {
		t.Fatalf("unmarshal oidc start: %v", err)
	}
	verifier.wantNonce = svc.states[start.State].Nonce
	callbackReq := httptest.NewRequest(http.MethodGet, "/api/auth/oidc/"+provider.ID.String()+"/callback?state="+start.State+"&code=ok", nil)
	callbackRec := httptest.NewRecorder()
	mux.ServeHTTP(callbackRec, callbackReq)
	if callbackRec.Code != http.StatusOK {
		t.Fatalf("oidc callback status = %d body = %s", callbackRec.Code, callbackRec.Body.String())
	}

	badJSON := doJSON(mux, http.MethodPost, "/api/auth/login", `{bad`, "")
	if badJSON.Code != http.StatusBadRequest {
		t.Fatalf("bad json status = %d body = %s", badJSON.Code, badJSON.Body.String())
	}
	noBearerLogout := doJSON(mux, http.MethodPost, "/api/auth/logout", ``, "")
	if noBearerLogout.Code != http.StatusUnauthorized {
		t.Fatalf("logout without bearer status = %d", noBearerLogout.Code)
	}
	missingUser := doJSON(mux, http.MethodGet, "/api/users/usr_missing", ``, "")
	if missingUser.Code != http.StatusNotFound {
		t.Fatalf("missing user status = %d", missingUser.Code)
	}
	badReset := doJSON(mux, http.MethodPost, "/api/users/"+created.ID.String()+"/reset-password", `{"actor_id":"usr_admin","password":""}`, "")
	if badReset.Code != http.StatusBadRequest {
		t.Fatalf("bad reset status = %d", badReset.Code)
	}
	badRefresh := doJSON(mux, http.MethodPost, "/api/auth/refresh", `{"refresh_token":"bad"}`, "")
	if badRefresh.Code != http.StatusUnauthorized {
		t.Fatalf("bad refresh status = %d", badRefresh.Code)
	}
	badRoleBinding := doJSON(mux, http.MethodPost, "/api/role-bindings", `{"subject_type":"user","subject_id":"usr_1","role_id":"bad","scope_kind":"platform"}`, "")
	if badRoleBinding.Code != http.StatusBadRequest {
		t.Fatalf("bad role binding status = %d", badRoleBinding.Code)
	}
}

func doJSON(handler http.Handler, method string, path string, body string, bearer string) *httptest.ResponseRecorder {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
