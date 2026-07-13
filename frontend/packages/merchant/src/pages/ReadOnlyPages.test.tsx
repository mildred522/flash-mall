import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import InventoryPage from './InventoryPage';
import RefundsPage from './RefundsPage';

const mocks = vi.hoisted(() => ({ authed: vi.fn() }));

vi.mock('@flash-mall/shared', async (importOriginal) => ({
  ...await importOriginal<typeof import('@flash-mall/shared')>(),
  authed: mocks.authed,
}));

describe('merchant read-only pages', () => {
  it('shows merchant inventory audit records', async () => {
    mocks.authed.mockResolvedValue({ ok: true, status: 200, data: { items: [{
      id: 1, product_id: 100, order_id: '', change_type: 'ADJUST', delta: 5,
      before_available: 10, after_available: 15, reason: 'merchant stock adjust',
      request_id: 'req-1', create_time: '2026-07-13 12:00:00',
    }], total: 1 } });
    render(<InventoryPage />);
    expect(await screen.findByText('ADJUST')).toBeInTheDocument();
    expect(screen.getByText('req-1')).toBeInTheDocument();
    expect(mocks.authed).toHaveBeenCalledWith('/api/merchant/inventory/stock-changes?page=1&page_size=100');
  });

  it('shows refunds without audit actions', async () => {
    mocks.authed.mockResolvedValue({ ok: true, status: 200, data: { items: [{
      refund_id: 'refund-1', order_id: 'order-1', payment_order_id: 'pay-1', user_id: 1001,
      merchant_id: 1000, merchant_name: '自营店', product_id: 100, refund_amount_fen: 9900,
      status: 1, status_text: 'pending', reason: '尺码问题', audit_remark: '', operator_id: 0,
      request_time: '2026-07-13 12:00:00', audit_time: '', finish_time: '',
    }], total: 1 } });
    render(<RefundsPage />);
    expect(await screen.findByText('refund-1')).toBeInTheDocument();
    expect(screen.getByText('尺码问题')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /审核|通过|拒绝/ })).not.toBeInTheDocument();
    expect(mocks.authed).toHaveBeenCalledWith('/api/merchant/refunds?page=1&page_size=100&status=-1');
  });
});
