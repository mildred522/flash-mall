import { Tag } from 'antd';
import type { AdminMutationResp } from '@flash-mall/shared';

export type SupplierFormValues = { name: string; status: number };

export function supplierMutationError(data: AdminMutationResp): string {
  return data.error === 'supplier has active products'
    ? '该供应商仍有关联的启用商品，请先下架或迁移商品'
    : data.error || '';
}

export function SupplierStatusTag({ status }: { status: number }) {
  return status === 1 ? <Tag color="green">启用</Tag> : <Tag color="red">停用</Tag>;
}
