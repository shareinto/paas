# identity-access 测试可用版本任务

## 目标

实现用户、身份、角色、权限、RoleBinding、Group、ServiceAccount 和 AccessToken 的最小 RBAC 能力，并支持 OIDC 登录和平台自主创建本地用户两种身份来源。

## 任务清单

- [x] 定义领域模型：`User`、`Identity`、`Group`、`GroupMember`、`Role`、`Permission`、`RoleBinding`、`ServiceAccount`、`AccessToken`。
- [x] 定义 Identity Provider 枚举：`local`、`oidc`。
- [x] 定义本地用户凭据模型，密码使用强哈希存储，不允许明文保存。
- [x] 定义 OIDC Provider 配置模型，包含 issuer、client_id、client_secret 引用、scope、redirect_uri、启用状态。
- [x] 实现 OIDC Identity 映射规则：`issuer` + `subject` 唯一绑定平台 `User`。
- [x] 定义权限格式 `resource:action`，实现权限字符串校验。
- [x] 定义作用域枚举：`platform`、`tenant`、`project`、`application`、`environment`。
- [x] 定义内置角色：`platform_admin`、`tenant_owner`、`tenant_admin`、`project_admin`、`developer`、`viewer`、`operator`、`prod_approver`、`security_auditor`。
- [x] 建立最小 repository 接口和数据表迁移草案。
- [x] 建立 MySQL 迁移草案：`users`、`identities`、`local_credentials`、`oidc_providers`、`access_tokens`、RBAC 相关表。
- [x] 实现管理员创建本地用户、禁用用户和重置密码用例。
- [x] 实现本地账号密码登录，登录成功后签发 AccessToken/RefreshToken。
- [x] 实现 OIDC 登录发起、callback 校验、User/Identity 查找或创建、Token 签发。
- [x] 实现用户直接 RoleBinding 查询和 Group RoleBinding 查询。
- [x] 实现权限合并逻辑，支持作用域覆盖判断。
- [x] 实现 `PermissionChecker`，输入 subject、resource scope、action，输出允许或拒绝。
- [x] 实现 AccessToken 创建、哈希存储、校验和撤销。
- [x] 提供 HTTP API：本地登录、OIDC 登录发起、OIDC callback、登出、刷新令牌、当前用户、用户管理、角色查询、RoleBinding 管理。
- [x] 为其他模块暴露 `AuthService`、`PermissionChecker`、`SubjectQuery` port。
- [x] 编写单元测试：本地密码校验、OIDC state/nonce 校验、OIDC identity 映射、作用域继承、Group 权限合并、ServiceAccount 权限、无权限拒绝。
- [x] 编写契约测试，确保 OIDC client_secret、平台 token、密码哈希不会返回前端或写入日志。

## 完成标准

- [x] 其他模块可以调用 `PermissionChecker` 做权限前置校验。
- [x] 用户、Group、ServiceAccount 权限路径均可测试。
- [x] 本地用户创建和登录可测试。
- [x] OIDC mock provider 登录可测试。
- [x] AccessToken 不明文保存。
- [x] 密码、OIDC client_secret 和 token 不在 API 响应或日志中泄露。
- [x] 单元测试通过。
