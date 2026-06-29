# application-workload 测试可用版本任务

## 目标

实现 Application、ApplicationSource、Workload、WorkloadStageConfig 和 ApplicationStageState。创建应用只维护应用基础信息；应用详情页 Stage 来自租户 DeliveryFlow 模板实时投影，源码仓库与 BuildSpec 在创建 BuildPipeline 时配置。

## 任务清单

- [x] 定义领域模型：`Application`、`ApplicationSource`、`Workload`、`WorkloadStageConfig`、`WorkloadDefaultConfig`、`ApplicationStageState`、`ApplicationStageEvent`。
- [x] 建立数据表迁移草案：`applications`、`application_sources`、`workloads`、`workload_stage_configs`、`workload_default_configs`、`application_stage_states`、`application_stage_events`。
- [x] 定义 Stage 状态枚举：`draft`、`pending_cluster_binding`、`cluster_bound`、`deployable`、`deploying`、`running`、`degraded`、`disabled`。
- [x] 定义 BuildSpec 校验：构建命令、产物拷贝命令、运行时镜像和产物放置路径。
- [x] 校验 `build_command` 不能为空。
- [x] 校验 `artifact_copy_command` 必填，且构建模板会校验产物输出目录非空。
- [x] 校验 `runtime_base_image` 必须来自平台允许列表，并校验可选 `artifact_deploy_path` 不允许路径逃逸。
- [x] 实现创建 Application 用例：创建应用基础信息，不创建应用级运行环境模型，并为创建人写入应用级 `application_admin` RoleBinding。
- [x] 保留 ApplicationSource 兼容模型；新的 `source_path` 和 BuildSpec 配置入口由 BuildPipeline 承载。
- [x] 应用详情页默认投影租户 DeliveryFlow 模板中的 `dev`、`test`、`staging`、`prod` Stage。
- [x] 无可用集群绑定时 Stage 进入 `pending_cluster_binding`，不触发 GitOps 部署。
- [x] StageClusterBinding 归属 release-delivery 的租户级 DeliveryFlow 模板，不在应用下创建。
- [x] 暴露 `ApplicationCommand`、`ApplicationQuery`、`WorkloadCommand`、`WorkloadQuery`、`ApplicationStageStateQuery`。
- [x] 提供 API：应用 CRUD、`我加入的应用` / `项目内应用` 查询、Workload CRUD、Workload 默认配置、Workload Stage 覆盖配置和应用 Stage 状态。
- [x] 编写测试：创建应用不生成应用级运行环境模型、创建人自动获得应用级角色绑定、`我加入的应用` 只认 application scope RoleBinding、ApplicationSource 兼容校验；新的 BuildPipeline 测试覆盖直接源码地址、monorepo source_path 和 BuildSpec 校验。

## 完成标准

> V2 变更：Application 已调整为业务交付上下文，Workload 是最小可部署单元。新开发任务见 `doc/tasks/workload-freight-v2/task.md`。

- [x] V1 历史完成：单镜像 Application 交付单元规则。
- [x] 创建 Application 不创建 SourceRepository。
- [x] 创建 Application 后创建人自动成为应用管理员。
- [x] BuildSpec 由 BuildPipeline 代码源配置固化，ApplicationSource 仅作为兼容模型保留。
- [x] 无集群时构建可运行但部署被阻止。
- [x] 测试通过。
