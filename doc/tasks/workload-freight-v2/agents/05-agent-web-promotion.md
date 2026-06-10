# Agent 05：Web 发布晋级页面提示词

你是 Workload Freight V2 的 Worker Agent，只负责 Web Console 的 `发布晋级` 页面和创建 Freight 交互。

## 必须先阅读

- `AGENTS.md`
- `doc/UI风格.md`
- `doc/需求文档.md`
- `doc/总体设计.md`
- `doc/tasks/web-console/task.md`
- `doc/tasks/workload-freight-v2/task.md`
- `doc/tasks/workload-freight-v2/progress.md`
- `output/ui-mockups/workload-freight-flow.html`（作为交互参考）

## 前置假设

- 后端 Freight API、creation-context API、eligible-freights API 按 `doc/tasks/workload-freight-v2/task.md` 暴露。
- 如果真实 API 尚未合并，先通过 mock API 和类型定义实现页面，不阻塞 UI 结构。

## 任务范围

只实现以下能力：

- `发布晋级` 页展示 Freight 时间轴，按创建时间从左到右排列。
- 展示 `dev`、`test`、`staging`、`prod` Stage 卡片。
- 每个 Stage 卡片提供 `发布` 按钮。
- 点击 Stage 的 `发布` 后调用 eligible-freights API 或 mock。
- 可发布 Freight 点亮，不可发布 Freight 禁用或置灰。
- 选择 Freight 后展示确认弹窗。
- 确认弹窗列出所有 Workload 和镜像版本。
- prod 发布展示审批人数、审批人范围和禁止自审批提示。
- 创建 Freight 使用抽屉或弹窗。
- 创建 Freight 时列出所有启用 Workload。
- 每个 Workload 必须选择一个镜像版本。
- 来源支持流水线产物和自定义镜像。
- 自定义镜像 tag 显示中文风险提示。
- 未覆盖全部 Workload 时禁用提交按钮。
- 补充组件测试。

## 禁止修改

- 不要实现 Workload 后端 CRUD。
- 不要实现 GitOps 发布后端。
- 不要改应用 Workload 页面主体逻辑。
- 不要新增英文用户可见文案。
- 不要把创建 Freight 或 Freight 详情做成独立 tab。

## UI 规则

- 所有用户可见文案必须使用中文。
- Freight 时间轴从左到右按创建时间排列。
- Stage 发布动作从 Stage 卡片上的 `发布` 按钮触发。
- 点击发布后，只有当前 Stage 可发布 Freight 点亮。
- 创建 Freight 和确认发布都使用抽屉或弹窗。
- 一个 Freight 至少包含一个 Workload，且必须覆盖所有启用 Workload。

## 推荐实现顺序

1. 搜索现有发布晋级页面、Freight 列表、Promotion API hooks。
2. 增加 Freight、FreightItem、Stage eligibility 的前端类型。
3. 实现 Freight 时间轴。
4. 实现 Stage 卡片和发布按钮。
5. 实现可发布 Freight 高亮状态。
6. 实现发布确认弹窗。
7. 实现创建 Freight 抽屉。
8. 补测试。

## 必须测试

至少覆盖：

- Freight 按创建时间从左到右显示。
- Stage 卡片显示 `发布` 按钮。
- 点击 `dev` 发布后，只点亮 dev 可发布 Freight。
- 不可发布 Freight 被禁用或置灰。
- 选择 Freight 后确认弹窗显示所有 Workload 镜像。
- prod 发布显示审批提示和禁止自审批提示。
- 创建 Freight 抽屉列出所有启用 Workload。
- 未给所有 Workload 选择镜像时提交按钮禁用。
- 自定义镜像 tag 显示风险提示。

## 验证命令

```bash
cd web/console && npm test -- --coverage
cd web/console && npm run build
```

如果已有测试命令不同，先查看 `web/console/package.json`，使用项目实际脚本。

## 完成汇报格式

完成后汇报：

```text
状态：DONE / DONE_WITH_CONCERNS / BLOCKED
改动文件：
实现内容：
测试命令和结果：
风险或遗留问题：
需要 Coordinator 关注：
```
