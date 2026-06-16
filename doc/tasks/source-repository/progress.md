# source-repository 进度

- [x] source-repository 模块完成
- [x] SourceRepository 管理完成
- [x] GitLab port 完成
- [x] 权限同步完成
- [x] 仓库登记完成
- [x] BuildSpec 建议完成
- [x] API 完成
- [x] 测试完成

## 完成记录

- 实现目录：`internal/modules/sourcerepository/`。
- 完成内容：`SourceRepository`、`RepositoryMigration`、`RepositoryPermissionSyncJob` 领域模型；MySQL 正式表仓储；MySQL 迁移；`GitSourceRepositoryPort`；服务层用例；HTTP API；Java 仓库扫描与 BuildSpec 建议生成。
- GitLab 权限映射：`tenant_owner -> Owner`、`project_admin -> Maintainer`、`developer -> Developer`、`viewer -> Reporter`。未列入映射的角色不会同步到 GitLab 成员权限。
- 当前版本 SourceRepository 通过用户填写的 HTTP 地址登记已有 GitLab 仓库；平台解析项目并扫描默认分支，仓库不存在、无权限或默认分支不可读时拒绝登记。RepositoryMigration API 保留兼容入口，但返回不再支持平台托管迁移。
- BuildSpec 建议只通过扫描结果返回，不写入 `ApplicationSource`；最终固化留给 `application-workload` 模块。
- 权限假设：登记仓库和权限同步均要求项目作用域 `project:update`。
- 验证命令：`go test ./internal/modules/sourcerepository -cover`，覆盖率 `91.0%`。
- 全量验证：`go test ./...` 通过。
- 2026-05-31 更新：`paas-server` 已支持通过 `GITLAB_BASE_URL`、`GITLAB_TOKEN` 接入真实 GitLab；SourceRepository 登记时会解析用户填写的 HTTP 地址并扫描默认分支，不再创建 GitLab Project。
- 2026-05-31 更新：补充测试可用版源码仓库管理能力，支持删除 SourceRepository 平台记录；GitLab 401 不再映射为平台会话过期，源码仓库 API 错误会输出到标准输出便于容器日志采集。
- 2026-06-09 更新：`internal/modules/sourcerepository/repository_mysql.go` 已改为逐表 SQL repository，业务状态写入正式表，不再依赖内存仓储或快照表。
