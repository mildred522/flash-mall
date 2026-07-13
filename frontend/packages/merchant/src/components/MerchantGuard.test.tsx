import { render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { setToken } from '@flash-mall/shared';
import MerchantGuard from './MerchantGuard';

const mocks = vi.hoisted(() => ({ authed: vi.fn() }));

vi.mock('@flash-mall/shared', async (importOriginal) => ({
  ...await importOriginal<typeof import('@flash-mall/shared')>(),
  authed: mocks.authed,
}));

function token(role = 'user') {
  const payload = btoa(JSON.stringify({ user_id: 1001, role, exp: Math.floor(Date.now() / 1000) + 3600 }));
  return `header.${payload}.signature`;
}

describe('MerchantGuard', () => {
  beforeEach(() => {
    localStorage.clear();
    mocks.authed.mockReset();
  });

  it('shows merchant login when no user is logged in', async () => {
    render(<MerchantGuard><div>经营工作台</div></MerchantGuard>);
    expect(await screen.findByText('Flash Mall 商家后台')).toBeInTheDocument();
    expect(screen.queryByText('经营工作台')).not.toBeInTheDocument();
  });

  it('shows a binding message when the user has no active merchant', async () => {
    setToken(token());
    mocks.authed.mockResolvedValue({ ok: false, status: 403, data: {} });
    render(<MerchantGuard><div>经营工作台</div></MerchantGuard>);
    expect(await screen.findByText('当前账号未绑定可用商家')).toBeInTheDocument();
  });

  it('renders merchant content after membership verification', async () => {
    setToken(token());
    mocks.authed.mockResolvedValue({
      ok: true,
      status: 200,
      data: { items: [{ merchant_id: 1000, name: '自营店', role: 'owner', status: 1 }] },
    });
    render(<MerchantGuard><div>经营工作台</div></MerchantGuard>);
    await waitFor(() => expect(screen.getByText('经营工作台')).toBeInTheDocument());
  });
});
