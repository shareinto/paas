# Stage 化交付流需求与 UI 原型说明

整理日期：2026-06-12

相关原型：

- `output/ui-mockups/stage-delivery-flow.html`
- `output/ui-mockups/stage-delivery-flow.test.js`

## 1. 讨论背景

本次讨论围绕 Kubernetes 集群管理、集群与发布阶段的绑定关系、DeliveryFlow 模板管理以及前端交互原型展开。

最初的问题集中在：

- 环境和 Stage 如何关联。
- 环境如何绑定 Kubernetes 集群。
- 集群绑定动作到底是绑定到某个应用的环境，还是绑定到所有应用的同名环境。
- DeliveryFlow 应该如何表达晋级路线、审批、验证和角色权限。

经过讨论后，原先以应用环境为中心的设计被认为无法满足租户级集群池和交付流模板管理需求，因此最终调整为 Stage 化交付模型。

## 2. 关键结论

### 2.1 Environment 与 Stage 的关系

最终模型中，不再把 Environment 作为核心发布绑定对象。

应用发布视角统一使用 Stage：

- Stage 表示租户定义的发布阶段，例如 `dev`、`test`、`staging`、`prod`。
- 每个应用根据租户的 DeliveryFlow 模板生成自己的应用 Stage。
- 原先 Environment 承载的发布目标、部署状态、验证状态等能力，合并到应用 Stage 模型中表达。

也就是说，用户在控制台里看到的是应用 Stage 发布，而不是单独维护应用环境和 Stage 两套概念。

### 2.2 集群绑定范围

最终需求明确为：Kubernetes 集群绑定到租户级 Stage 名称，而不是绑定到单个应用的某个环境。

规则：

- 集群绑定的是所有应用下某个名称的 Stage。
- 例如集群绑定到 `dev` Stage 后，租户下所有应用的 `dev` Stage 都可以选择该集群作为发布目标。
- 一个 Stage 可以绑定多个集群。
- 一个集群也可以被多个 Stage 绑定。
- Stage 与集群是多对多关系。

这与“只绑定到某个应用环境”的旧理解不同。旧理解会导致每个应用都要重复配置同名环境的集群绑定，无法形成租户级统一集群池。

### 2.3 应用级部署配置

虽然集群池是按租户 Stage 统一绑定，但具体某个应用发布到某个 Stage 和某个集群时，仍然需要应用级配置。

应用级配置包括：

- Namespace。
- Manifest 路径。
- 应用 Stage 与目标集群的部署配置。
- 本次发布选择的目标集群子集。

原型中用 `AppStageClusterConfig` 表达该层配置。Namespace 默认使用项目名称，用户可以在发布时覆盖。

## 3. DeliveryFlow 模板需求

每个租户的管理员可以管理 DeliveryFlow 模板。

模板内容包括：

- Stage 列表。
- Stage 顺序。
- 晋级路线。
- 哪些角色可以 verify。
- 哪些角色可以 approve。
- 是否需要部署前审批。
- 是否需要部署后验证。
- Stage 显示名。
- Stage 颜色。
- Stage 启用或禁用状态。

模板是租户级资源，会影响租户下所有应用。

### 3.1 Stage key 稳定性

Stage key 创建后必须保持稳定。

约束：

- Stage key 创建后不可修改；删除 Stage 会物理删除模板项和该 Stage 的集群池绑定。
- 禁止修改已有 Stage key。
- 删除 Stage 会物理删除模板项和该 Stage 的集群池绑定。
- 历史发布记录必须保留。
- 允许调整显示名、颜色、顺序和策略。

### 3.2 模板变更生效

模板变更后：

- 自动影响租户下所有应用。
- 后续发布按最新模板规则执行。
- 进行中的发布也按最新模板规则校验。

## 4. 审批与验证规则

### 4.1 Approve

Approve 表示部署前或晋级前的审批门禁。

规则：

