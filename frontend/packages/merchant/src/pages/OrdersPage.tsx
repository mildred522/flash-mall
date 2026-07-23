import { useEffect, useState } from 'react';
import { Button, Image, Popconfirm, Space, Table, Tag, message } from 'antd';
import { authed, formatPriceFen, PRODUCT_IMAGE_FALLBACK_DATA_URI, STATUS_MAP } from '@flash-mall/shared';
import type { ActionResp, AdminOrderListItem, AdminOrderListResp } from '@flash-mall/shared';

export default function OrdersPage() {
  const [orders, setOrders] = useState<AdminOrderListItem[]>([]);
  const [loading, setLoading] = useState(true);

  const loadOrders = async () => {
    setLoading(true);
    const response = await authed<AdminOrderListResp>('/api/merchant/orders?page=1&page_size=100&status=-1');
    if (response.ok) setOrders(response.data.items || []);
    else message.error('订单列表加载失败');
    setLoading(false);
  };

  useEffect(() => { void loadOrders(); }, []);

  const ship = async (orderID: string) => {
    const response = await authed<ActionResp>('/api/merchant/orders/ship', {
      method: 'POST', jsonBody: { order_id: orderID },
    });
    if (!response.ok || response.data.error) {
      message.error(response.data.error || '发货失败');
      return;
    }
    message.success('发货成功');
    await loadOrders();
  };

  return (
    <Table<AdminOrderListItem>
      rowKey="order_id"
      loading={loading}
      dataSource={orders}
      pagination={{ pageSize: 20 }}
      title={() => (
        <div style={{ display: 'flex', justifyContent: 'space-between' }}>
          <strong>本店订单</strong>
          <Button onClick={() => void loadOrders()}>刷新</Button>
        </div>
      )}
      columns={[
        { title: '订单号', dataIndex: 'order_id', width: 220 },
        {
          title: '商品', dataIndex: 'product_name', width: 260,
          render: (_, row) => (
            <Space>
              {row.image_url
                ? <Image width={48} height={48} src={row.image_url} fallback={PRODUCT_IMAGE_FALLBACK_DATA_URI} alt={`${row.product_name} 商品图`} />
                : <span style={{ color: '#999', width: 48 }}>无图</span>}
              <span>{row.product_name || row.product_id}</span>
            </Space>
          ),
        },
        { title: '用户ID', dataIndex: 'user_id', width: 100 },
        { title: '数量', dataIndex: 'amount', width: 80 },
        { title: '金额', dataIndex: 'payable_amount_fen', width: 110, render: (value: number) => `¥${formatPriceFen(value)}` },
        {
          title: '状态', dataIndex: 'status', width: 100,
          render: (value: number, row) => <Tag>{STATUS_MAP[value]?.text || row.status_text}</Tag>,
        },
        { title: '下单时间', dataIndex: 'create_time', width: 180 },
        {
          title: '操作', width: 100,
          render: (_, row) => (
            <Space>
              {row.status === 1 && (
                <Popconfirm title="确认该订单已经完成出库并发货？" onConfirm={() => ship(row.order_id)}>
                  <Button type="link">发货</Button>
                </Popconfirm>
              )}
              {row.status !== 1 && <span style={{ color: '#999' }}>无操作</span>}
            </Space>
          ),
        },
      ]}
    />
  );
}
