include "common.thrift"

namespace go flashmall.merchant

struct MerchantDTO {
  1: i64 merchant_id,
  2: string name,
  3: string contact_phone,
  4: i32 status,
}

struct MerchantApplicationDTO {
  1: i64 apply_id,
  2: i64 user_id,
  3: string merchant_name,
  4: string contact_phone,
  5: i32 status,
  6: optional i64 merchant_id,
  7: optional string remark,
}

struct GetMerchantMeRequest {
  1: common.RequestMeta meta,
}

struct GetMerchantMeResponse {
  1: MerchantDTO merchant,
}

struct ApplyMerchantRequest {
  1: common.RequestMeta meta,
  2: string merchant_name,
  3: string contact_phone,
  4: optional string description,
}

struct ApplyMerchantResponse {
  1: i64 apply_id,
}

struct ListMerchantApplicationsRequest {
  1: common.RequestMeta meta,
  2: common.PageRequest page,
  3: optional i32 status,
}

struct ListMerchantApplicationsResponse {
  1: list<MerchantApplicationDTO> items,
  2: common.PageResult page,
}

struct AuditMerchantApplyRequest {
  1: common.RequestMeta meta,
  2: i64 apply_id,
  3: bool approve,
  4: string remark,
}

service MerchantService {
  GetMerchantMeResponse GetMerchantMe(1: GetMerchantMeRequest req) throws (1: common.BizException biz),
  ApplyMerchantResponse ApplyMerchant(1: ApplyMerchantRequest req) throws (1: common.BizException biz),
  ListMerchantApplicationsResponse ListMerchantApplications(1: ListMerchantApplicationsRequest req) throws (1: common.BizException biz),
  common.Empty AuditMerchantApply(1: AuditMerchantApplyRequest req) throws (1: common.BizException biz),
}
