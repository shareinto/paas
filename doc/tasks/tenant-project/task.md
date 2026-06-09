# tenant-project 测试可用版本任务

## 目标

实现租户和项目边界，为其他模块提供资源归属查询和项目级权限作用域。

## 任务清单

- [x] 定义领域模型：`Tenant`、`TenantMember`、`Project`。
- [x] 建立数据表迁移草案：`tenants`、`tenant_members`、`projects`。
- [x] 实现创建 Tenant、查询 Tenant、更新 Tenant 基本信息。
- [x] 实现 TenantMember 添加、移除、查询。
- [x] 实现创建 Project、查询 Project、更新 Project 基本信息。
- [x] 实现项目归属校验：Project 必须属于某个 Tenant。
- [x] 暴露 `TenantQuery`、`ProjectQuery`、`ProjectMembershipQuery` port。
- [x] 在创建和修改接口中接入 `PermissionChecker`。
- [x] 发布 `TenantCreated`、`ProjectCreated` 事件。
- [x] 记录创建租户、创建项目、修改成员权限审计日志。
- [x] 编写单元测试：租户创建、项目创建、项目归属、成员查询。

## 完成标准

- [x] 其他模块可以通过 Project ID 查询 Tenant ID。
- [x] 项目作用域权限校验有稳定输入。
- [x] 租户和项目关键操作有审计日志。
- [x] 单元测试通过。
