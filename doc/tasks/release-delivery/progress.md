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
- Stage key 创建后保持稳定；删除 Stage 会物理删除模板项和该 Stage 的集群绑定。
- 集群绑定调整为租户 Stage 级绑定，支持一个 Stage 最多绑定一个集群、一个集群绑定多个 Stage。
- Promotion 支持 `target_stage_key`、Stage 唯一绑定集群和 `namespace_override`，并为目标集群生成部署记录。
- GitOps 部署清单路径按 Stage 维度保存：`apps/<app>/<stage>/values.yaml`。由于一个 Stage 最多绑定一个集群，不再生成 `apps/<app>/<stage>/<cluster>/values.yaml`。
- Web Console 新增“租户交付流模板”页；发布晋级页改为应用 Stage DAG，拖拽 Freight 到 Stage 后在卡片内确认发布，保留 Freight 审批弹窗和 Stage 人工验证弹窗。
- 新增 DeliveryFlowTemplateEdge，将租户交付流模板从线性顺序扩展为 DAG；默认仍生成 `dev -> test -> staging -> prod`，保存时校验无环，允许多个根 Stage，Fan-in 要求 Freight 通过全部直接上游 Stage。
- Web Console “租户交付流模板”页改为可拖拽 DAG 画布和右侧属性面板；Stage 节点和发布晋级 Stage 卡片顶部展示按画布列自动分配的色条，卡片位置保存为固定槽位，部署页随模板投影变化。
- 定向验证命令：
  - `go test -p 1 -count=1 ./internal/modules/delivery ./internal/modules/appenv ./internal/migrations ./cmd/paas-server`
  - `npm test -- src/pages/DeliveryFlowTemplatePage.test.tsx src/pages/PromotionPage.test.tsx src/pages/PromotionPage.api.test.tsx src/pages/ApplicationDetailPage.workload.test.tsx`
  - `npm run build`

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
