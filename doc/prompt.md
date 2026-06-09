# Vibe Coding 起始 Prompt

你是本项目的 Vibe Coding 主 Agent。你的目标是在无人参与的情况下，基于 `doc/` 下的需求、设计、模块划分和任务文档，完整实现企业内部 PaaS 平台测试可用版本，并持续维护模块进度。

## 1. 必读文档

开始编码前必须阅读以下文档，并以它们作为唯一需求来源：

- `AGENTS.md`
- `doc/需求.md`
- `doc/总体设计.md`
- `doc/模块划分.md`
- `doc/Java构建适配方案.md`
- `doc/UI风格.md`
- `doc/progress.md`
- `doc/tasks/<模块名称>/task.md`
- `doc/tasks/<模块名称>/progress.md`

如果文档之间冲突，优先级如下：

```text
AGENTS.md
doc/总体设计.md
doc/Java构建适配方案.md
doc/模块划分.md
doc/需求.md
doc/tasks/<模块名称>/task.md
doc/tasks/<模块名称>/progress.md
```

如果仍存在冲突，主 Agent 必须选择更保守、更符合测试可用版本范围、更符合模块边界的方案，并把假设记录到相关模块的 `progress.md`。

## 2. 总体目标

实现一个企业内部 PaaS 平台测试可用版本：

- 后端使用 Go。
- 后端数据库必须使用 MySQL 8.0+，存储引擎为 InnoDB，字符集为 utf8mb4。
- 前端使用 React + TypeScript + Vite + Ant Design。
- 前端 UI 必须参考根目录 `xx.png` 和 `doc/UI风格.md`，采用企业级运维控制台风格。
- 前端所有用户可见文案必须使用中文；技术枚举、API 字段、代码片段和日志原文可以保留英文。
- 用户认证必须同时支持 OIDC 登录和平台自主创建的本地用户账号密码登录。
- 平台作为统一入口，普通用户不直接访问 Jenkins、GitLab 部署清单仓库、Argo CD 或 Kubernetes。
- 控制面不直接调用 Kubernetes API Server。
- 每个 Kubernetes 集群内运行 Argo CD 和 PaaS Agent。
- 应用以最小独立交付单元建模。
- 当前测试可用版本构建支持 Java，管理员通过构建环境维护构建镜像。
- Jenkins 只运行平台统一 Job 模板，按 PaaS 固化的 BuildSpec 执行构建。

## 3. 主 Agent 职责

主 Agent 负责任务编排、质量门禁和进度跟踪，不直接绕过模块任务随意实现功能。

主 Agent 必须：

- 读取 `doc/progress.md`，确定未完成模块。
- 按依赖顺序启动子 Agent。
- 为每个子 Agent 提供对应模块的 `task.md`、`progress.md` 和相关设计文档。
- 确保子 Agent 不越过模块边界访问其他模块内部实现。
- 合并子 Agent 产出后运行全量测试、覆盖率检查和 e2e 测试。
- 只有测试通过且覆盖率达标，才更新 checklist。
- 每完成一个模块，更新：
  - `doc/tasks/<模块名称>/progress.md`
  - `doc/progress.md`

主 Agent 不得：

- 跳过测试。
- 因外部系统不可用而停止实现；必须使用 fake、mock 或 adapter 测试替代。
- 将未完成或未测试的模块标记为完成。
- 暴露 Jenkins、GitLab、Argo CD、Kubernetes 凭据给前端或日志。

## 4. 子 Agent 分工

主 Agent 必须为每个模块生成一个子 Agent。每个子 Agent 只负责一个模块。

模块列表：

```text
shared-kernel
identity-access
tenant-project
source-repository
application-environment
build
release-delivery
gitops-deployment
cluster-agent
paas-agent
web-console
audit
notification
integrations
```

推荐执行顺序：

```text
1. shared-kernel
2. identity-access
3. tenant-project
4. integrations
5. source-repository
6. application-environment
7. build
8. release-delivery
9. gitops-deployment
10. cluster-agent
11. paas-agent
12. audit
13. notification
14. web-console
```

允许并行的任务：

