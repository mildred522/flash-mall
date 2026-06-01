import { useRef } from 'react';
import { Button, Tag, message, Popconfirm } from 'antd';
import { ProTable, type ActionType, type ProColumns } from '@ant-design/pro-components';
import { authed } from '@flash-mall/shared';
import { STATUS_MAP, formatPriceFen } from '@flash-mall/shared';
import type { AdminOrderListItem, AdminOrderListResp, ActionResp } from '@flash-mall/shared';

const statusColors: Record<string, string> = {
  pending: 'orange', paid: 'blue', closed: 'default', shipped: 'purple',
  completed: 'green', refund: 'orange', refunded: 'red',
};

export default function OrdersPage() {
  const actionRef = useRef<ActionType>();

  const handleShip = async (orderId: string) => {
    const res = await authed<ActionResp>('/api/admin/orders/ship', { method: 'POST', jsonBody: { order_id: orderId } });
    if (res.ok) { message.success('发货成功'); actionRef.current?.reload(); }
    else message.error('发货失败');
  };

  const handleRefund = async (orderId: string) => {
    const res = await authed<ActionResp>('/api/admin/orders/refund', { method: 'POST', jsonBody: { order_id: orderId } });
    if (res.ok) { message.success('退款成功'); actionRef.current?.reload(); }
    else message.error('退款失败');
  };

  const columns: ProColumns<AdminOrderListItem>[] = [
    { title: '订单号', dataIndex: 'order_id', ellipsis: true, width: 200 },
    { title: '用户ID', dataIndex: 'user_id', width: 100 },
    { title: '商品', dataIndex: 'product_name', ellipsis: true },
    { title: '数量', dataIndex: 'amount', width: 80, search: false },
    {
      title: '状态', dataIndex: 'status', width: 100, search: false,
      render: (_, row) => {
        const st = STATUS_MAP[row.status];
        return st ? <Tag color={statusColors[st.cls] || 'default'}>{st.text}</Tag> : <Tag>未知</Tag>;
      },
      valueEnum: {
        '-1': { text: '全部' },
        '0': { text: '待支付' }, '1': { text: '已支付' }, '2': { text: '已关闭' },
        '3': { text: '已发货' }, '4': { text: '已收货' }, '5': { text: '退款中' }, '6': { text: '已退款' },
      },
    },
    {
      title: '金额', dataIndex: 'payable_amount_fen', width: 120, search: false,
      render: (_, row) => `¥${formatPriceFen(row.payable_amount_fen)}`,
    },
    { title: '下单时间', dataIndex: 'create_time', width: 180, search: false },
    {
      title: '操作', width: 180, search: false,
      render: (_, row) => (
        <>
          {row.status === 1 && (
            <Popconfirm title="确认发货？" onConfirm={() => handleShip(row.order_id)}>
              <Button type="link" size="small">发货</Button>
            </Popconfirm>
          )}
          {(row.status === 0 || row.status === 1) && (
            <Popconfirm title="确认退款？" onConfirm={() => handleRefund(row.order_id)}>
              <Button type="link" size="small" danger>退款</Button>
            </Popconfirm>
          )}
        </>
      ),
    },
  ];

  return (
    <ProTable<AdminOrderListItem>
      actionRef={actionRef}
      columns={columns}
      rowKey="order_id"
      request={async (params) => {
        const query = new URLSearchParams();
        query.set('page', String(params.current || 1));
        query.set('page_size', String(params.pageSize || 20));
        if (params.status !== undefined && params.status !== '-1') query.set('status', String(params.status));
        if (params.user_id) query.set('user_id', String(params.user_id));
        if (params.order_id) query.set('order_id', params.order_id);

        const res = await authed<AdminOrderListResp>(`/api/admin/orders?${query}`);
        return {
          data: res.ok ? res.data.items || [] : [],
          total: res.ok ? res.data.total : 0,
          success: res.ok,
        };
      }}
      search={{ labelWidth: 'auto' }}
      pagination={{ defaultPageSize: 20 }}
    />
  );
}
