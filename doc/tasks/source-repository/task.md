# source-repository 测试可用版本任务

## 目标

实现 SourceRepository 登记管理、GitLab 已有仓库校验、权限同步和 BuildSpec 建议生成。

## 任务清单

- [x] 定义领域模型：`SourceRepository`、`RepositoryMigration`、`RepositoryPermissionSyncJob`。
- [x] 建立数据表迁移草案：`source_repositories`、`repository_migrations`、`repository_permission_sync_jobs`。
- [x] 定义 `GitSourceRepositoryPort`，覆盖按 HTTP 地址解析已有项目、扫描文件树、成员同步。
- [x] 实现登记 SourceRepository 用例，校验 Project 归属、权限、HTTP 地址和默认分支可读性。
- [x] 实现 SourceRepository 查询接口：按 Project 查询、按 ID 查询、查询关联 Application。
- [x] 实现 GitLab 权限映射：`tenant_owner -> Owner`、`project_admin -> Maintainer`、`developer -> Developer`、`viewer -> Reporter`。
- [x] RepositoryMigration API 保留兼容入口，但当前测试可用版本返回不再支持平台托管迁移。
- [x] 实现 Java 仓库扫描建议：检测 `pom.xml`、`build.gradle`、`target/*.jar`、`target/*.war`。
- [x] 生成 BuildSpec 建议：`source_path`、`build_command`、`artifact_copy_command`、`runtime_base_image`。
- [x] 确保扫描建议只作为建议，不直接写入 ApplicationSource。
- [x] 提供 API：登记仓库、查询仓库、迁移兼容错误入口。
- [x] 编写测试：fake GitLab 解析仓库、权限映射、Java 扫描建议。

## 完成标准

- [x] Application 创建前可以查询可用 SourceRepository。
- [x] 登记并扫描成功后 SourceRepository 可用于绑定 Application。
- [x] BuildSpec 建议生成但不自动决定最终配置。
- [x] 测试通过。
