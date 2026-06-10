# Workload Freight V2 开发计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Application 调整为业务交付上下文，引入动态 Workload，并支持用户手动创建覆盖全部 Workload 的 Freight，再以类 Kargo 方式在 Stage 中发布。

**Architecture:** `application-environment` 拥有 Workload 与环境部署配置，`build` 继续只产出 BuildArtifact，`release-delivery` 生成 Workload Release 候选并负责手动 Freight 与 Promotion，`gitops-deployment` 按 FreightItem 写入各 Workload 的环境 values。Web Console 以 `应用 Workload` 和 `发布晋级` 作为主要入口，创建 Workload、部署配置、创建 Freight 均使用弹窗或抽屉。

**Tech Stack:** Go、MySQL、React、TypeScript、Vite、Ant Design、TanStack Query、GitLab、Jenkins、Argo CD。

---

## 1. 背景和范围

当前已完成版本按单 Application 主镜像建模，并在构建成功后自动生成 Freight。新需求要求：

- 一个 Application 可以有动态数量的 Workload。
- 一个镜像对应一个 Workload。
- Workload 支持 Deployment 和 StatefulSet。
- Workload 可以选择流水线产物，也可以输入自定义镜像。
- 创建 Freight 时必须包含该 Application 下所有启用 Workload，不能漏选，不能重复。
- FreightItem 表示一个 Workload 的一个镜像版本。
- Freight 在所有 Stage 中流转，dev/prod 等环境差异由 WorkloadEnvironmentConfig 或环境 values 表达。
- 用户在发布晋级页从 Stage 发起发布，点击发布后可发布 Freight 点亮，用户选择 Freight 后创建 Promotion。

不纳入本计划：

- 跨 Application 的组合发布。
- 图形化 Helm chart 全量编辑器。
- 直接暴露 GitLab 部署清单仓库、Argo CD UI、Jenkins UI 或 kubeconfig。

## 2. 领域模型

### 2.1 Application

Application 表示业务交付上下文，字段沿用现有应用主体信息。Application 不再表达单镜像部署单元。

### 2.2 Workload

建议字段：

```text
id
tenant_id
project_id
app_id
name
display_name
workload_type            # deployment | statefulset
description
status                   # enabled | disabled | deleted
image_source_mode         # pipeline_artifact | custom_image | mixed
created_by
created_at
updated_at
```

规则：

- `name` 在同一 Application 下唯一。
- `workload_type` 仅允许 `deployment` 或 `statefulset`。
- `enabled` Workload 必须进入新 Freight。
- `disabled` 或 `deleted` Workload 不进入新 Freight。

### 2.3 WorkloadEnvironmentConfig

建议字段：

```text
id
app_id
workload_id
env_id
replicas
service_ports_json
resource_requests_json
resource_limits_json
probes_json
ingress_hosts_json
env_vars_json
secret_refs_json
config_files_json
writable_dirs_json
volume_mounts_json
init_containers_json
values_override_json
created_at
updated_at
```

规则：

- 环境变量、域名、配置文件、可写目录等 dev/prod 差异保存在这里或生成后的环境 values 中。
- Freight 不保存这些环境差异。
- 配置变更必须审计。

### 2.4 Release

Release 表示 Workload 的可发布镜像版本候选。

建议新增或调整字段：

```text
id
app_id
workload_id
build_run_id
build_artifact_id
image_repository
image_tag
image_digest
commit_sha
source_ref
source_type              # pipeline_artifact
status
created_at
```

规则：

- BuildSucceeded 只创建 Release 候选。
- BuildSucceeded 不自动创建 Freight。
- 缺少 digest 或 commit 的流水线产物不能作为可发布 Release 候选。

### 2.5 Freight 和 FreightItem

Freight 表示 Application 级完整发布包。

Freight 建议字段：

```text
id
app_id
name
description
status
created_by
created_at
```

FreightItem 建议字段：

```text
id
freight_id
app_id
workload_id
source_type              # pipeline_artifact | custom_image
release_id
build_artifact_id
image_repository
image_tag
image_digest
image_ref
commit_sha
created_at
```

规则：

- 创建 Freight 时必须读取 Application 下全部 `enabled` Workload。
- 每个 `enabled` Workload 必须且只能有一个 FreightItem。
- `source_type=pipeline_artifact` 时必须引用同一 Workload 的 Release 或 BuildArtifact。
- `source_type=custom_image` 时允许 tag 或 digest；tag 需要 UI 风险提示和审计记录。
- Freight 创建后不可原地修改镜像组合；需要新版本时创建新的 Freight。

## 3. API 计划

### 3.1 Workload API

