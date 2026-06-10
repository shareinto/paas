# Workload Freight V2 进度

## 总体状态

- [ ] 未开始
- [ ] 开发中
- [ ] 后端完成
- [ ] 前端完成
- [ ] 联调完成
- [ ] 验收完成

## 任务进度

- [ ] 数据库迁移
- [ ] application-environment 模块 Workload 与环境配置
- [ ] build 模块 Workload 适配
- [ ] release-delivery 模块手动 Freight
- [ ] gitops-deployment 多 Workload values 写入
- [ ] 权限和审计
- [ ] 应用详情导航调整
- [ ] 应用 Workload 页面
- [ ] 创建 Freight 抽屉
- [ ] 发布晋级页类 Kargo 交互
- [ ] 后端测试
- [ ] 前端测试
- [ ] 端到端验收

## 当前结论

- Application 调整为业务交付上下文。
- Workload 是最小可部署单元，一个镜像对应一个 Workload。
- Freight 由用户手动创建，且必须包含所有启用 Workload。
- Stage 发布按钮负责触发可发布 Freight 选择。
- dev/prod 等环境差异由 WorkloadEnvironmentConfig 或环境 values 承载，不进入 Freight。
