import { useEffect, useRef, useState } from 'react';
import { Button, Form, message } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { ProTable, type ActionType } from '@ant-design/pro-components';
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
import SupplierDetailModal from '../components/suppliers/SupplierDetailModal';
import SupplierEditorModal from '../components/suppliers/SupplierEditorModal';
import { createSupplierColumns } from '../components/suppliers/supplierColumns';
import { supplierMutationError, type SupplierFormValues } from '../components/suppliers/supplierModel';

export default function SuppliersPage() {
  const actionRef = useRef<ActionType | undefined>(undefined);
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
  const openProducts = (supplierId: number) => {
    window.dispatchEvent(new CustomEvent('flash-admin:navigate', {
      detail: { path: '/admin/products', supplierId },
    }));
  };
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
      const res = await authed<AdminSupplierDetailResp>(
        `/api/admin/suppliers/detail?supplier_id=${encodeURIComponent(String(supplierId))}`,
      );
      const error = supplierMutationError(res.data as AdminMutationResp);
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
    if (initialSupplierId) void openDetail(initialSupplierId);
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
        const error = supplierMutationError(res.data);
        if (res.ok && !error) {
          message.success('供应商已更新');
          closeSupplierModal();
          if (detail?.supplier_id === editingSupplier.supplier_id) void openDetail(editingSupplier.supplier_id);
          reload();
        } else {
          message.error(error || '供应商更新失败');
        }
      } else {
        const body: AdminSupplierCreateReq = { name: values.name, status: values.status };
        const res = await authed<AdminSupplierCreateResp>('/api/admin/suppliers/create', { method: 'POST', jsonBody: body });
        const error = supplierMutationError(res.data);
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
    const error = supplierMutationError(res.data);
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

  const columns = createSupplierColumns({
    detail: (supplierId) => void openDetail(supplierId),
    edit: openEdit,
    products: openProducts,
    security: openSecurityLogs,
    status: (supplier) => void toggleStatus(supplier),
  });

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
          return { data: res.ok ? res.data.items || [] : [], total: res.ok ? res.data.total : 0, success: res.ok };
        }}
        pagination={{ defaultPageSize: 20 }}
        toolBarRender={() => [
          <Button key="create" type="primary" icon={<PlusOutlined />} onClick={openCreate}>新增供应商</Button>,
        ]}
      />
      <SupplierEditorModal open={modalOpen} editingSupplier={editingSupplier} form={form} submitting={submitting}
        onCancel={closeSupplierModal} onSave={() => void saveSupplier()} />
      <SupplierDetailModal open={detailOpen} detail={detail} loading={detailLoading} onClose={() => setDetailOpen(false)}
        onSecurity={openSecurityLogs} onEdit={openEdit} onToggle={(supplier) => void toggleStatus(supplier)}
        onProducts={openProducts} />
    </>
  );
}
