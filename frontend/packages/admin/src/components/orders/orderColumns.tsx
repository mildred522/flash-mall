import { Button, Popconfirm, Space } from 'antd';
import type { ProColumns } from '@ant-design/pro-components';
import { formatPriceFen } from '@flash-mall/shared';
import type { AdminOrderListItem } from '@flash-mall/shared';
import { OrderStatusTag } from './orderModel';

type Actions = {
  detail: (orderId: string) => void;
  logs: (orderId: string) => void;
  security: (orderId: string) => void;
  ship: (orderId: string) => void;
  refund: (orderId: string) => void;
  navigate: (path: string, key: 'userId' | 'productId', value: number) => void;
};

export function createOrderColumns(actions: Actions): ProColumns<AdminOrderListItem>[] {
  return [
    { title: '订单号', dataIndex: 'order_id', ellipsis: true, width: 200 },
    { title: '用户ID', dataIndex: 'user_id', width: 100, valueType: 'digit', render: (_, row) => (
      <Button type="link" size="small" onClick={() => actions.navigate('/admin/users', 'userId', row.user_id)}>{row.user_id}</Button>
    ) },
    { title: '商品ID', dataIndex: 'product_id', hideInTable: true, valueType: 'digit' },
    { title: '开始日期', dataIndex: 'created_from', hideInTable: true, valueType: 'date' },
    { title: '结束日期', dataIndex: 'created_to', hideInTable: true, valueType: 'date' },
    { title: '商品', dataIndex: 'product_name', ellipsis: true, render: (_, row) => (
      <Button type="link" size="small" onClick={() => actions.navigate('/admin/products', 'productId', row.product_id)}>
        {row.product_name || row.product_id}
      </Button>
    ) },
    { title: '数量', dataIndex: 'amount', width: 80, search: false },
    { title: '状态', dataIndex: 'status', width: 100,
      render: (_, row) => <OrderStatusTag status={row.status} fallback={row.status_text} />,
      valueEnum: {
        '-1': { text: '全部' }, '0': { text: '待支付' }, '1': { text: '已支付' }, '2': { text: '已关闭' },
        '3': { text: '已发货' }, '4': { text: '已收货' }, '5': { text: '退款中' }, '6': { text: '已退款' },
      } },
    { title: '金额', dataIndex: 'payable_amount_fen', width: 120, search: false,
      render: (_, row) => `¥${formatPriceFen(row.payable_amount_fen)}` },
    { title: '下单时间', dataIndex: 'create_time', width: 180, search: false },
    { title: '操作', width: 300, search: false, render: (_, row) => (
      <Space size={4}>
        <Button type="link" size="small" onClick={() => actions.detail(row.order_id)}>详情</Button>
        <Button type="link" size="small" onClick={() => actions.logs(row.order_id)}>日志</Button>
        <Button type="link" size="small" onClick={() => actions.security(row.order_id)}>安全</Button>
        {row.status === 1 && (
          <Popconfirm title="确认发货？" onConfirm={() => actions.ship(row.order_id)}>
            <Button type="link" size="small">发货</Button>
          </Popconfirm>
        )}
        {(row.status === 0 || row.status === 1) && (
          <Popconfirm title="确认退款？" onConfirm={() => actions.refund(row.order_id)}>
            <Button type="link" size="small" danger>退款</Button>
          </Popconfirm>
        )}
      </Space>
    ) },
  ];
}
