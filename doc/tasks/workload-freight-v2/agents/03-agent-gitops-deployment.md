# Agent 03：GitOps Deployment 提示词

你是 Workload Freight V2 的 Worker Agent，只负责 GitOps 部署侧能力。

## 必须先阅读

- `AGENTS.md`
- `doc/需求文档.md`
- `doc/总体设计.md`
- `doc/模块划分.md`
- `doc/tasks/gitops-deployment/task.md`
- `doc/tasks/workload-freight-v2/task.md`
- `doc/tasks/workload-freight-v2/progress.md`

## 前置假设

- Agent 01 提供 Workload 和 WorkloadStageConfig 查询能力。
- Agent 02 提供 Freight、FreightItem、Promotion 相关数据。
- 如果当前分支暂时没有这些实现，优先定义或使用 port 接口，不复制其他模块逻辑。

## 任务范围

只实现以下能力：

- 单 Workload 标准 Helm chart values 结构。
- 将 WorkloadStageConfig 渲染为环境 values。
- Promotion 部署时遍历 FreightItem。
- 分别更新每个 Workload 的环境 values 中的 image 字段。
- Deployment 记录关联 Promotion、Freight 和 Workload 变更摘要。
- 回滚时从历史 FreightItem 写回各 Workload 镜像版本。
- 多 Workload 发布、回滚、MR 创建失败的测试。

## 禁止修改

- 不要实现 Workload CRUD。
- 不要实现 Freight 创建接口。
- 不要改 Web Console 页面。
- 不要直接调用 Kubernetes API Server。
- 不要暴露 Argo CD UI、部署清单仓库或 kubeconfig。

## 关键领域规则

- Freight 只携带镜像版本组合。
- 环境变量、域名、配置文件、可写目录等差异来自 WorkloadStageConfig 或环境 values。
- 发布 Freight 到环境时，只更新该环境下各 Workload 的镜像版本和必要部署 values。
- 所有 Stage 在 PaaS 审批/验证门禁满足后直接 commit 到清单仓库。
- 回滚通过修改 Git 期望状态完成，不直接操作 Kubernetes。

## 推荐实现顺序

1. 搜索 `internal/modules/gitops` 中 values 写入、Deployment、ManifestRevision 和回滚实现。
2. 先写多 Workload values 更新测试。
3. 定义 values 结构，兼容现有单应用 values。
4. 实现 WorkloadStageConfig 到 values 的渲染。
5. 实现 FreightItem 循环写入。
6. 实现回滚使用历史 FreightItem。
7. 补 ManifestRevision 和 Deployment 变更摘要。

## 必须测试

至少覆盖：

- 一个 Freight 含两个 FreightItem 时，两个 Workload 的 values 都更新。
- Deployment 类型 Workload 渲染正确。
- StatefulSet 类型 Workload 渲染正确。
- 所有 Stage 直接 commit 策略仍可用。
- staging/prod 等高风险环境的准入由 PaaS Promotion 审批流控制。
- 回滚写回历史 FreightItem 镜像。
- GitLab commit 失败时 Deployment 进入失败状态并展示中文原因。

## 验证命令

局部测试：

```bash
go test ./internal/modules/gitops ./internal/migrations
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
