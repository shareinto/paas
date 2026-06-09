# gitops-deployment 进度

- [x] gitops-deployment 模块完成
- [x] DeploymentTemplate 完成
- [x] DeploymentTemplateRevision 完成
- [x] ManifestRevision 完成
- [x] Deployment 完成
- [x] ManifestRepositoryPort 完成
- [x] 用户应用模板定制完成
- [x] 清单更新完成
- [x] Argo CD Application 清单生成完成
- [x] 回滚完成
- [x] 测试完成

## 说明

- 已实现内存仓储、部署模板版本、应用模板从平台基础模板复制、模板校验、values.yaml 更新、dev/test 直接 commit、staging/prod 创建 MR、ManifestRevision 记录、Deployment 查询、Agent 状态映射和回滚 digest 场景测试。
- 已补齐平台基础模板、应用模板创建/查询/更新 API；模板变更、ManifestRevision 和 Deployment 创建会记录审计事件。staging/prod MR 通过 PromotionID、ManifestRevision 和分支命名与 PaaS 审批流关联。
- 当前功能测试通过；质量门禁已通过：`go test ./... -coverprofile=coverage.out` 中 `internal/modules/gitops` 覆盖率为 90.5%，达到 `doc/prompt.md` 要求的每个后端模块 90%。
