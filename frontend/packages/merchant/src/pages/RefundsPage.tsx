import { useEffect, useState } from 'react';
import { Button, Table, Tag, message } from 'antd';
import { authed, formatPriceFen } from '@flash-mall/shared';
import type { MerchantRefundItem, MerchantRefundListResp } from '@flash-mall/shared';

export default function RefundsPage() {
  const [items, setItems] = useState<MerchantRefundItem[]>([]);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    setLoading(true);
    const response = await authed<MerchantRefundListResp>('/api/merchant/refunds?page=1&page_size=100&status=-1');
    if (response.ok) setItems(response.data.items || []);
    else message.error('退款列表加载失败');
    setLoading(false);
  };

  useEffect(() => { void load(); }, []);
  return (
    <Table<MerchantRefundItem>
      rowKey="refund_id"
      loading={loading}
      dataSource={items}
      pagination={{ pageSize: 20 }}
      title={() => <div style={{ display: 'flex', justifyContent: 'space-between' }}><strong>本店退款</strong><Button onClick={() => void load()}>刷新</Button></div>}
      columns={[
        { title: '退款号', dataIndex: 'refund_id', width: 200 },
        { title: '订单号', dataIndex: 'order_id', width: 200 },
        { title: '商品ID', dataIndex: 'product_id', width: 100 },
        { title: '用户ID', dataIndex: 'user_id', width: 100 },
        { title: '退款金额', dataIndex: 'refund_amount_fen', width: 120, render: (value: number) => `¥${formatPriceFen(value)}` },
        { title: '状态', dataIndex: 'status_text', width: 110, render: (value: string) => <Tag>{value}</Tag> },
        { title: '原因', dataIndex: 'reason' },
        { title: '申请时间', dataIndex: 'request_time', width: 180 },
      ]}
    />
  );
}
