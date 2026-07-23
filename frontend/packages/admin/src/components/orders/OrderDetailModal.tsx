import { Button, Descriptions, Modal, Popconfirm } from 'antd';
import { formatPriceFen } from '@flash-mall/shared';
import type { OrderDetailResp } from '@flash-mall/shared';
import { OrderStatusTag } from './orderModel';

type Props = {
  open: boolean;
  loading: boolean;
  detail: OrderDetailResp | null;
  onClose: () => void;
  onSecurity: (orderId: string) => void;
  onShip: (orderId: string) => void;
  onRefund: (orderId: string) => void;
  onNavigate: (path: string, key: 'userId' | 'productId', value: number) => void;
};

export default function OrderDetailModal(props: Props) {
  const { open, loading, detail, onClose, onSecurity, onShip, onRefund, onNavigate } = props;
  const closeThen = (action: () => void) => {
    onClose();
    action();
  };
  return (
    <Modal title="订单详情" open={open} onCancel={onClose} loading={loading} width={760} footer={[
      <Button key="close" onClick={onClose}>关闭</Button>,
      <Button key="security" disabled={!detail?.order_id}
        onClick={() => detail?.order_id && closeThen(() => onSecurity(detail.order_id))}>安全日志</Button>,
      detail?.status === 1 && (
        <Popconfirm key="ship" title="确认发货？" onConfirm={() => onShip(detail.order_id)}><Button>发货</Button></Popconfirm>
      ),
      (detail?.status === 0 || detail?.status === 1) && (
        <Popconfirm key="refund" title="确认退款？" onConfirm={() => onRefund(detail.order_id)}><Button danger>退款</Button></Popconfirm>
      ),
      <Button key="user" disabled={!detail?.user_id}
        onClick={() => detail?.user_id && closeThen(() => onNavigate('/admin/users', 'userId', detail.user_id))}>查看用户</Button>,
      <Button key="product" type="primary" disabled={!detail?.product_id}
        onClick={() => detail?.product_id && closeThen(() => onNavigate('/admin/products', 'productId', detail.product_id))}>查看商品</Button>,
    ]}>
      {detail && (
        <Descriptions column={2} bordered size="small">
          <Descriptions.Item label="订单号" span={2}>{detail.order_id}</Descriptions.Item>
          <Descriptions.Item label="用户ID">{detail.user_id || '-'}</Descriptions.Item>
          <Descriptions.Item label="商品ID">{detail.product_id}</Descriptions.Item>
          <Descriptions.Item label="商品">{detail.product_name}</Descriptions.Item>
          <Descriptions.Item label="数量">{detail.amount}</Descriptions.Item>
          <Descriptions.Item label="订单状态"><OrderStatusTag status={detail.status} fallback={detail.status_text} /></Descriptions.Item>
          <Descriptions.Item label="原单价">¥{formatPriceFen(detail.origin_unit_price_fen)}</Descriptions.Item>
          <Descriptions.Item label="活动单价">¥{formatPriceFen(detail.sale_unit_price_fen)}</Descriptions.Item>
          <Descriptions.Item label="优惠">¥{formatPriceFen(detail.discount_amount_fen)}</Descriptions.Item>
          <Descriptions.Item label="应付">¥{formatPriceFen(detail.payable_amount_fen)}</Descriptions.Item>
          <Descriptions.Item label="支付单号">{detail.payment_order_id || '-'}</Descriptions.Item>
          <Descriptions.Item label="支付状态">{detail.payment_status_text || detail.payment_status || '-'}</Descriptions.Item>
          <Descriptions.Item label="活动类型">{detail.promotion_type || '-'}</Descriptions.Item>
          <Descriptions.Item label="活动标签">{detail.promotion_tag || '-'}</Descriptions.Item>
          <Descriptions.Item label="下单时间" span={2}>{detail.create_time}</Descriptions.Item>
        </Descriptions>
      )}
    </Modal>
  );
}
