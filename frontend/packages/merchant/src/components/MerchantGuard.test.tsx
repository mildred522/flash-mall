import { render, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { setToken } from '@flash-mall/shared';
import MerchantGuard from './MerchantGuard';

const mocks = vi.hoisted(() => ({ authed: vi.fn() }));

vi.mock('@flash-mall/shared', async (importOriginal) => ({
  ...await importOriginal<typeof import('@flash-mall/shared')>(),
  authed: mocks.authed,
}));

vi.mock('./MerchantOnboarding', () => ({
  default: ({ application, disabled, initializing }: {
    application: { status?: number } | null;
    disabled: boolean;
    initializing: boolean;
  }) => (
    <div>
      {disabled ? '商家已停用' : null}
      {initializing ? '正在初始化商家身份' : null}
      {!disabled && !initializing && application?.status === 0 ? '申请审核中' : null}
      {!disabled && !initializing && !application ? '申请成为商家' : null}
    </div>
  ),
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

  it('shows the application form when the user has no binding or application', async () => {
    setToken(token());
    mocks.authed
      .mockResolvedValueOnce({ ok: true, status: 200, data: { items: [] } })
      .mockResolvedValueOnce({ ok: true, status: 200, data: { application: null } });

    render(<MerchantGuard><div>经营工作台</div></MerchantGuard>);
    expect(await screen.findByText('申请成为商家')).toBeInTheDocument();
  });

  it('shows the pending application returned for the current user', async () => {
    setToken(token());
    mocks.authed
      .mockResolvedValueOnce({ ok: true, status: 200, data: { items: [] } })
      .mockResolvedValueOnce({ ok: true, status: 200, data: { application: { status: 0 } } });

    render(<MerchantGuard><div>经营工作台</div></MerchantGuard>);
    expect(await screen.findByText('申请审核中')).toBeInTheDocument();
  });

  it('shows a disabled state without querying applications', async () => {
    setToken(token());
    mocks.authed.mockResolvedValueOnce({
      ok: true,
      status: 200,
      data: { items: [{ merchant_id: 1000, name: '停用店铺', role: 'owner', status: 2 }] },
    });

    render(<MerchantGuard><div>经营工作台</div></MerchantGuard>);
    expect(await screen.findByText('商家已停用')).toBeInTheDocument();
    expect(mocks.authed).toHaveBeenCalledTimes(1);
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

  it('offers a retry when merchant state loading fails', async () => {
    setToken(token());
    mocks.authed.mockResolvedValue({ ok: false, status: 502, data: {} });
    render(<MerchantGuard><div>经营工作台</div></MerchantGuard>);
    expect(await screen.findByText('入驻状态加载失败')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /重\s*试/ })).toBeInTheDocument();
  });
});
