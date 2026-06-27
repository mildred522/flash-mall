import { useEffect, useRef, useState } from 'react';
import { Button, Descriptions, Form, Input, InputNumber, Modal, Popconfirm, Select, Space, Tag, message } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { ProTable, type ActionType, type ProColumns } from '@ant-design/pro-components';
import { authed, formatPriceFen } from '@flash-mall/shared';
import type {
  AdminMutationResp,
  AdminProductCreateReq,
  AdminProductCreateResp,
  AdminProductDetailResp,
  AdminProductItem,
  AdminProductListResp,
  AdminProductStockAdjustReq,
  AdminProductStockAdjustResp,
  AdminProductUpdateReq,
  AdminSupplierItem,
  AdminSupplierListResp,
} from '@flash-mall/shared';

type ProductFormValues = {
  name: string;
  origin_price_fen: number;
  sale_price_fen: number;
  supplier_id: number;
  stock_available?: number;
  status: number;
};

type StockFormValues = {
  delta: number;
  bucket_idx: number;
};

function mutationError(data: AdminMutationResp): string {
  if (data.error === 'sale_price_fen must be <= origin_price_fen') {
    return '现价不能高于原价';
  }
  if (data.error === 'active supplier not found') {
    return '请选择启用中的供应商';
  }
  return data.error || '';
}

function productStatusTag(product: Pick<AdminProductItem, 'status'>) {
  return product.status === 1 ? <Tag color="green">上架</Tag> : <Tag color="red">下架</Tag>;
}

