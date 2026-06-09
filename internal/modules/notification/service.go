package notification

import (
	"bytes"
	"context"
	"text/template"

	"github.com/shareinto/paas/internal/shared"
)

type Service struct {
	repo   Repository
	sender Sender
	ids    shared.IDGenerator
	clock  shared.Clock
}

type Options struct {
	Repository  Repository
	Sender      Sender
	IDGenerator shared.IDGenerator
	Clock       shared.Clock
}

func NewService(opts Options) *Service {
	ids := opts.IDGenerator
	if ids == nil {
		ids = shared.RandomIDGenerator{}
	}
	clock := opts.Clock
	if clock == nil {
		clock = shared.SystemClock{}
	}
	sender := opts.Sender
	if sender == nil {
		sender = &FakeSender{}
	}
	return &Service{repo: opts.Repository, sender: sender, ids: ids, clock: clock}
}

func (s *Service) EnsureDefaults(ctx context.Context) error {
	now := s.clock.Now()
	for _, templateSpec := range defaultTemplates {
		template := NotificationTemplate{ID: shared.ID("template_" + templateSpec.EventType), EventType: templateSpec.EventType, TitleTemplate: templateSpec.TitleTemplate, ContentTemplate: templateSpec.ContentTemplate, Enabled: true, CreatedAt: now, UpdatedAt: now}
		if err := s.repo.CreateTemplate(ctx, template); err != nil && shared.CodeOf(err) != shared.CodeConflict {
			return err
		}
	}
	channel := NotificationChannel{ID: "channel_fake_default", Name: "默认测试通知渠道", Type: ChannelFake, Enabled: true, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateChannel(ctx, channel); err != nil && shared.CodeOf(err) != shared.CodeConflict {
		return err
	}
	return nil
}

func (s *Service) CreateChannel(ctx context.Context, channel NotificationChannel) (NotificationChannel, error) {
	normalized, err := normalizeChannel(channel)
	if err != nil {
		return NotificationChannel{}, err
	}
	now := s.clock.Now()
	if normalized.ID.IsZero() {
		normalized.ID, err = s.ids.NewID("channel")
		if err != nil {
			return NotificationChannel{}, err
		}
	}
	normalized.Enabled = true
	normalized.CreatedAt = now
	normalized.UpdatedAt = now
	return normalized, s.repo.CreateChannel(ctx, normalized)
}

func (s *Service) HandleEvent(ctx context.Context, event Event) (Notification, error) {
	if !supportedEvent(event.Type) {
		return Notification{}, shared.NewError(shared.CodeInvalidArgument, "unsupported notification event")
	}
	key := dedupeKey(event)
	if existing, err := s.repo.FindNotificationByDedupeKey(ctx, key); err == nil {
		return existing, nil
	}
	template, err := s.repo.FindTemplateByEventType(ctx, event.Type)
	if err != nil {
		return Notification{}, err
	}
	if !template.Enabled {
		return Notification{}, shared.NewError(shared.CodeFailedPrecondition, "notification template is disabled")
	}
	channel, err := s.repo.GetDefaultChannel(ctx)
	if err != nil {
		return Notification{}, err
	}
	title, content, err := render(template, event.Payload)
	if err != nil {
		return Notification{}, err
	}
	id, err := s.ids.NewID("notification")
	if err != nil {
		return Notification{}, err
	}
	now := s.clock.Now()
	notification := Notification{ID: id, TenantID: event.TenantID, ProjectID: event.ProjectID, EventType: event.Type, DedupeKey: key, ChannelID: channel.ID, Title: title, Content: content, Status: NotificationPending, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.CreateNotification(ctx, notification); err != nil {
		return Notification{}, err
	}
	return s.SendPending(ctx, notification.ID)
}

func (s *Service) SendPending(ctx context.Context, id shared.ID) (Notification, error) {
	notification, err := s.repo.GetNotification(ctx, id)
	if err != nil {
		return Notification{}, err
	}
	if notification.Status == NotificationSucceeded {
		return notification, nil
	}
	channel, err := s.repo.GetDefaultChannel(ctx)
	if err != nil {
		return Notification{}, err
	}
	now := s.clock.Now()
	notification.Status = NotificationSending
	notification.Attempts++
	notification.UpdatedAt = now
	if err := s.repo.UpdateNotification(ctx, notification); err != nil {
		return Notification{}, err
	}
	if err := s.sender.Send(ctx, channel, Message{Title: notification.Title, Content: notification.Content}); err != nil {
		notification.Status = NotificationFailed
		notification.ErrorMessage = err.Error()
		notification.UpdatedAt = s.clock.Now()
		_ = s.repo.UpdateNotification(ctx, notification)
		return notification, err
	}
	sentAt := s.clock.Now()
	notification.Status = NotificationSucceeded
	notification.ErrorMessage = ""
	notification.UpdatedAt = sentAt
	notification.SentAt = &sentAt
	return notification, s.repo.UpdateNotification(ctx, notification)
}

func (s *Service) ListNotifications(ctx context.Context, page shared.PageRequest) (shared.PageResult[Notification], error) {
	return s.repo.ListNotifications(ctx, page)
}

func render(templateSpec NotificationTemplate, payload map[string]any) (string, string, error) {
	title, err := renderString(templateSpec.TitleTemplate, payload)
	if err != nil {
		return "", "", err
	}
	content, err := renderString(templateSpec.ContentTemplate, payload)
	if err != nil {
		return "", "", err
	}
	return title, content, nil
}

func renderString(tmpl string, payload map[string]any) (string, error) {
	parsed, err := template.New("notification").Option("missingkey=zero").Parse(tmpl)
	if err != nil {
		return "", shared.WrapError(shared.CodeInvalidArgument, "invalid notification template", err)
	}
	var buf bytes.Buffer
	if err := parsed.Execute(&buf, payload); err != nil {
		return "", shared.WrapError(shared.CodeInvalidArgument, "render notification template failed", err)
	}
	return buf.String(), nil
}

var defaultTemplates = []NotificationTemplate{
	{EventType: "BuildFailed", TitleTemplate: "构建失败：{{.application_name}}", ContentTemplate: "构建 {{.build_run_id}} 执行失败，请在平台控制台查看日志。"},
	{EventType: "PromotionCreated", TitleTemplate: "发布待处理：{{.environment_name}}", ContentTemplate: "变更包 {{.freight_id}} 已创建发布晋级。"},
	{EventType: "PromotionApproved", TitleTemplate: "发布已审批通过", ContentTemplate: "发布 {{.promotion_id}} 已通过审批。"},
	{EventType: "PromotionRejected", TitleTemplate: "发布已被拒绝", ContentTemplate: "发布 {{.promotion_id}} 已被拒绝。"},
	{EventType: "DeploymentFailed", TitleTemplate: "部署失败：{{.environment_name}}", ContentTemplate: "部署 {{.deployment_id}} 失败，请检查环境事件。"},
	{EventType: "ClusterUnreachable", TitleTemplate: "集群离线：{{.cluster_name}}", ContentTemplate: "集群 {{.cluster_id}} 心跳超时，已标记为离线。"},
}
