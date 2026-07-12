import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import PaymentPage from './PaymentPage';

describe('PaymentPage', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    window.history.replaceState({}, '', '/shop');
  });

  it('confirms a signed payment and keeps a repeat action for idempotency demonstration', async () => {
    window.history.replaceState({}, '', '/pay?token=signed-token');
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockImplementation(async (_input, init) => {
      const isConfirm = init?.method === 'POST';
      return new Response(JSON.stringify({
        code: 'OK',
        data: isConfirm
          ? { order_id: 'order-1', payment_order_id: 'pay:order-1', status: 'paid' }
          : { order_id: 'order-1', payment_order_id: 'pay:order-1', out_trade_no: 'sandbox-order-1', payable_amount_fen: 9900, status: 'pending', expires_at: 1_900_000_000 },
      }), { status: 200, headers: { 'Content-Type': 'application/json' } });
    });

    render(<PaymentPage />);
    await screen.findByText('¥99.00');
    fireEvent.click(screen.getByRole('button', { name: '确认付款' }));

    await screen.findByRole('button', { name: '再次提交确认（验证幂等）' });
    const confirmCall = fetchMock.mock.calls.find(([, init]) => init?.method === 'POST');
    expect(confirmCall).toBeTruthy();
    expect(JSON.parse(String(confirmCall?.[1]?.body))).toEqual({ token: 'signed-token' });

    fireEvent.click(screen.getByRole('button', { name: '再次提交确认（验证幂等）' }));
    await waitFor(() => expect(fetchMock.mock.calls.filter(([, init]) => init?.method === 'POST')).toHaveLength(2));
  });
});
