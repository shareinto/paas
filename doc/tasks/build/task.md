# build 测试可用版本任务

## 目标

实现 Jenkins 平台托管构建能力，按 BuildSpec 触发统一 Job，采集日志，记录 BuildArtifact，并发布构建结果事件。

## 任务清单

- [x] 定义领域模型：`BuildPipeline`、`BuildRun`、`BuildArtifact`、`BuildSpec`。
- [x] 建立数据表迁移草案：`build_pipelines`、`build_runs`、`build_artifacts`。
- [x] 定义 BuildRun 状态：`queued`、`running`、`succeeded`、`failed`、`aborted`、`unstable`、`unknown`。
- [x] 定义 `BuildRunnerPort`，覆盖创建 Job、触发构建、查询 queue item、读取 progressiveText、取消构建。
- [x] 实现 BuildPipeline 查询或创建逻辑，点击构建时使用固定 Job 名称 `paas/{tenant_name}/{project_name}/{app_name}`，不存在则创建，存在则更新。
- [x] 触发构建时读取 ApplicationSource 的 BuildSpec。
- [x] 触发构建前按本次 BuildRun 渲染 Jenkinsfile 并更新固定 Job，触发 Jenkins 时不传递构建参数。
- [x] 实现构建命令审计：记录 build_command、artifact_copy_command、runtime_base_image、artifact_deploy_path。
- [x] 实现 BuildRun 创建、queueId 记录、build number 回填。
- [x] 实现 SSE 日志流接口 `GET /api/builds/{buildRunId}/logs/stream`。
- [x] 实现日志脱敏，过滤平台 token、GitLab token、镜像仓库密码等敏感值。
- [x] 处理 Jenkins 回调，更新 BuildRun 终态。
- [x] 构建成功后写入主镜像 BuildArtifact。
- [x] 构建成功发布 `BuildSucceeded` 事件，构建失败发布 `BuildFailed` 事件。
- [x] 提供 API：触发构建、构建列表、构建详情、日志流、取消构建。
- [x] 编写测试：fake Jenkins Job 创建、触发构建、日志 offset、回调幂等、jar/war 后缀失败、日志脱敏。

## 完成标准

- [x] Jenkins 对用户隐藏。
- [x] 构建只使用平台全局 Job 模板，按多个代码源渲染检出、构建和收集产物阶段。
- [x] Spring Boot jar 和 Tomcat war 参数路径均可测试。
- [x] 构建成功能生成主镜像 BuildArtifact。
- [x] 测试通过。
