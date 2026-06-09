# integrations 测试可用版本任务

## 目标

封装 GitLab、Jenkins、Argo CD、镜像仓库等外部系统调用，为业务模块提供稳定 adapter，实现外部系统与核心领域解耦。

## 任务清单

- [x] 建立 `internal/integrations/gitlab` 目录和 GitLab client 配置。
- [x] 实现 GitLab SourceRepository adapter：创建 Project、初始化仓库、保护分支、Webhook、成员同步。
- [x] 实现 GitLab ManifestRepository adapter：读取文件、提交文件、创建 Merge Request、查询 Merge Request。
- [x] 建立 `internal/integrations/jenkins` 目录和 Jenkins client 配置。
- [x] 实现 Jenkins adapter：创建 Job、触发 buildWithParameters、查询 queue item、读取 progressiveText、取消构建。
- [x] Jenkins adapter 支持 BuildSpec 参数透传。
- [x] 建立 `internal/integrations/argocd` 占位目录，当前测试可用版本不直接暴露 Argo CD UI，不在控制面直接操作集群。
- [x] 建立 `internal/integrations/registry` 目录和镜像仓库配置占位。
- [x] 所有 adapter 支持超时、重试、错误映射和日志脱敏。
- [x] 所有 adapter 凭据通过配置注入，不写入代码。
- [x] 为每个 adapter 提供 fake 实现，用于模块测试。
- [x] 编写 mock HTTP server 测试：GitLab、Jenkins、ManifestRepository。

## 完成标准

- [x] 业务模块只依赖 port，不直接依赖外部 SDK。
- [x] GitLab 和 Jenkins 关键调用可 fake 测试。
- [x] 外部凭据不泄露到日志或 API 响应。
- [x] 测试通过。
