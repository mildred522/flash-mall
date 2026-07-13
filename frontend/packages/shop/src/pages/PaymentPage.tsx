import { useCallback, useEffect, useState } from 'react';
import { api, formatPriceFen } from '@flash-mall/shared';
import type { ActionResp, PaymentStatusResp } from '@flash-mall/shared';

export default function PaymentPage() {
  const token = new URLSearchParams(window.location.search).get('token')?.trim() || '';
  const [payment, setPayment] = useState<PaymentStatusResp | null>(null);
  const [loading, setLoading] = useState(true);
  const [confirming, setConfirming] = useState(false);
  const [error, setError] = useState('');

  const loadStatus = useCallback(async () => {
    if (!token) {
      setError('支付令牌缺失');
      setLoading(false);
      return;
    }
    const response = await api<PaymentStatusResp>(`/api/payment/status?token=${encodeURIComponent(token)}`);
    if (response.ok) {
      setPayment(response.data);
      setError('');
    } else {
      setError(response.status === 401 ? '支付令牌无效' : '支付状态加载失败');
    }
    setLoading(false);
  }, [token]);

  useEffect(() => { void loadStatus(); }, [loadStatus]);

  const confirmPayment = async () => {
    setConfirming(true);
    setError('');
    const response = await api<ActionResp>('/api/payment/sandbox/confirm', { method: 'POST', jsonBody: { token } });
    setConfirming(false);
    if (!response.ok) {
      setError(response.status === 409 ? '二维码已过期，请回到商城重新拉起支付' : '付款确认失败，请重试');
      return;
    }
    setPayment((current) => current ? { ...current, status: 'paid' } : current);
  };

  return (
    <main className="payer-page">
      <section className="payer-card">
        <p className="payment-modal-kicker">FLASH MALL · SANDBOX PAY</p>
        <h1>确认沙箱付款</h1>
        {loading ? <p>正在校验支付令牌...</p> : error && !payment ? <div className="payer-error">{error}</div> : payment && (
          <>
            <div className="payer-amount">¥{formatPriceFen(payment.payable_amount_fen)}</div>
            <dl className="payment-facts">
              <div><dt>订单号</dt><dd>{payment.order_id}</dd></div>
              <div><dt>支付单</dt><dd>{payment.payment_order_id}</dd></div>
              <div><dt>当前状态</dt><dd>{payment.status}</dd></div>
            </dl>
            {error && <div className="payer-error">{error}</div>}
            {payment.status === 'expired' ? (
              <p className="payer-error">二维码已过期，请回到商城重新获取。</p>
            ) : (
              <button className="button primary payer-confirm" type="button" onClick={confirmPayment} disabled={confirming}>
                {confirming ? '正在确认...' : payment.status === 'paid' ? '再次提交确认（验证幂等）' : '确认付款'}
              </button>
            )}
            {payment.status === 'paid' && <p className="payer-success">支付成功。重复提交不会重复扣库存或发布订单事件。</p>}
          </>
        )}
      </section>
    </main>
  );
}
