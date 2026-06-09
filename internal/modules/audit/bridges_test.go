package audit

import (
	"context"
	"testing"
	"time"

	"github.com/shareinto/paas/internal/modules/appenv"
	"github.com/shareinto/paas/internal/modules/build"
	"github.com/shareinto/paas/internal/modules/clusteragent"
	"github.com/shareinto/paas/internal/modules/delivery"
	"github.com/shareinto/paas/internal/modules/gitops"
	"github.com/shareinto/paas/internal/modules/identityaccess"
	"github.com/shareinto/paas/internal/modules/sourcerepository"
	"github.com/shareinto/paas/internal/modules/tenantproject"
	"github.com/shareinto/paas/internal/shared"
)

func TestAuditBridgesCoverCriticalEventsAndBuildSpecDetails(t *testing.T) {
	repo := NewMemoryRepository()
	svc := NewService(Options{Repository: repo, IDGenerator: &staticIDs{ids: []shared.ID{
		"audit_1", "audit_2", "audit_3", "audit_4", "audit_5", "audit_6", "audit_7", "audit_8", "audit_9", "audit_10", "audit_11", "audit_12", "audit_13", "audit_14", "audit_15", "audit_16",
	}}, Clock: fixedClock{now: time.Date(2026, 5, 30, 15, 0, 0, 0, time.UTC)}})
	ctx := context.Background()
	now := time.Date(2026, 5, 30, 15, 0, 0, 0, time.UTC)
	_ = (IdentityAccessLogger{Logger: svc}).Log(ctx, identityaccess.AuditEvent{ActorID: "user_1", Action: "auth.login_local", ResourceType: "user", ResourceID: "user_1", Result: "succeeded", Summary: "本地账号密码登录", OccurredAt: now})
	_ = (IdentityAccessLogger{Logger: svc}).Log(ctx, identityaccess.AuditEvent{ActorID: "user_2", Action: "auth.login_oidc", ResourceType: "user", ResourceID: "user_2", Result: "succeeded", Summary: "企业身份登录", OccurredAt: now})
	_ = (TenantProjectLogger{Logger: svc}).Log(ctx, tenantproject.AuditEvent{ActorID: "admin", Action: "tenant.create", ResourceType: "tenant", ResourceID: "tenant_1", Result: "succeeded", Summary: "创建租户", OccurredAt: now})
	_ = (TenantProjectLogger{Logger: svc}).Log(ctx, tenantproject.AuditEvent{ActorID: "admin", Action: "project.create", ResourceType: "project", ResourceID: "project_1", Result: "succeeded", Summary: "创建项目", OccurredAt: now})
	_ = (TenantProjectLogger{Logger: svc}).Log(ctx, tenantproject.AuditEvent{ActorID: "admin", Action: "tenant_member.upsert", ResourceType: "tenant", ResourceID: "tenant_1", Result: "succeeded", Summary: "修改成员权限", OccurredAt: now})
	_ = (ApplicationEnvironmentLogger{Logger: svc}).Log(ctx, appenv.AuditEvent{ActorID: "user_1", Action: "application.create", ResourceType: "application", ResourceID: "app_1", Result: "succeeded", Summary: "创建应用", OccurredAt: now})
	_ = (ApplicationEnvironmentLogger{Logger: svc}).Log(ctx, appenv.AuditEvent{ActorID: "user_1", Action: "environment_config.update", ResourceType: "environment_config", ResourceID: "cfg_1", Result: "succeeded", Summary: "修改环境配置", OccurredAt: now})
	_ = (ApplicationEnvironmentLogger{Logger: svc}).Log(ctx, appenv.AuditEvent{ActorID: "user_1", Action: "environment_secret.update", ResourceType: "environment_secret", ResourceID: "secret_1", Result: "succeeded", Summary: "修改密钥", OccurredAt: now})
	_ = (SourceRepositoryLogger{Logger: svc}).Log(ctx, sourcerepository.AuditEvent{ActorID: "user_1", Action: "repository_migration.create", ResourceType: "repository_migration", ResourceID: "migration_1", Result: "succeeded", Summary: "迁移仓库", OccurredAt: now})
	_ = (BuildLogger{Logger: svc}).Log(ctx, build.AuditEvent{ActorID: "user_1", Action: "build.trigger", ResourceType: "build_run", ResourceID: "build_1", Result: "succeeded", Summary: "触发构建", Details: map[string]string{"build_command": "mvn clean package", "artifact_copy_command": "cp -ar target/app.jar \"$PAAS_ARTIFACT_OUTPUT/app.jar\"", "runtime_base_image": "registry/runtime:17", "token": "plain"}, OccurredAt: now})
	_ = (BuildLogger{Logger: svc}).Log(ctx, build.AuditEvent{ActorID: "user_1", Action: "build.cancel", ResourceType: "build_run", ResourceID: "build_1", Result: "succeeded", Summary: "取消构建", OccurredAt: now})
	_ = (DeliveryLogger{Logger: svc}).Log(ctx, delivery.AuditEvent{ActorID: "user_1", Action: "freight.create", ResourceType: "freight", ResourceID: "freight_1", Result: "succeeded", Summary: "创建 Freight", OccurredAt: now})
	_ = (DeliveryLogger{Logger: svc}).Log(ctx, delivery.AuditEvent{ActorID: "user_1", Action: "promotion.create", ResourceType: "promotion", ResourceID: "promotion_1", Result: "succeeded", Summary: "创建 Promotion", OccurredAt: now})
	_ = (DeliveryLogger{Logger: svc}).Log(ctx, delivery.AuditEvent{ActorID: "user_2", Action: "promotion.approve", ResourceType: "promotion", ResourceID: "promotion_1", Result: "succeeded", Summary: "审批发布", OccurredAt: now})
	_ = (GitOpsLogger{Logger: svc}).Log(ctx, gitops.AuditEvent{ActorID: "user_1", TenantID: "tenant_1", ProjectID: "project_1", Action: "manifest_revision.create", ResourceType: "manifest_revision", ResourceID: "manifest_1", Result: "succeeded", Summary: "回滚清单修改", OccurredAt: now})
	_ = (ClusterAgentLogger{Logger: svc}).Log(ctx, clusteragent.AuditEvent{TenantID: "tenant_1", Action: "cluster.unreachable", ResourceType: "cluster", ResourceID: "cluster_1", Result: "failed", Summary: "Agent 离线", OccurredAt: now})

	result, err := svc.List(ctx, Query{}, shared.PageRequest{Page: 1, PageSize: 50})
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	seen := map[string]AuditLog{}
	for _, item := range result.Items {
		seen[item.Action] = item
	}
	for _, action := range []string{"auth.login_local", "auth.login_oidc", "tenant.create", "project.create", "tenant_member.upsert", "application.create", "environment_config.update", "environment_secret.update", "repository_migration.create", "build.trigger", "build.cancel", "freight.create", "promotion.create", "promotion.approve", "manifest_revision.create", "cluster.unreachable"} {
		if _, ok := seen[action]; !ok {
			t.Fatalf("missing critical audit action %s in %#v", action, seen)
		}
	}
	buildLog := seen["build.trigger"]
	if buildLog.Details["build_command"] == "" || buildLog.Details["artifact_copy_command"] == "" || buildLog.Details["runtime_base_image"] == "" {
		t.Fatalf("build spec audit details missing: %#v", buildLog.Details)
	}
	if buildLog.Details["token"] != "[REDACTED]" {
		t.Fatalf("sensitive audit detail was not redacted: %#v", buildLog.Details)
	}
}
