# application-environment 测试可用版本任务

## 目标

实现 Application、ApplicationSource、Environment、EnvironmentClusterBinding 和 EnvironmentState。当前交互中创建应用只维护应用基础信息和默认环境，源码仓库与 BuildSpec 在创建 BuildPipeline 时配置。

## 任务清单

- [x] 定义领域模型：`Application`、`ApplicationSource`、`Environment`、`EnvironmentConfig`、`EnvironmentSecret`、`EnvironmentRoute`、`EnvironmentClusterBinding`、`EnvironmentState`。
- [x] 建立数据表迁移草案：`applications`、`application_sources`、`environments`、`environment_configs`、`environment_secrets`、`environment_routes`、`environment_cluster_bindings`、`environment_states`。
- [x] 定义环境状态枚举：`draft`、`pending_cluster_binding`、`cluster_bound`、`deployable`、`deploying`、`running`、`degraded`、`disabled`。
- [x] 定义 BuildSpec 校验：构建命令、产物拷贝命令、运行时镜像和产物放置路径。
- [x] 校验 `build_command` 不能为空。
- [x] 校验 `artifact_copy_command` 必填，且构建模板会校验产物输出目录非空。
- [x] 校验 `runtime_base_image` 必须来自平台允许列表，并校验可选 `artifact_deploy_path` 不允许路径逃逸。
- [x] 实现创建 Application 用例：创建应用基础信息和默认环境。
- [x] 保留 ApplicationSource 兼容模型；新的 `source_path` 和 BuildSpec 配置入口由 BuildPipeline 承载。
- [x] 创建应用时默认创建 `dev`、`test`、`staging`、`prod` 环境。
- [x] 无可用集群时环境进入 `pending_cluster_binding`，不创建 EnvironmentClusterBinding 和 Argo CD Application。
- [x] 有可用集群时创建 EnvironmentClusterBinding，并调用 GitOpsEnvironmentProvisioner 创建清单路径和 Argo CD Application 清单。
- [x] 暴露 `ApplicationCommand`、`ApplicationQuery`、`EnvironmentCommand`、`EnvironmentQuery`、`EnvironmentBindingCommand`、`EnvironmentStateQuery`。
- [x] 提供 API：应用 CRUD、环境列表、环境详情、环境状态、环境事件。
- [x] 编写测试：多环境默认创建、无集群 pending、ApplicationSource 兼容校验；新的 BuildPipeline 测试覆盖 SourceRepository、monorepo source_path 和 BuildSpec 校验。

## 完成标准

> V2 变更：Application 已调整为业务交付上下文，Workload 是最小可部署单元。新开发任务见 `doc/tasks/workload-freight-v2/task.md`。

- [x] V1 历史完成：单镜像 Application 交付单元规则。
- [x] 创建 Application 不创建 SourceRepository。
- [x] BuildSpec 由 BuildPipeline 代码源配置固化，ApplicationSource 仅作为兼容模型保留。
- [x] 无集群时构建可运行但部署被阻止。
- [x] 测试通过。
