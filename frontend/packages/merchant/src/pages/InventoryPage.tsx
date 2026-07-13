import { useEffect, useState } from 'react';
import { Button, Table, Tag, message } from 'antd';
import { authed } from '@flash-mall/shared';
import type { MerchantStockChangeItem, MerchantStockChangeListResp } from '@flash-mall/shared';

export default function InventoryPage() {
  const [items, setItems] = useState<MerchantStockChangeItem[]>([]);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    setLoading(true);
    const response = await authed<MerchantStockChangeListResp>('/api/merchant/inventory/stock-changes?page=1&page_size=100');
    if (response.ok) setItems(response.data.items || []);
    else message.error('库存流水加载失败');
    setLoading(false);
  };

  useEffect(() => { void load(); }, []);
  return (
    <Table<MerchantStockChangeItem>
      rowKey="id"
      loading={loading}
      dataSource={items}
      pagination={{ pageSize: 20 }}
      title={() => <div style={{ display: 'flex', justifyContent: 'space-between' }}><strong>本店库存流水</strong><Button onClick={() => void load()}>刷新</Button></div>}
      columns={[
        { title: '商品ID', dataIndex: 'product_id', width: 100 },
        { title: '类型', dataIndex: 'change_type', width: 120, render: (value: string) => <Tag>{value}</Tag> },
        { title: '变化量', dataIndex: 'delta', width: 100, render: (value: number) => <span style={{ color: value >= 0 ? '#389e0d' : '#cf1322' }}>{value > 0 ? `+${value}` : value}</span> },
        { title: '变化前', dataIndex: 'before_available', width: 90 },
        { title: '变化后', dataIndex: 'after_available', width: 90 },
        { title: '原因', dataIndex: 'reason' },
        { title: '请求ID', dataIndex: 'request_id', width: 190 },
        { title: '时间', dataIndex: 'create_time', width: 180 },
      ]}
    />
  );
}
