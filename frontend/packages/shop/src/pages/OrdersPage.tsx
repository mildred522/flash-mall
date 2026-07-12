import { useState, useEffect, useRef, useCallback } from 'react';
import { authed } from '@flash-mall/shared';
import type { OrderListItem, OrderListResp, ActionResp, PaymentIntentResp, PaymentStatusResp } from '@flash-mall/shared';
import OrderCard from '../components/OrderCard';
import PaymentModal from '../components/PaymentModal';

interface PendingOrder {
  orderId: string;
  requestId: string;
}

interface Props {
  pendingOrder?: PendingOrder | null;
  onPendingHandled?: () => void;
}

export default function OrdersPage({ pendingOrder, onPendingHandled }: Props) {
  const [orders, setOrders] = useState<OrderListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [banner, setBanner] = useState<{ orderId: string; status: number } | null>(null);
  const [paymentIntent, setPaymentIntent] = useState<PaymentIntentResp | null>(null);
  const [paymentState, setPaymentState] = useState('pending');
  const [paymentError, setPaymentError] = useState('');
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const loadOrders = useCallback(async () => {
    setLoading(true);
    const res = await authed<OrderListResp>('/api/orders');
    if (res.ok) setOrders(res.data.items || []);
    setLoading(false);
  }, []);

  useEffect(() => { loadOrders(); }, [loadOrders]);

  // Start polling when a pending order arrives
  useEffect(() => {
    if (!pendingOrder) return;
    const { orderId } = pendingOrder;
    setBanner({ orderId, status: 0 });
    let attempts = 0;
    pollRef.current = setInterval(async () => {
      attempts++;
      if (attempts > 30) {
        if (pollRef.current) clearInterval(pollRef.current);
        return;
      }
      try {
        const result = await authed<{ status: number }>(`/api/orders/detail?order_id=${encodeURIComponent(orderId)}`);
        if (!result.ok) return;
        const status = result.data.status;
        setBanner({ orderId, status });
        if (status !== 0) {
          if (pollRef.current) clearInterval(pollRef.current);
          await loadOrders();
          if (status === 1) {
            setTimeout(() => setBanner(null), 3000);
          }
        }
      } catch (_) { /* ignore */ }
    }, 1000);

    return () => {
      if (pollRef.current) clearInterval(pollRef.current);
    };
  }, [pendingOrder, loadOrders]);

  // Cleanup banner when pending is handled
  useEffect(() => {
    if (!pendingOrder && banner) setBanner(null);
  }, [pendingOrder]); // eslint-disable-line react-hooks/exhaustive-deps

  const handlePay = async (orderId: string) => {
    setBanner({ orderId, status: 0 });
    setPaymentError('');
    const intentResponse = await authed<PaymentIntentResp>('/api/order/pay', { method: 'POST', jsonBody: { order_id: orderId } });
    if (!intentResponse.ok) {
      setPaymentError('支付单拉起失败，请稍后重试');
      return;
    }
    const intent = intentResponse.data;
    if (intent.status === 'paid') {
      setBanner({ orderId, status: 1 });
      await loadOrders();
      return;
    }
    setPaymentIntent(intent);
    setPaymentState(intent.status || 'pending');

    let attempts = 0;
    if (pollRef.current) clearInterval(pollRef.current);
    pollRef.current = setInterval(async () => {
      attempts++;
      if (attempts > 900) {
        if (pollRef.current) clearInterval(pollRef.current);
        return;
      }
      try {
        const result = await authed<PaymentStatusResp>(`/api/payment/status?payment_order_id=${encodeURIComponent(intent.payment_order_id)}`);
        if (!result.ok) return;
        const status = result.data.status;
        setPaymentState(status);
        if (status === 'paid') {
          setBanner({ orderId, status: 1 });
          if (pollRef.current) clearInterval(pollRef.current);
          await loadOrders();
          setTimeout(() => setBanner(null), 3000);
        } else if (status === 'expired' || status === 'closed' || status === 'failed') {
          if (pollRef.current) clearInterval(pollRef.current);
        }
      } catch (_) { /* ignore */ }
    }, 1000);
  };

  const handleConfirm = async (orderId: string) => {
    await authed<ActionResp>('/api/order/confirm-receipt', { method: 'POST', jsonBody: { order_id: orderId } });
    loadOrders();
  };

  const handleRefund = async (orderId: string) => {
    await authed<ActionResp>('/api/order/refund', { method: 'POST', jsonBody: { order_id: orderId } });
    loadOrders();
  };

  const BANNER_MAP: Record<number, { text: string; cls: string; icon: string }> = {
    0: { text: '待支付', cls: 'pending', icon: '⏳' },
    1: { text: '已支付', cls: 'paid', icon: '✅' },
    2: { text: '已关闭', cls: 'closed', icon: '🔒' },
  };

  return (
    <div className="container">
      <section className="section">
        <div className="section-title">
          <h2>我的订单</h2>
          <button className="button soft" onClick={loadOrders}>刷新</button>
        </div>
        {banner && (() => {
          const b = BANNER_MAP[banner.status] || { text: '处理中', cls: 'pending', icon: '⏳' };
          return (
            <div className={`payment-banner ${b.cls}`}>
              <span className="payment-banner-icon">{b.icon}</span>
              <span className="payment-banner-text">订单 {banner.orderId} · {b.text}</span>
              {banner.status === 0 && (
                <button className="btn-pay" onClick={() => handlePay(banner.orderId)}>去付款</button>
              )}
            </div>
          );
        })()}
        {loading ? (
          <p style={{ textAlign: 'center', padding: 40 }}>加载中...</p>
        ) : orders.length === 0 ? (
          <div className="orders-empty">暂无订单，去逛逛吧</div>
        ) : (
          <div className="orders-list">
            {orders.map((order) => (
              <OrderCard
                key={order.order_id}
                order={order}
                onPay={handlePay}
                onConfirm={handleConfirm}
                onRefund={handleRefund}
              />
            ))}
          </div>
        )}
        {paymentError && <div className="payer-error">{paymentError}</div>}
      </section>
      {paymentIntent && (
        <PaymentModal intent={paymentIntent} status={paymentState} onClose={() => setPaymentIntent(null)} />
      )}
    </div>
  );
}
