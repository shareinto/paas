# gitops-deployment 测试可用版本任务

## 目标

实现部署模板定制、部署清单变更、ManifestRevision、Deployment 和 DeploymentEvent，使用 GitLab 清单仓库驱动 Argo CD GitOps 部署。

## 任务清单

- [x] 定义领域模型：`DeploymentTemplate`、`DeploymentTemplateRevision`、`ManifestRevision`、`Deployment`、`DeploymentEvent`。
- [x] 建立数据表迁移草案：`deployment_templates`、`deployment_template_revisions`、`manifest_revisions`、`deployments`、`deployment_events`。
- [x] 定义 Deployment 状态：`pending`、`syncing`、`progressing`、`succeeded`、`failed`、`degraded`、`unknown`。
- [x] 定义 `ManifestRepositoryPort`，覆盖读取、提交、创建 MR、查询 MR。
- [x] 实现平台基础部署模板只读管理，作为创建用户模板的初始化来源。
- [x] 实现 application 级 DeploymentTemplate 创建，从平台基础模板生成独立副本或 overlay。
- [x] 实现 DeploymentTemplateRevision，每次用户修改模板生成新版本。
- [x] 支持用户通过 PaaS 修改模板内容，不直接暴露 GitLab 部署清单仓库。
- [x] 支持 initContainer、目录创建、目录权限、volumeMount、安全上下文等模板定制场景。
- [x] 实现模板语法校验和策略校验，校验通过后才能用于发布。
- [x] 实现环境清单目录生成规则：`apps/{app}/{env}/values.yaml`。
- [x] 实现 Argo CD Application 清单生成，按 StageClusterBinding 创建。
- [x] 实现发布时修改 values 中的镜像 repository、tag、digest。
- [x] dev/test 默认直接 commit 到清单仓库。
- [x] staging/prod 默认创建 Merge Request，并与 PaaS 审批流关联。
- [x] 每次模板变更记录 DeploymentTemplateRevision 和审计日志。
- [x] 每次清单变更记录 ManifestRevision，并关联使用的 DeploymentTemplateRevision。
- [x] 创建 Deployment 并绑定 Promotion、Stage、ClusterBinding。
- [x] 接收 Agent 状态后更新 Deployment 状态。
- [x] 实现回滚清单修改，目标镜像 digest 来自历史 Freight。
- [x] 提供 Deployment 查询 API。
- [x] 编写测试：应用级模板副本不影响平台基础模板和其他应用模板。
- [x] 编写测试：initContainer mkdir/chmod 场景可以通过模板校验并渲染到清单。
- [x] 编写测试：模板语法错误或策略违规时拒绝发布。
- [x] 编写测试：values.yaml 更新、dev/test commit、staging/prod MR、ManifestRevision 记录、回滚 digest、Agent 状态映射。

## 完成标准

- [x] PaaS 通过修改 GitLab 清单仓库触发部署。
- [x] 用户可以通过 PaaS 定制自己应用的部署模板。
- [x] 用户模板变更不会影响平台基础模板或其他用户模板。
- [x] 不直接调用 Kubernetes API Server。
- [x] 每次清单变更可审计、可追踪 commit 或 MR。
- [x] Agent 上报能驱动 Deployment 终态。
- [x] 测试通过。
