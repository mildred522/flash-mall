import type { AdminMutationResp } from './common';

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
