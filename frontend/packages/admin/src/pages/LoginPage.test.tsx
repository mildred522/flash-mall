import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import LoginPage from './LoginPage';

describe('LoginPage', () => {
  afterEach(() => vi.restoreAllMocks());

  it('logs in the demo administrator with one click', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      code: 'OK',
      data: { access_token: 'admin-access', refresh_token: 'admin-refresh' },
    }), { status: 200, headers: { 'Content-Type': 'application/json' } }));
    const onLogin = vi.fn();

    render(<LoginPage onLogin={onLogin} />);
    fireEvent.click(screen.getByRole('button', { name: '演示管理员一键登录' }));

    await waitFor(() => expect(onLogin).toHaveBeenCalledOnce());
    const init = fetchMock.mock.calls[0][1] as RequestInit;
    expect(JSON.parse(String(init.body))).toEqual({ phone: '13800000002', password: 'flashmall123' });
  });
});
