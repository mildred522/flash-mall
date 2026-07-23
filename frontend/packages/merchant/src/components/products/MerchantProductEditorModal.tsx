import { Form, Image, Input, InputNumber, Modal, Space } from 'antd';
import type { FormInstance } from 'antd';
import { PRODUCT_IMAGE_FALLBACK_DATA_URI } from '@flash-mall/shared';
import type { AdminProductItem } from '@flash-mall/shared';
import type { ProductFormValues } from './productModel';

type Props = {
  form: FormInstance<ProductFormValues>;
  editing: AdminProductItem | null;
  open: boolean;
  saving: boolean;
  imageFile: File | null;
  onImageFile: (file: File | null) => void;
  onCancel: () => void;
  onSave: () => void;
};

export default function MerchantProductEditorModal(props: Props) {
  const watchedImageURL = Form.useWatch('image_url', props.form);

  return (
    <Modal
      title={props.editing ? '编辑商品' : '新增商品'}
      open={props.open}
      onCancel={props.onCancel}
      onOk={props.onSave}
      confirmLoading={props.saving}
      destroyOnHidden
    >
      <Form form={props.form} layout="vertical" preserve={false}>
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
            onChange={(event) => props.onImageFile(event.target.files?.[0] || null)}
          />
        </Form.Item>
        {(watchedImageURL || props.imageFile) && (
          <Space style={{ marginBottom: 16 }}>
            {watchedImageURL && <Image width={88} height={88} src={watchedImageURL} fallback={PRODUCT_IMAGE_FALLBACK_DATA_URI} alt="商品图片预览" />}
            {props.imageFile && <span>待上传：{props.imageFile.name}</span>}
          </Space>
        )}
        <Form.Item name="origin_price_fen" label="原价(分)" rules={[{ required: true }]}>
          <InputNumber min={0} precision={0} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item name="sale_price_fen" label="售价(分)" rules={[{ required: true }]}>
          <InputNumber min={0} precision={0} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item
          name="supplier_id"
          label="供应商ID"
          rules={[{ required: true }, {
            validator: (_, value) => value > 0
              ? Promise.resolve()
              : Promise.reject(new Error('供应商ID必须大于0')),
          }]}
        >
          <InputNumber min={1} precision={0} style={{ width: '100%' }} />
        </Form.Item>
        {!props.editing && (
          <Form.Item name="stock_available" label="初始库存">
            <InputNumber min={0} precision={0} style={{ width: '100%' }} />
          </Form.Item>
        )}
        <Form.Item name="status" label="状态" rules={[{ required: true }]}>
          <InputNumber min={1} max={2} precision={0} style={{ width: '100%' }} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
