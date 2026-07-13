import { useState, useEffect, useRef, useCallback } from 'react';
import { authed } from '@flash-mall/shared';
import type { OrderListItem, OrderListResp, ActionResp } from '@flash-mall/shared';
import OrderCard from '../components/OrderCard';

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
        const resp = await fetch(`/api/order/status?order_id=${orderId}`);
        if (!resp.ok) return;
        const data = await resp.json();
        const status = typeof data.status === 'string' ? parseInt(data.status, 10) : data.status;
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
    await authed<ActionResp>('/api/order/pay', { method: 'POST', jsonBody: { order_id: orderId } });
    // Start polling after pay attempt
    let attempts = 0;
    if (pollRef.current) clearInterval(pollRef.current);
    pollRef.current = setInterval(async () => {
      attempts++;
      if (attempts > 10) {
        if (pollRef.current) clearInterval(pollRef.current);
        await loadOrders();
        return;
      }
      try {
        const resp = await fetch(`/api/order/status?order_id=${orderId}`);
        if (!resp.ok) return;
        const data = await resp.json();
        const status = typeof data.status === 'string' ? parseInt(data.status, 10) : data.status;
        setBanner({ orderId, status });
        if (status !== 0) {
          if (pollRef.current) clearInterval(pollRef.current);
          await loadOrders();
          if (status === 1) setTimeout(() => setBanner(null), 3000);
        }
      } catch (_) { /* ignore */ }
    }, 1000);
  };

  const handleShip = async (orderId: string) => {
    await authed<ActionResp>('/api/order/ship', { method: 'POST', jsonBody: { order_id: orderId } });
    loadOrders();
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
                onShip={handleShip}
                onConfirm={handleConfirm}
                onRefund={handleRefund}
              />
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
