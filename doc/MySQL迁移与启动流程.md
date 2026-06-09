# MySQL 迁移与启动流程

本文档定义测试可用版的 MySQL 迁移执行方式。所有业务表迁移以 Go 代码中的 `database.Migration` 为正式来源，由 `internal/migrations.All()` 统一聚合并按版本号升序执行。

## 迁移来源

- `internal/modules/identityaccess/migrations.go`
- `internal/modules/tenantproject/migrations.go`
- `internal/modules/sourcerepository/migrations.go`
- `internal/modules/appenv/migrations.go`
- `internal/modules/build/migrations.go`
- `internal/modules/delivery/migrations.go`
- `internal/modules/audit/migrations.go`
- `internal/modules/notification/migrations.go`
- `internal/modules/clusteragent/migrations.go`
- `internal/modules/gitops/migrations.go`

统一入口：

```text
internal/migrations.All()
```

迁移执行器会自动创建 `schema_migrations` 表，并记录已应用的迁移版本。迁移 SQL 使用 MySQL 8.0 方言，业务表统一使用 InnoDB、`utf8mb4` 和 `utf8mb4_unicode_ci`。

## 启动前准备

确认 MySQL 连接环境变量已配置：

```bash
export MYSQL_HOST=127.0.0.1
export MYSQL_PORT=3306
export MYSQL_DATABASE=paas
export MYSQL_USER=paas
export MYSQL_PASSWORD='按环境注入'
```

连接验证：

```bash
mysql -h"$MYSQL_HOST" -P"$MYSQL_PORT" -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" "$MYSQL_DATABASE" -e \
  "SELECT VERSION(), @@character_set_server, @@collation_server;"
```

## 启动时自动迁移

`paas-server` 默认不连接 MySQL 执行迁移，避免本地开发启动被外部数据库阻塞。试点或部署环境需要显式开启：

```bash
export PAAS_AUTO_MIGRATE=true
export PAAS_HTTP_ADDR=:8080
go run ./cmd/paas-server
```

启动流程：

1. 读取 `MYSQL_*` 环境变量。
2. 连接 MySQL 并 ping。
3. 创建 `schema_migrations` 表。
4. 读取 `internal/migrations.All()`。
5. 按版本号升序执行未应用迁移。
6. 迁移成功后继续启动 PaaS 控制面 HTTP 服务。

如果迁移失败，`paas-server` 会停止启动，并在日志中输出失败原因。

## 回滚方式

当前回滚入口在代码层通过 `database.NewMigrator(db).Down(ctx, migrations.All())` 提供。回滚会按版本号倒序执行已应用迁移的 `Down` SQL。

试点环境回滚前必须先备份数据库：

```bash
mysqldump -h"$MYSQL_HOST" -P"$MYSQL_PORT" -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" \
  --single-transaction --routines --triggers "$MYSQL_DATABASE" > paas-before-rollback.sql
```

测试可用版本阶段不建议在生产数据上直接执行全量 Down；若需要回退单个问题迁移，应先编写专门的修复迁移。

## 验证

迁移后检查版本记录：

```sql
SELECT version, name, applied_at
FROM schema_migrations
ORDER BY version;
```

至少应包含以下迁移名称：

```text
identity_access_core
tenant_project_core
source_repository_core
application_environment_core
build_core
release_delivery_core
audit_logs
notification_core
cluster_agent_core
gitops_deployment_core
```
