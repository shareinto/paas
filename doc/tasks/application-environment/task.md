# application-environment 测试可用版本任务

## 目标

实现 Application、ApplicationSource、Environment、EnvironmentClusterBinding 和 EnvironmentState，创建应用时绑定已有 SourceRepository 并固化 BuildSpec。

## 任务清单

- [x] 定义领域模型：`Application`、`ApplicationSource`、`Environment`、`EnvironmentConfig`、`EnvironmentSecret`、`EnvironmentRoute`、`EnvironmentClusterBinding`、`EnvironmentState`。
- [x] 建立数据表迁移草案：`applications`、`application_sources`、`environments`、`environment_configs`、`environment_secrets`、`environment_routes`、`environment_cluster_bindings`、`environment_states`。
- [x] 定义环境状态枚举：`draft`、`pending_cluster_binding`、`cluster_bound`、`deployable`、`deploying`、`running`、`degraded`、`disabled`。
- [x] 定义 BuildSpec 校验：构建命令、产物拷贝命令、运行时镜像和产物放置路径。
- [x] 校验 `build_command` 不能为空。
- [x] 校验 `artifact_copy_command` 必填，且构建模板会校验产物输出目录非空。
- [x] 校验 `runtime_base_image` 必须来自平台允许列表，并校验可选 `artifact_deploy_path` 不允许路径逃逸。
- [x] 实现创建 Application 用例：必须指定已有 SourceRepository。
- [x] 实现创建 ApplicationSource，并保存 `source_path` 和 BuildSpec。
- [x] 创建应用时默认创建 `dev`、`test`、`staging`、`prod` 环境。
- [x] 无可用集群时环境进入 `pending_cluster_binding`，不创建 EnvironmentClusterBinding 和 Argo CD Application。
- [x] 有可用集群时创建 EnvironmentClusterBinding，并调用 GitOpsEnvironmentProvisioner 创建清单路径和 Argo CD Application 清单。
- [x] 暴露 `ApplicationCommand`、`ApplicationQuery`、`EnvironmentCommand`、`EnvironmentQuery`、`EnvironmentBindingCommand`、`EnvironmentStateQuery`。
- [x] 提供 API：应用 CRUD、环境列表、环境详情、环境状态、环境事件。
- [x] 编写测试：创建应用必须指定 SourceRepository、monorepo source_path、多环境默认创建、BuildSpec 校验、无集群 pending。

## 完成标准

- [x] Application 是最小独立交付单元。
- [x] 创建 Application 不创建 SourceRepository。
- [x] BuildSpec 被固化到 ApplicationSource。
- [x] 无集群时构建可运行但部署被阻止。
- [x] 测试通过。
