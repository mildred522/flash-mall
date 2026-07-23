import type { AdminMutationResp } from './common';

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
