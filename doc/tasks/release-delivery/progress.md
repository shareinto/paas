# release-delivery 进度

- [x] release-delivery 模块完成
- [x] Release 完成
- [x] Freight 完成
- [x] DeliveryFlow 完成
- [x] Promotion 完成
- [x] prod 审批完成
- [x] 回滚 Promotion 完成
- [x] API 完成
- [x] 测试完成

## 2026-05-30 完成记录

- 新增 `internal/modules/delivery` 模块，包含领域模型、端口、服务、内存仓库、HTTP API 和 MySQL 迁移草案。
- 消费 `BuildSucceeded` 载荷生成 `Release`、`Freight` 和 `FreightItem`，并创建默认 `dev -> test -> staging -> prod` DeliveryFlow。
- Promotion 创建会校验目标环境存在 active `EnvironmentClusterBinding`；`pending_cluster_binding` 环境会阻止发布。
- 非 prod Promotion 直接调用 `GitOpsDeploymentCommand` 写入发布意图；release-delivery 不直接写 GitLab 清单仓库。
- prod Promotion 进入审批流程，支持审批通过、拒绝和禁止自审批。
- 支持回滚 Promotion，目标为历史 Freight。
- 记录创建 Freight、创建 Promotion、审批、拒绝、回滚和中止审计日志。
- 测试命令：
  - `go test ./internal/modules/delivery -cover`：覆盖率 91.0%。
  - `go test ./...`：通过。
