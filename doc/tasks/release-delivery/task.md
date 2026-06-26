# release-delivery 测试可用版本任务

## 目标

实现 Release、Freight、DeliveryFlow、Promotion 和审批流程，使构建产物可以按租户交付流模板 DAG 晋级；默认模板兼容 dev/test/staging/prod。

> V2 变更：BuildSucceeded 生成 Workload Release 候选后，自动尝试生成覆盖所有启用 Workload 的 Freight；其他 Workload 使用最近一次成功构建产物补齐。自动生成失败时保留版本源变更提示；用户仍可手动创建 Freight。新开发任务见 `doc/tasks/workload-freight-v2/task.md`。

## 任务清单

- [x] 定义领域模型：`Release`、`Freight`、`FreightItem`、`DeliveryFlow`、`DeliveryStage`、`Promotion`、`PromotionApproval`。
- [x] 建立数据表迁移草案：`releases`、`freights`、`freight_items`、`delivery_flows`、`delivery_stages`、`promotions`、`promotion_approvals`。
- [x] 定义 Promotion 状态：`created`、`pending_approval`、`approved`、`rejected`、`manifest_updating`、`manifest_updated`、`syncing`、`healthy`、`failed`、`aborted`。
- [x] 消费 `BuildSucceeded` 事件，生成 Release。
- [x] V1 历史完成：基于 Release 自动生成单项 Freight。
- [x] 创建默认 DeliveryFlow：`dev -> test -> staging -> prod`，并支持租户交付流模板 DAG 依赖边。
- [x] 实现 Promotion 创建，校验目标 Stage 有 active StageClusterBinding。
- [x] Stage 为 `pending_cluster_binding` 时阻止 Promotion。
- [x] prod Promotion 进入审批流程。
- [x] 实现审批通过、审批拒绝、禁止自审批。
- [x] 审批通过后调用 `GitOpsDeploymentCommand` 修改部署清单。
- [x] 实现回滚 Promotion，目标为历史 Freight。
- [x] 提供 API：Freight 列表、Freight 详情、Promotion 创建、详情、审批、拒绝、中止。
- [x] 记录创建 Freight、创建 Promotion、审批、拒绝、回滚审计日志。
- [x] 编写测试：V1 BuildSucceeded 自动生成版本、晋级状态机、prod 审批、禁止自审批、pending Stage 阻止部署、回滚。

## 完成标准

- [x] 构建成功后可得到可发布版本。
- [x] Freight 可晋级到 dev/test/staging/prod。
- [x] prod 必须审批。
- [x] release-delivery 不直接写 GitLab 清单仓库。
- [x] 测试通过。
