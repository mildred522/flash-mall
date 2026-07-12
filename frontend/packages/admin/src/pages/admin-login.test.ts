import { beforeEach, describe, expect, it, vi } from 'vitest';
import { DEMO_ADMIN_CREDENTIALS, loginAdmin } from './admin-login';

describe('loginAdmin', () => {
  beforeEach(() => localStorage.clear());

  it('uses the auth API and persists both tokens', async () => {
    const request = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      data: { access_token: 'access', refresh_token: 'refresh' },
    });

    await expect(loginAdmin(DEMO_ADMIN_CREDENTIALS, request)).resolves.toBe(true);
    expect(request).toHaveBeenCalledWith('/api/auth/login', {
      method: 'POST',
      jsonBody: DEMO_ADMIN_CREDENTIALS,
    });
    expect(localStorage.getItem('fm_token')).toBe('access');
    expect(localStorage.getItem('fm_refresh')).toBe('refresh');
  });
});
