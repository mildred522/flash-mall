include "common.thrift"

namespace go flashmall.order

struct OrderItemDTO {
  1: i64 product_id,
  2: string product_name,
  3: i64 quantity,
  4: i64 price_cent,
}

struct OrderDTO {
  1: string order_id,
  2: i64 user_id,
  3: i64 merchant_id,
  4: i32 status,
  5: i64 total_cent,
  6: list<OrderItemDTO> items,
  7: optional string created_at,
}

struct CreateOrderRequest {
  1: common.RequestMeta meta,
  2: i64 user_id,
  3: i64 product_id,
  4: i64 quantity,
  5: optional string client_token,
}

struct CreateOrderResponse {
  1: string order_id,
}

struct GetOrderRequest {
  1: common.RequestMeta meta,
  2: string order_id,
}

struct GetOrderResponse {
  1: OrderDTO order,
}

struct ListOrdersRequest {
  1: common.RequestMeta meta,
  2: common.PageRequest page,
  3: optional i64 user_id,
  4: optional i64 merchant_id,
  5: optional i32 status,
}

struct ListOrdersResponse {
  1: list<OrderDTO> items,
  2: common.PageResult page,
}

struct OrderActionRequest {
  1: common.RequestMeta meta,
  2: string order_id,
  3: optional string reason,
}

service OrderService {
  CreateOrderResponse CreateOrder(1: CreateOrderRequest req) throws (1: common.BizException biz),
  GetOrderResponse GetOrder(1: GetOrderRequest req) throws (1: common.BizException biz),
  ListOrdersResponse ListOrders(1: ListOrdersRequest req) throws (1: common.BizException biz),
  common.Empty PayOrder(1: OrderActionRequest req) throws (1: common.BizException biz),
  common.Empty CancelOrder(1: OrderActionRequest req) throws (1: common.BizException biz),
  common.Empty ShipOrder(1: OrderActionRequest req) throws (1: common.BizException biz),
  common.Empty ConfirmReceipt(1: OrderActionRequest req) throws (1: common.BizException biz),
  common.Empty RequestRefund(1: OrderActionRequest req) throws (1: common.BizException biz),
  common.Empty AuditRefund(1: OrderActionRequest req) throws (1: common.BizException biz),
}
