# Hertz + Kitex P0 契约层方案

## 背景

当前项目已经具备用户、商品、订单、退款、商家后台、本地启动、Docker/K8s 和 CI 能力。主要问题不是功能缺失，而是随着业务扩展，`entry-api` 的职责越来越重，库存、订单、退款、商家之间的边界也越来越容易混在一起。

P0 的目标不是替换现有 go-zero 链路，而是先建立后续 Hertz Gateway 和 Kitex RPC 可以共用的契约层。

## P0 范围

P0 只做地基：

- 新增 `idl/`，定义商品、库存、订单、商家服务边界。
- 新增统一业务错误码 `app/common/apperror`。
- 新增统一 API 响应结构 `app/common/apiresponse`。
- 新增身份上下文 `app/common/authctx`。
- 新增链路上下文 `app/common/tracectx`。
- 新增 Kitex 工具检查和生成入口脚本。

P0 不做这些事情：

- 不替换 `entry-api`。
- 不下线 go-zero `order-rpc` / `product-rpc`。
- 不改主下单链路。
- 不生成和接入完整 Kitex 服务实现。
- 不引入 Hertz 运行时入口。

## 服务边界

### ProductService

负责商品资料和上下架状态，不直接承担库存一致性逻辑。

核心接口：

- `GetProduct`
- `ListProducts`
- `CreateProduct`
- `UpdateProductStatus`

### InventoryService

负责库存查询、库存种子、预扣、确认扣减、释放和对账。它是后续第一个建议落地的 Kitex 服务，因为当前项目多次出现 Redis 库存种子缺失和库存恢复问题。

核心接口：

- `GetStock`
- `SeedStock`
- `ReserveStock`
- `ConfirmDeduct`
- `ReleaseStock`
- `ReconcileStock`

### OrderService

负责订单生命周期，不直接拥有商品资料，也不直接散落处理库存补偿。订单服务后续通过 InventoryService 完成库存动作。

核心接口：

- `CreateOrder`
- `GetOrder`
- `ListOrders`
- `PayOrder`
- `CancelOrder`
- `ShipOrder`
- `ConfirmReceipt`
- `RequestRefund`
- `AuditRefund`

### MerchantService

负责商家身份、入驻申请、审核和商家上下文查询。平台后台和商家后台后续通过该服务明确权限边界。

核心接口：

- `GetMerchantMe`
- `ApplyMerchant`
- `ListMerchantApplications`
- `AuditMerchantApply`

## 错误码约定

统一业务错误码定义在 `app/common/apperror`，IDL 中的枚举定义在 `idl/common.thrift`。HTTP 和 RPC 边界必须使用同一组业务语义，避免同一个错误在不同服务里出现不同字符串。

示例：

- `STOCK_INSUFFICIENT`
- `ORDER_STATUS_INVALID`
- `REFUND_NOT_ALLOWED`
- `MERCHANT_NOT_BOUND`

## 响应结构约定

后续 Hertz Gateway 和当前 go-zero API 可以逐步统一为：

```json
{
  "code": "ORDER_STATUS_INVALID",
  "message": "当前订单状态不允许操作",
  "request_id": "req-xxx",
  "data": {}
}
```

P0 只提供 `apiresponse.Response`，不要求一次性改完所有旧接口。

## 身份上下文约定

`authctx.Identity` 是平台用户、管理员、商家的统一调用身份：

- `UserID`
- `Phone`
- `Role`
- `IsAdmin`
- `MerchantID`
- `RequestID`

后续 Hertz middleware 可以把登录态解析成 `Identity`，Kitex 调用时再把必要字段透传到 RPC metadata。

## 链路上下文约定

`tracectx.Trace` 负责框架无关的轻量上下文透传：

- `x-request-id`
- `x-trace-id`
- `x-user-id`
- `x-merchant-id`

这层不会替代现有 `app/common/observability`，而是作为 HTTP header 和 RPC metadata 之间的桥接。

## 后续路线

P1：实现 Kitex InventoryService 骨架，并把商品新增后的库存初始化改为调用库存服务。  
P2：实现 Kitex OrderService 骨架，先迁移创建订单链路中的库存预扣边界。  
P3：新增 Hertz Gateway，先接健康检查、商品查询等低风险接口。  
P4：逐步迁移订单、退款、商家后台接口。  
P5：清理旧 go-zero RPC 和重复逻辑。

## 面试叙述

这一步可以概括为：在正式引入 Hertz 和 Kitex 前，先做契约层治理，用 IDL 明确订单、商品、库存、商家的服务边界，并统一错误码、响应结构、身份上下文和链路上下文。这样后续框架迁移不是推倒重写，而是围绕稳定契约渐进演进。
