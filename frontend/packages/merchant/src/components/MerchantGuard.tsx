import { useEffect, useState, type ReactNode } from 'react';
import { Button, Result, Spin } from 'antd';
import { authed, clearAuth, isLoggedIn } from '@flash-mall/shared';
import type { MerchantMeResp } from '@flash-mall/shared';
import LoginPage from '../pages/LoginPage';

type GuardState = 'loading' | 'login' | 'unbound' | 'authorized';

export default function MerchantGuard({ children }: { children: ReactNode }) {
  const [state, setState] = useState<GuardState>('loading');

  const verify = () => {
    if (!isLoggedIn()) {
      setState('login');
      return;
    }
    setState('loading');
    authed<MerchantMeResp>('/api/merchant/me').then((response) => {
      const active = response.ok && (response.data.items || []).some((merchant) => merchant.status === 1);
      setState(active ? 'authorized' : 'unbound');
    });
  };

  useEffect(verify, []);

  if (state === 'loading') return <Spin size="large" style={{ display: 'block', margin: '120px auto' }} />;
  if (state === 'login') return <LoginPage onLogin={verify} />;
  if (state === 'unbound') {
    return (
      <Result
        status="403"
        title="当前账号未绑定可用商家"
        subTitle="请先提交商家入驻申请，或联系平台管理员启用商家绑定。"
        extra={<Button onClick={() => { clearAuth(); setState('login'); }}>切换账号</Button>}
      />
    );
  }
  return <>{children}</>;
}
