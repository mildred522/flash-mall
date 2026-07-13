# Hertz 支付与退款安全收口设计

## 目标

将旧 `codex/payment-reconciliation-refund-safety` 中仍有价值的业务规则按当前 Hertz + Go-zero RPC + Kitex 架构重写到 Hertz 主线，不合并旧 `order-api`、旧库存写路径或旧页面。

## 边界

- Hertz 负责 HTTP、鉴权、参数转换和响应。
- Go-zero `order-rpc` 负责支付与退款状态机、事务、状态日志和 Outbox。
- Kitex `inventory` 继续独占库存预占、确认和释放命令。
- RabbitMQ 消费者继续处理支付后的非关键投影。

## 支付审计

支付成功事务在更新 `orders`、`payment_order`、`payment_callback_event` 和 `order_outbox` 的同时，新增一条 `pending_payment -> paid` 的 `order_status_log`。重复回调保持业务幂等，不重复状态日志或 Outbox。

## 对账

复用当前 `reconciliation_issue`，增加稳定的 `issue_key` 唯一键。一次扫描覆盖：

1. 订单快照金额与支付金额不一致。
2. 支付成功但订单仍待支付。
3. 订单进入已支付、已发货、已完成或退款阶段，但支付单不是成功。

扫描使用 upsert 重新打开仍存在的问题，并确保并发或重复扫描不会制造重复开放记录。

## 退款 RPC

新增两个订单领域 RPC：

- `RequestRefund`：验证用户归属或管理员身份，锁定订单，创建稳定退款单，更新订单状态，写状态日志与 Outbox；重复请求返回同一退款单。
- `AuditRefund`：锁定退款单，通过时调用 Kitex `ReleaseStock`，随后提交退款成功状态、日志和 Outbox；驳回时恢复申请前订单状态。库存释放失败记录为可重试的退款失败，订单保持待退款。

旧的管理员“一键退款”入口通过先申请、再审核两个 RPC 兼容；任何一步失败后都能从退款队列继续处理。

## 幂等与失败语义

- 退款单 ID 由订单 ID 稳定派生，数据库继续保持一单一退款单。
- 支付和退款事件 ID 稳定，依赖 Outbox 唯一键去重。
- `RequestRefund` 对已申请状态返回成功。
- `AuditRefund` 对相同终态决策返回成功，对相反决策返回冲突。
- Kitex `ReleaseStock` 重复调用安全；数据库提交失败后重试不会重复释放库存。

## 不迁移内容

- `app/order/api` 下的旧 Handler、HTML 和类型。
- Product RPC `RevertStock` 与 Redis 库存回滚。
- 将退款失败写入订单主状态 `7`。
- 旧 `payment_reconciliation_issue` 独立表。

## 验证

先写失败测试，再实现：对账三类问题及去重、支付日志单次写入、退款申请重复调用、审核重复调用、驳回恢复状态、Kitex 释放失败与重试。最终运行格式化、目标包测试、全仓测试、`go vet`、六个服务构建和 GitHub 快速 CI。
