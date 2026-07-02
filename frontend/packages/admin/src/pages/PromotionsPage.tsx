import { useEffect, useRef, useState } from 'react';
import { Button, Descriptions, Form, Input, InputNumber, Modal, Popconfirm, Select, Space, Tag, message } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { ProTable, type ActionType, type ProColumns } from '@ant-design/pro-components';
import { authed, formatPriceFen } from '@flash-mall/shared';
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

type PromotionFormValues = {
  product_id: number;
  discount_value: number;
  threshold_amount?: number;
  starts_at?: string;
  ends_at?: string;
  status: number;
};

function mutationError(data: AdminMutationResp): string {
  if (data.error === 'active limited price promotion already exists') {
    return '该商品已有启用中的限时价规则，请先停用原规则';
  }
  if (data.error === 'active limited price promotion window overlaps') {
    return '该商品已有时间重叠的启用限时价规则';
  }
  if (data.error === 'ends_at must be after starts_at') {
    return '结束时间必须晚于开始时间';
  }
  if (data.error === 'discount_value must be <= product sale_price_fen') {
    return '限时价不能高于商品现价';
  }
  return data.error || '';
}

function promotionTypeText(type: string): string {
  return type === 'LIMITED_PRICE' || type === 'limited_price' ? '限时价' : type || '-';
}

function promotionStatusTag(promotion: Pick<AdminPromotionItem, 'status'>) {
  return promotion.status === 1 ? <Tag color="green">启用</Tag> : <Tag color="red">停用</Tag>;
}

