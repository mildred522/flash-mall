const TOKEN_KEY = 'fm_token';
const REFRESH_KEY = 'fm_refresh';

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token);
}

export function setRefreshToken(token: string): void {
  localStorage.setItem(REFRESH_KEY, token);
}

export function clearAuth(): void {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(REFRESH_KEY);
}

export interface TokenPayload {
  user_id: number;
  role: string;
  sid: string;
  exp: number;
}

export function decodeToken(token: string): TokenPayload | null {
  try {
    const payload = token.split('.')[1];
    if (!payload) return null;
    const json = atob(payload.replace(/-/g, '+').replace(/_/g, '/'));
    return JSON.parse(json);
  } catch {
    return null;
  }
}

export function getPayload(): TokenPayload | null {
  const token = getToken();
  return token ? decodeToken(token) : null;
}

export function isAdmin(): boolean {
  return getPayload()?.role === 'admin';
}

export function isLoggedIn(): boolean {
  const payload = getPayload();
  if (!payload) return false;
  return payload.exp * 1000 > Date.now();
}
