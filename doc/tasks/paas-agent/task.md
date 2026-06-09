# paas-agent 测试可用版本任务

## 目标

实现部署在 Kubernetes 集群内的 PaaS Agent，采集 Argo CD Application 和 Workload 状态，主动上报控制面，并执行受控任务。

## 任务清单

- [x] 初始化 `cmd/paas-agent` 启动入口。
- [x] 定义 Agent 配置：clusterId、controlPlaneURL、agentToken、上报周期、namespace 范围。
- [x] 实现控制面 HTTP client，支持心跳、状态上报、事件上报、任务拉取、任务结果回传。
- [x] 使用 Kubernetes client 读取 Argo CD Application。
- [x] watch Deployment、StatefulSet、DaemonSet、ReplicaSet、Pod、Event。
- [x] 将 Argo CD sync status、health status、operation state 转为 PaaS 标准状态。
- [x] 汇总 Workload 副本状态：desired、ready、updated、available。
- [x] 实现 10 秒心跳。
- [x] 实现 30 秒状态快照上报。
- [x] 状态变化时立即上报。
- [x] 实现受控任务 `argocd_refresh`。
- [x] 实现受控任务 `argocd_sync`，只作用于指定 Argo CD Application。
- [x] 限制 Agent 不任意 apply/delete 业务资源。
- [x] 提供 Kubernetes RBAC manifest 草案，最小权限为 get/list/watch 和指定 annotation patch。
- [x] 编写测试：fake Kubernetes client 状态采集、Argo CD Application 状态解析、控制面 mock 上报、任务执行结果。

## 完成标准

- [x] Agent 可独立运行在集群内。
- [x] Agent 不保存平台用户凭据。
- [x] Agent 不执行业务发布决策。
- [x] 状态上报格式符合控制面 Agent API。
- [x] 测试通过。
