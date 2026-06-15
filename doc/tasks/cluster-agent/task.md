# cluster-agent 测试可用版本任务

## 目标

实现控制面的集群管理和 Agent Gateway，接收 PaaS Agent 心跳、状态、事件上报，并分发受控任务。

## 任务清单

- [x] 定义领域模型：`Cluster`、`ClusterHeartbeat`、`ClusterResourceSnapshot`、`ClusterTask`、`ClusterTaskResult`。
- [x] 建立数据表迁移草案：`clusters`、`cluster_heartbeats`、`cluster_resource_snapshots`、`cluster_tasks`、`cluster_task_results`。
- [x] 定义 Cluster 状态：`ready`、`degraded`、`unreachable`、`draining`、`disabled`。
- [x] 实现 Cluster 注册、查询、更新、禁用、draining 标记。
- [x] 实现 Agent token 生成、哈希存储和轮换。
- [x] 实现 Agent 认证，限制 token 只能上报所属 clusterId。
- [x] 实现 Agent API：`POST /api/agent/v1/heartbeat`。
- [x] 实现 Agent API：`POST /api/agent/v1/status/report`。
- [x] 实现 Agent API：`POST /api/agent/v1/events/report`。
- [x] 实现 Agent API：`GET /api/agent/v1/tasks`。
- [x] 实现 Agent API：`POST /api/agent/v1/tasks/result`。
- [x] 心跳超时后将 Cluster 标记为 `unreachable`。
- [x] 将 Agent 状态上报转发给 `StageStateUpdater` 和 `DeploymentStatusUpdater`。
- [x] 实现 ClusterTask 创建、拉取、结果回写。
- [x] 提供控制面 Cluster API。
- [x] 编写测试：Agent token 绑定、心跳上报、超时 unreachable、状态转换、任务拉取和结果回传。

## 完成标准

- [x] 控制面不直接 watch Kubernetes。
- [x] Agent 只能上报所属集群。
- [x] Cluster 离线可被识别。
- [x] Agent 状态能更新环境和部署状态。
- [x] 测试通过。