```text
GET    /api/v1/applications/{app_id}/workloads
POST   /api/v1/applications/{app_id}/workloads
GET    /api/v1/applications/{app_id}/workloads/{workload_id}
PUT    /api/v1/applications/{app_id}/workloads/{workload_id}
POST   /api/v1/applications/{app_id}/workloads/{workload_id}:disable
POST   /api/v1/applications/{app_id}/workloads/{workload_id}:enable
DELETE /api/v1/applications/{app_id}/workloads/{workload_id}
```

### 3.2 Workload 环境配置 API

```text
GET /api/v1/applications/{app_id}/workloads/{workload_id}/environment-configs
PUT /api/v1/applications/{app_id}/workloads/{workload_id}/environment-configs/{env_id}
```

### 3.3 Release 候选 API

```text
GET /api/v1/applications/{app_id}/workloads/{workload_id}/releases
GET /api/v1/applications/{app_id}/workloads/{workload_id}/artifacts
```

### 3.4 Freight API

```text
GET  /api/v1/applications/{app_id}/freights
POST /api/v1/applications/{app_id}/freights
GET  /api/v1/applications/{app_id}/freights/{freight_id}
GET  /api/v1/applications/{app_id}/freights/creation-context
```

`creation-context` 返回：

```text
enabled_workloads
latest_releases_by_workload
latest_artifacts_by_workload
stage_eligibility
```

### 3.5 Promotion API

```text
GET  /api/v1/applications/{app_id}/delivery/stages
GET  /api/v1/applications/{app_id}/delivery/stages/{stage_id}/eligible-freights
POST /api/v1/applications/{app_id}/delivery/stages/{stage_id}/promotions
```

## 4. 后端任务

### Task 1: 数据库迁移

**Files:**
- Modify/Create: `api-server` 迁移目录中的 MySQL migration 文件。
- Modify: release、freight、artifact 相关 repository。

- [ ] 新增 `workloads` 表。
- [ ] 新增 `workload_environment_configs` 表。
- [ ] 为 `application_sources` 或构建配置补充 `workload_id`。
- [ ] 为 `build_artifacts` 补充 `workload_id`。
- [ ] 为 `releases` 补充 `workload_id` 和镜像字段。
- [ ] 调整 `freight_items`，补充 `workload_id`、`source_type`、`image_ref`、`release_id`、`build_artifact_id`。
- [ ] 增加唯一约束：同一 Freight 中 `workload_id` 唯一。
- [ ] 增加查询索引：`app_id + workload_id`、`workload_id + created_at`。
- [ ] 编写迁移测试或 repository 集成测试。

### Task 2: application-environment 模块

**Files:**
- Modify: `application-environment` domain、service、repository、api。

- [ ] 新增 Workload domain 和状态枚举。
- [ ] 新增 WorkloadEnvironmentConfig domain。
- [ ] 实现 Workload 创建、编辑、启用、禁用、删除。
- [ ] 校验同一 Application 下 Workload 名称唯一。
- [ ] 校验 Workload 类型只允许 Deployment 或 StatefulSet。
- [ ] 实现按 Application 查询启用 Workload。
- [ ] 实现 Workload 环境配置保存和查询。
- [ ] 为 Workload 创建和配置变更记录审计日志。
- [ ] 补充单元测试和 API 测试。

### Task 3: build 模块适配 Workload

**Files:**
- Modify: `build` domain、service、callback、repository。

- [ ] BuildPipeline 绑定目标 Workload。
- [ ] BuildRun 和 BuildArtifact 保存 `workload_id`。
- [ ] Jenkins 回调写入 BuildArtifact 时带上 `workload_id`。
- [ ] BuildSucceeded 事件 payload 包含 `app_id`、`workload_id`、`build_run_id`、`build_artifact_id`。
- [ ] 确认 build 模块仍不生成 Release 或 Freight。
- [ ] 补充构建成功、构建失败、重复回调的测试。

### Task 4: release-delivery 模块

**Files:**
- Modify: `release-delivery` domain、service、repository、api。

- [ ] BuildSucceeded 消费逻辑改为只创建 Workload Release 候选。
- [ ] 删除或停用自动创建 Freight 的事件处理路径。
- [ ] 实现 Freight creation-context 查询。
- [ ] 实现手动创建 Freight。
- [ ] 校验 FreightItem 覆盖全部启用 Workload。
- [ ] 校验同一 Freight 中 Workload 不重复。
- [ ] 校验 pipeline_artifact 必须属于对应 Workload。
- [ ] 校验 custom_image 镜像地址格式。
- [ ] 实现 Stage eligible-freights 查询。
- [ ] Promotion 创建前校验 Freight 完整性和 Stage 顺序。
- [ ] 补充 Release、Freight、Promotion 状态机测试。

### Task 5: gitops-deployment 模块

**Files:**
- Modify: `gitops-deployment` deployment template、values writer、manifest revision。

