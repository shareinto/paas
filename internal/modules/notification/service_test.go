package notification

import (
	"context"
	"encoding/json"
	"errors"
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

func TestHandleEventRendersTemplateSendsAndDedupes(t *testing.T) {
	repo := newTestRepository(t)
	sender := &FakeSender{}
	svc := NewService(Options{Repository: repo, Sender: sender, IDGenerator: &staticIDs{ids: []shared.ID{"notification_1"}}, Clock: fixedClock{now: time.Date(2026, 5, 30, 11, 0, 0, 0, time.UTC)}})
	if err := svc.EnsureDefaults(context.Background()); err != nil {
		t.Fatalf("defaults: %v", err)
	}
	event := Event{Type: "BuildFailed", TenantID: "tenant_1", ProjectID: "project_1", Payload: map[string]any{"application_name": "订单服务", "build_run_id": "build_1"}}
	first, err := svc.HandleEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("handle event: %v", err)
	}
	second, err := svc.HandleEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("handle duplicate: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("duplicate event should return existing notification")
	}
	if len(sender.Messages) != 1 || sender.Messages[0].Title != "构建失败：订单服务" {
		t.Fatalf("unexpected sent messages: %#v", sender.Messages)
	}
	if first.Status != NotificationSucceeded || first.Attempts != 1 {
		t.Fatalf("unexpected notification status: %#v", first)
	}
}

func TestSendFailureCanRetry(t *testing.T) {
	repo := newTestRepository(t)
	sender := &FakeSender{Err: errors.New("network")}
	svc := NewService(Options{Repository: repo, Sender: sender, IDGenerator: &staticIDs{ids: []shared.ID{"notification_1"}}, Clock: fixedClock{now: time.Date(2026, 5, 30, 11, 0, 0, 0, time.UTC)}})
	if err := svc.EnsureDefaults(context.Background()); err != nil {
		t.Fatalf("defaults: %v", err)
	}
	event := Event{Type: "DeploymentFailed", Payload: map[string]any{"environment_name": "生产", "deployment_id": "deployment_1"}}
	notification, err := svc.HandleEvent(context.Background(), event)
	if err == nil {
		t.Fatalf("expected send error")
	}
	if notification.Status != NotificationFailed || notification.Attempts != 1 {
		t.Fatalf("unexpected failed notification: %#v", notification)
	}
	sender.Err = nil
	retried, err := svc.SendPending(context.Background(), notification.ID)
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if retried.Status != NotificationSucceeded || retried.Attempts != 2 {
		t.Fatalf("unexpected retried notification: %#v", retried)
	}
}

func TestCreateChannelValidationAndHTTPList(t *testing.T) {
	repo := newTestRepository(t)
	sender := &FakeSender{}
	svc := NewService(Options{Repository: repo, Sender: sender, IDGenerator: &staticIDs{ids: []shared.ID{"channel_1", "notification_1"}}, Clock: fixedClock{now: time.Date(2026, 5, 30, 11, 0, 0, 0, time.UTC)}})
	if _, err := svc.CreateChannel(context.Background(), NotificationChannel{Name: " ", Type: ChannelFake}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("blank channel name should fail, got %v", err)
	}
	channel, err := svc.CreateChannel(context.Background(), NotificationChannel{Name: "平台通知", Type: ChannelWebhook, Target: "https://hook.example"})
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if channel.ID != "channel_1" || !channel.Enabled {
		t.Fatalf("unexpected channel: %#v", channel)
	}
	if err := svc.EnsureDefaults(context.Background()); err != nil {
		t.Fatalf("defaults: %v", err)
	}
	if _, err := svc.HandleEvent(context.Background(), Event{Type: "PromotionApproved", Payload: map[string]any{"promotion_id": "promotion_1"}}); err != nil {
		t.Fatalf("handle event: %v", err)
	}

	mux := http.NewServeMux()
	NewHandler(svc).Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/notifications?page=1&page_size=10", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", rec.Code, rec.Body.String())
	}
	var page shared.PageResult[Notification]
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if page.Total != 1 || page.Items[0].Title != "发布已审批通过" {
		t.Fatalf("unexpected notifications: %#v", page)
	}
}

func TestUnsupportedEventAndInvalidTemplateFail(t *testing.T) {
	repo := newTestRepository(t)
	svc := NewService(Options{Repository: repo, IDGenerator: &staticIDs{ids: []shared.ID{"notification_1"}}, Clock: fixedClock{now: time.Date(2026, 5, 30, 11, 0, 0, 0, time.UTC)}})
	if _, err := svc.HandleEvent(context.Background(), Event{Type: "Unknown"}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("unsupported event should fail, got %v", err)
	}
	if _, _, err := render(NotificationTemplate{TitleTemplate: "{{", ContentTemplate: "ok"}, nil); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("invalid template should fail, got %v", err)
	}
}

