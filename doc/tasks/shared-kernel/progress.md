# shared-kernel 进度

- [x] shared-kernel 模块完成
- [x] 基础类型完成
- [x] 错误模型完成
- [x] 事件基础结构完成
- [x] 测试完成
- [x] 与其他模块无反向业务依赖

## 本次实现记录

- 初始化 Go module：`github.com/shareinto/paas`。
- 新增启动入口：`cmd/paas-server`、`cmd/paas-agent`。
- 新增 shared-kernel：统一 `ID`、`IDGenerator`、分页结构、错误码与 HTTP 映射、`DomainEvent`、`Clock`、状态枚举校验、fake clock、fake id generator。
- 新增 MySQL platform：MySQL 8.0+ 连接配置、utf8mb4/collation DSN、迁移接口、事务接口。
- 2026-05-31 更新：新增 `repository_snapshots` 迁移和 `SnapshotStore`，用于 `PAAS_REPOSITORY_DRIVER=mysql` 第一阶段业务仓储持久化。
- 测试命令：`go test ./... --% -coverprofile=coverage.out`，`go tool cover --% -func=coverage.out`。
- 覆盖率结果：总体 95.1%，`internal/shared` 98.5%，`internal/platform/database` 92.5%。
- 真实 MySQL 集成测试已通过；通过 `doc/中间件部署.md` 中的 SSH 信息读取远端 `.env` 后注入 `MYSQL_*` 环境变量，覆盖迁移 up/down、事务提交和事务回滚路径。mock 测试同时覆盖错误路径。
