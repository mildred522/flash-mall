import { useEffect, useRef, useState } from 'react';
import { Button, Descriptions, Form, Input, Modal, Popconfirm, Select, Space, Tag, message } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { ProTable, type ActionType, type ProColumns } from '@ant-design/pro-components';
import { authed } from '@flash-mall/shared';
import type {
  AdminMutationResp,
  AdminSupplierDetailResp,
  AdminSupplierCreateReq,
  AdminSupplierCreateResp,
  AdminSupplierItem,
  AdminSupplierListResp,
  AdminSupplierUpdateReq,
} from '@flash-mall/shared';

type SupplierFormValues = {
  name: string;
  status: number;
};

function mutationError(data: AdminMutationResp): string {
  if (data.error === 'supplier has active products') {
    return '该供应商仍有关联的启用商品，请先下架或迁移商品';
  }
  return data.error || '';
}

function supplierStatusTag(supplier: Pick<AdminSupplierItem, 'status'>) {
  return supplier.status === 1 ? <Tag color="green">启用</Tag> : <Tag color="red">停用</Tag>;
}

export default function SuppliersPage() {
  const actionRef = useRef<ActionType>();
  const [form] = Form.useForm<SupplierFormValues>();
  const [modalOpen, setModalOpen] = useState(false);
  const [editingSupplier, setEditingSupplier] = useState<AdminSupplierItem | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detail, setDetail] = useState<AdminSupplierItem | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [initialSupplierId] = useState(() => {
    const supplierId = window.__flashAdminSupplierSupplierId || 0;
    window.__flashAdminSupplierSupplierId = 0;
    return supplierId;
  });

  const reload = () => actionRef.current?.reload();

  const openSecurityLogs = (supplierId: number) => {
    window.dispatchEvent(new CustomEvent('flash-admin:navigate', {
      detail: { path: '/admin/security', securityKeyword: `supplier:${supplierId}` },
    }));
  };

  const openCreate = () => {
    setEditingSupplier(null);
    form.resetFields();
    form.setFieldsValue({ name: '', status: 1 });
    setModalOpen(true);
  };

  const closeSupplierModal = () => {
    setModalOpen(false);
    setEditingSupplier(null);
    form.resetFields();
  };

  const openEdit = (supplier: AdminSupplierItem) => {
    setEditingSupplier(supplier);
    form.setFieldsValue({ name: supplier.name, status: supplier.status });
    setModalOpen(true);
  };

  const openDetail = async (supplierId: number) => {
    if (!supplierId) return;
    setDetailOpen(true);
    setDetail(null);
    setDetailLoading(true);
    try {
      const res = await authed<AdminSupplierDetailResp>(`/api/admin/suppliers/detail?supplier_id=${encodeURIComponent(String(supplierId))}`);
      const error = mutationError(res.data as AdminMutationResp);
      if (res.ok && !error) {
        setDetail(res.data);
      } else {
        message.error(error || '供应商详情加载失败');
        setDetailOpen(false);
      }
    } finally {
      setDetailLoading(false);
    }
  };

  useEffect(() => {
    if (initialSupplierId) {
      void openDetail(initialSupplierId);
    }
  }, [initialSupplierId]);

  const saveSupplier = async () => {
    const values = await form.validateFields();
    setSubmitting(true);
    try {
      if (editingSupplier) {
        const body: AdminSupplierUpdateReq = {
          supplier_id: editingSupplier.supplier_id,
          name: values.name,
          status: values.status,
        };
        const res = await authed<AdminMutationResp>('/api/admin/suppliers/update', { method: 'POST', jsonBody: body });
        const error = mutationError(res.data);
        if (res.ok && !error) {
          message.success('供应商已更新');
          closeSupplierModal();
          if (detail?.supplier_id === editingSupplier.supplier_id) {
            void openDetail(editingSupplier.supplier_id);
          }
          reload();
        } else {
          message.error(error || '供应商更新失败');
        }
      } else {
        const body: AdminSupplierCreateReq = { name: values.name, status: values.status };
        const res = await authed<AdminSupplierCreateResp>('/api/admin/suppliers/create', { method: 'POST', jsonBody: body });
        const error = mutationError(res.data);
        if (res.ok && !error) {
          message.success(`供应商已创建：${res.data.supplier_id}`);
          closeSupplierModal();
          reload();
        } else {
          message.error(error || '供应商创建失败');
        }
      }
    } finally {
      setSubmitting(false);
    }
  };

  const toggleStatus = async (supplier: AdminSupplierItem) => {
    const nextStatus = supplier.status === 1 ? 2 : 1;
    const res = await authed<AdminMutationResp>('/api/admin/suppliers/update', {
      method: 'POST',
      jsonBody: { supplier_id: supplier.supplier_id, status: nextStatus } satisfies AdminSupplierUpdateReq,
    });
    const error = mutationError(res.data);
    if (res.ok && !error) {
      message.success(nextStatus === 1 ? '供应商已启用' : '供应商已停用');
      if (detail?.supplier_id === supplier.supplier_id) {
        setDetail({ ...detail, status: nextStatus, status_text: nextStatus === 1 ? 'active' : 'inactive' });
      }
      reload();
    } else {
      message.error(error || '供应商状态更新失败');
    }
  };

  const columns: ProColumns<AdminSupplierItem>[] = [
    { title: '供应商ID', dataIndex: 'supplier_id', width: 110, search: false },
    { title: '关键词', dataIndex: 'keyword', hideInTable: true },
    { title: '名称', dataIndex: 'name', ellipsis: true, search: false },
    {
      title: '商品数',
      dataIndex: 'product_count',
      width: 100,
      search: false,
      render: (_, row) => (
        <Button type="link" size="small" onClick={() => window.dispatchEvent(new CustomEvent('flash-admin:navigate', { detail: { path: '/admin/products', supplierId: row.supplier_id } }))}>
          {row.product_count || 0}
        </Button>
      ),
    },
    {
      title: '启用商品',
      dataIndex: 'active_products',
      width: 100,
      search: false,
      render: (_, row) => row.active_products > 0 ? <Tag color="blue">{row.active_products}</Tag> : <Tag>{row.active_products || 0}</Tag>,
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 110,
      render: (_, row) => supplierStatusTag(row),
      valueEnum: {
        '-1': { text: '全部' },
        '1': { text: '启用' },
        '2': { text: '停用' },
      },
    },
    {
      title: '操作',
      width: 260,
      search: false,
      render: (_, row) => (
        <Space size={4}>
          <Button type="link" size="small" onClick={() => openDetail(row.supplier_id)}>详情</Button>
          <Button type="link" size="small" onClick={() => openEdit(row)}>编辑</Button>
          <Button type="link" size="small" onClick={() => window.dispatchEvent(new CustomEvent('flash-admin:navigate', { detail: { path: '/admin/products', supplierId: row.supplier_id } }))}>商品</Button>
          <Button type="link" size="small" onClick={() => openSecurityLogs(row.supplier_id)}>安全</Button>
          <Popconfirm
            title={row.status === 1 ? '确认停用该供应商？' : '确认启用该供应商？'}
            onConfirm={() => toggleStatus(row)}
          >
            <Button type="link" size="small" danger={row.status === 1}>
              {row.status === 1 ? '停用' : '启用'}
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <>
      <ProTable<AdminSupplierItem>
        actionRef={actionRef}
        columns={columns}
        rowKey="supplier_id"
        search={{ labelWidth: 'auto' }}
        request={async (params) => {
          const query = new URLSearchParams();
          query.set('page', String(params.current || 1));
          query.set('page_size', String(params.pageSize || 20));
          if (params.keyword) query.set('keyword', String(params.keyword));
          if (params.status !== undefined && String(params.status) !== '-1') query.set('status', String(params.status));

          const res = await authed<AdminSupplierListResp>(`/api/admin/suppliers?${query}`);
          return {
            data: res.ok ? res.data.items || [] : [],
            total: res.ok ? res.data.total : 0,
            success: res.ok,
          };
        }}
        pagination={{ defaultPageSize: 20 }}
        toolBarRender={() => [
          <Button key="create" type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新增供应商
          </Button>,
        ]}
      />

      <Modal
        title={editingSupplier ? '编辑供应商' : '新增供应商'}
        open={modalOpen}
        onCancel={closeSupplierModal}
        onOk={saveSupplier}
        confirmLoading={submitting}
        destroyOnClose
      >
        <Form form={form} layout="vertical" preserve={false}>
          <Form.Item name="name" label="供应商名称" rules={[{ required: true, message: '请输入供应商名称' }]}>
            <Input maxLength={80} />
          </Form.Item>
          <Form.Item name="status" label="状态" rules={[{ required: true, message: '请选择状态' }]}>
            <Select
              options={[
                { value: 1, label: '启用' },
                { value: 2, label: '停用' },
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="供应商详情"
        open={detailOpen}
        footer={[
          <Button key="close" onClick={() => setDetailOpen(false)}>关闭</Button>,
          <Button
            key="security"
            disabled={!detail}
            onClick={() => {
              if (!detail) return;
              setDetailOpen(false);
              openSecurityLogs(detail.supplier_id);
            }}
          >
            安全日志
          </Button>,
          <Button
            key="edit"
            disabled={!detail}
            onClick={() => {
              if (!detail) return;
              setDetailOpen(false);
              openEdit(detail);
            }}
          >
            编辑
          </Button>,
          <Button
            key="status"
            danger={detail?.status === 1}
            disabled={!detail}
            onClick={() => {
              if (!detail) return;
              void toggleStatus(detail);
            }}
          >
            {detail?.status === 1 ? '停用' : '启用'}
          </Button>,
          <Button
            key="products"
            type="primary"
            disabled={!detail}
            onClick={() => {
              if (!detail) return;
              setDetailOpen(false);
              window.dispatchEvent(new CustomEvent('flash-admin:navigate', { detail: { path: '/admin/products', supplierId: detail.supplier_id } }));
            }}
          >
            查看商品
          </Button>,
        ]}
        destroyOnHidden
        onCancel={() => setDetailOpen(false)}
      >
        {detail ? (
          <Descriptions column={2} bordered size="small">
            <Descriptions.Item label="供应商ID">{detail.supplier_id}</Descriptions.Item>
            <Descriptions.Item label="状态">{supplierStatusTag(detail)}</Descriptions.Item>
            <Descriptions.Item label="名称" span={2}>{detail.name || '-'}</Descriptions.Item>
            <Descriptions.Item label="商品数">{detail.product_count || 0}</Descriptions.Item>
            <Descriptions.Item label="启用商品">{detail.active_products || 0}</Descriptions.Item>
          </Descriptions>
        ) : detailLoading ? (
          <div>加载中...</div>
        ) : (
          <div>暂无供应商详情</div>
        )}
      </Modal>
    </>
  );
}
