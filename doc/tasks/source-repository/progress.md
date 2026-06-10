# source-repository 进度

- [x] source-repository 模块完成
- [x] SourceRepository 管理完成
- [x] GitLab port 完成
- [x] 权限同步完成
- [x] 仓库迁移完成
- [x] BuildSpec 建议完成
- [x] API 完成
- [x] 测试完成

## 完成记录

- 实现目录：`internal/modules/sourcerepository/`。
- 完成内容：`SourceRepository`、`RepositoryMigration`、`RepositoryPermissionSyncJob` 领域模型；MySQL 正式表仓储；MySQL 迁移；`GitSourceRepositoryPort`；服务层用例；HTTP API；Java 仓库扫描与 BuildSpec 建议生成。
- GitLab 权限映射：`tenant_owner -> Owner`、`project_admin -> Maintainer`、`developer -> Developer`、`viewer -> Reporter`。未列入映射的角色不会同步到 GitLab 成员权限。
- 迁移流程实现为 Worker 执行点 `ProcessRepositoryMigration`，覆盖 `pending -> creating_target_repo -> cloning_source -> pushing_target -> verifying -> analyzing -> ready_for_application_binding -> succeeded`，失败进入 `failed` 并支持重试，未完成任务支持取消。
- BuildSpec 建议只通过扫描结果返回，不写入 `ApplicationSource`；最终固化留给 `application-environment` 模块。
- 权限假设：创建仓库、迁移仓库、权限同步均要求项目作用域 `project:update`。
- 验证命令：`go test ./internal/modules/sourcerepository -cover`，覆盖率 `91.0%`。
- 全量验证：`go test ./...` 通过。
- 2026-05-31 更新：`paas-server` 已支持通过 `GITLAB_BASE_URL`、`GITLAB_TOKEN`、`GITLAB_ROOT_GROUP_PATH` 接入真实 GitLab；SourceRepository 创建会自动确保 GitLab 根 Group、Tenant Group、Project Subgroup，并在 Subgroup 下创建 GitLab Project。
- 2026-05-31 更新：补充测试可用版源码仓库管理能力，支持删除 SourceRepository 并同步删除 GitLab Project；创建失败会保留失败状态并记录失败审计，GitLab 401 不再映射为平台会话过期，源码仓库 API 错误会输出到标准输出便于容器日志采集。
- 2026-06-09 更新：`internal/modules/sourcerepository/repository_mysql.go` 已改为逐表 SQL repository，业务状态写入正式表，不再依赖内存仓储或快照表。
