export interface ProductCard {
  product_id: number;
  name: string;
  image_url: string;
  origin_price_fen: number;
  final_price_fen: number;
  promotion_tag: string;
  stock_available: number;
  merchant_id?: number;
  merchant_name?: string;
  merchant_logo?: string;
  store_url?: string;
  store_status?: number;
  slot_no?: number;
}

export interface CatalogResp {
  items: ProductCard[];
  total: number;
  page: number;
  page_size: number;
}

export interface MerchantStoreProfile {
  merchant_id: number;
  merchant_name: string;
  logo_url: string;
  banner_url: string;
  description: string;
  version: number;
}

export interface PublicStoreDetail {
  merchant_id: number;
  merchant_name: string;
  logo_url: string;
  banner_url: string;
  description: string;
  status: number;
  product_count: number;
}

export interface StoreProductsResp {
  items: ProductCard[];
  total: number;
  page: number;
  page_size: number;
}

export interface ProductDetailResp {
  item: ProductCard;
  store_products: ProductCard[];
}

export type ShowcaseInvalidReason =
  | 'product_not_found'
  | 'product_inactive'
  | 'merchant_not_found'
  | 'merchant_inactive'
  | 'out_of_stock';

export interface ShowcaseSlot {
  slot_no: number;
  product_id: number;
  empty: boolean;
  valid: boolean;
  invalid_reason?: ShowcaseInvalidReason;
  product?: ProductCard;
}

export interface ShowcaseResp {
  version: number;
  operator_id: number;
  publish_time: string;
  items: ShowcaseSlot[];
}

export interface ShowcaseCandidate {
  product: ProductCard;
  score: number;
  sales_7d: number;
  sales_score: number;
  stock_score: number;
  promotion_score: number;
  freshness_score: number;
  diversity_score: number;
  reasons: string[];
}

export interface ShowcaseCandidatesResp {
  items: ShowcaseCandidate[];
  total: number;
  page: number;
  page_size: number;
}

export interface ShowcasePublishReq {
  expected_version: number;
  items: Array<{ slot_no: number; product_id: number }>;
}

export interface OrderListItem {
  order_id: string;
  product_id: number;
  product_name: string;
  amount: number;
  status: number;
  status_text: string;
  payable_amount_fen: number;
  create_time: string;
}

export interface OrderListResp {
  items: OrderListItem[];
}

export interface OrderDetailResp {
  order_id: string;
  user_id?: number;
  product_id: number;
  product_name: string;
  amount: number;
  status: number;
  status_text: string;
  origin_unit_price_fen: number;
  sale_unit_price_fen: number;
  payable_amount_fen: number;
  discount_amount_fen: number;
  promotion_type: string;
  promotion_tag: string;
  payment_order_id: string;
  payment_status: number;
  payment_status_text: string;
  create_time: string;
  error?: string;
}

export interface CreateOrderResp {
  order_id: string;
  status: string;
  payable_amount_fen: number;
  payment_order_id: string;
}

export interface ActionResp {
  order_id: string;
  status: string;
  error?: string;
}

export interface PaymentIntentResp {
  order_id: string;
  payment_order_id: string;
  out_trade_no: string;
  payable_amount_fen: number;
  status: string;
  qr_url: string;
  expires_at: number;
}

export interface PaymentStatusResp {
  order_id: string;
  payment_order_id: string;
  out_trade_no: string;
  payable_amount_fen: number;
  status: string;
  expires_at: number;
}

export interface LoginResp {
  access_token: string;
  refresh_token?: string;
  token_type: string;
  expires_at: number;
  user_id: number;
  display_name: string;
  phone: string;
}

export interface SendCodeResp {
  sent: boolean;
  expires_at: number;
  debug_code?: string;
}

export interface MeResp {
  user_id: number;
  display_name: string;
  phone: string;
  role: string;
}

export interface MerchantMeItem {
  merchant_id: number;
  name: string;
  role: string;
  status: number;
}

export interface MerchantMeResp {
  items: MerchantMeItem[];
}

export type MerchantApplicationStatus = 0 | 1 | 2;
export type MerchantApplicationStatusText = 'pending' | 'approved' | 'rejected';

export interface MerchantApplicationItem {
  apply_id: number;
  merchant_name: string;
  contact_phone: string;
  status: MerchantApplicationStatus;
  status_text: MerchantApplicationStatusText;
  merchant_id: number;
  audit_remark: string;
  create_time: string;
  audit_time: string;
}

export interface MerchantApplicationResp {
  application: MerchantApplicationItem | null;
}

export interface MerchantApplyResp {
  apply_id: number;
  status: 'pending';
}

export interface AdminMerchantApplicationItem extends MerchantApplicationItem {
  user_id: number;
  operator_id: number;
}

export interface AdminMerchantApplicationListResp {
  items: AdminMerchantApplicationItem[];
  total: number;
  page: number;
  page_size: number;
}

export interface AdminMerchantAuditResp {
  apply_id: number;
  merchant_id: number;
  status: MerchantApplicationStatus;
}

export interface MerchantDashboardStats {
  merchant_id: number;
  order_count: number;
  paid_order_count: number;
  ship_pending_count: number;
  refund_pending_count: number;
  sales_amount_fen: number;
}

export interface MerchantStockChangeItem {
  id: number;
  product_id: number;
  order_id: string;
  change_type: string;
  delta: number;
  before_available: number;
  after_available: number;
  reason: string;
  request_id: string;
  trace_id: string;
  operator_user_id: number;
  operator_merchant_id: number;
  operator_role: string;
  create_time: string;
}

