# application-environment 进度

- [x] application-environment 模块完成
- [x] Application 模型完成
- [x] ApplicationSource 和 BuildSpec 完成
- [x] 默认环境创建完成
- [x] EnvironmentClusterBinding 完成
- [x] EnvironmentState 完成
- [x] API 完成
- [x] 测试完成

## 2026-05-30 完成记录

- 新增 `internal/modules/appenv` 模块，包含领域模型、端口、服务、MySQL 正式表仓库、HTTP API 和 MySQL 迁移。
- 当时实现要求创建 Application 时绑定已有 `SourceRepository`，不会创建源码仓库；2026-06-11 起交互调整为创建 BuildPipeline 时绑定 SourceRepository。
- `ApplicationSource` 固化 `source_path` 和 `BuildSpec`，当前测试可用版本支持 `java_springboot` 与 `java_tomcat`。
- BuildSpec 校验覆盖构建命令、产物拷贝命令、运行时基础镜像允许列表、产物放置路径和默认分支。
- 创建应用时默认创建 `dev`、`test`、`staging`、`prod` 环境。
- 无可用集群时环境进入 `pending_cluster_binding`，不创建 `EnvironmentClusterBinding`，不调用 GitOps。
- 有可用集群时通过 `ClusterPlacementQuery` 选择集群，通过 `GitOpsEnvironmentProvisioner` 创建清单路径和 Argo CD Application 清单，控制面不直接调用 Kubernetes API。
- 测试命令：
  - `go test ./internal/modules/appenv -cover`：覆盖率 92.5%。
  - `go test ./...`：通过。

## 2026-05-31 更新记录

- 创建 Application 时继续要求选择已有平台托管 `SourceRepository`，前端创建向导已按所选项目过滤可用源码仓库。
- 2026-06-02 更新：创建 Application 只保存应用、代码源和默认环境，不再立即创建 Jenkins 流水线；Jenkins 流水线改为点击构建时按固定名称创建或更新。
- 应用支持编辑显示名、描述、启停、代码源、BuildSpec、构建环境和运行时环境；应用名和所属项目保持不可编辑。

## 2026-06-11 交互调整

- 控制台主路径调整为 `租户 -> 项目 -> 应用`，项目详情页通过 `应用` 和 `源码仓库` 页签展示项目内资源。
- SourceRepository 是 Project 下的独立资源，创建 Application 时不再选择源码仓库，也不固化 BuildSpec。
- 用户在应用详情中创建 BuildPipeline 时选择 Workload、SourceRepository、source_path、构建环境、运行时环境和 BuildSpec。
- application-environment 模块继续维护 Application、Environment、Workload 和历史兼容的 ApplicationSource 查询；新的源码绑定入口归属 build 模块。
