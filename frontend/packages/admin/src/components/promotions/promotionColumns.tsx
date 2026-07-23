import { Button, Popconfirm, Space, Tag } from 'antd';
import type { ProColumns } from '@ant-design/pro-components';
import { formatPriceFen } from '@flash-mall/shared';
import type { AdminProductItem, AdminPromotionItem } from '@flash-mall/shared';
import { PromotionStatusTag, promotionTypeText } from './promotionModel';

type Actions = {
  detail: (promotionId: number) => void;
  edit: (promotion: AdminPromotionItem) => void;
  security: (promotionId: number) => void;
  status: (promotion: AdminPromotionItem) => void;
};

export function createPromotionColumns(products: AdminProductItem[], actions: Actions): ProColumns<AdminPromotionItem>[] {
  const productNames = new Map(products.map((product) => [product.product_id, product.name]));
  return [
    { title: '规则ID', dataIndex: 'promotion_id', width: 100, search: false },
    { title: '关键词', dataIndex: 'keyword', hideInTable: true },
    { title: '商品ID', dataIndex: 'product_id', hideInTable: true, valueType: 'digit' },
    { title: '商品', dataIndex: 'product_name', ellipsis: true, search: false,
      render: (_, row) => `${row.product_name || productNames.get(row.product_id) || '商品'} (${row.product_id})` },
    { title: '类型', dataIndex: 'type', width: 130, search: false, render: (_, row) => promotionTypeText(row.type) },
    { title: '限时价', dataIndex: 'discount_value', width: 120, search: false,
      render: (_, row) => row.sale_price_fen > 0
        ? `¥${formatPriceFen(row.sale_price_fen)} -> ¥${formatPriceFen(row.discount_value)}`
        : `¥${formatPriceFen(row.discount_value)}` },
    { title: '开始时间', dataIndex: 'starts_at', width: 170, search: false, render: (_, row) => row.starts_at || '-' },
    { title: '结束时间', dataIndex: 'ends_at', width: 170, search: false, render: (_, row) => row.ends_at || '-' },
    { title: '时间态', dataIndex: 'effect_status', width: 100, valueEnum: {
      active: { text: '生效中' }, scheduled: { text: '未开始' }, expired: { text: '已结束' }, inactive: { text: '停用' },
    }, render: (_, row) => {
      const color = row.effect_status === 'active' ? 'blue' : row.effect_status === 'scheduled' ? 'gold'
        : row.effect_status === 'expired' ? 'default' : 'red';
      return <Tag color={color}>{row.effect_status_text || '-'}</Tag>;
    } },
    { title: '状态', dataIndex: 'status', width: 100, render: (_, row) => <PromotionStatusTag status={row.status} />,
      valueEnum: { '-1': { text: '全部' }, '1': { text: '启用' }, '2': { text: '停用' } } },
    { title: '操作', width: 260, search: false, render: (_, row) => (
      <Space size={4}>
        <Button type="link" size="small" onClick={() => actions.detail(row.promotion_id)}>详情</Button>
        <Button type="link" size="small" onClick={() => actions.edit(row)}>编辑</Button>
        <Button type="link" size="small" onClick={() => actions.security(row.promotion_id)}>安全</Button>
        <Popconfirm title={row.status === 1 ? '确认停用该促销？' : '确认启用该促销？'} onConfirm={() => actions.status(row)}>
          <Button type="link" size="small" danger={row.status === 1}>{row.status === 1 ? '停用' : '启用'}</Button>
        </Popconfirm>
      </Space>
    ) },
  ];
}
