export interface OrderListItem {
  order_id: string;
  product_id: number;
  product_name: string;
  image_url: string;
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
  image_url: string;
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
