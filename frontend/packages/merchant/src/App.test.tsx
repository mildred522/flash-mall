import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import App from './App';

vi.mock('./components/MerchantGuard', () => ({ default: ({ children }: { children: React.ReactNode }) => children }));
vi.mock('./pages/DashboardPage', () => ({ default: () => <div>商家数据概览</div> }));

describe('Merchant App', () => {
  it('uses a merchant-only navigation menu', async () => {
    render(<App />);
    expect(await screen.findByText('商家数据概览')).toBeInTheDocument();
    expect(screen.getByText('商品管理')).toBeInTheDocument();
    expect(screen.getByText('库存流水')).toBeInTheDocument();
    expect(screen.getByText('订单发货')).toBeInTheDocument();
    expect(screen.getByText('退款查看')).toBeInTheDocument();
    expect(screen.queryByText('用户管理')).not.toBeInTheDocument();
  });
});
