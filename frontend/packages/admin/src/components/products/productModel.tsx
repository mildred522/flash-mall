import { Tag } from 'antd';
import type { AdminMutationResp, AdminProductItem } from '@flash-mall/shared';

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

export function mutationError(data: AdminMutationResp): string {
  if (data.error === 'sale_price_fen must be <= origin_price_fen') return '现价不能高于原价';
  if (data.error === 'active supplier not found') return '请选择启用中的供应商';
  return data.error || '';
}

export function ProductStatusTag({ status }: Pick<AdminProductItem, 'status'>) {
  return status === 1 ? <Tag color="green">上架</Tag> : <Tag color="red">下架</Tag>;
}
