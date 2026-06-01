import { getToken, clearAuth } from './auth';
import type { ApiResponse } from './types';

const BASE = '';

async function refreshToken(): Promise<string | null> {
  const token = getToken();
  if (!token) return null;
  try {
    const res = await fetch(`${BASE}/api/auth/refresh`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
      body: JSON.stringify({ refresh_token: localStorage.getItem('fm_refresh') || '' }),
    });
    if (!res.ok) return null;
    const data = await res.json();
    if (data.access_token) {
      localStorage.setItem('fm_token', data.access_token);
      if (data.refresh_token) localStorage.setItem('fm_refresh', data.refresh_token);
      return data.access_token;
    }
    return null;
  } catch {
    return null;
  }
}

function mergeHeaders(base: Record<string, string>, extra: Record<string, string>): Record<string, string> {
  return { ...base, ...extra };
}

export async function api<T>(
  path: string,
  options: RequestInit & { jsonBody?: unknown } = {},
): Promise<ApiResponse<T>> {
  const { jsonBody, ...init } = options;
  const existingHeaders: Record<string, string> = init.headers ? Object.fromEntries(
    new Headers(init.headers as HeadersInit).entries()
  ) : {};
  const body = jsonBody !== undefined ? JSON.stringify(jsonBody) : init.body;
  if (jsonBody !== undefined) existingHeaders['Content-Type'] = 'application/json';

  const res = await fetch(`${BASE}${path}`, { ...init, headers: existingHeaders, body });
  const status = res.status;
  let data: T;
  try {
    data = await res.json();
  } catch {
    data = {} as T;
  }
  return { ok: res.ok, status, data };
}

export async function authed<T>(
  path: string,
  options: RequestInit & { jsonBody?: unknown } = {},
): Promise<ApiResponse<T>> {
  const token = getToken();
  if (!token) return { ok: false, status: 401, data: {} as T };

  const existingHeaders: Record<string, string> = options.headers ? Object.fromEntries(
    new Headers(options.headers as HeadersInit).entries()
  ) : {};
  const headers = mergeHeaders(existingHeaders, { Authorization: `Bearer ${token}` });
  const result = await api<T>(path, { ...options, headers });

  if (result.status === 401) {
    const newToken = await refreshToken();
    if (newToken) {
      const retryHeaders = mergeHeaders(existingHeaders, { Authorization: `Bearer ${newToken}` });
      return api<T>(path, { ...options, headers: retryHeaders });
    }
    clearAuth();
  }
  return result;
}
