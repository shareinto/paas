# build 进度

- [x] build 模块完成
- [x] BuildPipeline 完成
- [x] BuildRun 完成
- [x] BuildArtifact 完成
- [x] Jenkins port 完成
- [x] BuildSpec 参数传递完成
- [x] SSE 日志完成
- [x] 回调处理完成
- [x] 测试完成

## 2026-05-30 完成记录

- 新增 `internal/modules/build` 模块，包含领域模型、端口、服务、内存仓库、HTTP API 和 MySQL 迁移草案。
- `BuildRunnerPort` 覆盖统一 Job 创建、触发构建、queue item 查询、progressiveText 读取和取消构建。
- BuildPipeline 查询或创建使用平台托管固定 Job 名称 `paas/{tenant_name}/{project_name}/{app_name}`，Jenkins 不暴露给用户。
- 触发构建时通过 `ApplicationQuery` 读取 `ApplicationSource` 内固化的 BuildSpec，并在触发 Jenkins 前按本次 BuildRun 渲染和更新 Jenkinsfile。
- 构建审计记录 `build_command`、`artifact_copy_command`、`runtime_base_image`、`artifact_deploy_path`。
- BuildRun 支持 queueId 记录、build number 回填、取消、日志 offset 更新和终态回调。
- SSE 日志接口从 progressiveText 读取增量日志，并脱敏平台 token、GitLab token、镜像仓库密码和配置注入的敏感值。
- 构建成功写入主镜像 `BuildArtifact` 并发布 `BuildSucceeded`；失败、取消、unstable 等终态发布 `BuildFailed`。
- 回调处理对已终态 BuildRun 幂等。
- 测试命令：
  - `go test ./internal/modules/build -cover`：覆盖率 93.2%。
  - `go test ./...`：通过。

## 2026-05-31 更新记录

- 新增平台级构建管理，支持构建环境、运行时环境和全局构建模板管理。
- `ApplicationSource` 记录用户创建应用时选择的构建环境，`Application` 记录运行时环境，首次构建创建 `BuildPipeline` 时按全局模板渲染 Pipeline Job XML。
- Jenkinsfile 中渲染当前 BuildRun 专属回调地址 `/api/builds/{buildRunId}/callback`。
- 日志接口在 Jenkins build number 未就绪时返回排队状态事件，前端可持续刷新等待日志。
- 2026-06-02 更新：应用创建阶段不再创建 Jenkins Job；点击构建时按固定名称创建或更新 Jenkins Job 后触发，不再计算 md5 指纹。
- 2026-06-02 更新：构建 Job 改为使用单个全局模板，多个代码源会渲染多个检出、构建和收集产物阶段；Dockerfile 从平台专用 Git 仓库检出，不放入用户源码仓库。
- 2026-06-04 更新：默认 Jenkinsfile 模板不再依赖 `paas-ci-helper`，改为直接使用 `docker buildx` 按 PaaS 渲染出的镜像目标构建镜像，并通过 `curl` 回调 PaaS 控制面。
- 2026-06-04 更新：触发 Jenkins 时不再传递构建参数；PaaS 每次构建前按本次源码 ref、commit、运行时和回调地址重新渲染并更新固定 Job。
