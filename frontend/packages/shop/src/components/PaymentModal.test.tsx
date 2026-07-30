import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import PaymentModal from './PaymentModal';

describe('PaymentModal', () => {
  it('renders the exact QR URL and payment facts', () => {
    const qrURL = 'http://192.168.1.8:8889/pay?token=signed-token';
    render(<PaymentModal intent={{
      order_id: 'order-1',
      payment_order_id: 'pay:order-1',
      out_trade_no: 'sandbox-order-1',
      payable_amount_fen: 9900,
      status: 'pending',
      provider: 'alipay_sandbox',
      qr_url: qrURL,
      expires_at: 1_900_000_000,
    }} status="pending" onClose={vi.fn()} />);

    expect(screen.getByTitle('支付二维码')).toBeInTheDocument();
    expect(screen.getByText('¥99.00')).toBeInTheDocument();
    expect(screen.getByText('order-1')).toBeInTheDocument();
    expect(screen.getByText('支付宝沙箱支付')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: '在本机打开付款页' })).toHaveAttribute('href', qrURL);
  });
});
