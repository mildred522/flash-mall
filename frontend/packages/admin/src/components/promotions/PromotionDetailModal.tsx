import { Button, Descriptions, Modal } from 'antd';
import { formatPriceFen } from '@flash-mall/shared';
import type { AdminPromotionItem } from '@flash-mall/shared';
import { PromotionStatusTag, promotionTypeText } from './promotionModel';

type Props = {
  open: boolean;
  detail: AdminPromotionItem | null;
  loading: boolean;
  onClose: () => void;
  onSecurity: (promotionId: number) => void;
  onEdit: (promotion: AdminPromotionItem) => void;
  onToggle: (promotion: AdminPromotionItem) => void;
  onNavigate: (path: string, productId: number) => void;
};

export default function PromotionDetailModal(props: Props) {
  const { open, detail, loading, onClose, onSecurity, onEdit, onToggle, onNavigate } = props;
  const closeThen = (action: () => void) => {
    onClose();
    action();
  };

  return (
    <Modal title="促销详情" open={open} destroyOnHidden onCancel={onClose} footer={[
      <Button key="close" onClick={onClose}>关闭</Button>,
      <Button key="security" disabled={!detail} onClick={() => detail && closeThen(() => onSecurity(detail.promotion_id))}>安全日志</Button>,
      <Button key="edit" disabled={!detail} onClick={() => detail && closeThen(() => onEdit(detail))}>编辑</Button>,
      <Button key="status" danger={detail?.status === 1} disabled={!detail} onClick={() => detail && onToggle(detail)}>
        {detail?.status === 1 ? '停用' : '启用'}
      </Button>,
      <Button key="product" disabled={!detail} onClick={() => detail && closeThen(() => onNavigate('/admin/products', detail.product_id))}>
        查看商品
      </Button>,
      <Button key="orders" type="primary" disabled={!detail} onClick={() => detail && closeThen(() => onNavigate('/admin/orders', detail.product_id))}>
        查看订单
      </Button>,
    ]}>
      {detail ? (
        <Descriptions column={2} bordered size="small">
          <Descriptions.Item label="规则ID">{detail.promotion_id}</Descriptions.Item>
          <Descriptions.Item label="状态"><PromotionStatusTag status={detail.status} /></Descriptions.Item>
          <Descriptions.Item label="商品" span={2}>{`${detail.product_name || '商品'} (${detail.product_id})`}</Descriptions.Item>
          <Descriptions.Item label="类型">{promotionTypeText(detail.type)}</Descriptions.Item>
          <Descriptions.Item label="时间态">{detail.effect_status_text || '-'}</Descriptions.Item>
          <Descriptions.Item label="商品售价">¥{formatPriceFen(detail.sale_price_fen)}</Descriptions.Item>
          <Descriptions.Item label="限时价">¥{formatPriceFen(detail.discount_value)}</Descriptions.Item>
          <Descriptions.Item label="开始时间">{detail.starts_at || '-'}</Descriptions.Item>
          <Descriptions.Item label="结束时间">{detail.ends_at || '-'}</Descriptions.Item>
        </Descriptions>
      ) : loading ? <div>加载中...</div> : <div>暂无促销详情</div>}
    </Modal>
  );
}
