import { Button, Popconfirm, Space, Tag } from 'antd';
import type { ProColumns } from '@ant-design/pro-components';
import { formatPriceFen } from '@flash-mall/shared';
import type { AdminProductItem, AdminSupplierItem } from '@flash-mall/shared';
import ProductThumbnail from './ProductThumbnail';
import { ProductStatusTag } from './productModel';

type Actions = {
  detail: (productId: number) => void;
  edit: (product: AdminProductItem) => void;
  stock: (product: AdminProductItem) => void;
  security: (productId: number) => void;
  navigate: (path: string, productId: number) => void;
  status: (product: AdminProductItem) => void;
};

export function createProductColumns(suppliers: AdminSupplierItem[], actions: Actions): ProColumns<AdminProductItem>[] {
  const supplierNames = new Map(suppliers.map((supplier) => [supplier.supplier_id, supplier.name]));
  return [
    { title: '商品ID', dataIndex: 'product_id', width: 100, valueType: 'digit' },
    { title: '供应商ID', dataIndex: 'supplier_id', hideInTable: true, valueType: 'digit' },
    { title: '关键词', dataIndex: 'keyword', hideInTable: true },
    { title: '活动状态', dataIndex: 'promotion_status', hideInTable: true, valueEnum: {
      '-1': { text: '全部' }, '1': { text: '有活动' }, '2': { text: '无活动' },
    } },
    { title: '库存状态', dataIndex: 'stock_status', hideInTable: true, valueEnum: {
      '-1': { text: '全部' }, '1': { text: '库存充足' }, '2': { text: '低库存' }, '3': { text: '缺货' },
    } },
    { title: '商品', dataIndex: 'name', width: 240, search: false, render: (_, row) => (
      <Space size={10}>
        <ProductThumbnail src={row.image_url} alt={`${row.name} 商品图`} />
        <span style={{ display: 'inline-block', maxWidth: 164, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {row.name || `商品 ${row.product_id}`}
        </span>
      </Space>
    ) },
    { title: '原价', dataIndex: 'origin_price_fen', width: 120, search: false, render: (_, row) => `¥${formatPriceFen(row.origin_price_fen)}` },
    { title: '售价', dataIndex: 'sale_price_fen', width: 120, search: false, render: (_, row) => `¥${formatPriceFen(row.sale_price_fen)}` },
    { title: '当前价', dataIndex: 'promotion_price_fen', width: 120, search: false,
      render: (_, row) => `¥${formatPriceFen(row.promotion_price_fen > 0 ? row.promotion_price_fen : row.sale_price_fen)}` },
    { title: '活动', dataIndex: 'promotion_tag', width: 100, search: false,
      render: (_, row) => row.promotion_tag ? <Tag color="blue">{row.promotion_tag}</Tag> : '-' },
    { title: '供应商', dataIndex: 'supplier_id', width: 150, search: false, render: (_, row) => {
      const name = supplierNames.get(row.supplier_id) || row.supplier_name;
      return name ? `${name} (${row.supplier_id})` : row.supplier_id;
    } },
    { title: '库存', dataIndex: 'stock_available', width: 110, search: false, render: (_, row) => {
      const stock = row.stock_available || 0;
      if (stock <= 0) return <Tag color="red">缺货</Tag>;
      if (stock <= 100) return <Tag color="orange">{stock}</Tag>;
      return stock;
    } },
    { title: '状态', dataIndex: 'status', width: 100, render: (_, row) => <ProductStatusTag status={row.status} />,
      valueEnum: { '-1': { text: '全部' }, '1': { text: '上架' }, '2': { text: '下架' } } },
    { title: '操作', width: 320, search: false, render: (_, row) => (
      <Space size={4}>
        <Button type="link" size="small" onClick={() => actions.detail(row.product_id)}>详情</Button>
        <Button type="link" size="small" onClick={() => actions.edit(row)}>编辑</Button>
        <Button type="link" size="small" onClick={() => actions.stock(row)}>库存</Button>
        <Button type="link" size="small" onClick={() => actions.security(row.product_id)}>安全</Button>
        <Button type="link" size="small" onClick={() => actions.navigate('/admin/promotions', row.product_id)}>促销</Button>
        <Button type="link" size="small" onClick={() => actions.navigate('/admin/orders', row.product_id)}>订单</Button>
        <Popconfirm title={row.status === 1 ? '确认下架该商品？' : '确认上架该商品？'} onConfirm={() => actions.status(row)}>
          <Button type="link" size="small" danger={row.status === 1}>{row.status === 1 ? '下架' : '上架'}</Button>
        </Popconfirm>
      </Space>
    ) },
  ];
}
