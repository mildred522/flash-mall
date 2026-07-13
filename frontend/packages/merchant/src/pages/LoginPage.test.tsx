import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import LoginPage from './LoginPage';

describe('Merchant LoginPage registration', () => {
  beforeEach(() => localStorage.clear());
  afterEach(() => vi.restoreAllMocks());

  it('shows the local debug code and signs the new user in', async () => {
    const fetchMock = vi.spyOn(globalThis, 'fetch')
      .mockResolvedValueOnce(new Response(JSON.stringify({
        code: 'OK',
        data: { sent: true, expires_at: 1783940400, debug_code: '654321' },
      }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
      .mockResolvedValueOnce(new Response(JSON.stringify({
        code: 'OK',
        data: {
          access_token: 'merchant-access',
          refresh_token: 'merchant-refresh',
          token_type: 'Bearer',
          expires_at: 1783940400,
          user_id: 1003,
          display_name: '',
          phone: '13800000003',
        },
      }), { status: 200, headers: { 'Content-Type': 'application/json' } }));
    const onLogin = vi.fn();
    const user = userEvent.setup();

    render(<LoginPage onLogin={onLogin} />);
    await user.click(screen.getByRole('tab', { name: '注册' }));
    await user.type(screen.getByLabelText('手机号'), '13800000003');
    await user.click(screen.getByRole('button', { name: '发送验证码' }));

    expect(await screen.findByText('演示验证码：654321')).toBeInTheDocument();
    expect(JSON.parse(String(fetchMock.mock.calls[0][1]?.body))).toEqual({
      phone: '13800000003',
      scene: 'register',
    });

    await user.type(screen.getByLabelText('验证码'), '654321');
    await user.type(screen.getByLabelText('密码'), 'merchant-pass');
    await user.click(screen.getByRole('button', { name: '注册账号' }));

    await waitFor(() => expect(onLogin).toHaveBeenCalledOnce());
    expect(JSON.parse(String(fetchMock.mock.calls[1][1]?.body))).toEqual({
      phone: '13800000003',
      password: 'merchant-pass',
      code: '654321',
      display_name: '',
    });
    expect(localStorage.getItem('fm_token')).toBe('merchant-access');
    expect(localStorage.getItem('fm_refresh')).toBe('merchant-refresh');
  });

  it('does not render a debug-code alert when the server omits it', async () => {
    vi.spyOn(globalThis, 'fetch').mockResolvedValue(new Response(JSON.stringify({
      code: 'OK',
      data: { sent: true, expires_at: 1783940400 },
    }), { status: 200, headers: { 'Content-Type': 'application/json' } }));
    const user = userEvent.setup();

    render(<LoginPage onLogin={() => undefined} />);
    await user.click(screen.getByRole('tab', { name: '注册' }));
    await user.type(screen.getByLabelText('手机号'), '13800000004');
    await user.click(screen.getByRole('button', { name: '发送验证码' }));

    await waitFor(() => expect(screen.getByRole('button', { name: /重新发送/ })).toBeDisabled());
    expect(screen.queryByText(/演示验证码/)).not.toBeInTheDocument();
  });
});
