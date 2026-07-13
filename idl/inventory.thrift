include "common.thrift"

namespace go flashmall.inventory

struct StockDTO {
  1: i64 product_id,
  2: i64 available,
  3: i64 reserved,
  4: i64 total,
}

struct GetStockRequest {
  1: common.RequestMeta meta,
  2: i64 product_id,
}

struct GetStockResponse {
  1: StockDTO stock,
}

struct BatchGetStockRequest {
  1: common.RequestMeta meta,
  2: list<i64> product_ids,
}

struct BatchGetStockResponse {
  1: list<StockDTO> stocks,
}

struct SeedStockRequest {
  1: common.RequestMeta meta,
  2: i64 product_id,
  3: i64 total,
  4: optional i32 shard_count,
}

struct AdjustStockRequest {
  1: common.RequestMeta meta,
  2: i64 product_id,
  3: i64 delta,
  4: optional i32 bucket_idx,
  5: optional string reason,
}

struct AdjustStockResponse {
  1: StockDTO before,
  2: StockDTO after,
}

struct ReserveStockRequest {
  1: common.RequestMeta meta,
  2: string order_id,
  3: i64 product_id,
  4: i64 quantity,
}

struct ConfirmDeductRequest {
  1: common.RequestMeta meta,
  2: string order_id,
}

struct ReleaseStockRequest {
  1: common.RequestMeta meta,
  2: string order_id,
  3: optional string reason,
}

struct ReconcileStockRequest {
  1: common.RequestMeta meta,
  2: i64 product_id,
}

struct ReconcileStockResponse {
  1: StockDTO before,
  2: StockDTO after,
  3: bool changed,
}

service InventoryService {
  GetStockResponse GetStock(1: GetStockRequest req) throws (1: common.BizException biz),
  BatchGetStockResponse BatchGetStock(1: BatchGetStockRequest req) throws (1: common.BizException biz),
  common.Empty SeedStock(1: SeedStockRequest req) throws (1: common.BizException biz),
  AdjustStockResponse AdjustStock(1: AdjustStockRequest req) throws (1: common.BizException biz),
  common.Empty ReserveStock(1: ReserveStockRequest req) throws (1: common.BizException biz),
  common.Empty ConfirmDeduct(1: ConfirmDeductRequest req) throws (1: common.BizException biz),
  common.Empty ReleaseStock(1: ReleaseStockRequest req) throws (1: common.BizException biz),
  ReconcileStockResponse ReconcileStock(1: ReconcileStockRequest req) throws (1: common.BizException biz),
}
