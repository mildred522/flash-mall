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
