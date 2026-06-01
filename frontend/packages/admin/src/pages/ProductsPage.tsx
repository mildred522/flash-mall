import { useRef } from 'react';
import { Tag } from 'antd';
import { ProTable, type ActionType, type ProColumns } from '@ant-design/pro-components';
import { authed, formatPriceFen } from '@flash-mall/shared';
import type { AdminProductItem, AdminProductListResp } from '@flash-mall/shared';

export default function ProductsPage() {
  const actionRef = useRef<ActionType>();

  const columns: ProColumns<AdminProductItem>[] = [
    { title: '商品ID', dataIndex: 'product_id', width: 100 },
    { title: '名称', dataIndex: 'name', ellipsis: true },
    {
      title: '原价', dataIndex: 'origin_price_fen', width: 120, search: false,
      render: (_, row) => `¥${formatPriceFen(row.origin_price_fen)}`,
    },
    {
      title: '售价', dataIndex: 'sale_price_fen', width: 120, search: false,
      render: (_, row) => `¥${formatPriceFen(row.sale_price_fen)}`,
    },
    { title: '库存', dataIndex: 'stock_available', width: 100, search: false },
    {
      title: '状态', dataIndex: 'status', width: 100, search: false,
      render: (_, row) => row.status === 1 ? <Tag color="green">上架</Tag> : <Tag color="red">下架</Tag>,
    },
  ];

  return (
    <ProTable<AdminProductItem>
      actionRef={actionRef}
      columns={columns}
      rowKey="product_id"
      search={false}
      request={async (params) => {
        const query = new URLSearchParams();
        query.set('page', String(params.current || 1));
        query.set('page_size', String(params.pageSize || 20));

        const res = await authed<AdminProductListResp>(`/api/admin/products?${query}`);
        return {
          data: res.ok ? res.data.items || [] : [],
          total: res.ok ? res.data.total : 0,
          success: res.ok,
        };
      }}
      pagination={{ defaultPageSize: 20 }}
    />
  );
}
