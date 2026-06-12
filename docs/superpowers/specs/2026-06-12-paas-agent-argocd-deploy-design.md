# paas-agent Argo CD 部署设计

## 背景

本次任务是在现有 Kubernetes 集群中部署 `paas-agent`。集群 kubeconfig 已配置，`kubectl` 可用；`paas-server` 对外地址为 `http://122.152.196.135:18080`；部署必须通过 `/windows/go/src/github.com/shareinto/manifests` 中的 Argo CD manifests 约定完成。

当前 `paas` 仓库已经提供 `deploy/paas-agent` Helm chart 和 `deploy/paas-agent/Dockerfile`。`manifests` 仓库约定为：`*-applications` 目录存放环境级 Argo CD Application，其他顶层目录存放被 Application 引用的 chart 或 manifests。

## 目标

1. 构建可在 ARM 节点运行的 `paas-agent` 镜像。
2. 将镜像推送到 `cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg/paas-agent`。
3. 在 `paas-server` 注册当前集群，使用第一个现有租户，集群名为 `llt-arm-cluster`。
4. 在 manifests 中新增 `paas-agent` chart 仓库目录和 `llt-applications` 下的 Argo CD Application。
5. 通过 Argo CD 将 `paas-agent` 部署到 Kubernetes 集群内，并确认心跳和状态上报可用。

## 方案

采用 manifests GitOps 方案。将 `deploy/paas-agent` Helm chart 同步到 `/windows/go/src/github.com/shareinto/manifests/paas-agent`，在 `/windows/go/src/github.com/shareinto/manifests/llt-applications/paas-agent-application.yaml` 新增 Argo CD Application，指向 `ssh://git@gitops:2422/k8s/paas-agent.git`，目标 namespace 为 `paas-system`。

镜像使用 Docker buildx 构建 `linux/arm64` 平台产物并直接 push。tag 使用日期、当前 commit 短 SHA 和 `arm64` 后缀，便于追踪部署来源。

控制面集群注册使用 `admin/password` 登录，调用 `GET /api/tenants` 取第一个租户，再调用 `POST /api/clusters` 创建集群并获取一次性 `agent_token`。用户已确认允许 token 写入 manifests，因此 chart values 中直接设置 `secret.token`，由 Helm 渲染 Kubernetes Secret。

## 配置

`paas-agent` 关键配置如下：

- `config.clusterID`：控制面注册返回的集群 ID。
- `config.controlPlaneURL`：`http://122.152.196.135:18080`。
- `config.argocdNamespace`：`argocd`。
- `config.agentNamespaces`：以 `argocd` 和当前业务命名空间为初始采集范围，若无法自动判断则使用 `argocd,macc,paas-system`。
- `secret.token`：控制面注册返回的一次性 agent token。
- `image.repository`：`cloud-docker-register-registry.cn-hangzhou.cr.aliyuncs.com/sbg/paas-agent`。
- `image.tag`：本次构建生成的 ARM64 tag。

## 验证

部署前验证：

- `helm lint` 检查 chart。
- `helm template` 检查渲染出的 Deployment、ServiceAccount、RBAC、ConfigMap 和 Secret。

部署后验证：

- Argo CD Application 同步成功。
- `kubectl -n paas-system rollout status deployment/paas-agent` 成功。
- `kubectl -n paas-system logs deployment/paas-agent` 可看到 agent 启动日志，日志不输出 token。
- `paas-server` 中对应 cluster 有心跳或状态上报记录。

## 风险与处理

- token 写入 manifests 会进入 Git 历史。该行为已由用户确认允许；如后续需要收敛暴露面，应改为预创建 Kubernetes Secret 并在 chart 中使用 `secret.existingSecret`。
- 如果 `paas-agent` chart 仓库目录是新建独立 Git 仓库，需要初始化 remote 或确认已有 `ssh://git@gitops:2422/k8s/paas-agent.git` 可推送。
- 如果集群节点无法拉取阿里云镜像仓库，需要补充 `imagePullSecrets` 或确认节点已有拉取权限。
