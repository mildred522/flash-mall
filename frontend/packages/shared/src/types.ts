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
  create_time: string;
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

export interface AdminProductItem {
  product_id: number;
  name: string;
  origin_price_fen: number;
  sale_price_fen: number;
  stock_available: number;
  status: number;
}

export interface AdminProductListResp {
  items: AdminProductItem[];
  total: number;
}

export interface AdminUserItem {
  user_id: number;
  display_name: string;
  phone: string;
  role: string;
  create_time: string;
}

export interface AdminUserListResp {
  items: AdminUserItem[];
  total: number;
}

export interface AdminDashboardStats {
  total_orders: number;
  total_revenue_fen: number;
  total_users: number;
  total_products: number;
  pending_orders: number;
  paid_orders: number;
  shipped_orders: number;
  completed_orders: number;
}
