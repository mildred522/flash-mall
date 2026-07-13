import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import OrdersPage from './OrdersPage';

const mocks = vi.hoisted(() => ({ authed: vi.fn() }));

vi.mock('@flash-mall/shared', async (importOriginal) => ({
  ...await importOriginal<typeof import('@flash-mall/shared')>(),
  authed: mocks.authed,
}));

describe('Merchant OrdersPage', () => {
  it('ships paid orders without exposing platform actions', async () => {
    mocks.authed.mockImplementation(async (path: string) => {
      if (path.startsWith('/api/admin/')) throw new Error(`merchant page called admin API: ${path}`);
      if (path.startsWith('/api/merchant/orders?')) {
        return {
          ok: true,
          status: 200,
          data: { items: [{
            order_id: 'order-1', user_id: 1001, product_id: 100, product_name: '商家风衣',
            amount: 1, status: 1, status_text: 'paid', payable_amount_fen: 9900,
            create_time: '2026-07-13 12:00:00',
          }], total: 1 },
        };
      }
      if (path === '/api/merchant/orders/ship') return { ok: true, status: 200, data: { order_id: 'order-1', status: 'shipped' } };
      return { ok: true, status: 200, data: {} };
    });
    const user = userEvent.setup();
    render(<OrdersPage />);

    await screen.findByText('order-1');
    expect(screen.queryByRole('button', { name: '退款' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: '关闭' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: '发货' }));
    await user.click(await screen.findByRole('button', { name: 'OK' }));

    await waitFor(() => expect(mocks.authed).toHaveBeenCalledWith(
      '/api/merchant/orders/ship',
      { method: 'POST', jsonBody: { order_id: 'order-1' } },
    ));
  });
});
