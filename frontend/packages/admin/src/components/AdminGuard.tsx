import { useState, useEffect, type ReactNode } from 'react';
import { isLoggedIn, isAdmin, authed } from '@flash-mall/shared';
import type { MeResp } from '@flash-mall/shared';
import LoginPage from '../pages/LoginPage';

interface Props {
  children: ReactNode;
}

export default function AdminGuard({ children }: Props) {
  const [ready, setReady] = useState(false);
  const [authorized, setAuthorized] = useState(false);

  useEffect(() => {
    if (!isLoggedIn() || !isAdmin()) {
      setReady(true);
      setAuthorized(false);
      return;
    }
    authed<MeResp>('/api/auth/me').then((res) => {
      setAuthorized(res.ok && res.data.role === 'admin');
      setReady(true);
    });
  }, []);

  if (!ready) return <div style={{ padding: 40, textAlign: 'center' }}>加载中...</div>;
  if (!authorized) return <LoginPage onLogin={() => window.location.reload()} />;
  return <>{children}</>;
}
