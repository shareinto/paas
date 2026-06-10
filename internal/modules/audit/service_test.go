package audit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/shareinto/paas/internal/shared"
	"github.com/shareinto/paas/internal/testsupport"
)

type staticIDs struct{ ids []shared.ID }

func (s *staticIDs) NewID(string) (shared.ID, error) {
	id := s.ids[0]
	s.ids = s.ids[1:]
	return id, nil
}

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

func newTestRepository(t *testing.T) *MySQLRepository {
	t.Helper()
	repo, err := NewMySQLRepository(context.Background(), testsupport.MySQLDB(t, Migrations...))
	if err != nil {
		t.Fatalf("NewMySQLRepository() error = %v", err)
	}
	return repo
}

func TestAuditLogIsAppendOnlyAndSanitizesSensitiveDetails(t *testing.T) {
	repo := newTestRepository(t)
	now := time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	svc := NewService(Options{Repository: repo, IDGenerator: &staticIDs{ids: []shared.ID{"audit_1"}}, Clock: fixedClock{now: now}})
	err := svc.Log(context.Background(), AuditLog{
		ActorID:      "user_1",
		TenantID:     "tenant_1",
		ResourceType: "access_token",
		ResourceID:   "token_1",
		Action:       "token.issue",
		Details:      map[string]string{"token": "plain", "note": "ok"},
	})
	if err != nil {
		t.Fatalf("log: %v", err)
	}
	log, err := svc.Get(context.Background(), "audit_1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if log.Details["token"] != "[REDACTED]" || log.Details["note"] != "ok" {
		t.Fatalf("sensitive details not sanitized: %#v", log.Details)
	}
	if !log.OccurredAt.Equal(now) || !log.CreatedAt.Equal(now) {
		t.Fatalf("timestamps not assigned")
	}
	if err := repo.Append(context.Background(), log); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate append should conflict, got %v", err)
	}
}

func TestAuditQueryFiltersByTenantActorResourceActionAndTime(t *testing.T) {
	repo := newTestRepository(t)
	now := time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	svc := NewService(Options{Repository: repo, IDGenerator: &staticIDs{ids: []shared.ID{"audit_1", "audit_2"}}, Clock: fixedClock{now: now}})
	_ = svc.Log(context.Background(), AuditLog{TenantID: "tenant_1", ProjectID: "project_1", ActorID: "user_1", ResourceType: "promotion", ResourceID: "promotion_1", Action: "promotion.approve", OccurredAt: now})
	_ = svc.Log(context.Background(), AuditLog{TenantID: "tenant_2", ActorID: "user_2", ResourceType: "build", ResourceID: "build_1", Action: "build.cancel", OccurredAt: now.Add(time.Hour)})
	from := now.Add(-time.Minute)
	to := now.Add(time.Minute)
	result, err := svc.List(context.Background(), Query{TenantID: "tenant_1", ProjectID: "project_1", ActorID: "user_1", ResourceType: "promotion", ResourceID: "promotion_1", Action: "promotion.approve", From: &from, To: &to}, shared.PageRequest{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if result.Total != 1 || result.Items[0].ID != "audit_1" {
		t.Fatalf("unexpected query result: %#v", result)
	}
}

func TestAuditHTTPHandlerListsGetsAndValidatesQuery(t *testing.T) {
	repo := newTestRepository(t)
	now := time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	svc := NewService(Options{Repository: repo, IDGenerator: &staticIDs{ids: []shared.ID{"audit_1"}}, Clock: fixedClock{now: now}})
	if err := svc.Log(context.Background(), AuditLog{TenantID: "tenant_1", ActorID: "user_1", ResourceType: "build", ResourceID: "build_1", Action: "build.trigger", Result: "succeeded"}); err != nil {
		t.Fatalf("log: %v", err)
	}
	mux := http.NewServeMux()
	NewHandler(svc).Register(mux)

	list := httptest.NewRecorder()
	mux.ServeHTTP(list, httptest.NewRequest(http.MethodGet, "/api/audit/logs?tenant_id=tenant_1&page=1&page_size=10", nil))
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", list.Code, list.Body.String())
	}
	var page shared.PageResult[AuditLog]
	if err := json.Unmarshal(list.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if page.Total != 1 || page.Items[0].Action != "build.trigger" {
		t.Fatalf("unexpected page: %#v", page)
	}

	get := httptest.NewRecorder()
	mux.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/audit/logs/audit_1", nil))
	if get.Code != http.StatusOK {
		t.Fatalf("get status = %d body=%s", get.Code, get.Body.String())
	}

	invalid := httptest.NewRecorder()
	mux.ServeHTTP(invalid, httptest.NewRequest(http.MethodGet, "/api/audit/logs?from=bad-time", nil))
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid query status = %d body=%s", invalid.Code, invalid.Body.String())
	}
	invalidTo := httptest.NewRecorder()
	mux.ServeHTTP(invalidTo, httptest.NewRequest(http.MethodGet, "/api/audit/logs?to=bad-time", nil))
	if invalidTo.Code != http.StatusBadRequest {
		t.Fatalf("invalid to query status = %d body=%s", invalidTo.Code, invalidTo.Body.String())
	}
	missing := httptest.NewRecorder()
	mux.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/api/audit/logs/missing", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing get status = %d body=%s", missing.Code, missing.Body.String())
	}
}

