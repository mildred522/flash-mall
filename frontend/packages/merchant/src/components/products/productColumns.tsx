import { Button, Image, Space, Tag } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { formatPriceFen } from '@flash-mall/shared';
import type { AdminProductItem } from '@flash-mall/shared';

type ProductActions = {
  edit: (product: AdminProductItem) => void;
  stock: (product: AdminProductItem) => void;
};

export function createMerchantProductColumns(actions: ProductActions): ColumnsType<AdminProductItem> {
  return [
    { title: '商品ID', dataIndex: 'product_id', width: 100 },
    {
      title: '图片', dataIndex: 'image_url', width: 88,
      render: (value: string, row) => value
        ? <Image width={48} height={48} src={value} alt={`${row.name} 商品图`} style={{ objectFit: 'cover', borderRadius: 6 }} />
        : <span style={{ color: '#999' }}>无图</span>,
    },
    { title: '名称', dataIndex: 'name' },
    { title: '供应商ID', dataIndex: 'supplier_id', width: 110 },
    { title: '售价', dataIndex: 'sale_price_fen', width: 110, render: (value: number) => `¥${formatPriceFen(value)}` },
    { title: '库存', dataIndex: 'stock_available', width: 90 },
    {
      title: '状态', dataIndex: 'status', width: 90,
      render: (value: number) => value === 1 ? <Tag color="green">上架</Tag> : <Tag>下架</Tag>,
    },
    {
      title: '操作', width: 180,
      render: (_, row) => (
        <Space>
          <Button type="link" onClick={() => actions.edit(row)}>编辑</Button>
          <Button type="link" onClick={() => actions.stock(row)}>调整库存</Button>
        </Space>
      ),
    },
  ];
}
