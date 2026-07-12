import { useState } from 'react';
import { STATUS_MAP, formatPriceFen, PRODUCT_META, resolveProductImage } from '@flash-mall/shared';
import type { OrderListItem } from '@flash-mall/shared';

interface Props {
  order: OrderListItem;
  onPay: (orderId: string) => void;
  onConfirm: (orderId: string) => void;
  onRefund: (orderId: string) => void;
}

export default function OrderCard({ order, onPay, onConfirm, onRefund }: Props) {
  const st = STATUS_MAP[order.status] || { text: '未知', cls: 'unknown' };
  const meta = PRODUCT_META[order.product_id];
  const icon = meta?.icon || '📦';
  const imageURL = resolveProductImage(order.product_id);
  const [imageFailed, setImageFailed] = useState(false);

  return (
    <div className="order-card">
      <div className="order-card-thumb">
        {imageURL && !imageFailed ? (
          <img src={imageURL} alt={order.product_name || '商品图片'} loading="lazy" onError={() => setImageFailed(true)} />
        ) : (
          <span style={{ fontSize: 32 }}>{icon}</span>
        )}
      </div>
      <div className="order-card-info">
        <div className="order-card-name">{order.product_name || '商品'}</div>
        <div className="order-card-meta">
          <span>数量: {order.amount}</span>
          <span>订单号: {order.order_id}</span>
        </div>
        <div className="order-card-time">{order.create_time}</div>
      </div>
      <div className="order-card-right">
        <div className="order-card-price">¥{formatPriceFen(order.payable_amount_fen)}</div>
        <span className={`order-status-badge ${st.cls}`}>{st.text}</span>
        {order.status === 0 && <button className="btn-pay" onClick={() => onPay(order.order_id)}>去付款</button>}
        {order.status === 3 && <button className="btn-confirm" onClick={() => onConfirm(order.order_id)}>确认收货</button>}
        {(order.status === 0 || order.status === 1) && (
          <button className="btn-refund" onClick={() => onRefund(order.order_id)}>申请退款</button>
        )}
      </div>
    </div>
  );
}
