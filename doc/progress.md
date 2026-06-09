# PaaS 平台模块总进度

## 使用说明

- 根级 checklist 用于标记模块整体是否完成。
- 每个模块的详细进度维护在对应的 `doc/tasks/<模块名称>/progress.md`。
- 只有模块自身 `progress.md` 中所有关键项完成后，才勾选这里的模块完成项。

## 模块总进度

- [x] [shared-kernel](tasks/shared-kernel/progress.md)
- [x] [identity-access](tasks/identity-access/progress.md)
- [x] [tenant-project](tasks/tenant-project/progress.md)
- [x] [source-repository](tasks/source-repository/progress.md)
- [x] [application-environment](tasks/application-environment/progress.md)
- [x] [build](tasks/build/progress.md)
- [x] [release-delivery](tasks/release-delivery/progress.md)
- [x] [gitops-deployment](tasks/gitops-deployment/progress.md)
- [x] [cluster-agent](tasks/cluster-agent/progress.md)
- [x] [paas-agent](tasks/paas-agent/progress.md)
- [x] [web-console](tasks/web-console/progress.md)
- [x] [audit](tasks/audit/progress.md)
- [x] [notification](tasks/notification/progress.md)
- [x] [integrations](tasks/integrations/progress.md)

## 测试可用版本验收与试点进度

测试可用版本功能开发已完成，下一阶段目标是把当前开发版推进为可验收、可部署、可内部试点的版本。

- [x] 端到端验收清单完成
- [x] 真实后端 API 与 Web Console 对齐完成
- [x] MySQL 正式迁移文件和启动流程完成
- [x] MySQL 业务仓储驱动切换完成
- [x] Web Console 源码仓库管理页完成
- [x] `paas-server` 和 `paas-agent` 配置样例完成
- [x] 本地和测试环境部署说明完成
- [x] GitLab 源码仓库真实接入配置完成
- [ ] GitLab 真实环境验收完成
- [x] Jenkins 真实接入配置和模板管理完成
- [x] 创建应用选择真实源码仓库，点击构建时按固定名称创建或更新 Jenkins 流水线链路完成
- [ ] Jenkins 真实环境验收完成
- [ ] Argo CD、Kubernetes 和 PaaS Agent 真实集成验证完成
- [x] 凭据、token、secret 脱敏和暴露面检查完成
- [x] 健康检查、配置校验、关键日志和基础指标完成
- [x] 管理员初始化、OIDC Provider 配置和 Agent token 轮换流程完成
- [ ] Java Spring Boot 试点应用跑通
- [ ] Java Tomcat 试点应用跑通
- [ ] 试点阻断问题修复完成

## 测试可用版本测试门禁

- [x] `go test ./... -coverprofile=coverage.out` 通过
- [x] `web/console` 执行 `npm test -- --coverage` 通过
- [x] `web/console` 执行 `npm run e2e` 通过
- [x] `web/console` 执行 `npm run build` 通过
- [ ] 真实集成验收记录完成
