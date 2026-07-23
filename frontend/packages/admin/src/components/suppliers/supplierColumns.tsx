import { Button, Popconfirm, Space, Tag } from 'antd';
import type { ProColumns } from '@ant-design/pro-components';
import type { AdminSupplierItem } from '@flash-mall/shared';
import { SupplierStatusTag } from './supplierModel';

type Actions = {
  detail: (supplierId: number) => void;
  edit: (supplier: AdminSupplierItem) => void;
  products: (supplierId: number) => void;
  security: (supplierId: number) => void;
  status: (supplier: AdminSupplierItem) => void;
};

export function createSupplierColumns(actions: Actions): ProColumns<AdminSupplierItem>[] {
  return [
    { title: '供应商ID', dataIndex: 'supplier_id', width: 110, search: false },
    { title: '关键词', dataIndex: 'keyword', hideInTable: true },
    { title: '名称', dataIndex: 'name', ellipsis: true, search: false },
    { title: '商品数', dataIndex: 'product_count', width: 100, search: false, render: (_, row) => (
      <Button type="link" size="small" onClick={() => actions.products(row.supplier_id)}>{row.product_count || 0}</Button>
    ) },
    { title: '启用商品', dataIndex: 'active_products', width: 100, search: false,
      render: (_, row) => row.active_products > 0
        ? <Tag color="blue">{row.active_products}</Tag> : <Tag>{row.active_products || 0}</Tag> },
    { title: '状态', dataIndex: 'status', width: 110,
      render: (_, row) => <SupplierStatusTag status={row.status} />,
      valueEnum: { '-1': { text: '全部' }, '1': { text: '启用' }, '2': { text: '停用' } } },
    { title: '操作', width: 260, search: false, render: (_, row) => (
      <Space size={4}>
        <Button type="link" size="small" onClick={() => actions.detail(row.supplier_id)}>详情</Button>
        <Button type="link" size="small" onClick={() => actions.edit(row)}>编辑</Button>
        <Button type="link" size="small" onClick={() => actions.products(row.supplier_id)}>商品</Button>
        <Button type="link" size="small" onClick={() => actions.security(row.supplier_id)}>安全</Button>
        <Popconfirm title={row.status === 1 ? '确认停用该供应商？' : '确认启用该供应商？'}
          onConfirm={() => actions.status(row)}>
          <Button type="link" size="small" danger={row.status === 1}>{row.status === 1 ? '停用' : '启用'}</Button>
        </Popconfirm>
      </Space>
    ) },
  ];
}
