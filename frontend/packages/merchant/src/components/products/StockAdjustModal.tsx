import { Form, InputNumber, Modal } from 'antd';
import type { FormInstance } from 'antd';
import type { AdminProductItem } from '@flash-mall/shared';
import type { StockFormValues } from './productModel';

type Props = {
  form: FormInstance<StockFormValues>;
  product: AdminProductItem | null;
  open: boolean;
  saving: boolean;
  onCancel: () => void;
  onSave: () => void;
};

export default function StockAdjustModal(props: Props) {
  return (
    <Modal
      title={props.product ? `调整库存：${props.product.name}` : '调整库存'}
      open={props.open}
      onCancel={props.onCancel}
      onOk={props.onSave}
      confirmLoading={props.saving}
      destroyOnHidden
    >
      <Form form={props.form} layout="vertical" preserve={false}>
        <Form.Item
          name="delta"
          label="库存变化量"
          rules={[{ required: true }, {
            validator: (_, value) => value !== 0
              ? Promise.resolve()
              : Promise.reject(new Error('变化量不能为0')),
          }]}
        >
          <InputNumber precision={0} style={{ width: '100%' }} />
        </Form.Item>
        <Form.Item name="bucket_idx" label="库存分桶">
          <InputNumber min={0} precision={0} style={{ width: '100%' }} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