- 哪些角色可审批由 DeliveryFlow 模板的 `approve_roles` 控制。
- 审批动作作用于 Freight。
- 审批结论决定 Freight 是否允许继续晋级到目标 Stage。
- 审批入口放在 Freight 卡片上。
- 审批必须使用弹窗，不使用右侧抽屉。

### 4.2 Verify

Verify 表示部署后的人工验证门禁。

规则：

- 哪些角色可验证由 DeliveryFlow 模板的 `verify_roles` 控制。
- 验证动作作用于发布后的 Stage。
- Verify 只要求已有部署记录，不强制依赖健康状态。
- 健康状态、同步状态、Agent 状态作为验证证据展示。
- 验证结论决定 Freight 是否通过该 Stage，并影响下一 Stage 的可发布判断。
- 验证入口放在发布页的 Stage 卡片上。
- 验证必须使用弹窗，不使用右侧抽屉。

## 5. 前端 UI 原型范围

当前已生成独立 HTML 原型：

```text
output/ui-mockups/stage-delivery-flow.html
```

该原型是静态交互原型，不依赖后端接口，不启动开发服务器，直接用浏览器打开即可查看。

### 5.1 页面整体风格

UI 按企业级运维控制台风格设计：

- 中文文案。
- 左侧深色导航。
- 顶部全局工具栏。
- 主内容区信息密度适中。
- 不做营销页、Hero 页或装饰性大屏。
- 操作以按钮、卡片、弹窗、状态标签为主。

### 5.2 主视图

最终保留两个主视图：

1. 租户交付流模板。
2. 应用 Stage 发布。

已删除的视图：

- 独立 `Stage 绑定` Tab。
- 独立 `审批与验证` Tab。

删除原因：

- Stage 集群绑定应从模板 Stage 卡片进入，而不是独立页面。
- 审批和验证是发布上下文动作，应分别出现在 Freight 卡片和 Stage 卡片上，而不是独立 Tab。

## 6. 租户交付流模板页面

模板页用于租户管理员维护 DeliveryFlow 模板。

页面能力：

- 顶部展示模板统计：当前模板、Stage 数量、引用应用、生效方式。
- 右上角提供 `添加 Stage` 按钮。
- Stage 使用卡片展示。
- Stage 卡片顶部使用 Kargo 风格颜色条。
- Stage 名称显示在顶部色条上，使用白色字体。
- Stage 模板编辑人员可以设置 Stage 颜色。
- Stage 卡片上提供 `绑定集群`、`编辑`、`删除` 按钮。

### 6.1 添加和编辑 Stage

添加和编辑 Stage 使用弹窗，不使用右侧抽屉。

弹窗字段包括：

- Stage key。
- 显示名。
- Stage 颜色。
- 顺序。
- 状态。
- 部署前审批。
- 部署后验证。
- 允许审批角色。
- 允许验证角色。

### 6.2 删除 Stage

删除按钮出现在 Stage 卡片上。

产品语义：

- 删除不做物理删除。
- 删除会物理删除模板项和该 Stage 的集群池绑定。
- 保留历史发布记录。

### 6.3 绑定集群

绑定集群按钮出现在模板 Stage 卡片上。

点击后打开弹窗，弹窗中提供 Kubernetes 集群多选列表。

弹窗中明确提示：

- 绑定到租户级 Stage。
- 保存后进入该 Stage 的可选集群池。
- 同一集群可绑定多个 Stage。
- 绑定变更仅影响后续发布。

## 7. 应用 Stage 发布页面

发布页用于应用维度查看 Freight、Stage 状态并发起发布、审批、验证。

页面包括：

- Freight 卡片列表。
- 应用 Stage 卡片列表。
- 应用 Stage 集群配置侧栏。

### 7.1 Freight 卡片

Freight 卡片展示：

- Freight 编号。
- 镜像摘要。
- 当前状态。
- 审批按钮。

审批按钮点击后打开 `Freight 审批` 弹窗。

审批弹窗能力：

- 展示审批 Freight。
- 选择目标 Stage。
- 展示晋级来源和审批角色。
- 填写审批意见。
- 支持 `审批拒绝` 和 `审批通过`。

