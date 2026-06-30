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
- [x] Workload 模板固定渲染平台管理的 initContainer（名称 `init`），用于目录创建、目录权限等初始化工作；该能力不作为用户配置项暴露，渲染时固定 `securityContext.runAsUser: 0`。`/logs` 固定执行 `chown 10001:0` 与 `chmod 775`，其它可写目录按平台配置的 `owner_group/mode` 生成权限修复命令。
- [x] Workload 模板默认渲染 `app-logs` hostPath 日志卷（`/cloud`），并将所有业务容器和 initContainer 的 `/logs` 挂载到 `macc/$(APP_NAME)/$(POD_NAME)` 子路径。
- [x] Workload 配置支持 Pod 网络模式，默认 `容器网络`；选择 `Host 网络` 时渲染 `hostNetwork: true`。
- [x] Workload 模板固定渲染软 Pod 反亲和（`preferredDuringSchedulingIgnoredDuringExecution`，权重 `100`，`topologyKey=kubernetes.io/hostname`），通过当前 Workload 的 `app.kubernetes.io/name` 与 `app.kubernetes.io/instance` 匹配同组 Pod；业务 Workload 资源名保留原名，不追加 Stage 后缀。
- [x] 实现模板语法校验和策略校验，校验通过后才能用于发布。
- [x] 实现环境清单目录生成规则：`apps/{app}/{env}/values.yaml`。
- [x] 实现 Argo CD Application 清单生成，按 StageClusterBinding 创建。
- [x] 实现发布时修改 values 中的镜像 repository、tag、digest。
- [x] 所有 Stage 在 PaaS 审批/验证门禁满足后，由 PaaS 直接 commit 到清单仓库。
- [x] staging/prod 等高风险环境的准入由 PaaS Promotion 审批流控制，不再依赖 GitLab Merge Request 作为发布门禁。
- [x] 每次模板变更记录 DeploymentTemplateRevision 和审计日志。
- [x] 每次清单变更记录 ManifestRevision，并关联使用的 DeploymentTemplateRevision。
- [x] 创建 Deployment 并绑定 Promotion、Stage、ClusterBinding。
- [x] 提供 Stage 发布历史记录：按 `Application + Stage + Deployment/ManifestRevision` 查询已提交 GitOps 的历史发布，展示发布包版本、发布人、Git commit、Manifest 路径和渲染 YAML。
- [x] Stage 发布历史 YAML 差异默认对比该 Stage 上一条已成功提交 GitOps 的发布；首次发布展示完整 YAML。
- [x] Stage 发布历史不展示当前 Argo CD 同步状态或健康状态，避免旧历史记录被当前运行态覆盖后产生误导；运行态状态仍在 Stage 当前详情中查看。
- [x] 接收 Agent 状态后更新 Deployment 状态。
- [x] 实现回滚清单修改，目标镜像 digest 来自历史 Freight。
- [x] 提供 Deployment 查询 API。
- [x] 编写测试：应用级模板副本不影响平台基础模板和其他应用模板。
- [x] 编写测试：initContainer `mkdir/chown/chmod` 场景和 `emptyDir.sizeLimit` 可以通过模板校验并渲染到清单。
- [x] 编写测试：模板语法错误或策略违规时拒绝发布。
- [x] 编写测试：values.yaml 更新、所有 Stage commit、ManifestRevision 记录、回滚 digest、Agent 状态映射。

## 完成标准

- [x] PaaS 通过修改 GitLab 清单仓库触发部署。
- [x] 用户可以通过 PaaS 定制自己应用的部署模板。
- [x] 用户模板变更不会影响平台基础模板或其他用户模板。
- [x] 不直接调用 Kubernetes API Server。
- [x] 每次清单变更可审计、可追踪 commit。
- [x] Stage 历史发布记录可追踪每次 GitOps commit，并支持查看本次发布相对上一条发布的 YAML 差异。
- [x] Agent 上报能驱动 Deployment 终态。
- [x] 测试通过。
