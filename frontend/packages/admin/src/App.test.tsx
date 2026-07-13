import { act, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import App from './App';

vi.mock('./components/AdminGuard', () => ({ default: ({ children }: { children: React.ReactNode }) => children }));
vi.mock('./pages/DashboardPage', () => ({ default: () => <div>管理员数据概览</div> }));
vi.mock('./pages/OrdersPage', () => ({ default: () => <div>订单页</div> }));
vi.mock('./pages/ProductsPage', () => ({ default: () => <div>商品页</div> }));
vi.mock('./pages/PromotionsPage', () => ({ default: () => <div>促销页</div> }));
vi.mock('./pages/SecurityEventsPage', () => ({ default: () => <div>安全页</div> }));
vi.mock('./pages/SuppliersPage', () => ({ default: () => <div>供应商页</div> }));
vi.mock('./pages/UsersPage', () => ({ default: () => <div>用户页</div> }));
vi.mock('./pages/MerchantApplicationsPage', () => ({ default: () => <div>商家入驻审核页</div> }));
vi.mock('./pages/ShowcasePage', () => ({ default: () => <div>首页橱窗工作台</div> }));

describe('Admin App', () => {
  it('provides the merchant onboarding review route', async () => {
    render(<App />);
    expect(await screen.findByText('管理员数据概览')).toBeInTheDocument();
    expect(screen.getByText('商家入驻')).toBeInTheDocument();
    expect(screen.getByText('首页橱窗')).toBeInTheDocument();

    act(() => window.dispatchEvent(new CustomEvent('flash-admin:navigate', {
      detail: { path: '/admin/merchant-applications' },
    })));
    expect(await screen.findByText('商家入驻审核页')).toBeInTheDocument();

    act(() => window.dispatchEvent(new CustomEvent('flash-admin:navigate', {
      detail: { path: '/admin/showcase' },
    })));
    expect(await screen.findByText('首页橱窗工作台')).toBeInTheDocument();
    expect(window.location.pathname).toBe('/admin/showcase');
  });
});
