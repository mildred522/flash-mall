import type { FormInstance } from 'antd';
import { Form, InputNumber, Modal } from 'antd';
import type { AdminProductItem } from '@flash-mall/shared';
import type { StockFormValues } from './productModel';

type Props = {
  open: boolean;
  product: AdminProductItem | null;
  form: FormInstance<StockFormValues>;
  submitting: boolean;
  onCancel: () => void;
  onSave: () => void;
};

export default function StockAdjustModal({ open, product, form, submitting, onCancel, onSave }: Props) {
  return (
    <Modal title={product ? `调整库存：${product.name}` : '调整库存'} open={open} onCancel={onCancel} onOk={onSave} confirmLoading={submitting} destroyOnHidden>
      <Form form={form} layout="vertical" preserve={false}>
        <Form.Item name="delta" label="库存变化量" rules={[
          { required: true, message: '请输入库存变化量' },
          { validator: (_, value) => value === 0 ? Promise.reject(new Error('库存变化量不能为 0')) : Promise.resolve() },
        ]}>
          <InputNumber precision={0} style={{ width: '100%' }} placeholder="正数入库，负数扣减" />
        </Form.Item>
        <Form.Item name="bucket_idx" label="库存分桶">
          <InputNumber min={0} precision={0} style={{ width: '100%' }} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
