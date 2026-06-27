# Trading Loop V3: 订单状态机可靠化设计

## 背景

Flash-Mall 已经具备可演示的交易主链路：认证、下单、DTM SAGA、Redis 预扣、库存分桶、延迟关单、支付回调校验、订单详情、Outbox/RabbitMQ、K8s 部署和压测证据链。当前主要短板不是缺少新页面，而是交易生命周期的工程一致性不足。

本轮迭代将项目从“下单链路可靠”推进到“订单状态全生命周期可靠”。重点是统一接口契约和迁移源，将支付、发货、收货、退款等状态变更收敛到 order-rpc 的状态机，并让所有关键状态变更都有状态日志、Outbox 事件、测试和可验证对账。

## 当前问题

1. `app/entry/api/entry.api` 与实际 `routes.go`、`types.go` 不同步，后续 goctl 生成或接口维护存在破坏风险。
2. `app/entry/api/desc/order.sql` 与 `scripts/k8s/init-db.sql` 不同步，订单库真实结构和描述源分裂。
3. 支付、发货、收货、退款主要在 entry-api 直接改库，和下单主链路的 RPC/SAGA/Outbox 成熟度不一致。
4. 退款库存归还是 best-effort RPC，缺少可靠重试、异常状态和对账证据。
5. 用户端可触发发货，后台端也可发货，角色边界不符合真实电商流程。
6. Outbox 目前主要覆盖 `order.created`，没有覆盖支付、发货、收货、退款等关键状态变更。

## 目标

本迭代交付以下能力：

1. 统一订单 API 契约和订单库迁移描述，降低后续生成和部署风险。
2. 在 order-rpc 中建立统一订单状态机，集中处理状态转移、幂等、状态日志和事件写入。
3. 将 entry-api 生命周期接口改为调用 order-rpc，API 层只负责鉴权、参数解析和响应转换。
4. 扩展 Outbox 事件，覆盖订单关键生命周期。
5. 将待支付取消和已支付退款拆成两条路径：取消订单负责关闭并释放库存，退款流程负责申请、审批和可靠归还库存。
6. 调整 shop/admin 前端角色边界，使演示流程符合业务语义。
7. 补齐单元测试、集成测试入口和一致性验证脚本。

## 非目标

1. 不新增独立支付服务。本轮仍使用现有 `payment_order` 和 mock 支付回调模型。
2. 不替换 DTM、RabbitMQ、Redis 或 go-zero。
3. 不重写整个前端，只做角色边界、按钮和调用路径收敛。
4. 不引入复杂财务结算、真实第三方支付退款或物流系统。

## 状态机设计

订单状态继续沿用当前数值：

| 状态 | 含义 |
| --- | --- |
| 0 | 待支付 |
| 1 | 已支付 |
| 2 | 已关闭 |
| 3 | 已发货 |
| 4 | 已收货/已完成 |
| 5 | 退款申请中 |
| 6 | 已退款 |
| 7 | 退款异常 |

合法状态转移：

| 操作 | from | to | 发起方 |
| --- | --- | --- | --- |
| 支付成功 | 0 | 1 | 用户或支付回调 |
| 超时关单 | 0 | 2 | 系统任务 |
| 发货 | 1 | 3 | 管理员 |
| 确认收货 | 3 | 4 | 用户 |
| 取消订单 | 0 | 2 | 用户 |
| 申请退款 | 1 | 5 | 用户 |
| 审批退款通过 | 5 | 6 | 管理员 |
| 退款归还库存失败 | 5 | 7 | 系统 |
| 退款异常重试成功 | 7 | 6 | 管理员或系统 |

允许操作：

| 当前状态 | 允许操作 |
| --- | --- |
| 0 待支付 | 支付、取消、系统超时关单 |
| 1 已支付 | 发货、申请退款 |
| 2 已关闭 | 无用户操作，可重放库存补偿 |
| 3 已发货 | 确认收货 |
| 4 已完成 | 本轮不支持退款 |
| 5 退款申请中 | 后台审批退款 |
| 6 已退款 | 无操作 |
| 7 退款异常 | 后台重试审批退款 |

约束：

1. 所有状态转移必须使用 compare-and-set 条件更新，避免并发覆盖。
2. 所有成功转移必须写入 `order_status_log`。
3. 所有成功转移必须写入 `order_outbox`。
4. 幂等重复请求如果目标状态已达成，应返回成功。
5. 非法转移返回 `FailedPrecondition`。
6. 权限不匹配返回 `PermissionDenied`。
7. 退款异常不是终态，后台可以重试审批。重试成功转为 `refunded`，重试失败保持 `refund_failed`。

## RPC 设计

在 `app/order/rpc/order.proto` 增加生命周期方法：

1. `PayOrder`
2. `ShipOrder`
3. `ConfirmReceipt`
4. `CancelOrder`
5. `RequestRefund`
6. `ApproveRefund`

请求中包含：

1. `order_id`
2. `operator_id`
3. `operator_role`
4. `reason`
5. `event_id` 或 `request_id`，用于幂等和事件追踪

返回中包含：

1. `order_id`
2. `status`
3. `status_text`

entry-api 对应 handler 只做：

1. 解析 JWT 身份。
2. 区分用户和管理员角色。
3. 调用 order-rpc。
4. 将 gRPC 错误转换为 HTTP 错误。

## Outbox 设计

扩展当前 `order_outbox` 写入逻辑，新增通用 helper：

1. `InsertOrderEventOutbox(tx, eventType, orderID, payload)`
2. 保留 `InsertOrderCreatedOutbox` 作为兼容包装。

事件类型：

