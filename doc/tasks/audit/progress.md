# audit 进度

- [x] audit 模块完成
- [x] AuditLog 模型完成
- [x] AuditLogger port 完成
- [x] 查询 API 完成
- [x] 关键事件覆盖完成
- [x] BuildSpec 审计完成
- [x] 测试完成

## 说明

- 已实现统一审计日志模型、不可变写入、敏感字段脱敏、条件查询、API 和迁移草案。
- 已新增统一审计桥接 adapter，覆盖 identity-access、tenant-project、source-repository、application-environment、build、release-delivery、gitops-deployment 和 cluster-agent 的 AuditLogger 接口。
- 已补充环境配置和环境密钥变更审计；BuildSpec 审计字段 `build_command`、`artifact_copy_command`、`runtime_base_image`、`artifact_deploy_path` 通过统一 audit 模块测试验证，敏感字段会脱敏。
