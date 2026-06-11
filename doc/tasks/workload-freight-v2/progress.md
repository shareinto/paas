# Workload Freight V2 进度

## 总体状态

- [ ] 未开始
- [x] 开发中
- [ ] 后端完成
- [ ] 前端完成
- [ ] 联调完成
- [ ] 验收完成

## 任务进度

- [x] 数据库迁移
- [x] application-environment 模块 Workload 与环境配置
- [x] build 模块 Workload 适配
- [x] release-delivery 模块手动 Freight
- [ ] gitops-deployment 多 Workload values 写入
- [x] 权限和审计
- [ ] 应用详情导航调整
- [ ] 应用 Workload 页面
- [ ] 创建 Freight 抽屉
- [ ] 发布晋级页类 Kargo 交互
- [x] 后端测试
- [ ] 前端测试
- [ ] 端到端验收

## 合并记录

- 2026-06-10：已合并 `feature/workload-v2-backend`，提交 `106ef2a`，包含 Workload 与 WorkloadEnvironmentConfig 后端基础、迁移、仓储、API、审计和测试。
- 2026-06-11：已合并 `feature/workload-v2-release-freight`，提交 `827d38d`，包含 Build Workload 绑定、Workload Release 候选、手动 Freight、FreightItem 来源、Stage eligible-freights、Promotion 前置校验、权限和审计。

## 已运行测试

- `feature/workload-v2-backend` 合并前：`git diff --check` 通过。
- `feature/workload-v2-backend` 合并前：`go test ./internal/modules/appenv ./internal/migrations` 通过。
- `feature/workload-v2-backend` 合并前：`go test ./...` 通过。
- `feature/workload-v2-release-freight` 合并前：`git diff --check` 通过。
- `feature/workload-v2-release-freight` 合并前：`go test -count=1 ./internal/modules/build ./internal/modules/delivery ./internal/modules/audit ./internal/modules/identityaccess ./internal/migrations` 通过。
- `feature/workload-v2-release-freight` 合并前：`go test -count=1 ./...` 通过。

## 当前结论

- Application 调整为业务交付上下文。
- Workload 是最小可部署单元，一个镜像对应一个 Workload。
- Freight 由用户手动创建，且必须包含所有启用 Workload。
- Stage 发布按钮负责触发可发布 Freight 选择。
- dev/prod 等环境差异由 WorkloadEnvironmentConfig 或环境 values 承载，不进入 Freight。
