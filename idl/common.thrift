namespace go flashmall.common

enum ErrorCode {
  OK = 0,
  INVALID_ARGUMENT = 1001,
  UNAUTHORIZED = 1002,
  FORBIDDEN = 1003,
  NOT_FOUND = 1004,
  CONFLICT = 1005,
  TOO_MANY_REQUESTS = 1006,
  INTERNAL = 1007,
  PRODUCT_NOT_FOUND = 2001,
  PRODUCT_OFF_SHELF = 2002,
  STOCK_NOT_FOUND = 3001,
  STOCK_INSUFFICIENT = 3002,
  STOCK_RESERVE_FAILED = 3003,
  STOCK_RECONCILE_FAILED = 3004,
  ORDER_NOT_FOUND = 4001,
  ORDER_STATUS_INVALID = 4002,
  ORDER_CREATE_FAILED = 4003,
  ORDER_ALREADY_PAID = 4004,
  PAYMENT_STATUS_INVALID = 5001,
  PAYMENT_FAILED = 5002,
  REFUND_NOT_ALLOWED = 6001,
  REFUND_NOT_FOUND = 6002,
  REFUND_STATUS_INVALID = 6003,
  MERCHANT_NOT_FOUND = 7001,
  MERCHANT_NOT_BOUND = 7002,
  MERCHANT_APPLY_PENDING = 7003,
  MERCHANT_APPLY_REJECTED = 7004,
}

struct RequestMeta {
  1: optional string request_id,
  2: optional string trace_id,
  3: optional i64 user_id,
  4: optional i64 merchant_id,
  5: optional string role,
}

struct PageRequest {
  1: optional i64 page = 1,
  2: optional i64 page_size = 20,
}

struct PageResult {
  1: i64 total,
  2: i64 page,
  3: i64 page_size,
}

struct Empty {}

exception BizException {
  1: ErrorCode code,
  2: string message,
  3: optional string request_id,
}
