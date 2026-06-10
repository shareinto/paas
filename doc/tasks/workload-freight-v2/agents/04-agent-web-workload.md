# Agent 04：Web 应用 Workload 页面提示词

你是 Workload Freight V2 的 Worker Agent，只负责 Web Console 的 `应用 Workload` 页面。

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

- 后端 Workload API 按 `doc/tasks/workload-freight-v2/task.md` 暴露。
- 如果真实 API 尚未合并，先通过 mock API 和类型定义实现页面，不阻塞 UI 结构。

## 任务范围

只实现以下能力：

- 应用详情主要入口包含 `应用 Workload` 和 `发布晋级`。
- `应用 Workload` 列表展示 Workload 名称、类型、镜像来源、最近 Release、各环境状态。
- `创建 Workload` 使用抽屉或弹窗。
- Workload 表单支持 Deployment/StatefulSet 切换。
- Workload 表单支持流水线产物来源和自定义镜像来源。
- 部署配置使用抽屉或弹窗。
- 部署配置支持端口、资源、探针、域名、配置文件、环境变量、可写目录。
- 对接或 mock Workload API hooks。
- 补充组件测试。

## 禁止修改

- 不要实现发布晋级页 Freight 时间轴。
- 不要实现创建 Freight 抽屉。
- 不要改后端业务逻辑。
- 不要新增英文用户可见文案。
- 不要做营销页或大屏风格。

## UI 规则

- 所有用户可见文案必须使用中文。
- 使用企业级运维控制台风格。
- 不使用卡片套卡片。
- 创建和编辑流程使用抽屉或弹窗，不新增独立 tab。
- 表格、表单、状态标签保持 Ant Design 风格一致。
- 自定义镜像 tag 场景可展示风险提示，但 Freight 创建里的强校验由 Agent 05 负责。

## 推荐实现顺序

1. 搜索 `web/console/src/pages/ApplicationDetailPage.tsx` 和现有 API hooks。
2. 调整应用详情 tab，但不要破坏已有构建、审计等入口。
3. 增加 Workload 类型和 mock 数据。
4. 实现 Workload 列表。
5. 实现创建 Workload 抽屉。
6. 实现部署配置抽屉。
7. 补测试。

## 必须测试

至少覆盖：

- 应用详情能看到 `应用 Workload` tab。
- Workload 列表展示 Deployment 和 StatefulSet。
- 点击 `创建 Workload` 打开抽屉。
- Workload 类型切换可用。
- 自定义镜像输入可用。
- 部署配置抽屉展示端口、资源、探针、域名、配置文件、可写目录字段。
- 页面没有英文用户可见按钮和标题。

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
