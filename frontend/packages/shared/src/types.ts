export interface ProductCard {
  product_id: number;
  name: string;
  origin_price_fen: number;
  final_price_fen: number;
  promotion_tag: string;
  stock_available: number;
}

export interface CatalogResp {
  items: ProductCard[];
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

export interface LoginResp {
  access_token: string;
  token_type: string;
  expires_at: number;
  user_id: number;
  display_name: string;
  phone: string;
}

export interface MeResp {
  user_id: number;
  display_name: string;
  phone: string;
  role: string;
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
