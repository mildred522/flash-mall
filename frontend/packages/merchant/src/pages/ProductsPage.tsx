import { useEffect, useState } from 'react';
import { Button, Form, Table, message } from 'antd';
import { PlusOutlined } from '@ant-design/icons';
import { authed, uploadProductImage } from '@flash-mall/shared';
import type {
  AdminMutationResp,
  AdminProductCreateReq,
  AdminProductCreateResp,
  AdminProductItem,
  AdminProductListResp,
  AdminProductStockAdjustResp,
  AdminProductUpdateReq,
} from '@flash-mall/shared';
import MerchantProductEditorModal from '../components/products/MerchantProductEditorModal';
import StockAdjustModal from '../components/products/StockAdjustModal';
import { createMerchantProductColumns } from '../components/products/productColumns';
import {
  productMutationError,
  type ProductFormValues,
  type StockFormValues,
} from '../components/products/productModel';

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
      const error = productMutationError(response.data);
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
        message.error(productMutationError(response.data) || '库存调整失败');
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
        columns={createMerchantProductColumns({ edit: openEdit, stock: openStock })}
      />
      <MerchantProductEditorModal
        form={productForm}
        editing={editing}
        open={productOpen}
        saving={saving}
        imageFile={imageFile}
        onImageFile={setImageFile}
        onCancel={() => setProductOpen(false)}
        onSave={saveProduct}
      />
      <StockAdjustModal
        form={stockForm}
        product={stockProduct}
        open={stockOpen}
        saving={saving}
        onCancel={() => setStockOpen(false)}
        onSave={adjustStock}
      />
    </>
  );
}
