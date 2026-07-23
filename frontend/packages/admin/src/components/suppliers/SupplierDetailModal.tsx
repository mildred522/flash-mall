import { Button, Descriptions, Modal } from 'antd';
import type { AdminSupplierItem } from '@flash-mall/shared';
import { SupplierStatusTag } from './supplierModel';

type Props = {
  open: boolean;
  detail: AdminSupplierItem | null;
  loading: boolean;
  onClose: () => void;
  onSecurity: (supplierId: number) => void;
  onEdit: (supplier: AdminSupplierItem) => void;
  onToggle: (supplier: AdminSupplierItem) => void;
  onProducts: (supplierId: number) => void;
};

export default function SupplierDetailModal(props: Props) {
  const { open, detail, loading, onClose, onSecurity, onEdit, onToggle, onProducts } = props;
  const closeThen = (action: () => void) => {
    onClose();
    action();
  };
  return (
    <Modal title="供应商详情" open={open} destroyOnHidden onCancel={onClose} footer={[
      <Button key="close" onClick={onClose}>关闭</Button>,
      <Button key="security" disabled={!detail}
        onClick={() => detail && closeThen(() => onSecurity(detail.supplier_id))}>安全日志</Button>,
      <Button key="edit" disabled={!detail} onClick={() => detail && closeThen(() => onEdit(detail))}>编辑</Button>,
      <Button key="status" danger={detail?.status === 1} disabled={!detail} onClick={() => detail && onToggle(detail)}>
        {detail?.status === 1 ? '停用' : '启用'}
      </Button>,
      <Button key="products" type="primary" disabled={!detail}
        onClick={() => detail && closeThen(() => onProducts(detail.supplier_id))}>查看商品</Button>,
    ]}>
      {detail ? (
        <Descriptions column={2} bordered size="small">
          <Descriptions.Item label="供应商ID">{detail.supplier_id}</Descriptions.Item>
          <Descriptions.Item label="状态"><SupplierStatusTag status={detail.status} /></Descriptions.Item>
          <Descriptions.Item label="名称" span={2}>{detail.name || '-'}</Descriptions.Item>
          <Descriptions.Item label="商品数">{detail.product_count || 0}</Descriptions.Item>
          <Descriptions.Item label="启用商品">{detail.active_products || 0}</Descriptions.Item>
        </Descriptions>
      ) : loading ? <div>加载中...</div> : <div>暂无供应商详情</div>}
    </Modal>
  );
}
