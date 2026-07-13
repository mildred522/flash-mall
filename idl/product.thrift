include "common.thrift"

namespace go flashmall.product

struct ProductDTO {
  1: i64 product_id,
  2: string name,
  3: i64 merchant_id,
  4: i64 price_cent,
  5: i64 origin_price_cent,
  6: i64 stock,
  7: i32 status,
  8: optional string image_url,
}

struct GetProductRequest {
  1: common.RequestMeta meta,
  2: i64 product_id,
}

struct GetProductResponse {
  1: ProductDTO product,
}

struct ListProductsRequest {
  1: common.RequestMeta meta,
  2: common.PageRequest page,
  3: optional i64 merchant_id,
  4: optional string keyword,
  5: optional i32 status,
}

struct ListProductsResponse {
  1: list<ProductDTO> items,
  2: common.PageResult page,
}

struct CreateProductRequest {
  1: common.RequestMeta meta,
  2: string name,
  3: i64 merchant_id,
  4: i64 price_cent,
  5: i64 origin_price_cent,
  6: i64 stock,
  7: optional string image_url,
}

struct CreateProductResponse {
  1: i64 product_id,
}

struct UpdateProductStatusRequest {
  1: common.RequestMeta meta,
  2: i64 product_id,
  3: i32 status,
}

service ProductService {
  GetProductResponse GetProduct(1: GetProductRequest req) throws (1: common.BizException biz),
  ListProductsResponse ListProducts(1: ListProductsRequest req) throws (1: common.BizException biz),
  CreateProductResponse CreateProduct(1: CreateProductRequest req) throws (1: common.BizException biz),
  common.Empty UpdateProductStatus(1: UpdateProductStatusRequest req) throws (1: common.BizException biz),
}
