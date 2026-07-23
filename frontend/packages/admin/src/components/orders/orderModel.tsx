import { Tag } from 'antd';
import { STATUS_MAP } from '@flash-mall/shared';
import type { ActionResp, AdminOrderStatusLogResp, OrderDetailResp } from '@flash-mall/shared';

const statusColors: Record<string, string> = {
  pending: 'orange', paid: 'blue', closed: 'default', shipped: 'purple',
  completed: 'green', refund: 'orange', refunded: 'red',
};

export function OrderStatusTag({ status, fallback }: { status: number; fallback?: string }) {
  const current = STATUS_MAP[status];
  return current
    ? <Tag color={statusColors[current.cls] || 'default'}>{current.text}</Tag>
    : <Tag>{fallback || '未知'}</Tag>;
}

export function orderActionError(data: ActionResp | OrderDetailResp | AdminOrderStatusLogResp): string {
  if (data.error === 'order not in paid status' || data.error === 'order is not in paid status') {
    return '订单不是已支付状态，不能发货';
  }
  if (data.error === 'order cannot be refunded' || data.error === 'order cannot be refunded in current status') {
    return '当前订单状态不能退款';
  }
  if (data.error === 'status changed concurrently' || data.error === 'order status changed concurrently') {
    return '订单状态已变化，请刷新后重试';
  }
  if (data.error === 'order not found') return '订单不存在';
  return data.error || '';
}
