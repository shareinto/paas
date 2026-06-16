# MySQL 迁移与启动流程

本文档定义开发阶段和测试可用版的 MySQL 迁移执行方式。所有业务表迁移以 Go 代码中的 `database.Migration` 为正式来源，由 `internal/migrations.All()` 统一聚合并按版本号升序执行。

## 开发阶段策略

当前项目仍处于开发阶段，数据库策略以“当前代码定义的新建库表”为准：

- 开发库允许清库重建，不承诺从任意旧开发库结构平滑升级。
- schema 大改后，优先重建开发数据库，再由当前 `internal/migrations.All()` 全量建表。
- 开发阶段可以重整尚未进入试点或生产的 migration，但必须同步清理依赖旧结构的开发库。
- 共享开发库清库前必须通知使用者，并按需备份应用、构建、Freight、Stage、审计等数据。

这意味着：开发阶段遇到缺字段、缺表或迁移漂移时，标准处理方式是清库重建，而不是为每个旧开发库状态追加补偿迁移。

进入试点、生产，或任何需要保留业务数据的环境后，应切换为不可变迁移策略：已执行过的 migration 不再修改，只能追加更高版本 migration 完成表结构变更。

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

注意：`schema_migrations` 只按版本判断是否已执行。若某个 migration 版本已经记录成功，后续修改同一个 migration 文件不会让数据库自动重放该版本。因此开发阶段修改旧 migration 后，必须清库重建；持久环境则必须新增更高版本 migration。

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

`PAAS_AUTO_MIGRATE=true` 不是 schema 修复器。它只执行 `schema_migrations` 中尚未记录的版本，不会修复“版本已记录但实际表结构仍是旧版”的开发库漂移。

## 开发库清库重建流程

仅在开发阶段或临时验收环境使用以下流程。执行前确认该库没有需要保留的业务数据。

### 使用启动脚本重建

推荐使用启动脚本完成清库、建库、全量 migration 和前后端启动：

```bash
./scripts/dev-up.sh --recreate-db
```

该命令会读取 `deploy/config/paas-dev.env`，通过 `mysql` 客户端删除并重建 `MYSQL_DATABASE`，强制设置 `PAAS_AUTO_MIGRATE=true`，启动 `paas-server`，等待 `/readyz`，再通过 `mysql` 客户端直连数据库注入 SBG/MACC 开发测试数据，最后启动 Web Console。

执行 `--recreate-db` 前脚本会检查后端是否已在运行；如果当前 `PAAS_HTTP_ADDR` 对应的 `/readyz` 已可用，脚本会拒绝重建并退出。此时应先停止旧的 `paas-server` 或上一轮 `dev-up` 进程，再重新执行重建命令，避免旧进程占用端口导致新后端无法启动。

如果本机未安装 `mysql` 命令，脚本会自动使用 Docker 镜像 `m.daocloud.io/docker.io/library/mysql:8.0` 作为一次性 MySQL 客户端；可通过 `PAAS_MYSQL_CLIENT_IMAGE` 覆盖该镜像。

重建但跳过开发测试数据：

```bash
./scripts/dev-up.sh --recreate-db --no-seed-dev-data
```

不重建数据库时，直接运行：

```bash
./scripts/dev-up.sh
```

不重建但需要注入或更新开发测试数据：

```bash
./scripts/dev-up.sh --seed-dev-data
```

如只需执行未应用 migration、不清库，可运行：

```bash
PAAS_AUTO_MIGRATE=true ./scripts/dev-up.sh
```

### 手工重建

1. 停止 `paas-server` 和 Web Console。
2. 如需保留现场，先备份数据库：

```bash
mysqldump -h"$MYSQL_HOST" -P"$MYSQL_PORT" -u"$MYSQL_USER" -p"$MYSQL_PASSWORD" \
  --single-transaction --routines --triggers "$MYSQL_DATABASE" > paas-dev-before-recreate.sql
```

3. 删除并重建开发数据库：

```sql
DROP DATABASE paas;
CREATE DATABASE paas CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

如数据库名不是 `paas`，应替换为当前 `MYSQL_DATABASE` 的值。

4. 使用当前代码全量建表并启动控制面：

```bash
export PAAS_AUTO_MIGRATE=true
export PAAS_HTTP_ADDR=:8080
go run ./cmd/paas-server
```

5. 检查迁移记录和服务健康状态：

```sql
SELECT version, name, applied_at
FROM schema_migrations
ORDER BY version;
```

```bash
curl -fsS http://127.0.0.1:8080/healthz
```

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
application_workload_core
build_core
release_delivery_core
audit_logs
notification_core
cluster_agent_core
gitops_deployment_core
```
