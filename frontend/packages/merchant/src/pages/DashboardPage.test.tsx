import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import DashboardPage from './DashboardPage';

const mocks = vi.hoisted(() => ({ authed: vi.fn() }));

vi.mock('@flash-mall/shared', async (importOriginal) => ({
  ...await importOriginal<typeof import('@flash-mall/shared')>(),
  authed: mocks.authed,
}));

describe('Merchant DashboardPage', () => {
  it('renders merchant-scoped operating statistics', async () => {
    mocks.authed.mockResolvedValue({
      ok: true,
      status: 200,
      data: {
        merchant_id: 1000,
        order_count: 12,
        paid_order_count: 8,
        ship_pending_count: 3,
        refund_pending_count: 1,
        sales_amount_fen: 123400,
      },
    });
    render(<DashboardPage />);

    expect(await screen.findByText('总订单')).toBeInTheDocument();
    expect(screen.getByText('待发货')).toBeInTheDocument();
    expect(screen.getByText('1,234.00')).toBeInTheDocument();
    expect(mocks.authed).toHaveBeenCalledWith('/api/merchant/dashboard/stats');
  });
});