export default function PromotionsPage() {
  const actionRef = useRef<ActionType>();
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
      const res = await authed<AdminPromotionDetailResp>(`/api/admin/promotions/detail?promotion_id=${encodeURIComponent(String(promotionId))}`);
      const error = mutationError(res.data as AdminMutationResp);
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
    if (initialPromotionId) {
      void openDetail(initialPromotionId);
    }
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
        const error = mutationError(res.data);
        if (res.ok && !error) {
          message.success('促销规则已更新');
          closePromotionModal();
          if (detail?.promotion_id === editingPromotion.promotion_id) {
            void openDetail(editingPromotion.promotion_id);
          }
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
        const error = mutationError(res.data);
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
    const error = mutationError(res.data);
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

  const productOptions = [...products];
  if (editingPromotion && !productOptions.some((product) => product.product_id === editingPromotion.product_id)) {
    productOptions.push({
      product_id: editingPromotion.product_id,
      name: editingPromotion.product_name || `商品 ${editingPromotion.product_id}`,
      origin_price_fen: 0,
      sale_price_fen: 0,
      supplier_id: 0,
      supplier_name: '',
      stock_available: 0,
      promotion_price_fen: 0,
      promotion_type: '',
      promotion_tag: '',
      status: 1,
      status_text: 'active',
    });
  }

  const productNameById = new Map(productOptions.map((product) => [product.product_id, product.name]));

  const columns: ProColumns<AdminPromotionItem>[] = [
    { title: '规则ID', dataIndex: 'promotion_id', width: 100, search: false },
    { title: '关键词', dataIndex: 'keyword', hideInTable: true },
    { title: '商品ID', dataIndex: 'product_id', hideInTable: true, valueType: 'digit' },
    {
      title: '商品',
      dataIndex: 'product_name',
      ellipsis: true,
      search: false,
      render: (_, row) => `${row.product_name || productNameById.get(row.product_id) || '商品'} (${row.product_id})`,
    },
    { title: '类型', dataIndex: 'type', width: 130, search: false, render: (_, row) => promotionTypeText(row.type) },
    {
      title: '限时价',
      dataIndex: 'discount_value',
      width: 120,
      search: false,
      render: (_, row) => row.sale_price_fen > 0
        ? `¥${formatPriceFen(row.sale_price_fen)} -> ¥${formatPriceFen(row.discount_value)}`
        : `¥${formatPriceFen(row.discount_value)}`,
    },
    { title: '开始时间', dataIndex: 'starts_at', width: 170, search: false, render: (_, row) => row.starts_at || '-' },
    { title: '结束时间', dataIndex: 'ends_at', width: 170, search: false, render: (_, row) => row.ends_at || '-' },
    {
      title: '时间态',
      dataIndex: 'effect_status',
      width: 100,
      valueEnum: {
        active: { text: '生效中' },
        scheduled: { text: '未开始' },
        expired: { text: '已结束' },
        inactive: { text: '停用' },
      },
      render: (_, row) => {
        const color = row.effect_status === 'active' ? 'blue' : row.effect_status === 'scheduled' ? 'gold' : row.effect_status === 'expired' ? 'default' : 'red';
        return <Tag color={color}>{row.effect_status_text || '-'}</Tag>;
      },
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (_, row) => promotionStatusTag(row),
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
          <Button type="link" size="small" onClick={() => openDetail(row.promotion_id)}>详情</Button>
          <Button type="link" size="small" onClick={() => openEdit(row)}>编辑</Button>
          <Button type="link" size="small" onClick={() => openSecurityLogs(row.promotion_id)}>安全</Button>
          <Popconfirm
            title={row.status === 1 ? '确认停用该促销？' : '确认启用该促销？'}
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
          return {
            data: res.ok ? res.data.items || [] : [],
            total: res.ok ? res.data.total : 0,
            success: res.ok,
          };
        }}
        pagination={{ defaultPageSize: 20 }}
        toolBarRender={() => [
          <Button key="create" type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新增促销
          </Button>,
        ]}
      />

      <Modal
        title={editingPromotion ? '编辑促销' : '新增促销'}
        open={modalOpen}
        onCancel={closePromotionModal}
        onOk={savePromotion}
        confirmLoading={submitting}
        destroyOnClose
      >
        <Form form={form} layout="vertical" preserve={false}>
          <Form.Item
            name="product_id"
            label="商品"
            rules={[
              { required: true, message: '请选择商品' },
              {
                validator: (_, value) => value > 0
                  ? Promise.resolve()
                  : Promise.reject(new Error('请选择商品')),
              },
            ]}
          >
            <Select
              showSearch
              optionFilterProp="label"
              options={productOptions.map((product) => ({
                value: product.product_id,
                label: `${product.name} (${product.product_id})`,
              }))}
            />
          </Form.Item>
          <Form.Item name="discount_value" label="限时价(分)" rules={[{ required: true, message: '请输入限时价' }]}>
            <InputNumber min={1} precision={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="threshold_amount" label="门槛数量">
            <InputNumber min={0} precision={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="starts_at" label="开始时间">
            <Input placeholder="YYYY-MM-DD HH:mm:ss，留空立即生效" />
          </Form.Item>
          <Form.Item name="ends_at" label="结束时间">
            <Input placeholder="YYYY-MM-DD HH:mm:ss，留空长期有效" />
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
        title="促销详情"
        open={detailOpen}
        footer={[
          <Button key="close" onClick={() => setDetailOpen(false)}>关闭</Button>,
          <Button
            key="security"
            disabled={!detail}
            onClick={() => {
              if (!detail) return;
              setDetailOpen(false);
              openSecurityLogs(detail.promotion_id);
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
            key="product"
            disabled={!detail}
            onClick={() => {
              if (!detail) return;
              setDetailOpen(false);
              window.dispatchEvent(new CustomEvent('flash-admin:navigate', { detail: { path: '/admin/products', productId: detail.product_id } }));
            }}
          >
            查看商品
          </Button>,
          <Button
            key="orders"
            type="primary"
            disabled={!detail}
            onClick={() => {
              if (!detail) return;
              setDetailOpen(false);
              window.dispatchEvent(new CustomEvent('flash-admin:navigate', { detail: { path: '/admin/orders', productId: detail.product_id } }));
            }}
          >
            查看订单
          </Button>,
        ]}
        destroyOnHidden
        onCancel={() => setDetailOpen(false)}
      >
        {detail ? (
          <Descriptions column={2} bordered size="small">
            <Descriptions.Item label="规则ID">{detail.promotion_id}</Descriptions.Item>
            <Descriptions.Item label="状态">{promotionStatusTag(detail)}</Descriptions.Item>
            <Descriptions.Item label="商品" span={2}>{`${detail.product_name || '商品'} (${detail.product_id})`}</Descriptions.Item>
            <Descriptions.Item label="类型">{promotionTypeText(detail.type)}</Descriptions.Item>
            <Descriptions.Item label="时间态">{detail.effect_status_text || '-'}</Descriptions.Item>
            <Descriptions.Item label="商品售价">¥{formatPriceFen(detail.sale_price_fen)}</Descriptions.Item>
            <Descriptions.Item label="限时价">¥{formatPriceFen(detail.discount_value)}</Descriptions.Item>
            <Descriptions.Item label="开始时间">{detail.starts_at || '-'}</Descriptions.Item>
            <Descriptions.Item label="结束时间">{detail.ends_at || '-'}</Descriptions.Item>
          </Descriptions>
        ) : detailLoading ? (
          <div>加载中...</div>
        ) : (
          <div>暂无促销详情</div>
        )}
      </Modal>
    </>
  );
}
