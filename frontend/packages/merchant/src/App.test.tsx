import { act, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import App from './App';

vi.mock('./components/MerchantGuard', () => ({ default: ({ children }: { children: React.ReactNode }) => children }));
vi.mock('./pages/DashboardPage', () => ({ default: () => <div>商家数据概览</div> }));
vi.mock('./pages/ProductsPage', () => ({ default: () => <div>商家商品页</div> }));
vi.mock('./pages/InventoryPage', () => ({ default: () => <div>商家库存页</div> }));
vi.mock('./pages/OrdersPage', () => ({ default: () => <div>商家订单页</div> }));
vi.mock('./pages/RefundsPage', () => ({ default: () => <div>商家退款页</div> }));
vi.mock('./pages/StoreSettingsPage', () => ({ default: () => <div>商家店铺设置页</div> }));

describe('Merchant App', () => {
  it('uses a merchant-only navigation menu', async () => {
    render(<App />);
    expect(await screen.findByText('商家数据概览')).toBeInTheDocument();
    expect(screen.getByText('商品管理')).toBeInTheDocument();
    expect(screen.getByText('库存流水')).toBeInTheDocument();
    expect(screen.getByText('订单发货')).toBeInTheDocument();
    expect(screen.getByText('退款查看')).toBeInTheDocument();
    expect(screen.getByText('店铺设置')).toBeInTheDocument();
    expect(screen.queryByText('用户管理')).not.toBeInTheDocument();

    act(() => window.dispatchEvent(new CustomEvent('flash-merchant:navigate', { detail: { path: '/merchant/products' } })));
    expect(await screen.findByText('商家商品页')).toBeInTheDocument();

    act(() => window.dispatchEvent(new CustomEvent('flash-merchant:navigate', { detail: { path: '/merchant/store' } })));
    expect(await screen.findByText('商家店铺设置页')).toBeInTheDocument();
    expect(window.location.pathname).toBe('/merchant/store');
  });
});
