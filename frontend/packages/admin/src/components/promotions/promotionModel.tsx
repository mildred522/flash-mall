import { Tag } from 'antd';
import type { AdminMutationResp } from '@flash-mall/shared';

export type PromotionFormValues = {
  product_id: number;
  discount_value: number;
  threshold_amount?: number;
  starts_at?: string;
  ends_at?: string;
  status: number;
};

export function promotionMutationError(data: AdminMutationResp): string {
  if (data.error === 'active limited price promotion already exists') {
    return '该商品已有启用中的限时价规则，请先停用原规则';
  }
  if (data.error === 'active limited price promotion window overlaps') {
    return '该商品已有时间重叠的启用限时价规则';
  }
  if (data.error === 'ends_at must be after starts_at') {
    return '结束时间必须晚于开始时间';
  }
  if (data.error === 'discount_value must be <= product sale_price_fen') {
    return '限时价不能高于商品现价';
  }
  return data.error || '';
}

export function promotionTypeText(type: string): string {
  return type === 'LIMITED_PRICE' || type === 'limited_price' ? '限时价' : type || '-';
}

export function PromotionStatusTag({ status }: { status: number }) {
  return status === 1 ? <Tag color="green">启用</Tag> : <Tag color="red">停用</Tag>;
}
