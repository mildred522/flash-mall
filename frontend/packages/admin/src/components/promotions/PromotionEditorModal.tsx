import type { FormInstance } from 'antd';
import { Form, Input, InputNumber, Modal, Select } from 'antd';
import type { AdminProductItem, AdminPromotionItem } from '@flash-mall/shared';
import type { PromotionFormValues } from './promotionModel';

type Props = {
  open: boolean;
  editingPromotion: AdminPromotionItem | null;
  form: FormInstance<PromotionFormValues>;
  products: AdminProductItem[];
  submitting: boolean;
  onCancel: () => void;
  onSave: () => void;
};

export default function PromotionEditorModal(props: Props) {
  const { open, editingPromotion, form, products, submitting, onCancel, onSave } = props;
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

  return (
    <Modal title={editingPromotion ? '编辑促销' : '新增促销'} open={open} onCancel={onCancel} onOk={onSave}
      confirmLoading={submitting} destroyOnHidden>
      <Form form={form} layout="vertical" preserve={false}>
        <Form.Item name="product_id" label="商品" rules={[
          { required: true, message: '请选择商品' },
          { validator: (_, value) => value > 0 ? Promise.resolve() : Promise.reject(new Error('请选择商品')) },
        ]}>
          <Select showSearch optionFilterProp="label" options={productOptions.map((product) => ({
            value: product.product_id,
            label: `${product.name} (${product.product_id})`,
          }))} />
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
          <Select options={[{ value: 1, label: '启用' }, { value: 2, label: '停用' }]} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
