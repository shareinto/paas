# integrations 进度

- [x] integrations 模块完成
- [x] GitLab SourceRepository adapter 完成
- [x] GitLab ManifestRepository adapter 完成
- [x] Jenkins adapter 完成
- [x] BuildSpec 参数支持完成
- [x] Registry adapter 占位完成
- [x] fake adapter 完成
- [x] 测试完成

## 说明

- 已实现 GitLab SourceRepository、GitLab ManifestRepository、Jenkins、Registry/Argo CD 占位和 fake adapter，并补充 mock HTTP server 测试。
- 已补齐 GitLab/Jenkins client 层统一重试策略，覆盖 429/502/503/504 等临时不可用状态；adapter 凭据通过配置注入，日志读取会脱敏 token/password/secret。
- 2026-05-31 更新：GitLab SourceRepository adapter 支持 namespace-aware 创建，按 `Root Group -> Tenant Group -> Project Subgroup -> SourceRepository Project` 自动确保 GitLab Group/Subgroup，并在创建 Project 时传入 `namespace_id`。
- 2026-05-31 更新：Jenkins adapter 支持按 `JENKINS_BASE_URL`、`JENKINS_USERNAME`、`JENKINS_TOKEN` 接入真实 Jenkins，创建多级 Folder/Job，使用平台管理员维护的全局构建模板生成流水线；重复确保已存在的 Folder/Job 会按幂等成功处理。