1. `order.created`
2. `order.paid`
3. `order.closed`
4. `order.cancelled`
5. `order.shipped`
6. `order.completed`
7. `order.refund.requested`
8. `order.refunded`
9. `order.refund.failed`

事件 payload 至少包含：

1. `event_id`
2. `event_type`
3. `order_id`
4. `user_id`
5. `from_status`
6. `to_status`
7. `operator_id`
8. `operator_role`
9. `occurred_at`

## 取消与退款可靠化设计

用户取消订单接口只处理待支付订单：

1. 验证订单归属。
2. 只允许 `pending_payment` 状态取消。
3. 本地事务中 compare-and-set 更新订单状态为 `closed`，写入状态日志和 `order.cancelled` 事件。
4. 将订单重新放入现有延迟关单/补偿路径，复用 `CloseOrderJob` 对 closed 订单的幂等库存归还能力。
5. 如果库存归还失败，由现有 processing/retry/DLQ 机制继续重试和暴露。

用户退款接口只执行申请：

1. 验证订单归属。
2. 只允许 `paid` 状态申请退款。
3. 订单状态转为 `refund_requested`。
4. 写入状态日志和 `order.refund.requested` 事件。

后台审批退款接口执行真实退款：

1. 验证管理员权限。
2. 允许 `refund_requested` 或 `refund_failed` 状态审批。
3. 先调用 product-rpc `RevertStock` 和 order-rpc `PreDeductRollback` 归还库存与 Redis 预扣；两个操作必须基于订单 ID 幂等。
4. 库存归还成功后，本地事务中 compare-and-set 将订单从 `refund_requested` 或 `refund_failed` 更新为 `refunded`，更新支付单状态，写状态日志和 `order.refunded` 事件。
5. 如果库存归还失败，本地事务中 compare-and-set 将订单置为 `refund_failed`，写 `order.refund.failed` 事件，供后台和对账脚本发现。
6. 如果库存归还成功但最终 DB 状态更新失败，重试审批时库存归还不会重复加库存，后续只需完成状态转移。

说明：

1. 本轮不新增真实 payment-rpc，因此退款审批仍由 order-rpc 编排。
2. 库存归还依赖 `stock_log(order_id,type)` 幂等约束。
3. 后续可将审批退款升级为 DTM SAGA；本轮先以可测试、可重试的 order-rpc 编排完成可靠闭环。
4. 退款异常 `refund_failed` 必须在 admin 端可见，并允许后台再次触发审批重试。

## 契约与迁移治理

本轮同步以下文件：

1. `app/entry/api/entry.api`
2. `app/entry/api/internal/types/types.go`
3. `app/entry/api/internal/handler/routes.go`
4. `app/entry/api/desc/order.sql`
5. `scripts/k8s/init-db.sql`

原则：

1. `entry.api` 必须描述当前实际对外接口。
2. `desc/order.sql` 必须包含当前订单库核心表结构。
3. `init-db.sql` 继续保留幂等初始化能力。
4. 不在生成文件中做不必要重排，避免扩大 diff。

## 前端调整

shop：

1. 保留下单、支付、取消待支付订单、确认收货、申请退款。
2. 移除用户发货入口。
3. 待支付订单展示取消按钮，已支付订单展示申请退款按钮。

admin：

1. 保留发货。
2. 将退款操作改为审批退款。
3. 展示退款申请中和退款异常状态。
4. 对退款异常订单提供重试审批入口。

## 测试设计

后端测试：

1. 状态机合法转移测试。
2. 非法转移测试。
3. 幂等重复请求测试。
4. 用户订单归属测试。
5. 用户取消订单释放库存测试。
6. 管理员发货和审批退款测试。
7. 退款库存归还失败时的异常状态测试。
8. Outbox 事件写入测试。

前端验证：

1. `npm run build:shop`
2. `npm run build:admin`

Go 验证：

1. `go test ./app/order/rpc/internal/logic -count=1`
2. `go test ./app/entry/api/internal/handler -count=1`
3. `go test ./app/entry/api/... ./app/order/rpc/... ./app/product/rpc/... -count=1`

可选端到端验证：

1. 本地启动依赖和服务。
2. 下单、支付、后台发货、用户收货、用户申请退款、后台审批退款。
3. 查询 `order_status_log` 和 `order_outbox`，确认状态链和事件链完整。

## 实施顺序

1. 契约与 SQL 基线同步。
2. 新增 order-rpc 状态机 helper 和生命周期 RPC 方法。
3. 修改 entry-api 生命周期 handler 调用 RPC。
4. 扩展 Outbox helper 和事件写入。
5. 重构取消订单、退款申请和审批退款。
6. 调整 shop/admin 前端角色边界。
7. 补测试并运行验证命令。

## 风险与缓解

| 风险 | 缓解 |
| --- | --- |
| API/RPC 重构引发回归 | 先补状态机单测，再替换 API 调用路径 |
| 退款流程复杂导致实现过大 | 先落用户申请和后台审批，再处理异常对账 |
| SQL 描述同步造成误改 | `init-db.sql` 保持幂等，`desc/order.sql` 只作为结构基线 |
| Outbox payload 不一致 | 使用统一 helper 构造事件 |
| 前端角色变化影响演示 | 保留完整端到端路径，只改变操作入口 |

## 成功标准

1. 生命周期接口不再由 entry-api 直接改订单状态。
2. 每次关键状态转移都有 `order_status_log`。
3. 每次关键状态转移都有 `order_outbox` 事件。
4. 用户不能发货，只能确认收货和申请退款。
5. 管理员能发货和审批退款。
6. 取消订单和退款库存归还具备幂等、失败可见和测试覆盖。
7. Go 测试和前端构建通过。