- [ ] 定义单 Workload 标准 Helm chart values 结构。
- [ ] 将 WorkloadEnvironmentConfig 渲染为环境 values。
- [ ] Promotion 部署时遍历 FreightItem，分别更新各 Workload values 的 image 字段。
- [ ] Deployment 记录关联 Promotion、Freight 和 Workload 变更摘要。
- [ ] 回滚时从历史 FreightItem 写回各 Workload 镜像版本。
- [ ] 补充多 Workload Freight 发布、回滚、MR 创建失败的测试。

### Task 6: 审计与权限

**Files:**
- Modify: `identity-access` 权限定义、`audit` 事件枚举、相关 use case。

- [ ] 增加 Workload 创建、编辑、禁用、删除权限点。
- [ ] 增加 Workload 部署配置编辑权限点。
- [ ] 增加 Freight 手动创建权限点。
- [ ] 审计记录包含 Workload、FreightItem 来源类型和自定义镜像风险信息。
- [ ] 确认 API 响应不返回 secret、token、kubeconfig。

## 5. 前端任务

### Task 7: 应用详情导航

**Files:**
- Modify: `web/console` 应用详情页面、路由和菜单。

- [ ] 应用详情主要展示 `应用 Workload` 和 `发布晋级` tab。
- [ ] 移除独立的创建 Freight tab。
- [ ] 移除独立的 Freight 详情 tab。
- [ ] 保留低频信息入口，但不影响主流程。

### Task 8: 应用 Workload 页面

**Files:**
- Modify/Create: Workload list、Workload drawer、deployment config drawer。

- [ ] Workload 列表展示名称、类型、镜像来源、最近 Release、各环境状态。
- [ ] `创建 Workload` 按钮打开抽屉或弹窗。
- [ ] Workload 表单支持 Deployment/StatefulSet 切换。
- [ ] Workload 表单支持流水线产物来源和自定义镜像来源。
- [ ] 部署配置抽屉支持端口、资源、探针、域名、配置文件、环境变量、可写目录。
- [ ] 所有用户可见文案使用中文。
- [ ] 补充组件测试。

### Task 9: 创建 Freight 抽屉

**Files:**
- Modify/Create: Freight creation drawer、Freight API hooks。

- [ ] 从 creation-context 加载所有启用 Workload。
- [ ] 每个 Workload 必须显示一行选择器。
- [ ] 支持从流水线产物选择镜像版本。
- [ ] 支持输入自定义镜像。
- [ ] 自定义镜像 tag 显示风险提示。
- [ ] 未覆盖全部 Workload 时禁用提交按钮。
- [ ] 提交成功后刷新 Freight 时间轴。
- [ ] 补充缺少 Workload、重复 Workload、自定义镜像的组件测试。

### Task 10: 发布晋级页

**Files:**
- Modify: 发布晋级页面和 Promotion API hooks。

- [ ] 展示 Freight 时间轴，按创建时间从左到右排列。
- [ ] 展示 `dev`、`test`、`staging`、`prod` Stage 卡片。
- [ ] 每个 Stage 卡片提供 `发布` 按钮。
- [ ] 点击 Stage 的 `发布` 后调用 eligible-freights API。
- [ ] 可发布 Freight 点亮，不可发布 Freight 禁用或置灰。
- [ ] 选择 Freight 后展示确认弹窗。
- [ ] 确认弹窗列出所有 Workload 和镜像版本。
- [ ] prod 发布展示审批人数、审批人范围和禁止自审批提示。
- [ ] 补充 Stage 选择、Freight 高亮、prod 审批提示的测试。

## 6. 验证计划

- [ ] `go test ./...`
- [ ] `web/console` 执行 `npm test -- --coverage`
- [ ] `web/console` 执行 `npm run build`
- [ ] 发布晋级页面 mock API 端到端测试通过。
- [ ] 至少一个 Application 包含两个 Workload：一个流水线产物镜像，一个自定义镜像。
- [ ] 创建 Freight 缺少任一启用 Workload 时被后端拒绝。
- [ ] Freight 发布到 dev 后，GitOps values 中两个 Workload 镜像均更新。
- [ ] prod 发布进入审批，且禁止发起人自审批。

## 7. 迁移策略

- 现有单镜像 Application 迁移为一个默认 Workload。
- 默认 Workload 名称可以沿用 Application name。
- 现有 Release、BuildArtifact、FreightItem 通过迁移脚本补齐 `workload_id`。
- 迁移后旧 Freight 应保持可读和可回滚。
- 新建 Freight 统一使用完整 Workload 覆盖规则。

## 8. 文档同步

实现过程中如 API、字段或交互发生变化，必须同步更新：

- `doc/需求.md`
- `doc/需求文档.md`
- `doc/总体设计.md`
- `doc/模块划分.md`
- `doc/UI风格.md`
- `doc/验收清单.md`
