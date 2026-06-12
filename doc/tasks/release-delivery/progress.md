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

## 2026-06-12 Stage 化交付流更新

- 新增租户级 DeliveryFlowTemplate、DeliveryFlowTemplateStage、StageClusterBinding、AppStage、FreightApproval 和 StageVerification。
- Stage key 创建后保持稳定；删除语义为禁用。
- 集群绑定调整为租户 Stage 级集群池，支持一个 Stage 多集群、一个集群绑定多个 Stage。
- Promotion 支持 `target_stage_key`、`target_cluster_ids` 和 `namespace_override`，并为目标集群子集生成独立部署记录。
- GitOps 部署路径支持 Stage + Cluster 维度：`apps/<app>/<stage>/<cluster>/values.yaml` 和 `argocd/apps/<app>-<stage>-<cluster>.yaml`。
- Web Console 新增“租户交付流模板”页；发布晋级页改为发布确认弹窗、Freight 审批弹窗和 Stage 人工验证弹窗。
- 定向验证命令：
  - `go test -p 1 -count=1 ./internal/modules/delivery ./internal/modules/gitops`
  - `npm test -- --run src/pages/DeliveryFlowTemplatePage.test.tsx src/pages/PromotionPage.test.tsx src/pages/PromotionPage.api.test.tsx src/app/App.test.tsx`

## 2026-05-30 完成记录

- 新增 `internal/modules/delivery` 模块，包含领域模型、端口、服务、MySQL 正式表仓库、HTTP API 和 MySQL 迁移。
- 消费 `BuildSucceeded` 载荷生成 `Release`、`Freight` 和 `FreightItem`，并创建默认 `dev -> test -> staging -> prod` DeliveryFlow。
- Promotion 创建会校验目标环境存在 active `EnvironmentClusterBinding`；`pending_cluster_binding` 环境会阻止发布。
- 非 prod Promotion 直接调用 `GitOpsDeploymentCommand` 写入发布意图；release-delivery 不直接写 GitLab 清单仓库。
- prod Promotion 进入审批流程，支持审批通过、拒绝和禁止自审批。
- 支持回滚 Promotion，目标为历史 Freight。
- 记录创建 Freight、创建 Promotion、审批、拒绝、回滚和中止审计日志。
- 测试命令：
  - `go test ./internal/modules/delivery -cover`：覆盖率 91.0%。
  - `go test ./...`：通过。
