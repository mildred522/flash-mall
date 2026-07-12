import { api, setRefreshToken, setToken } from '@flash-mall/shared';
import type { ApiResponse, LoginResp } from '@flash-mall/shared';

export const DEMO_ADMIN_CREDENTIALS = Object.freeze({
  phone: '13800000002',
  password: 'flashmall123',
});

type Request = (path: string, options: RequestInit & { jsonBody?: unknown }) => Promise<ApiResponse<LoginResp>>;

export async function loginAdmin(credentials: { phone: string; password: string }, request: Request = api): Promise<boolean> {
  const response = await request('/api/auth/login', {
    method: 'POST',
    jsonBody: credentials,
  });
  if (!response.ok || !response.data.access_token) return false;

  setToken(response.data.access_token);
  if (response.data.refresh_token) setRefreshToken(response.data.refresh_token);
  return true;
}
