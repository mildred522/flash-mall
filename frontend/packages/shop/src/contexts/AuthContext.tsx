import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from 'react';
import { api, authed, setToken, setRefreshToken, clearAuth, isLoggedIn, getPayload } from '@flash-mall/shared';
import type { LoginResp, MeResp, ApiResponse } from '@flash-mall/shared';

interface AuthState {
  token: string | null;
  user: MeResp | null;
  loading: boolean;
  login: (phone: string, password: string) => Promise<boolean>;
  register: (phone: string, password: string, code: string) => Promise<boolean>;
  logout: () => Promise<void>;
  refresh: () => Promise<void>;
}

const AuthCtx = createContext<AuthState>({
  token: null,
  user: null,
  loading: true,
  login: async () => false,
  register: async () => false,
  logout: async () => {},
  refresh: async () => {},
});

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setTokenState] = useState<string | null>(null);
  const [user, setUser] = useState<MeResp | null>(null);
  const [loading, setLoading] = useState(true);

  const loadUser = useCallback(async () => {
    if (!isLoggedIn()) {
      setTokenState(null);
      setUser(null);
      setLoading(false);
      return;
    }
    const res = await authed<MeResp>('/api/auth/me');
    if (res.ok) {
      setTokenState(localStorage.getItem('fm_token'));
      setUser(res.data);
    } else {
      clearAuth();
      setTokenState(null);
      setUser(null);
    }
    setLoading(false);
  }, []);

  useEffect(() => { loadUser(); }, [loadUser]);

  const login = async (phone: string, password: string): Promise<boolean> => {
    const res = await api<LoginResp>('/api/auth/login', {
      method: 'POST',
      jsonBody: { phone, password },
    });
    if (res.ok && res.data.access_token) {
      setToken(res.data.access_token);
      if (res.data.refresh_token) setRefreshToken(res.data.refresh_token);
      setTokenState(res.data.access_token);
      await loadUser();
      return true;
    }
    return false;
  };

  const register = async (phone: string, password: string, code: string): Promise<boolean> => {
    const res = await api<LoginResp>('/api/auth/register', {
      method: 'POST',
      jsonBody: { phone, password, code, display_name: '' },
    });
    if (res.ok && res.data.access_token) {
      setToken(res.data.access_token);
      if (res.data.refresh_token) setRefreshToken(res.data.refresh_token);
      setTokenState(res.data.access_token);
      await loadUser();
      return true;
    }
    return false;
  };

  const logout = async () => {
    await authed('/api/auth/logout', { method: 'POST' });
    clearAuth();
    setTokenState(null);
    setUser(null);
  };

  const refresh = async () => {
    const refreshToken = localStorage.getItem('fm_refresh');
    if (!refreshToken) return;
    const res = await api<LoginResp>('/api/auth/refresh', {
      method: 'POST',
      jsonBody: { refresh_token: refreshToken },
    });
    if (res.ok && res.data.access_token) {
      setToken(res.data.access_token);
      if (res.data.refresh_token) setRefreshToken(res.data.refresh_token);
      setTokenState(res.data.access_token);
    }
  };

  return (
    <AuthCtx.Provider value={{ token, user, loading, login, register, logout, refresh }}>
      {children}
    </AuthCtx.Provider>
  );
}

export function useAuth() {
  return useContext(AuthCtx);
}
