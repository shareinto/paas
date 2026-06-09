# tenant-project 进度

- [x] tenant-project 模块完成
- [x] Tenant 模型完成
- [x] Project 模型完成
- [x] 成员关系完成
- [x] 对外查询 port 完成
- [x] 审计事件完成
- [x] 测试完成

## 完成记录

- 实现目录：`internal/modules/tenantproject/`。
- 完成内容：租户、项目、租户成员领域模型；内存仓储；服务层用例；`TenantQuery`、`ProjectQuery`、`ProjectMembershipQuery` port；MySQL 迁移草案；权限检查、领域事件和审计记录。
- 权限假设：创建租户要求平台作用域 `tenant:update`；租户成员变更要求租户作用域 `tenant:update`；创建项目要求租户作用域 `project:update`；更新项目要求项目作用域 `project:update`。
- 验证命令：`go test ./internal/modules/tenantproject -cover`，覆盖率 `90.1%`。
- 全量验证：`go test ./...` 通过。
- 2026-05-31 更新：补充测试可用版项目管理能力，新增项目 HTTP 管理接口，支持创建项目、更新项目和删除项目；删除项目前会由组合层校验是否存在关联 Application 或 SourceRepository，存在关联资源时拒绝删除。验证命令：`go test ./internal/modules/tenantproject ./cmd/paas-server` 通过。
