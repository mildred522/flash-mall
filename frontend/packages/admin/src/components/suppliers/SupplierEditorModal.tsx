import type { FormInstance } from 'antd';
import { Form, Input, Modal, Select } from 'antd';
import type { AdminSupplierItem } from '@flash-mall/shared';
import type { SupplierFormValues } from './supplierModel';

type Props = {
  open: boolean;
  editingSupplier: AdminSupplierItem | null;
  form: FormInstance<SupplierFormValues>;
  submitting: boolean;
  onCancel: () => void;
  onSave: () => void;
};

export default function SupplierEditorModal(props: Props) {
  const { open, editingSupplier, form, submitting, onCancel, onSave } = props;
  return (
    <Modal title={editingSupplier ? '编辑供应商' : '新增供应商'} open={open} onCancel={onCancel} onOk={onSave}
      confirmLoading={submitting} destroyOnHidden>
      <Form form={form} layout="vertical" preserve={false}>
        <Form.Item name="name" label="供应商名称" rules={[{ required: true, message: '请输入供应商名称' }]}>
          <Input maxLength={80} />
        </Form.Item>
        <Form.Item name="status" label="状态" rules={[{ required: true, message: '请选择状态' }]}>
          <Select options={[{ value: 1, label: '启用' }, { value: 2, label: '停用' }]} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