### 7.2 Stage 卡片

Stage 卡片展示：

- Stage 名称。
- Stage key。
- 集群池数量。
- 当前 Freight。
- 验证状态。
- 发布按钮。
- 验证按钮。

验证按钮点击后打开 `人工验证` 弹窗。

验证弹窗能力：

- 展示验证 Stage。
- 展示部署证据。
- 展示 Argo CD 同步和健康状态。
- 展示 Agent 状态。
- 填写验证备注。
- 支持 `验证不通过` 和 `验证通过`。

### 7.3 发布弹窗

发布按钮点击后打开发布确认弹窗。

弹窗字段包括：

- 目标 Stage。
- 选择 Freight。
- 选择目标集群子集。
- Namespace。

发布规则：

- 发布时从 Stage 已绑定的集群池中选择目标集群子集。
- Namespace 默认使用项目名称。
- 用户可以手动覆盖 Namespace。
- 创建后为每个目标集群生成独立部署记录。

## 8. 当前原型中的交互脚本

原型中包含以下关键交互函数：

- `showScreen`：切换主视图。
- `openStageTemplateModal`：打开添加或编辑 Stage 弹窗。
- `saveStageTemplate`：保存 Stage 模板配置。
- `deleteStageTemplate`：删除 Stage。
- `openClusterBindingModal`：打开 Stage 绑定集群弹窗。
- `saveClusterBinding`：保存 Stage 集群绑定。
- `openPromotion`：打开发布弹窗。
- `submitPromotion`：提交发布并更新 Stage 验证状态。
- `openVerifyModal`：打开人工验证弹窗。
- `completeVerify`：处理验证通过或不通过。
- `openApprovalModal`：打开 Freight 审批弹窗。
- `completeApproval`：处理审批通过或拒绝。

## 9. 原型验证

当前原型有静态检查脚本：

```text
output/ui-mockups/stage-delivery-flow.test.js
```

检查内容包括：

- 只保留两个主视图。
- 不保留独立 Stage 绑定 Tab。
- 不保留独立审批与验证 Tab。
- Stage 卡片包含颜色条和白色 Stage 名称。
- 模板页包含添加、编辑、删除、绑定集群按钮。
- 集群绑定使用弹窗和多选列表。
- 发布弹窗支持目标集群子集和 Namespace 覆盖。
- Stage 卡片包含验证按钮。
- Freight 卡片包含审批按钮。
- 验证和审批都使用弹窗，不使用右侧抽屉。

验证命令：

```bash
node output/ui-mockups/workload-freight-flow.test.js && node output/ui-mockups/stage-delivery-flow.test.js
```

最近一次验证结果：

```text
workload-freight-flow static prototype checks passed
stage-delivery-flow static prototype checks passed
```

## 10. 后续实现建议

后端和正式前端实现时，建议优先补齐以下领域对象和接口边界：

- `DeliveryFlowTemplate`：租户级交付流模板。
- `DeliveryFlowStage`：模板中的 Stage 定义。
- `StageClusterBinding` 或沿用文档中约定的 `EnvironmentClusterBinding` 命名时，需要明确其语义已变为租户 Stage 级绑定。
- `AppStage`：应用按模板生成的 Stage 实例。
- `AppStageClusterConfig`：应用 Stage 在某个集群上的部署配置。
- `Freight`：待晋级的发布载荷。
- `Promotion`：一次 Freight 晋级或发布动作。
- `Approval`：审批记录。
- `Verification`：人工验证记录。

需要特别注意：

- PaaS 控制面不直接调用 Kubernetes API Server。
- 集群侧状态应由 PaaS Agent 和 Argo CD 上报或执行。
- 普通用户不直接访问 Jenkins、Argo CD、部署清单仓库或 kubeconfig。
- 审批、验证、发布、回滚和集群绑定都应有审计记录。
- 不要在日志、返回值或审计中暴露凭据、token、secret 或 kubeconfig。
