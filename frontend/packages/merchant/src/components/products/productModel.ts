import type { AdminMutationResp } from '@flash-mall/shared';

export type ProductFormValues = {
  name: string;
  image_url?: string;
  origin_price_fen: number;
  sale_price_fen: number;
  supplier_id: number;
  stock_available?: number;
  status: number;
};

export type StockFormValues = {
  delta: number;
  bucket_idx: number;
};

export function productMutationError(data: AdminMutationResp): string {
  if (data.error === 'sale_price_fen must be <= origin_price_fen') return '售价不能高于原价';
  if (data.error === 'active supplier not found') return '供应商不可用';
  return data.error || '';
}
