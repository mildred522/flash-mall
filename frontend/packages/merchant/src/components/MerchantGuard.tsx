import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { Button, Result, Spin } from 'antd';
import { authed, clearAuth, isLoggedIn } from '@flash-mall/shared';
import type { MerchantApplicationItem, MerchantApplicationResp, MerchantMeResp } from '@flash-mall/shared';
import LoginPage from '../pages/LoginPage';
import MerchantOnboarding from './MerchantOnboarding';

type GuardState =
  | 'loading'
  | 'login'
  | 'onboarding'
  | 'approved_initializing'
  | 'disabled'
  | 'authorized'
  | 'error';

export default function MerchantGuard({ children }: { children: ReactNode }) {
  const [state, setState] = useState<GuardState>('loading');
  const [application, setApplication] = useState<MerchantApplicationItem | null>(null);

  const verify = useCallback(async () => {
    if (!isLoggedIn()) {
      setState('login');
      return;
    }
    setState('loading');

    const merchantResponse = await authed<MerchantMeResp>('/api/merchant/me');
    if (!merchantResponse.ok) {
      setState(merchantResponse.status === 401 ? 'login' : 'error');
      return;
    }
    const merchants = merchantResponse.data.items || [];
    if (merchants.some((merchant) => merchant.status === 1)) {
      setState('authorized');
      return;
    }
    if (merchants.length > 0) {
      setApplication(null);
      setState('disabled');
      return;
    }

    const applicationResponse = await authed<MerchantApplicationResp>('/api/merchant/application');
    if (!applicationResponse.ok) {
      setState(applicationResponse.status === 401 ? 'login' : 'error');
      return;
    }
    const latest = applicationResponse.data.application;
    setApplication(latest);
    setState(latest?.status === 1 ? 'approved_initializing' : 'onboarding');
  }, []);

  useEffect(() => {
    void verify();
  }, [verify]);

  const switchAccount = () => {
    clearAuth();
    setApplication(null);
    setState('login');
  };

  if (state === 'loading') {
    return <Spin size="large" style={{ display: 'block', margin: '120px auto' }} />;
  }
  if (state === 'login') {
    return <LoginPage onLogin={verify} />;
  }
  if (state === 'error') {
    return (
      <Result
        status="error"
        title="入驻状态加载失败"
        subTitle="暂时无法读取商家身份，请检查服务状态后重试。"
        extra={[
          <Button key="retry" type="primary" onClick={verify}>重试</Button>,
          <Button key="switch" onClick={switchAccount}>切换账号</Button>,
        ]}
      />
    );
  }
  if (state === 'disabled' || state === 'approved_initializing' || state === 'onboarding') {
    return (
      <MerchantOnboarding
        application={application}
        disabled={state === 'disabled'}
        initializing={state === 'approved_initializing'}
        onRefresh={verify}
        onSwitchAccount={switchAccount}
      />
    );
  }
  return <>{children}</>;
}
