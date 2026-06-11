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
- [x] gitops-deployment 多 Workload values 写入
- [x] 权限和审计
- [x] 应用详情导航调整
- [x] 应用 Workload 页面
- [ ] 创建 Freight 抽屉
- [ ] 发布晋级页类 Kargo 交互
- [x] 后端测试
- [ ] 前端测试
- [ ] 端到端验收

## 合并记录

- 2026-06-10：已合并 `feature/workload-v2-backend`，提交 `106ef2a`，包含 Workload 与 WorkloadEnvironmentConfig 后端基础、迁移、仓储、API、审计和测试。
- 2026-06-11：已合并 `feature/workload-v2-release-freight`，提交 `827d38d`，包含 Build Workload 绑定、Workload Release 候选、手动 Freight、FreightItem 来源、Stage eligible-freights、Promotion 前置校验、权限和审计。
- 2026-06-11：已合并 `feature/workload-v2-gitops`，提交 `47db0f3`，包含多 Workload values 渲染、WorkloadEnvironmentConfig 写入、Deployment workload_summary、回滚镜像写回和 GitOps 失败 Deployment 记录。
- 2026-06-11：已合并 `feature/workload-v2-web-workload`，提交 `e26c006`，包含应用详情默认 Workload 入口、Workload 列表、创建 Workload 抽屉、部署配置展示、真实 API 映射和前端测试。

## 已运行测试

- `feature/workload-v2-backend` 合并前：`git diff --check` 通过。
- `feature/workload-v2-backend` 合并前：`go test ./internal/modules/appenv ./internal/migrations` 通过。
- `feature/workload-v2-backend` 合并前：`go test ./...` 通过。
- `feature/workload-v2-release-freight` 合并前：`git diff --check` 通过。
- `feature/workload-v2-release-freight` 合并前：`go test -count=1 ./internal/modules/build ./internal/modules/delivery ./internal/modules/audit ./internal/modules/identityaccess ./internal/migrations` 通过。
- `feature/workload-v2-release-freight` 合并前：`go test -count=1 ./...` 通过。
- `feature/workload-v2-gitops` 合并前：`git diff --check` 通过。
- `feature/workload-v2-gitops` 合并前：`go test -count=1 ./internal/modules/gitops ./internal/migrations ./internal/modules/delivery` 通过。
- `feature/workload-v2-gitops` 合并前：`go test -count=1 ./...` 通过。
- `feature/workload-v2-web-workload` 合并前：`git diff --check` 通过。
- `feature/workload-v2-web-workload` 合并前：`cd web/console && npm test -- --coverage` 通过。
- `feature/workload-v2-web-workload` 合并前：`cd web/console && npm run build` 通过，仅 Vite chunk size 警告。
- `feature/workload-v2-web-workload` 合并后：`cd web/console && npm test -- src/api/index.workload.test.ts src/pages/ApplicationDetailPage.workload.test.tsx src/pages/ApplicationDetailPage.api.test.tsx` 通过。
- `feature/workload-v2-web-workload` 合并后：`cd web/console && npm run build` 通过，仅 Vite chunk size 警告。

## 当前结论

- Application 调整为业务交付上下文。
- Workload 是最小可部署单元，一个镜像对应一个 Workload。
- Freight 由用户手动创建，且必须包含所有启用 Workload。
- Stage 发布按钮负责触发可发布 Freight 选择。
- dev/prod 等环境差异由 WorkloadEnvironmentConfig 或环境 values 承载，不进入 Freight。
