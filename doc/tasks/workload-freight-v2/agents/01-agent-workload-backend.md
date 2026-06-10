# Agent 01：Workload 后端基础提示词

你是 Workload Freight V2 的 Worker Agent，只负责 Application/Workload 后端基础能力。

## 必须先阅读

- `AGENTS.md`
- `doc/需求文档.md`
- `doc/总体设计.md`
- `doc/模块划分.md`
- `doc/验收清单.md`
- `doc/tasks/application-environment/task.md`
- `doc/tasks/workload-freight-v2/task.md`
- `doc/tasks/workload-freight-v2/progress.md`

## 任务范围

只实现以下能力：

- `workloads` 数据模型、仓储、迁移和测试。
- `workload_environment_configs` 数据模型、仓储、迁移和测试。
- Workload 创建、编辑、启用、禁用、删除。
- Workload 列表、详情、按 Application 查询启用 Workload。
- Workload 类型校验：只允许 Deployment 或 StatefulSet。
- 同一 Application 下 Workload 名称唯一。
- WorkloadEnvironmentConfig 查询和保存。
- Workload 创建、状态变更、部署配置变更的审计日志。
- 必要的 API handler 和 DTO。

## 禁止修改

- 不要实现 Freight 创建逻辑。
- 不要修改 Promotion 状态机。
- 不要修改 GitOps values 写入逻辑。
- 不要改 Web Console 页面。
- 不要改 Jenkins 适配器。
- 不要做无关重构。

## 关键领域规则

- Application 是业务交付上下文。
- Workload 是最小可部署单元。
- 一个 Workload 对应一个容器镜像和一个 Kubernetes 工作负载。
- Workload 支持 Deployment 和 StatefulSet。
- `enabled` Workload 后续必须进入新 Freight。
- `disabled` 或 `deleted` Workload 不进入新 Freight。
- 环境变量、域名、配置文件、可写目录等环境差异属于 WorkloadEnvironmentConfig，不属于 Freight。

## 推荐实现顺序

1. 搜索现有 `internal/modules/appenv` 的 domain、service、repository、api 和 migrations 模式。
2. 先写 Workload domain 和校验测试。
3. 写迁移和 MySQL 仓储测试。
4. 写 service 用例测试。
5. 实现 API handler 和 DTO。
6. 增加审计事件。
7. 跑测试并修复失败。

## 必须测试

至少覆盖：

- 创建 Workload 成功。
- 同一 Application 下 Workload 名称重复失败。
- 非法 Workload 类型失败。
- 禁用 Workload 后不出现在启用列表。
- 保存并查询 WorkloadEnvironmentConfig。
- 配置里包含端口、资源、探针、域名、配置文件和可写目录。
- Workload 创建和配置变更记录审计。

## 验证命令

优先运行局部测试：

```bash
go test ./internal/modules/appenv ./internal/migrations
```

如果改动影响共享类型，再运行：

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