func TestNotificationRepositoryEdgesAndErrorWriter(t *testing.T) {
	repo := newTestRepository(t)
	svc := NewService(Options{Repository: repo, IDGenerator: &staticIDs{ids: []shared.ID{"notification_1"}}, Clock: fixedClock{now: time.Date(2026, 5, 30, 11, 0, 0, 0, time.UTC)}})
	if err := svc.EnsureDefaults(context.Background()); err != nil {
		t.Fatalf("defaults: %v", err)
	}
	if err := svc.EnsureDefaults(context.Background()); err != nil {
		t.Fatalf("defaults should be idempotent: %v", err)
	}
	if _, err := svc.CreateChannel(context.Background(), NotificationChannel{Name: "Webhook", Type: ChannelWebhook}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("webhook without target should fail, got %v", err)
	}
	notification, err := svc.HandleEvent(context.Background(), Event{Type: "ClusterUnreachable", Payload: map[string]any{"cluster_name": "生产集群", "cluster_id": "cluster_1"}})
	if err != nil {
		t.Fatalf("handle event: %v", err)
	}
	again, err := svc.SendPending(context.Background(), notification.ID)
	if err != nil {
		t.Fatalf("send succeeded notification: %v", err)
	}
	if again.Attempts != 1 {
		t.Fatalf("succeeded notification should not be resent: %#v", again)
	}
	if _, err := repo.GetNotification(context.Background(), "missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing notification should fail, got %v", err)
	}
	rec := httptest.NewRecorder()
	writeError(rec, shared.NewError(shared.CodePermissionDenied, "denied"))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("writeError status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestNotificationHandleEventFailureBranches(t *testing.T) {
	now := time.Date(2026, 5, 30, 11, 0, 0, 0, time.UTC)
	repo := newTestRepository(t)
	svc := NewService(Options{Repository: repo, IDGenerator: &staticIDs{ids: []shared.ID{"notification_1"}}, Clock: fixedClock{now: now}})
	if _, err := svc.HandleEvent(context.Background(), Event{Type: "BuildFailed", Payload: map[string]any{"application_name": "订单服务"}}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing template should fail, got %v", err)
	}
	if err := repo.CreateTemplate(context.Background(), NotificationTemplate{ID: "template_1", EventType: "BuildFailed", TitleTemplate: "构建失败", ContentTemplate: "失败", Enabled: false}); err != nil {
		t.Fatalf("create disabled template: %v", err)
	}
	if _, err := svc.HandleEvent(context.Background(), Event{Type: "BuildFailed", Payload: map[string]any{"application_name": "订单服务"}}); shared.CodeOf(err) != shared.CodeFailedPrecondition {
		t.Fatalf("disabled template should fail, got %v", err)
	}

	repo = newTestRepository(t)
	svc = NewService(Options{Repository: repo, IDGenerator: &staticIDs{ids: []shared.ID{"notification_1"}}, Clock: fixedClock{now: now}})
	if err := repo.CreateTemplate(context.Background(), NotificationTemplate{ID: "template_1", EventType: "BuildFailed", TitleTemplate: "构建失败", ContentTemplate: "失败", Enabled: true}); err != nil {
		t.Fatalf("create template: %v", err)
	}
	if _, err := svc.HandleEvent(context.Background(), Event{Type: "BuildFailed", Payload: map[string]any{"application_name": "订单服务"}}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing channel should fail, got %v", err)
	}

	repo = newTestRepository(t)
	svc = NewService(Options{Repository: repo, IDGenerator: &staticIDs{ids: []shared.ID{"notification_1"}}, Clock: fixedClock{now: now}})
	if err := repo.CreateTemplate(context.Background(), NotificationTemplate{ID: "template_1", EventType: "BuildFailed", TitleTemplate: "{{.bad", ContentTemplate: "失败", Enabled: true}); err != nil {
		t.Fatalf("create invalid template: %v", err)
	}
	if err := repo.CreateChannel(context.Background(), NotificationChannel{ID: "channel_1", Name: "默认", Type: ChannelFake, Enabled: true}); err != nil {
		t.Fatalf("create channel: %v", err)
	}
	if _, err := svc.HandleEvent(context.Background(), Event{Type: "BuildFailed", Payload: map[string]any{"application_name": "订单服务"}}); shared.CodeOf(err) != shared.CodeInvalidArgument {
		t.Fatalf("render error should fail, got %v", err)
	}
}

func TestNotificationRepositoryConflictAndMissingBranches(t *testing.T) {
	repo := newTestRepository(t)
	ctx := context.Background()
	template := NotificationTemplate{ID: "template_1", EventType: "BuildFailed", Enabled: true}
	if err := repo.CreateTemplate(ctx, template); err != nil {
		t.Fatalf("create template: %v", err)
	}
	if _, err := repo.FindTemplateByEventType(ctx, "Missing"); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing template should fail, got %v", err)
	}
	if _, err := repo.GetDefaultChannel(ctx); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("missing default channel should fail, got %v", err)
	}
	notification := Notification{ID: "notification_1", DedupeKey: "key_1"}
	if err := repo.CreateNotification(ctx, notification); err != nil {
		t.Fatalf("create notification: %v", err)
	}
	if err := repo.CreateNotification(ctx, notification); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate notification id should conflict, got %v", err)
	}
	if err := repo.CreateNotification(ctx, Notification{ID: "notification_2", DedupeKey: "key_1"}); shared.CodeOf(err) != shared.CodeConflict {
		t.Fatalf("duplicate dedupe key should conflict, got %v", err)
	}
	if err := repo.UpdateNotification(ctx, Notification{ID: "missing"}); shared.CodeOf(err) != shared.CodeNotFound {
		t.Fatalf("update missing notification should fail, got %v", err)
	}
	if _, err := repo.ListNotifications(ctx, shared.PageRequest{Page: 2, PageSize: 10}); err != nil {
		t.Fatalf("list page: %v", err)
	}
}
