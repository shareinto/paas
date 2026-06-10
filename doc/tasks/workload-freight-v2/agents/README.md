# Workload Freight V2 多 Agent 开发提示词

本目录保存可以直接复制给不同 Codex 会话的固定提示词。适用于当前 Codex 不支持 subagent，但用户可以打开多个 Codex 会话并让它们在不同 git worktree 中并行工作的场景。

## 推荐目录和分支

在主仓库根目录执行：

```bash
git worktree add ../paas-workload-backend -b feature/workload-v2-backend
git worktree add ../paas-release-freight -b feature/workload-v2-release-freight
git worktree add ../paas-gitops -b feature/workload-v2-gitops
git worktree add ../paas-web-workload -b feature/workload-v2-web-workload
git worktree add ../paas-web-promotion -b feature/workload-v2-web-promotion
```

每个 Codex 会话只进入自己的 worktree，不要多个 Agent 共用同一个工作目录。

## 启动顺序

1. 先在主仓库或单独会话使用 `00-coordinator.md`。
2. 先启动 `01-agent-workload-backend.md`，完成 Workload 后端基础。
3. Workload 后端基础稳定后，启动 `02-agent-release-freight.md`。
4. 后端 Freight 结构稳定后，启动 `03-agent-gitops-deployment.md`。
5. 后端 API 可用后，启动 `04-agent-web-workload.md` 和 `05-agent-web-promotion.md`。
6. 每个分支完成后，用 `06-agent-reviewer.md` 对对应分支做审查。

推荐合并顺序：

```text
01 Workload 后端
02 Release/Freight
03 GitOps Deployment
04 Web Workload
05 Web Promotion
集成修正
```

## 给每个 Agent 的启动语句

Agent 01：

```text
请严格按照仓库内 `doc/tasks/workload-freight-v2/agents/01-agent-workload-backend.md` 执行。你只负责这个文件定义的任务范围。先阅读要求的文档，再实现、测试并汇报结果。
```

Agent 02：

```text
请严格按照仓库内 `doc/tasks/workload-freight-v2/agents/02-agent-release-freight.md` 执行。你只负责这个文件定义的任务范围。先阅读要求的文档，再实现、测试并汇报结果。
```

Agent 03：

```text
请严格按照仓库内 `doc/tasks/workload-freight-v2/agents/03-agent-gitops-deployment.md` 执行。你只负责这个文件定义的任务范围。先阅读要求的文档，再实现、测试并汇报结果。
```

Agent 04：

```text
请严格按照仓库内 `doc/tasks/workload-freight-v2/agents/04-agent-web-workload.md` 执行。你只负责这个文件定义的任务范围。先阅读要求的文档，再实现、测试并汇报结果。
```

Agent 05：

```text
请严格按照仓库内 `doc/tasks/workload-freight-v2/agents/05-agent-web-promotion.md` 执行。你只负责这个文件定义的任务范围。先阅读要求的文档，再实现、测试并汇报结果。
```

Reviewer：

```text
请严格按照仓库内 `doc/tasks/workload-freight-v2/agents/06-agent-reviewer.md` 执行。你只做代码审查，不直接修改代码。请输出阻断问题、重要问题、建议和测试缺口。
```

## 通用约束

- 所有 Agent 必须遵守根目录 `AGENTS.md`。
- 所有用户可见文案必须使用中文。
- 普通用户不能直接访问 Jenkins UI、Argo CD UI、部署清单仓库或 kubeconfig。
- PaaS 控制面不直接调用 Kubernetes API Server。
- 不要在日志、响应、测试快照或文档中暴露 token、secret、password、kubeconfig。
- 不要做任务外重构。
- 如果发现文档与实现冲突，先记录冲突点，不要自行扩大范围。