func TestAuditDefaultsValidationAndNilBridge(t *testing.T) {
	if _, err := normalizeLog(AuditLog{ResourceType: "build"}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("missing action should fail, got %v", err)
	}
	if _, err := normalizeLog(AuditLog{Action: "build.trigger"}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("missing resource type should fail, got %v", err)
	}
	if _, err := normalizeLog(AuditLog{Action: "build.trigger", ResourceType: "build", Result: "bad"}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("bad result should fail, got %v", err)
	}
	log, err := normalizeLog(AuditLog{Action: " build.trigger ", ResourceType: " build ", ActorType: " ", Details: map[string]string{"kubeconfig": "raw"}})
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if log.ActorType != "user" || log.SubjectType != "user" || log.Result != ResultSucceeded || log.Details["kubeconfig"] != "[REDACTED]" {
		t.Fatalf("unexpected normalized log: %#v", log)
	}
	if err := logIfConfigured(context.Background(), nil, AuditLog{Action: "noop", ResourceType: "noop"}); err != nil {
		t.Fatalf("nil bridge logger should no-op: %v", err)
	}
}

func TestAuditRepositoryQueryRejectsEveryFilterMismatch(t *testing.T) {
	repo := newTestRepository(t)
	now := time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	log := AuditLog{TenantID: "tenant_1", ProjectID: "project_1", ActorID: "user_1", ResourceType: "build", ResourceID: "build_1", Action: "build.trigger", OccurredAt: now}
	if err := repo.Append(context.Background(), log); err != nil {
		t.Fatalf("append log: %v", err)
	}
	before := now.Add(time.Minute)
	after := now.Add(-time.Minute)
	cases := []Query{
		{TenantID: "other"},
		{ProjectID: "other"},
		{ActorID: "other"},
		{ResourceType: "promotion"},
		{ResourceID: "other"},
		{Action: "promotion.approve"},
		{From: &before},
		{To: &after},
	}
	for _, query := range cases {
		result, err := repo.List(context.Background(), query, shared.PageRequest{Page: 1, PageSize: 10})
		if err != nil {
			t.Fatalf("list query %#v: %v", query, err)
		}
		if result.Total != 0 {
			t.Fatalf("query should not match: %#v result=%#v", query, result)
		}
	}
	result, err := repo.List(context.Background(), Query{}, shared.PageRequest{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("empty list: %v", err)
	}
	if result.Total != 1 {
		t.Fatalf("empty query should match, got %#v", result)
	}
}
