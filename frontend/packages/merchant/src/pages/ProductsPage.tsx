import { useEffect, useState } from 'react';
import { Button, Form, Image, Input, InputNumber, Modal, Space, Table, Tag, message } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { authed, formatPriceFen, uploadProductImage } from '@flash-mall/shared';
import type {
  AdminMutationResp,
  AdminProductCreateReq,
  AdminProductCreateResp,
  AdminProductItem,
  AdminProductListResp,
  AdminProductStockAdjustResp,
  AdminProductUpdateReq,
} from '@flash-mall/shared';

type ProductFormValues = {
  name: string;
  image_url?: string;
  origin_price_fen: number;
  sale_price_fen: number;
  supplier_id: number;
  stock_available?: number;
  status: number;
};

type StockFormValues = { delta: number; bucket_idx: number };

function errorText(data: AdminMutationResp): string {
  if (data.error === 'sale_price_fen must be <= origin_price_fen') return '售价不能高于原价';
  if (data.error === 'active supplier not found') return '供应商不可用';
  return data.error || '';
}

export default function ProductsPage() {
  const [products, setProducts] = useState<AdminProductItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [productOpen, setProductOpen] = useState(false);
  const [stockOpen, setStockOpen] = useState(false);
  const [editing, setEditing] = useState<AdminProductItem | null>(null);
  const [stockProduct, setStockProduct] = useState<AdminProductItem | null>(null);
  const [imageFile, setImageFile] = useState<File | null>(null);
  const [productForm] = Form.useForm<ProductFormValues>();
  const [stockForm] = Form.useForm<StockFormValues>();
  const watchedImageURL = Form.useWatch('image_url', productForm);

  const loadProducts = async () => {
    setLoading(true);
    const response = await authed<AdminProductListResp>('/api/merchant/products?page=1&page_size=100&status=-1');
    if (response.ok) setProducts(response.data.items || []);
    else message.error('商品列表加载失败');
    setLoading(false);
  };

  useEffect(() => { void loadProducts(); }, []);

  const openCreate = () => {
    setEditing(null);
    setImageFile(null);
    productForm.setFieldsValue({
      name: '', image_url: '', origin_price_fen: 0, sale_price_fen: 0,
      supplier_id: 0, stock_available: 0, status: 1,
    });
    setProductOpen(true);
  };

  const openEdit = (product: AdminProductItem) => {
    setEditing(product);
    setImageFile(null);
    productForm.setFieldsValue({
      name: product.name,
      image_url: product.image_url || '',
      origin_price_fen: product.origin_price_fen,
      sale_price_fen: product.sale_price_fen,
      supplier_id: product.supplier_id,
      status: product.status,
    });
    setProductOpen(true);
  };

  const saveProduct = async () => {
    const values = await productForm.validateFields();
    if (values.sale_price_fen > values.origin_price_fen) {
      message.warning('售价不能高于原价');
      return;
    }
    setSaving(true);
    try {
      let imageURL = values.image_url?.trim() || '';
      if (imageFile) imageURL = await uploadProductImage(imageFile, '/api/merchant/products/image');
      const jsonBody: AdminProductCreateReq | AdminProductUpdateReq = editing
        ? { product_id: editing.product_id, ...values, image_url: imageURL }
        : { ...values, image_url: imageURL };
      const endpoint = editing ? '/api/merchant/products/update' : '/api/merchant/products/create';
      const response = await authed<AdminMutationResp | AdminProductCreateResp>(endpoint, { method: 'POST', jsonBody });
      const error = errorText(response.data);
      if (!response.ok || error) {
        message.error(error || '商品保存失败');
        return;
      }
      message.success(editing ? '商品已更新' : '商品已创建');
      setProductOpen(false);
      await loadProducts();
    } catch (error) {
      message.error(error instanceof Error ? error.message : '商品保存失败');
    } finally {
      setSaving(false);
    }
  };

  const openStock = (product: AdminProductItem) => {
    setStockProduct(product);
    stockForm.setFieldsValue({ delta: 0, bucket_idx: 0 });
    setStockOpen(true);
  };

  const adjustStock = async () => {
    if (!stockProduct) return;
    const values = await stockForm.validateFields();
    setSaving(true);
    try {
      const response = await authed<AdminProductStockAdjustResp>('/api/merchant/products/stock-adjust', {
        method: 'POST', jsonBody: { product_id: stockProduct.product_id, ...values },
      });
      if (!response.ok) {
        message.error(errorText(response.data) || '库存调整失败');
        return;
      }
      message.success(`库存已更新为 ${response.data.stock_available}`);
      setStockOpen(false);
      await loadProducts();
    } finally {
      setSaving(false);
    }
  };

  return (
    <>
      <Table<AdminProductItem>
        rowKey="product_id"
        loading={loading}
        dataSource={products}
        pagination={{ pageSize: 20 }}
        title={() => (
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <strong>本店商品</strong>
            <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新增商品</Button>
          </div>
        )}
        columns={[
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
                <Button type="link" onClick={() => openEdit(row)}>编辑</Button>
                <Button type="link" onClick={() => openStock(row)}>调整库存</Button>
              </Space>
            ),
          },
        ]}
      />

      <Modal
        title={editing ? '编辑商品' : '新增商品'}
        open={productOpen}
        onCancel={() => setProductOpen(false)}
        onOk={saveProduct}
        confirmLoading={saving}
        destroyOnHidden
      >
        <Form form={productForm} layout="vertical" preserve={false}>
          <Form.Item name="name" label="商品名称" rules={[{ required: true, message: '请输入商品名称' }]}>
            <Input maxLength={80} />
          </Form.Item>
          <Form.Item name="image_url" label="图片地址">
            <Input placeholder="https://... 或上传本地图片" />
          </Form.Item>
          <Form.Item label="上传图片">
            <input
              aria-label="上传图片"
              type="file"
              accept="image/jpeg,image/png,image/webp,image/gif"
              onChange={(event) => setImageFile(event.target.files?.[0] || null)}
            />
          </Form.Item>
          {(watchedImageURL || imageFile) && (
            <Space style={{ marginBottom: 16 }}>
              {watchedImageURL && <Image width={88} height={88} src={watchedImageURL} alt="商品图片预览" />}
              {imageFile && <span>待上传：{imageFile.name}</span>}
            </Space>
          )}
          <Form.Item name="origin_price_fen" label="原价(分)" rules={[{ required: true }]}>
            <InputNumber min={0} precision={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="sale_price_fen" label="售价(分)" rules={[{ required: true }]}>
            <InputNumber min={0} precision={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="supplier_id" label="供应商ID" rules={[{ required: true }, { validator: (_, value) => value > 0 ? Promise.resolve() : Promise.reject(new Error('供应商ID必须大于0')) }]}>
            <InputNumber min={1} precision={0} style={{ width: '100%' }} />
          </Form.Item>
          {!editing && (
            <Form.Item name="stock_available" label="初始库存">
              <InputNumber min={0} precision={0} style={{ width: '100%' }} />
            </Form.Item>
          )}
          <Form.Item name="status" label="状态" rules={[{ required: true }]}>
            <InputNumber min={1} max={2} precision={0} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={stockProduct ? `调整库存：${stockProduct.name}` : '调整库存'}
        open={stockOpen}
        onCancel={() => setStockOpen(false)}
        onOk={adjustStock}
        confirmLoading={saving}
        destroyOnHidden
      >
        <Form form={stockForm} layout="vertical" preserve={false}>
          <Form.Item name="delta" label="库存变化量" rules={[{ required: true }, { validator: (_, value) => value !== 0 ? Promise.resolve() : Promise.reject(new Error('变化量不能为0')) }]}>
            <InputNumber precision={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="bucket_idx" label="库存分桶">
            <InputNumber min={0} precision={0} style={{ width: '100%' }} />
          </Form.Item>
        </Form>
      </Modal>
    </>
  );
}
