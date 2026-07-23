import { useEffect, useRef, useState } from 'react';
import { Button, Form, message } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { ProTable, type ActionType } from '@ant-design/pro-components';
import { authed } from '@flash-mall/shared';
import type {
  AdminMutationResp,
  AdminProductItem,
  AdminProductListResp,
  AdminPromotionCreateReq,
  AdminPromotionCreateResp,
  AdminPromotionDetailResp,
  AdminPromotionItem,
  AdminPromotionListResp,
  AdminPromotionUpdateReq,
} from '@flash-mall/shared';
import PromotionDetailModal from '../components/promotions/PromotionDetailModal';
import PromotionEditorModal from '../components/promotions/PromotionEditorModal';
import { createPromotionColumns } from '../components/promotions/promotionColumns';
import { promotionMutationError, type PromotionFormValues } from '../components/promotions/promotionModel';

export default function PromotionsPage() {
  const actionRef = useRef<ActionType | undefined>(undefined);
  const [form] = Form.useForm<PromotionFormValues>();
  const [modalOpen, setModalOpen] = useState(false);
  const [editingPromotion, setEditingPromotion] = useState<AdminPromotionItem | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detail, setDetail] = useState<AdminPromotionItem | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [products, setProducts] = useState<AdminProductItem[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [initialPromotionId] = useState(() => {
    const promotionId = window.__flashAdminPromotionPromotionId || 0;
    window.__flashAdminPromotionPromotionId = 0;
    return promotionId;
  });
  const [initialProductId] = useState(() => {
    const productId = window.__flashAdminPromotionProductId || 0;
    window.__flashAdminPromotionProductId = 0;
    return productId;
  });
  const [initialEffectStatus] = useState(() => {
    const effectStatus = window.__flashAdminPromotionEffectStatus || '';
    window.__flashAdminPromotionEffectStatus = '';
    return effectStatus;
  });

  const reload = () => actionRef.current?.reload();
  const navigateToProduct = (path: string, productId: number) => {
    window.dispatchEvent(new CustomEvent('flash-admin:navigate', { detail: { path, productId } }));
  };
  const openSecurityLogs = (promotionId: number) => {
    window.dispatchEvent(new CustomEvent('flash-admin:navigate', {
      detail: { path: '/admin/security', securityKeyword: `promotion:${promotionId}` },
    }));
  };

  useEffect(() => {
    authed<AdminProductListResp>('/api/admin/products?page=1&page_size=100').then((res) => {
      if (res.ok) setProducts(res.data.items || []);
    });
  }, []);

  const openCreate = () => {
    setEditingPromotion(null);
    form.resetFields();
    form.setFieldsValue({
      product_id: products[0]?.product_id || 0,
      discount_value: 0,
      threshold_amount: 0,
      starts_at: '',
      ends_at: '',
      status: 1,
    });
    setModalOpen(true);
  };

  const closePromotionModal = () => {
    setModalOpen(false);
    setEditingPromotion(null);
    form.resetFields();
  };

  const openEdit = (promotion: AdminPromotionItem) => {
    setEditingPromotion(promotion);
    form.setFieldsValue({
      product_id: promotion.product_id,
      discount_value: promotion.discount_value,
      threshold_amount: promotion.threshold_amount,
      starts_at: promotion.starts_at,
      ends_at: promotion.ends_at,
      status: promotion.status,
    });
    setModalOpen(true);
  };

  const openDetail = async (promotionId: number) => {
    if (!promotionId) return;
    setDetailOpen(true);
    setDetail(null);
    setDetailLoading(true);
    try {
      const res = await authed<AdminPromotionDetailResp>(
        `/api/admin/promotions/detail?promotion_id=${encodeURIComponent(String(promotionId))}`,
      );
      const error = promotionMutationError(res.data as AdminMutationResp);
      if (res.ok && !error) {
        setDetail(res.data);
      } else {
        message.error(error || '促销详情加载失败');
        setDetailOpen(false);
      }
    } finally {
      setDetailLoading(false);
    }
  };

  useEffect(() => {
    if (initialPromotionId) void openDetail(initialPromotionId);
  }, [initialPromotionId]);

  const savePromotion = async () => {
    const values = await form.validateFields();
    const product = products.find((item) => item.product_id === values.product_id);
    if (product && product.sale_price_fen > 0 && values.discount_value > product.sale_price_fen) {
      message.warning('限时价不能高于商品现价');
      return;
    }
    setSubmitting(true);
    try {
      if (editingPromotion) {
        const body: AdminPromotionUpdateReq = {
          promotion_id: editingPromotion.promotion_id,
          product_id: values.product_id,
          discount_value: values.discount_value,
          threshold_amount: values.threshold_amount || 0,
          starts_at: values.starts_at?.trim() || '',
          ends_at: values.ends_at?.trim() || '',
          status: values.status,
        };
        const res = await authed<AdminMutationResp>('/api/admin/promotions/update', { method: 'POST', jsonBody: body });
        const error = promotionMutationError(res.data);
        if (res.ok && !error) {
          message.success('促销规则已更新');
          closePromotionModal();
          if (detail?.promotion_id === editingPromotion.promotion_id) void openDetail(editingPromotion.promotion_id);
          reload();
        } else {
          message.error(error || '促销规则更新失败');
        }
      } else {
        const body: AdminPromotionCreateReq = {
          product_id: values.product_id,
          type: 'LIMITED_PRICE',
          discount_value: values.discount_value,
          threshold_amount: values.threshold_amount || 0,
          starts_at: values.starts_at?.trim() || '',
          ends_at: values.ends_at?.trim() || '',
          status: values.status,
        };
        const res = await authed<AdminPromotionCreateResp>('/api/admin/promotions/create', { method: 'POST', jsonBody: body });
        const error = promotionMutationError(res.data);
        if (res.ok && !error) {
          message.success(`促销规则已创建：${res.data.promotion_id}`);
          closePromotionModal();
          reload();
        } else {
          message.error(error || '促销规则创建失败');
        }
      }
    } finally {
      setSubmitting(false);
    }
  };

  const toggleStatus = async (promotion: AdminPromotionItem) => {
    const nextStatus = promotion.status === 1 ? 2 : 1;
    const res = await authed<AdminMutationResp>('/api/admin/promotions/update', {
      method: 'POST',
      jsonBody: { promotion_id: promotion.promotion_id, status: nextStatus } satisfies AdminPromotionUpdateReq,
    });
    const error = promotionMutationError(res.data);
    if (res.ok && !error) {
      message.success(nextStatus === 1 ? '促销已启用' : '促销已停用');
      if (detail?.promotion_id === promotion.promotion_id) {
        setDetail({ ...detail, status: nextStatus, status_text: nextStatus === 1 ? 'active' : 'inactive' });
      }
      reload();
    } else {
      message.error(error || '促销状态更新失败');
    }
  };

  const columns = createPromotionColumns(products, {
    detail: (promotionId) => void openDetail(promotionId),
    edit: openEdit,
    security: openSecurityLogs,
    status: (promotion) => void toggleStatus(promotion),
  });

  return (
    <>
      <ProTable<AdminPromotionItem>
        actionRef={actionRef}
        columns={columns}
        rowKey="promotion_id"
        params={{ initialProductId, initialEffectStatus }}
        search={{ labelWidth: 'auto' }}
        request={async (params) => {
          const query = new URLSearchParams();
          query.set('page', String(params.current || 1));
          query.set('page_size', String(params.pageSize || 20));
          if (params.keyword) query.set('keyword', String(params.keyword));
          const productId = params.product_id || params.initialProductId;
          if (productId) query.set('product_id', String(productId));
          const effectStatus = params.effect_status || params.initialEffectStatus;
          if (effectStatus) query.set('effect_status', String(effectStatus));
          if (params.status !== undefined && String(params.status) !== '-1') query.set('status', String(params.status));
          const res = await authed<AdminPromotionListResp>(`/api/admin/promotions?${query}`);
          return { data: res.ok ? res.data.items || [] : [], total: res.ok ? res.data.total : 0, success: res.ok };
        }}
        pagination={{ defaultPageSize: 20 }}
        toolBarRender={() => [
          <Button key="create" type="primary" icon={<PlusOutlined />} onClick={openCreate}>新增促销</Button>,
        ]}
      />
      <PromotionEditorModal open={modalOpen} editingPromotion={editingPromotion} form={form} products={products}
        submitting={submitting} onCancel={closePromotionModal} onSave={() => void savePromotion()} />
      <PromotionDetailModal open={detailOpen} detail={detail} loading={detailLoading} onClose={() => setDetailOpen(false)}
        onSecurity={openSecurityLogs} onEdit={openEdit} onToggle={(promotion) => void toggleStatus(promotion)}
        onNavigate={navigateToProduct} />
    </>
  );
}
