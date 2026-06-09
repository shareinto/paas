# notification 测试可用版本任务

## 目标

实现构建失败、发布审批、部署失败、Agent 离线等事件的最小通知能力。

## 任务清单

- [x] 定义领域模型：`Notification`、`NotificationTemplate`、`NotificationChannel`。
- [x] 建立通知记录和模板数据表迁移草案。
- [x] 定义 `NotificationSender` port，支持 fake sender。
- [x] 实现通知模板渲染，输入事件 payload 输出通知标题和内容。
- [x] 消费事件：`BuildFailed`、`PromotionCreated`、`PromotionApproved`、`PromotionRejected`、`DeploymentFailed`、`ClusterUnreachable`。
- [x] 实现通知发送任务，支持失败重试和幂等去重。
- [x] 实现通知渠道配置占位，当前测试可用版本可先支持 webhook 或邮件中的一种 fake 实现。
- [x] 提供通知记录查询接口。
- [x] 编写测试：模板渲染、事件触发、失败重试、幂等去重。

## 完成标准

- [x] 通知模块不修改业务主数据。
- [x] 关键失败和审批事件可触发通知。
- [x] fake sender 支持模块独立测试。
- [x] 测试通过。
