import type { AdminMutationResp } from './common';

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
