# identity-access 进度

- [x] identity-access 模块完成
- [x] 领域模型完成
- [x] 本地用户创建和密码登录完成
- [x] OIDC Provider 配置和登录回调完成
- [x] Identity 到 User 映射完成
- [x] RBAC 权限判断完成
- [x] Token 管理完成
- [x] 对外 port 完成
- [x] API 完成
- [x] 凭据脱敏和审计完成
- [x] 测试完成

## 本次实现记录

- 新增 `internal/modules/identityaccess`，包含领域模型、port、MySQL 正式表 repository、服务层、HTTP API handler 和 MySQL 迁移。
- 本地用户密码使用 bcrypt 哈希保存，不保存明文密码。
- AccessToken/RefreshToken 只保存 SHA-256 哈希；登录响应返回一次性明文 token，后续存储和查询均使用哈希。
- OIDC 登录使用 `OIDCVerifier` port 支持 mock provider，按 `issuer` + `subject` 唯一映射平台 `User`。
- RBAC 支持 User、Group、ServiceAccount 三类主体，支持 `platform`、`tenant`、`project`、`application`、`environment` 作用域覆盖。
- API 响应 DTO 不返回 `password_hash`、`token_hash`、`client_secret_ref` 明文或 OIDC client secret。
- 测试命令：`go test ./... --% -coverprofile=coverage.out`，`go tool cover --% -func=coverage.out`。
- 覆盖率结果：总体 91.3%，`internal/modules/identityaccess` 90.0%。