export default function ProductsPage() {
  const actionRef = useRef<ActionType>();
  const [productForm] = Form.useForm<ProductFormValues>();
  const [stockForm] = Form.useForm<StockFormValues>();
  const [productModalOpen, setProductModalOpen] = useState(false);
  const [stockModalOpen, setStockModalOpen] = useState(false);
  const [editingProduct, setEditingProduct] = useState<AdminProductItem | null>(null);
  const [stockProduct, setStockProduct] = useState<AdminProductItem | null>(null);
  const [detailOpen, setDetailOpen] = useState(false);
  const [detail, setDetail] = useState<AdminProductItem | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [suppliers, setSuppliers] = useState<AdminSupplierItem[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [initialProductId] = useState(() => {
    const productId = window.__flashAdminProductProductId || 0;
    window.__flashAdminProductProductId = 0;
    return productId;
  });
  const [initialSupplierId] = useState(() => {
    const supplierId = window.__flashAdminProductSupplierId || 0;
    window.__flashAdminProductSupplierId = 0;
    return supplierId;
  });
  const [initialStockStatus] = useState(() => {
    const stockStatus = window.__flashAdminProductStockStatus;
    window.__flashAdminProductStockStatus = undefined;
    return stockStatus;
  });

  const reload = () => actionRef.current?.reload();

  const openSecurityLogs = (productId: number) => {
    window.dispatchEvent(new CustomEvent('flash-admin:navigate', {
      detail: { path: '/admin/security', securityKeyword: `product:${productId}` },
    }));
  };

  useEffect(() => {
    authed<AdminSupplierListResp>('/api/admin/suppliers?status=1&page=1&page_size=100').then((res) => {
      if (res.ok) setSuppliers(res.data.items || []);
    });
  }, []);

  const openDetail = async (productId: number) => {
    if (!productId) return;
    setDetailOpen(true);
    setDetail(null);
    setDetailLoading(true);
    try {
      const res = await authed<AdminProductDetailResp>(`/api/admin/products/detail?product_id=${encodeURIComponent(String(productId))}`);
      const error = mutationError(res.data as AdminMutationResp);
      if (res.ok && !error) {
        setDetail(res.data);
      } else {
        message.error(error || '商品详情加载失败');
        setDetailOpen(false);
      }
    } finally {
      setDetailLoading(false);
    }
  };

  useEffect(() => {
    if (initialProductId) {
      void openDetail(initialProductId);
    }
  }, [initialProductId]);

  const openCreate = () => {
    setEditingProduct(null);
    productForm.setFieldsValue({
      name: '',
      origin_price_fen: 0,
      sale_price_fen: 0,
      supplier_id: suppliers[0]?.supplier_id || 0,
      stock_available: 0,
      status: 1,
    });
    setProductModalOpen(true);
  };

  const openEdit = (product: AdminProductItem) => {
    setEditingProduct(product);
    productForm.setFieldsValue({
      name: product.name,
      origin_price_fen: product.origin_price_fen,
      sale_price_fen: product.sale_price_fen,
      supplier_id: product.supplier_id,
      status: product.status,
    });
    setProductModalOpen(true);
  };

  const openStock = (product: AdminProductItem) => {
    setStockProduct(product);
    stockForm.setFieldsValue({ delta: 0, bucket_idx: 0 });
    setStockModalOpen(true);
  };

  const saveProduct = async () => {
    const values = await productForm.validateFields();
    if (values.sale_price_fen > values.origin_price_fen) {
      message.warning('现价不能高于原价');
      return;
    }
    setSubmitting(true);
    try {
      if (editingProduct) {
        const body: AdminProductUpdateReq = {
          product_id: editingProduct.product_id,
          name: values.name,
          origin_price_fen: values.origin_price_fen,
          sale_price_fen: values.sale_price_fen,
          supplier_id: values.supplier_id,
          status: values.status,
        };
        const res = await authed<AdminMutationResp>('/api/admin/products/update', { method: 'POST', jsonBody: body });
        const error = mutationError(res.data);
        if (res.ok && !error) {
          message.success('商品已更新');
          setProductModalOpen(false);
          if (detail?.product_id === editingProduct.product_id) {
            void openDetail(editingProduct.product_id);
          }
          reload();
        } else {
          message.error(error || '商品更新失败');
        }
      } else {
        const body: AdminProductCreateReq = {
          name: values.name,
          origin_price_fen: values.origin_price_fen,
          sale_price_fen: values.sale_price_fen,
          supplier_id: values.supplier_id,
          stock_available: values.stock_available || 0,
          status: values.status,
        };
        const res = await authed<AdminProductCreateResp>('/api/admin/products/create', { method: 'POST', jsonBody: body });
        const error = mutationError(res.data);
        if (res.ok && !error) {
          message.success(`商品已创建：${res.data.product_id}`);
          setProductModalOpen(false);
          reload();
        } else {
          message.error(error || '商品创建失败');
        }
      }
    } finally {
      setSubmitting(false);
    }
  };

  const adjustStock = async () => {
    if (!stockProduct) return;
    const values = await stockForm.validateFields();
    const body: AdminProductStockAdjustReq = {
      product_id: stockProduct.product_id,
      delta: values.delta,
      bucket_idx: values.bucket_idx || 0,
    };
    setSubmitting(true);
    try {
      const res = await authed<AdminProductStockAdjustResp>('/api/admin/products/stock-adjust', { method: 'POST', jsonBody: body });
      const error = mutationError(res.data);
      if (res.ok && !error) {
        message.success(`库存已调整，当前库存 ${res.data.stock_available}`);
        if (detail?.product_id === stockProduct.product_id) {
          setDetail({ ...detail, stock_available: res.data.stock_available });
        }
        setStockModalOpen(false);
        reload();
      } else {
        message.error(error || '库存调整失败');
      }
    } finally {
      setSubmitting(false);
    }
  };

  const toggleStatus = async (product: AdminProductItem) => {
    const nextStatus = product.status === 1 ? 2 : 1;
    const res = await authed<AdminMutationResp>('/api/admin/products/update', {
      method: 'POST',
      jsonBody: { product_id: product.product_id, status: nextStatus } satisfies AdminProductUpdateReq,
    });
    const error = mutationError(res.data);
    if (res.ok && !error) {
      message.success(nextStatus === 1 ? '商品已上架' : '商品已下架');
      if (detail?.product_id === product.product_id) {
        setDetail({ ...detail, status: nextStatus, status_text: nextStatus === 1 ? 'active' : 'inactive' });
      }
      reload();
    } else {
      message.error(error || '状态更新失败');
    }
  };

  const supplierNameById = new Map(suppliers.map((supplier) => [supplier.supplier_id, supplier.name]));

  const columns: ProColumns<AdminProductItem>[] = [
    { title: '商品ID', dataIndex: 'product_id', width: 100, valueType: 'digit' },
    { title: '供应商ID', dataIndex: 'supplier_id', hideInTable: true, valueType: 'digit' },
    { title: '关键词', dataIndex: 'keyword', hideInTable: true },
    {
      title: '活动状态',
      dataIndex: 'promotion_status',
      hideInTable: true,
      valueEnum: {
        '-1': { text: '全部' },
        '1': { text: '有活动' },
        '2': { text: '无活动' },
      },
    },
    {
      title: '库存状态',
      dataIndex: 'stock_status',
      hideInTable: true,
      valueEnum: {
        '-1': { text: '全部' },
        '1': { text: '库存充足' },
        '2': { text: '低库存' },
        '3': { text: '缺货' },
      },
    },
    { title: '名称', dataIndex: 'name', ellipsis: true, search: false },
    {
      title: '原价', dataIndex: 'origin_price_fen', width: 120, search: false,
      render: (_, row) => `¥${formatPriceFen(row.origin_price_fen)}`,
    },
    {
      title: '售价', dataIndex: 'sale_price_fen', width: 120, search: false,
      render: (_, row) => `¥${formatPriceFen(row.sale_price_fen)}`,
    },
    {
      title: '当前价',
      dataIndex: 'promotion_price_fen',
      width: 120,
      search: false,
      render: (_, row) => row.promotion_price_fen > 0
        ? `¥${formatPriceFen(row.promotion_price_fen)}`
        : `¥${formatPriceFen(row.sale_price_fen)}`,
    },
    {
      title: '活动',
      dataIndex: 'promotion_tag',
      width: 100,
      search: false,
      render: (_, row) => row.promotion_tag ? <Tag color="blue">{row.promotion_tag}</Tag> : '-',
    },
    {
      title: '供应商',
      dataIndex: 'supplier_id',
      width: 150,
      search: false,
      render: (_, row) => supplierNameById.has(row.supplier_id)
        ? `${supplierNameById.get(row.supplier_id)} (${row.supplier_id})`
        : row.supplier_name
          ? `${row.supplier_name} (${row.supplier_id})`
        : row.supplier_id,
    },
    {
      title: '库存',
      dataIndex: 'stock_available',
      width: 110,
      search: false,
      render: (_, row) => {
        const stock = row.stock_available || 0;
        if (stock <= 0) return <Tag color="red">缺货</Tag>;
        if (stock <= 100) return <Tag color="orange">{stock}</Tag>;
        return stock;
      },
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (_, row) => productStatusTag(row),
      valueEnum: {
        '-1': { text: '全部' },
        '1': { text: '上架' },
        '2': { text: '下架' },
      },
    },
    {
      title: '操作',
      width: 320,
      search: false,
      render: (_, row) => (
        <Space size={4}>
          <Button type="link" size="small" onClick={() => openDetail(row.product_id)}>详情</Button>
          <Button type="link" size="small" onClick={() => openEdit(row)}>编辑</Button>
          <Button type="link" size="small" onClick={() => openStock(row)}>库存</Button>
          <Button type="link" size="small" onClick={() => openSecurityLogs(row.product_id)}>安全</Button>
          <Button type="link" size="small" onClick={() => window.dispatchEvent(new CustomEvent('flash-admin:navigate', { detail: { path: '/admin/promotions', productId: row.product_id } }))}>促销</Button>
          <Button type="link" size="small" onClick={() => window.dispatchEvent(new CustomEvent('flash-admin:navigate', { detail: { path: '/admin/orders', productId: row.product_id } }))}>订单</Button>
          <Popconfirm
            title={row.status === 1 ? '确认下架该商品？' : '确认上架该商品？'}
            onConfirm={() => toggleStatus(row)}
          >
            <Button type="link" size="small" danger={row.status === 1}>
              {row.status === 1 ? '下架' : '上架'}
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const supplierOptions = [...suppliers];
  if (editingProduct && editingProduct.supplier_id > 0 && !supplierOptions.some((s) => s.supplier_id === editingProduct.supplier_id)) {
    supplierOptions.push({
      supplier_id: editingProduct.supplier_id,
      name: `供应商 ${editingProduct.supplier_id}`,
      status: 1,
      status_text: 'active',
      product_count: 0,
      active_products: 0,
    });
  }

  return (
    <>
      <ProTable<AdminProductItem>
        actionRef={actionRef}
        columns={columns}
        rowKey="product_id"
        params={{ initialProductId, initialSupplierId, initialStockStatus }}
        request={async (params) => {
          const query = new URLSearchParams();
          query.set('page', String(params.current || 1));
          query.set('page_size', String(params.pageSize || 20));
          const productId = params.product_id || params.initialProductId;
          if (productId) query.set('product_id', String(productId));
          const supplierId = params.supplier_id || params.initialSupplierId;
          if (supplierId) query.set('supplier_id', String(supplierId));
          if (params.status !== undefined && String(params.status) !== '-1') query.set('status', String(params.status));
          if (params.promotion_status !== undefined && String(params.promotion_status) !== '-1') query.set('promotion_status', String(params.promotion_status));
          const stockStatus = params.stock_status !== undefined ? params.stock_status : params.initialStockStatus;
          if (stockStatus !== undefined && String(stockStatus) !== '-1') query.set('stock_status', String(stockStatus));
          if (params.keyword) query.set('keyword', String(params.keyword));

          const res = await authed<AdminProductListResp>(`/api/admin/products?${query}`);
          return {
            data: res.ok ? res.data.items || [] : [],
            total: res.ok ? res.data.total : 0,
            success: res.ok,
          };
        }}
        search={{ labelWidth: 'auto' }}
        pagination={{ defaultPageSize: 20 }}
        toolBarRender={() => [
          <Button key="create" type="primary" icon={<PlusOutlined />} onClick={openCreate}>
            新增商品
          </Button>,
        ]}
      />

      <Modal
        title={editingProduct ? '编辑商品' : '新增商品'}
        open={productModalOpen}
        onCancel={() => setProductModalOpen(false)}
        onOk={saveProduct}
        confirmLoading={submitting}
        destroyOnClose
      >
        <Form form={productForm} layout="vertical" preserve={false}>
          <Form.Item name="name" label="商品名称" rules={[{ required: true, message: '请输入商品名称' }]}>
            <Input maxLength={80} />
          </Form.Item>
          <Form.Item name="origin_price_fen" label="原价(分)" rules={[{ required: true, message: '请输入原价' }]}>
            <InputNumber min={0} precision={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="sale_price_fen" label="售价(分)" rules={[{ required: true, message: '请输入售价' }]}>
            <InputNumber min={0} precision={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item
            name="supplier_id"
            label="供应商"
            rules={[
              { required: true, message: '请选择供应商' },
              {
                validator: (_, value) => value > 0
                  ? Promise.resolve()
                  : Promise.reject(new Error('请选择供应商')),
              },
            ]}
          >
            <Select
              showSearch
              optionFilterProp="label"
              options={supplierOptions.map((supplier) => ({
                value: supplier.supplier_id,
                label: `${supplier.name} (${supplier.supplier_id})`,
              }))}
            />
          </Form.Item>
          {!editingProduct && (
            <Form.Item name="stock_available" label="初始库存">
              <InputNumber min={0} precision={0} style={{ width: '100%' }} />
            </Form.Item>
          )}
          <Form.Item name="status" label="状态" rules={[{ required: true, message: '请选择状态' }]}>
            <Select
              options={[
                { value: 1, label: '上架' },
                { value: 2, label: '下架' },
              ]}
            />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={stockProduct ? `调整库存：${stockProduct.name}` : '调整库存'}
        open={stockModalOpen}
        onCancel={() => setStockModalOpen(false)}
        onOk={adjustStock}
        confirmLoading={submitting}
        destroyOnClose
      >
        <Form form={stockForm} layout="vertical" preserve={false}>
          <Form.Item
            name="delta"
            label="库存变化量"
            rules={[
              { required: true, message: '请输入库存变化量' },
              {
                validator: (_, value) => value === 0
                  ? Promise.reject(new Error('库存变化量不能为 0'))
                  : Promise.resolve(),
              },
            ]}
          >
            <InputNumber precision={0} style={{ width: '100%' }} placeholder="正数入库，负数扣减" />
          </Form.Item>
          <Form.Item name="bucket_idx" label="库存分桶">
            <InputNumber min={0} precision={0} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="商品详情"
        open={detailOpen}
        footer={[
          <Button key="close" onClick={() => setDetailOpen(false)}>关闭</Button>,
          <Button
            key="security"
            disabled={!detail}
            onClick={() => {
              if (!detail) return;
              setDetailOpen(false);
              openSecurityLogs(detail.product_id);
            }}
          >
            安全日志
          </Button>,
          <Button
            key="stock"
            disabled={!detail}
            onClick={() => {
              if (!detail) return;
              openStock(detail);
            }}
          >
            调整库存
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
            {detail?.status === 1 ? '下架' : '上架'}
          </Button>,
          <Button
            key="promotions"
            disabled={!detail}
            onClick={() => {
              if (!detail) return;
              setDetailOpen(false);
              window.dispatchEvent(new CustomEvent('flash-admin:navigate', { detail: { path: '/admin/promotions', productId: detail.product_id } }));
            }}
          >
            查看促销
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
            <Descriptions.Item label="商品ID">{detail.product_id}</Descriptions.Item>
            <Descriptions.Item label="状态">{productStatusTag(detail)}</Descriptions.Item>
            <Descriptions.Item label="名称" span={2}>{detail.name || '-'}</Descriptions.Item>
            <Descriptions.Item label="供应商" span={2}>
              {detail.supplier_name ? `${detail.supplier_name} (${detail.supplier_id})` : detail.supplier_id || '-'}
            </Descriptions.Item>
            <Descriptions.Item label="原价">¥{formatPriceFen(detail.origin_price_fen)}</Descriptions.Item>
            <Descriptions.Item label="售价">¥{formatPriceFen(detail.sale_price_fen)}</Descriptions.Item>
            <Descriptions.Item label="当前价">
              ¥{formatPriceFen(detail.promotion_price_fen > 0 ? detail.promotion_price_fen : detail.sale_price_fen)}
            </Descriptions.Item>
            <Descriptions.Item label="库存">{detail.stock_available || 0}</Descriptions.Item>
            <Descriptions.Item label="活动类型">{detail.promotion_type || '-'}</Descriptions.Item>
            <Descriptions.Item label="活动标签">{detail.promotion_tag || '-'}</Descriptions.Item>
          </Descriptions>
        ) : detailLoading ? (
          <div>加载中...</div>
        ) : (
          <div>暂无商品详情</div>
        )}
      </Modal>
    </>
  );
}