- `identity-access` 和 `tenant-project` 可在 `shared-kernel` 完成后并行。
- `integrations` 可与 `source-repository`、`build` 的 port 设计并行，但不得破坏 port 边界。
- `cluster-agent` 和 `paas-agent` 可在 Agent API 契约稳定后并行。
- `web-console` 可基于 mock API 并行开发，但最终必须接入真实 API。

## 5. 子 Agent 工作模板

每个子 Agent 启动时必须使用以下工作模板：

```text
你是 <模块名称> 子 Agent。

必须阅读：
- AGENTS.md
- doc/总体设计.md
- doc/模块划分.md
- doc/Java构建适配方案.md
- doc/tasks/<模块名称>/task.md
- doc/tasks/<模块名称>/progress.md

你的边界：
- 只能实现 <模块名称> 的职责。
- 只能通过公开 port、API、事件或 fake adapter 与其他模块交互。
- 不允许直接访问其他模块拥有的数据表或内部包。
- 不允许为了通过测试而弱化领域规则、安全规则或审计规则。

交付要求：
- 完成 task.md 中所有 checklist。
- 补齐单元测试、模块集成测试和契约测试。
- 本模块单元测试覆盖率必须达到 90%。
- 更新 doc/tasks/<模块名称>/progress.md。
- 输出实现摘要、测试命令和覆盖率结果。
```

## 6. 工程结构要求

后端推荐结构：

```text
cmd/
  paas-server/
  paas-agent/

internal/
  modules/
    identityaccess/
    tenantproject/
    sourcerepository/
    appenv/
    build/
    delivery/
    gitops/
    clusteragent/
    audit/
    notification/
  integrations/
    gitlab/
    jenkins/
    argocd/
    registry/
  platform/
    config/
    database/
    http/
    eventbus/
    logging/
    security/
  shared/

web/
  console/

tests/
  integration/
  e2e/
```

前端推荐结构：

```text
web/console/
  package.json
  vite.config.ts
  src/
    app/
    pages/
    components/
    api/
    stores/
    routes/
    tests/
```

前端 UI 要求：

- 所有用户可见文案必须使用中文，包括导航、按钮、表单标签、校验提示、状态、空状态、错误提示和确认弹窗。
- 使用深色左侧导航、浅色内容区、顶部组织上下文和全局搜索。
- 登录页必须同时提供本地账号密码登录和 OIDC 登录入口。
- 创建应用向导采用三栏布局：左侧表单、中间仓库或模板预览、右侧校验清单。
- 步骤条采用参考图风格：已完成绿色勾选、当前步骤蓝色数字、未完成灰色数字。
- 部署模板配置页使用 Monaco Editor，并显示校验结果和版本记录。
- 不做营销页、大屏风格、大面积渐变或装饰型界面。

## 7. 模块边界规则

必须遵守以下边界：

- 每个模块只能通过自己的 repository 访问自己拥有的数据表。
- 其他模块需要数据时，必须通过 query port、command port、API 或事件投影访问。
- 外部系统调用只能放在 `internal/integrations/*` 或 `paas-agent` 内。
- 控制面业务模块不得直接调用 Kubernetes API Server。
- `paas-agent` 可以访问所在集群 Kubernetes API Server，但不执行业务发布决策。
- `build` 模块不生成 Release/Freight，只发布 `BuildSucceeded` 或 `BuildFailed` 事件。
- `release-delivery` 模块不直接写 GitLab 清单仓库，只调用 `GitOpsDeploymentCommand`。
- `web-console` 不直接访问 GitLab、Jenkins、Argo CD 或 Kubernetes。

identity-access 认证要求：

- 必须支持 `local` 和 `oidc` 两种 Identity Provider。
- 本地用户由平台管理员创建，密码必须强哈希存储，不允许明文保存。
- OIDC 用户通过 `issuer` + `subject` 唯一映射到平台 User。
- OIDC client_secret 只能通过受控配置或 secret 引用注入，不得返回前端或写入日志。
- 本地用户创建、密码重置、OIDC 登录、Identity 绑定和 Token 签发必须产生审计日志。

## 8. BuildSpec 规则

当前测试可用版本支持：

```text
java-springboot
java-tomcat
```

BuildSpec 字段：

```text
source_path
build_command
artifact_copy_command
runtime_base_image
artifact_deploy_path
default_ref
```

实现要求：

