# shared-kernel 测试可用版本任务

## 目标

提供所有模块可复用的最小基础类型和约定，避免业务模块互相复制通用结构。

## 任务清单

- [x] 初始化 Go 后端工程基础结构，包含 `cmd/paas-server`、`cmd/paas-agent`、`internal/` 目录。
- [x] 初始化 MySQL database platform 包，目标数据库为 MySQL 8.0+。
- [x] 提供 MySQL 连接配置，默认使用 InnoDB 和 utf8mb4。
- [x] 提供迁移执行接口，迁移 SQL 以 MySQL 8.0 方言为准。
- [x] 提供事务接口，支持业务模块在 application service 中声明事务边界。
- [x] 定义统一 ID 类型和 ID 生成接口，支持后续替换实现。
- [x] 定义分页请求和分页响应结构。
- [x] 定义统一错误码、业务错误类型和 HTTP 错误映射约定。
- [x] 定义 `DomainEvent` 基础结构，包含 `event_id`、`event_type`、`occurred_at`、`payload`。
- [x] 定义 `Clock` 接口和默认实现，便于测试中固定时间。
- [x] 定义通用状态枚举校验工具，不包含具体业务状态。
- [x] 定义基础测试工具，支持 fake clock、fake id generator。
- [x] 编写 shared-kernel 单元测试，覆盖错误映射、分页、事件基础字段。
- [x] 编写 MySQL platform 集成测试，覆盖连接、迁移 up/down、事务提交和事务回滚。

## 完成标准

- [x] 业务模块可以只依赖 shared-kernel 获取基础类型。
- [x] shared-kernel 不包含业务 service、repository、外部系统 client。
- [x] MySQL 连接、迁移和事务接口可被业务模块复用。
- [x] 单元测试通过。
