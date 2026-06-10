# Agent 02：Release/Freight 后端提示词

你是 Workload Freight V2 的 Worker Agent，只负责 Build、Release、Freight 和 Promotion 相关后端能力。

## 必须先阅读

- `AGENTS.md`
- `doc/需求文档.md`
- `doc/总体设计.md`
- `doc/模块划分.md`
- `doc/tasks/build/task.md`
- `doc/tasks/release-delivery/task.md`
- `doc/tasks/workload-freight-v2/task.md`
- `doc/tasks/workload-freight-v2/progress.md`

## 前置假设

- Agent 01 已经或将会提供 Workload 查询能力。
- 如果当前分支暂时没有 Agent 01 的代码，先通过接口或最小 port 设计对接，不要复制 Workload 领域实现。

## 任务范围

只实现以下能力：

- BuildPipeline、BuildRun、BuildArtifact 绑定 `workload_id`。
- BuildSucceeded 事件 payload 包含 `app_id`、`workload_id`、`build_run_id`、`build_artifact_id`。
- BuildSucceeded 只创建 Workload Release 候选。
- 停用或删除自动创建 Freight 的事件处理路径。
- Release 补充 Workload 和镜像信息。
- Freight creation-context API。
- 用户手动创建 Freight。
- FreightItem 支持 `pipeline_artifact` 和 `custom_image`。
- 校验 Freight 必须覆盖全部启用 Workload。
- 校验同一 Freight 中 Workload 不重复。
- 校验 pipeline artifact 必须属于对应 Workload。
- 校验 custom image 镜像地址格式。
- Stage eligible-freights API。
- Promotion 创建前校验 Freight 完整性和 Stage 顺序。
- Freight 创建、Promotion 创建、custom image 风险信息审计。

## 禁止修改

- 不要实现 Workload CRUD。
- 不要实现 GitOps values 写入。
- 不要改 Web Console 页面。
- 不要直接查询其他模块私有表；通过 port/query service 对接。
- 不要让 build 模块生成 Release 或 Freight。

## 关键领域规则

- Build 成功只产生 BuildArtifact 和 Workload Release 候选。
- Release 属于 Workload。
- Freight 属于 Application。
- 创建 Freight 时必须包含 Application 下所有启用 Workload。
- 每个启用 Workload 必须且只能对应一个 FreightItem。
- Freight 创建后不原地修改镜像组合。
- FreightItem 来源为 `custom_image` 时允许 tag 或 digest；tag 需要审计风险提示。
- dev/prod 环境差异不进入 Freight。

## 推荐实现顺序

1. 搜索现有 `internal/modules/build` 和 `internal/modules/delivery`。
2. 先补 BuildArtifact/Release/FreightItem 的 `workload_id` 迁移和仓储测试。
3. 改 BuildSucceeded 事件 payload 和消费逻辑。
4. 写“BuildSucceeded 不自动创建 Freight”的失败优先测试。
5. 实现手动 Freight 创建和完整性校验。
6. 实现 creation-context 和 eligible-freights API。
7. 补 Promotion 创建前校验。
8. 补审计。

## 必须测试

至少覆盖：

- BuildSucceeded 创建 Workload Release 候选。
- BuildSucceeded 不自动创建 Freight。
- Freight 缺少任一启用 Workload 时失败。
- Freight 对同一 Workload 出现多个 FreightItem 时失败。
- pipeline artifact 不属于目标 Workload 时失败。
- custom image 合法时可以创建 FreightItem。
- custom image tag 记录风险审计。
- Stage eligible-freights 只返回可发布 Freight。
- Promotion 创建前校验 Stage 顺序。

## 验证命令

局部测试：

```bash
go test ./internal/modules/build ./internal/modules/delivery ./internal/migrations
```

必要时全量：

```bash
go test ./...
```

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
