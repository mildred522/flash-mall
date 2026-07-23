import type { FormInstance } from 'antd';
import { Form, Input, InputNumber, Modal, Select } from 'antd';
import type { AdminProductItem, AdminSupplierItem } from '@flash-mall/shared';
import type { ProductFormValues } from './productModel';
import ProductThumbnail from './ProductThumbnail';

type Props = {
  open: boolean;
  editingProduct: AdminProductItem | null;
  form: FormInstance<ProductFormValues>;
  suppliers: AdminSupplierItem[];
  selectedImageFile: File | null;
  submitting: boolean;
  onCancel: () => void;
  onSave: () => void;
  onImageSelected: (file: File | null) => void;
};

export default function ProductEditorModal(props: Props) {
  const { open, editingProduct, form, suppliers, selectedImageFile, submitting, onCancel, onSave, onImageSelected } = props;
  const watchedImageURL = Form.useWatch('image_url', form);
  const supplierOptions = [...suppliers];
  if (editingProduct && editingProduct.supplier_id > 0 && !supplierOptions.some((item) => item.supplier_id === editingProduct.supplier_id)) {
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
    <Modal title={editingProduct ? '编辑商品' : '新增商品'} open={open} onCancel={onCancel} onOk={onSave} confirmLoading={submitting} destroyOnHidden>
      <Form form={form} layout="vertical" preserve={false}>
        <Form.Item name="name" label="商品名称" rules={[{ required: true, message: '请输入商品名称' }]}>
          <Input maxLength={80} />
        </Form.Item>
        <Form.Item name="image_url" label="图片地址">
          <Input placeholder="https://... 或上传本地图片" />
        </Form.Item>
        <Form.Item label="上传图片">
          <input id="admin-product-image-file" aria-label="上传图片" type="file" accept="image/jpeg,image/png,image/webp,image/gif"
            onChange={(event) => onImageSelected(event.target.files?.[0] || null)} />
        </Form.Item>
        {(watchedImageURL || selectedImageFile) && (
          <div style={{ display: 'flex', gap: 12, alignItems: 'center', marginBottom: 16 }}>
            {watchedImageURL && <ProductThumbnail src={watchedImageURL} alt="商品图片预览" width={96} height={96} />}
            {selectedImageFile && <span>待上传：{selectedImageFile.name}</span>}
          </div>
        )}
        <Form.Item name="origin_price_fen" label="原价(分)" rules={[{ required: true, message: '请输入原价' }]}>
          <InputNumber min={0} precision={0} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item name="sale_price_fen" label="售价(分)" rules={[{ required: true, message: '请输入售价' }]}>
          <InputNumber min={0} precision={0} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item name="supplier_id" label="供应商" rules={[
          { required: true, message: '请选择供应商' },
          { validator: (_, value) => value > 0 ? Promise.resolve() : Promise.reject(new Error('请选择供应商')) },
        ]}>
          <Select showSearch optionFilterProp="label" options={supplierOptions.map((supplier) => ({
            value: supplier.supplier_id,
            label: `${supplier.name} (${supplier.supplier_id})`,
          }))} />
        </Form.Item>
        {!editingProduct && (
          <Form.Item name="stock_available" label="初始库存">
            <InputNumber min={0} precision={0} style={{ width: '100%' }} />
          </Form.Item>
        )}
        <Form.Item name="status" label="状态" rules={[{ required: true, message: '请选择状态' }]}>
          <Select options={[{ value: 1, label: '上架' }, { value: 2, label: '下架' }]} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
