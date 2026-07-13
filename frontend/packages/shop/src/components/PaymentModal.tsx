import { QRCodeSVG } from '@rc-component/qrcode';
import { formatPriceFen } from '@flash-mall/shared';
import type { PaymentIntentResp } from '@flash-mall/shared';

interface Props {
  intent: PaymentIntentResp;
  status: string;
  onClose: () => void;
}

export default function PaymentModal({ intent, status, onClose }: Props) {
  const expiresAt = new Date(intent.expires_at * 1000).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
  return (
    <div className="payment-modal-backdrop" role="presentation">
      <section className="payment-modal" role="dialog" aria-modal="true" aria-labelledby="payment-title">
        <button className="payment-modal-close" type="button" aria-label="关闭付款窗口" onClick={onClose}>×</button>
        <p className="payment-modal-kicker">FLASH MALL SANDBOX</p>
        <h2 id="payment-title">扫码完成付款</h2>
        <div className="payment-amount">¥{formatPriceFen(intent.payable_amount_fen)}</div>
        <div className="payment-qr-shell">
          <QRCodeSVG value={intent.qr_url} size={220} level="M" marginSize={3} title="支付二维码" />
        </div>
        <div className={`payment-live-status ${status}`}>{status === 'paid' ? '支付成功' : status === 'expired' ? '二维码已过期' : '等待扫码确认'}</div>
        <dl className="payment-facts">
          <div><dt>订单号</dt><dd>{intent.order_id}</dd></div>
          <div><dt>支付单</dt><dd>{intent.payment_order_id}</dd></div>
          <div><dt>有效期至</dt><dd>{expiresAt}</dd></div>
        </dl>
        <a className="button primary payment-open-link" href={intent.qr_url} target="_blank" rel="noreferrer">在本机打开付款页</a>
        <p className="payment-modal-note">重复确认会复用同一支付事件，不会重复扣减库存。</p>
      </section>
    </div>
  );
}
