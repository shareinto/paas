# source-repository 测试可用版本任务

## 目标

实现平台托管 SourceRepository 管理、GitLab Project 创建、仓库迁移和 BuildSpec 建议生成。

## 任务清单

- [x] 定义领域模型：`SourceRepository`、`RepositoryMigration`、`RepositoryPermissionSyncJob`。
- [x] 建立数据表迁移草案：`source_repositories`、`repository_migrations`、`repository_permission_sync_jobs`。
- [x] 定义 `GitSourceRepositoryPort`，覆盖创建项目、初始化仓库、保护分支、配置 Webhook、成员同步。
- [x] 实现创建 SourceRepository 用例，校验 Project 归属和权限。
- [x] 实现 SourceRepository 查询接口：按 Project 查询、按 ID 查询、查询关联 Application。
- [x] 实现 GitLab 权限映射：`tenant_owner -> Owner`、`project_admin -> Maintainer`、`developer -> Developer`、`viewer -> Reporter`。
- [x] 实现 RepositoryMigration 状态机：`pending`、`creating_target_repo`、`cloning_source`、`pushing_target`、`verifying`、`analyzing`、`ready_for_application_binding`、`succeeded`、`failed`。
- [x] 实现 mirror 迁移流程任务接口，保留 Worker 执行点。
- [x] 实现 Java 仓库扫描建议：检测 `pom.xml`、`build.gradle`、`target/*.jar`、`target/*.war`。
- [x] 生成 BuildSpec 建议：`source_path`、`build_command`、`artifact_copy_command`、`runtime_base_image`。
- [x] 确保扫描建议只作为建议，不直接写入 ApplicationSource。
- [x] 提供 API：创建仓库、查询仓库、创建迁移、查询迁移、重试迁移、取消迁移。
- [x] 编写测试：fake GitLab 创建仓库、权限映射、迁移状态机、Java 扫描建议。

## 完成标准

- [x] Application 创建前可以查询可用 SourceRepository。
- [x] 迁移完成后 SourceRepository 可用于绑定 Application。
- [x] BuildSpec 建议生成但不自动决定最终配置。
- [x] 测试通过。