- `artifact_copy_command` 必须把产物写入 `$PAAS_ARTIFACT_OUTPUT`。
- `runtime_base_image` 必须来自平台维护的运行时环境。
- `artifact_deploy_path` 是可选产物放置路径；填写时必须是绝对路径，且不允许包含 `..`。
- `build_command` 由用户填写，但只能在 `source_path` 下执行。
- 不支持客户 Jenkinsfile。
- 不支持客户 Dockerfile 作为当前测试可用版本镜像构建入口。
- 触发 Jenkins 前必须按本次 BuildRun 渲染并更新固定 Job；触发 Jenkins 时不传递构建参数。

## 9. 测试和质量门禁

整个过程不会有人工参与，因此测试门禁必须自动化。

必须满足：

- 后端 Go 单元测试覆盖率总体不低于 90%。
- 每个后端模块单元测试覆盖率不低于 90%。
- 后端集成测试必须覆盖 MySQL 8.0 方言，允许使用 testcontainers MySQL、docker MySQL 或本地 MySQL。
- 不允许引入 PostgreSQL 专属 SQL、数据类型或迁移语法，例如 `RETURNING`、`jsonb`、原生 `uuid` 类型、数组列。
- 前端核心业务逻辑、API client、状态管理和关键页面测试覆盖率不低于 90%。
- 必须有 e2e 测试。
- 必须有模块集成测试和契约测试。
- 所有 fake、mock adapter 必须与 port 契约一致。
- 不能通过删除测试、降低断言或跳过失败用例来满足覆盖率。

建议测试命令：

```text
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out

cd web/console
npm test -- --coverage
npm run e2e
```

如果项目实际脚本不同，主 Agent 必须先补齐标准脚本，再执行门禁。

## 10. e2e 测试要求

必须至少覆盖以下端到端路径：

- 准备或迁移 SourceRepository。
- 创建 Application，并指定 BuildSpec。
- 首次构建。
- 查看实时构建日志。
- 构建成功后生成 Release 和 Freight。
- 发布到 dev。
- 晋级到 test。
- 申请发布 prod。
- 审批 prod。
- 查看环境运行状态。
- 回滚历史版本。
- 查看审计记录。

e2e 测试不得依赖真实 GitLab、Jenkins、Argo CD、Kubernetes。必须使用 fake service、mock server 或本地测试替身实现稳定自动化。
认证相关 e2e 至少覆盖本地用户登录和 OIDC mock provider 登录两条路径。

## 11. 进度更新规则

每个子 Agent 完成任务后必须：

- 将 `doc/tasks/<模块名称>/task.md` 中已完成的任务勾选。
- 将 `doc/tasks/<模块名称>/progress.md` 中已完成的进度勾选。
- 记录测试命令、覆盖率结果和 e2e 相关结果。

主 Agent 完成模块验收后必须：

- 如果模块所有关键项完成并测试通过，勾选 `doc/progress.md` 中对应模块。
- 如果模块未完成，不得勾选根级进度。
- 如果发现设计文档需要同步更新，必须更新相关 `doc/*.md`。

## 12. 最终验收标准

只有满足以下条件，主 Agent 才能声明项目完成：

- `doc/progress.md` 中 14 个模块全部完成。
- 所有 `doc/tasks/<模块名称>/progress.md` 全部完成。
- 后端、前端、模块集成测试全部通过。
- 单元测试覆盖率达到 90%。
- e2e 测试通过。
- 所有关键操作有审计日志。
- 用户无法直接访问 Jenkins UI、GitLab 部署清单仓库、Argo CD UI 或 kubeconfig。
- PaaS 控制面不直连 Kubernetes API Server。
- PaaS Agent 能上报 Argo CD 和 Workload 状态。
- Java Spring Boot jar 和 Tomcat war 构建链路均通过测试。

## 13. 当前启动指令

现在开始执行：

1. 读取 `doc/progress.md`。
2. 找到第一个未完成模块。
3. 启动对应子 Agent。
4. 子 Agent 按该模块 `task.md` 实现功能和测试。
5. 子 Agent 完成后由主 Agent 运行质量门禁。
6. 通过后更新模块进度和总进度。
7. 继续下一个模块，直到所有模块完成。

整个过程中不要等待人工确认。遇到不明确但不影响安全和架构边界的问题时，选择文档中更保守的测试可用版本方案，并记录假设。
