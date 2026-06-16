# AGENTS.md
开始编码之前，必须新建worktree,在.worktrees下创建新的目录，禁止在当前文件夹修改代码
开始编码之前，你如果判断可以开始启多个子agent并行开发，就直接开启子agent

本文件为在本仓库中工作的 coding agent 提供项目约定和工作指引。

## 项目文档

`doc/` 目录是本项目的需求和设计文档来源。开始实现、修改或评审代码前，应优先阅读并遵循以下文档：

- `doc/需求.md`：项目需求说明。
- `doc/总体设计.md`：平台总体设计和架构约束。
- `doc/模块划分.md`：模块边界、依赖关系和独立测试策略。
- `doc/Java构建适配方案.md`：测试可用版本构建适配、BuildSpec 和 Jenkins 统一模板规则。
- `doc/UI风格.md`：前端控制台视觉风格、布局和组件规范，参考根目录 `xx.png`。
- `doc/中间件部署.md`：开发环境 MySQL、Redis 中间件部署和连接信息。
- `doc/tasks/`：按模块拆分的测试可用版本任务和完成进度。

如果代码实现与 `doc/` 中的文档存在冲突，应先以文档为准，并在回复中明确指出冲突点和建议处理方式。

## 项目目标

本项目计划实现一个企业内部 PaaS 平台。平台应作为统一入口，隐藏 GitLab、Jenkins、Argo CD 和 Kubernetes 等底层系统细节，让用户在 PaaS 控制台中完成应用创建、构建、发布、环境管理、状态查看和回滚等操作。

核心约束：

- PaaS 控制面是用户唯一入口。
- 普通用户不直接访问 Jenkins、Argo CD、部署清单仓库或 kubeconfig。
- PaaS 控制面不直接调用 Kubernetes API Server。
- 每个 Kubernetes 集群内应部署独立 Argo CD 和 PaaS Agent。
- PaaS Agent 负责集群侧状态采集、事件上报和受控任务执行。
- 应用应按最小可独立交付单元建模，而不是默认表示完整业务系统。
- Web Console 所有用户可见文案必须使用中文。
- 用户认证必须同时支持 OIDC 登录和平台自主创建的本地用户账号密码登录。

## 工作方式

- 修改代码前先理解 `doc/` 中的需求、领域模型和架构边界。
- 优先保持实现与文档中的命名一致，例如 `Application`、`Environment`、`SourceRepository`、`EnvironmentClusterBinding`、`Release`、`Freight`、`Promotion`。
- 不要引入与文档相冲突的概念或术语，例如用 `Placement` 替代 `EnvironmentClusterBinding`。
- 对不明确的实现细节，应选择与文档中测试可用版本范围一致的保守方案。
- 若新增接口、数据表、领域对象或流程，应检查是否需要同步更新 `doc/` 文档。

## 代码质量要求

- 保持改动范围聚焦，避免无关重构。
- 遵循现有代码结构和命名风格。
- 涉及权限、发布、部署、回滚和审计的逻辑应补充测试。
- 涉及用户可见流程时，应同时考虑失败状态、重试和审计记录。
- 不要在日志或返回值中暴露凭据、token、secret 或 kubeconfig。

## 测试执行约束

- 不要直接裸跑 `go test ./...`。该命令会让多个 package 并发执行，可能为 MySQL 集成测试同时拉起多个 testcontainers MySQL 容器。
- 需要执行后端全量测试时，优先使用 `scripts/test-full.sh`。该脚本会启动或复用 1 个测试 MySQL，并默认用 `go test -p 1 -count=1 ./...` 串行运行 Go package。
- 如果只执行 Go 全量测试且不运行前端测试，应先设置共享 `PAAS_TEST_MYSQL_DSN`，再执行 `go test -p 1 -count=1 ./...`。
- 如需调整 Go package 并发度，只能通过 `PAAS_GO_TEST_P` 显式设置，并确认共享 MySQL 下不同 package 的测试库不会互相影响。