export interface MerchantStockChangeListResp {
  items: MerchantStockChangeItem[];
  total: number;
}

export interface MerchantRefundItem {
  refund_id: string;
  order_id: string;
  payment_order_id: string;
  user_id: number;
  merchant_id: number;
  merchant_name: string;
  product_id: number;
  refund_amount_fen: number;
  status: number;
  status_text: string;
  reason: string;
  audit_remark: string;
  operator_id: number;
  request_time: string;
  audit_time: string;
  finish_time: string;
}

export interface MerchantRefundListResp {
  items: MerchantRefundItem[];
  total: number;
}

export interface SystemHealthResp {
  overall: boolean;
  version: string;
  uptime: string;
  goroutines: number;
  server_time: number;
  dependencies: { name: string; ok: boolean; detail: string }[];
}

export interface ApiResponse<T> {
  ok: boolean;
  status: number;
  data: T;
}

export interface AdminMutationResp {
  ok?: boolean;
  error?: string;
}

export interface SecurityEventItem {
  event_type: string;
  result: string;
  user_id: number;
  subject: string;
  ip: string;
  user_agent: string;
  created_at: number;
}

export interface SecurityEventsRecentResp extends AdminMutationResp {
  items: SecurityEventItem[];
}

// Admin types
export interface AdminOrderListItem {
  order_id: string;
  user_id: number;
  product_id: number;
  product_name: string;
  amount: number;
  status: number;
  status_text: string;
  payable_amount_fen: number;
  create_time: string;
}

export interface AdminOrderListResp {
  items: AdminOrderListItem[];
  total: number;
}

export interface AdminOrderStatusLogItem {
  id: number;
  order_id: string;
  from_status: number;
  from_status_text: string;
  to_status: number;
  to_status_text: string;
  operator_id: number;
  remark: string;
  create_time: string;
}

export interface AdminOrderStatusLogResp extends AdminMutationResp {
  items: AdminOrderStatusLogItem[];
}

export interface AdminProductItem {
  product_id: number;
  name: string;
  image_url: string;
  origin_price_fen: number;
  sale_price_fen: number;
  supplier_id: number;
  supplier_name: string;
  stock_available: number;
  promotion_price_fen: number;
  promotion_type: string;
  promotion_tag: string;
  status: number;
  status_text: string;
}

export interface AdminProductListResp {
  items: AdminProductItem[];
  total: number;
}

export type AdminProductDetailResp = AdminProductItem;

export interface AdminProductCreateReq {
  name: string;
  image_url?: string;
  origin_price_fen: number;
  sale_price_fen: number;
  stock_available?: number;
  supplier_id?: number;
  status?: number;
}

export interface AdminProductCreateResp extends AdminMutationResp {
  product_id: number;
}

export interface AdminProductUpdateReq {
  product_id: number;
  name?: string;
  image_url?: string;
  origin_price_fen?: number;
  sale_price_fen?: number;
  supplier_id?: number;
  status?: number;
}

export interface AdminProductStockAdjustReq {
  product_id: number;
  delta: number;
  bucket_idx?: number;
}

export interface AdminProductStockAdjustResp extends AdminMutationResp {
  product_id: number;
  stock_available: number;
}

export interface AdminSupplierItem {
  supplier_id: number;
  name: string;
  status: number;
  status_text: string;
  product_count: number;
  active_products: number;
}

export interface AdminSupplierListResp {
  items: AdminSupplierItem[];
  total: number;
}

export type AdminSupplierDetailResp = AdminSupplierItem;

export interface AdminSupplierCreateReq {
  name: string;
  status?: number;
}

export interface AdminSupplierCreateResp extends AdminMutationResp {
  supplier_id: number;
}

export interface AdminSupplierUpdateReq {
  supplier_id: number;
  name?: string;
  status?: number;
}

export interface AdminPromotionItem {
  promotion_id: number;
  product_id: number;
  product_name: string;
  origin_price_fen: number;
  sale_price_fen: number;
  type: string;
  discount_value: number;
  threshold_amount: number;
  starts_at: string;
  ends_at: string;
  effect_status: string;
  effect_status_text: string;
  status: number;
  status_text: string;
}

export interface AdminPromotionListResp {
  items: AdminPromotionItem[];
  total: number;
}

export type AdminPromotionDetailResp = AdminPromotionItem;

export interface AdminPromotionCreateReq {
  product_id: number;
  type?: string;
  discount_value: number;
  threshold_amount?: number;
  starts_at?: string;
  ends_at?: string;
  status?: number;
}

export interface AdminPromotionCreateResp extends AdminMutationResp {
  promotion_id: number;
}

export interface AdminPromotionUpdateReq {
  promotion_id: number;
  product_id?: number;
  discount_value?: number;
  threshold_amount?: number;
  starts_at?: string;
  ends_at?: string;
  status?: number;
}

export interface AdminUserItem {
  user_id: number;
  display_name: string;
  phone: string;
  role: string;
  status: number;
  status_text: string;
  create_time: string;
}

export interface AdminUserListResp {
  items: AdminUserItem[];
  total: number;
}

export type AdminUserDetailResp = AdminUserItem;

export interface AdminUserStatusResp extends AdminMutationResp {
  user_id: number;
  status: number;
  status_text: string;
}

export interface AdminDashboardStats {
  total_orders: number;
  total_revenue_fen: number;
  total_users: number;
  total_products: number;
  total_suppliers: number;
  total_promotions: number;
  active_promotions: number;
  low_stock_products: number;
  out_of_stock_products: number;
  pending_orders: number;
  paid_orders: number;
  shipped_orders: number;
  completed_orders: number;
}
