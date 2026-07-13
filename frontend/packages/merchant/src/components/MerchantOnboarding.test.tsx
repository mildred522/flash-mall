import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { MerchantApplicationItem } from '@flash-mall/shared';
import MerchantOnboarding from './MerchantOnboarding';

const mocks = vi.hoisted(() => ({ authed: vi.fn() }));

vi.mock('@flash-mall/shared', async (importOriginal) => ({
  ...await importOriginal<typeof import('@flash-mall/shared')>(),
  authed: mocks.authed,
}));

const rejectedApplication: MerchantApplicationItem = {
  apply_id: 12,
  merchant_name: '旧商店名称',
  contact_phone: '13800000003',
  status: 2,
  status_text: 'rejected',
  merchant_id: 0,
  audit_remark: '名称不符合规范',
  create_time: '2026-07-13 15:00:00',
  audit_time: '2026-07-13 16:00:00',
};

describe('MerchantOnboarding', () => {
  beforeEach(() => {
    mocks.authed.mockReset();
    mocks.authed.mockImplementation(async (path: string) => {
      if (path === '/api/auth/me') {
        return { ok: true, status: 200, data: { user_id: 1003, display_name: '', phone: '13800000003', role: 'user' } };
      }
      return { ok: true, status: 200, data: { apply_id: 13, status: 'pending' } };
    });
  });

  it('shows an application form and fills the account phone', async () => {
    render(
      <MerchantOnboarding
        application={null}
        disabled={false}
        initializing={false}
        onRefresh={() => undefined}
        onSwitchAccount={() => undefined}
      />,
    );

    expect(await screen.findByText('申请成为商家')).toBeInTheDocument();
    expect(await screen.findByDisplayValue('13800000003')).toBeInTheDocument();
  });

  it('shows a pending application without another submit action', () => {
    render(
      <MerchantOnboarding
        application={{ ...rejectedApplication, status: 0, status_text: 'pending', audit_remark: '', audit_time: '' }}
        disabled={false}
        initializing={false}
        onRefresh={() => undefined}
        onSwitchAccount={() => undefined}
      />,
    );

    expect(screen.getByText('申请审核中')).toBeInTheDocument();
    expect(screen.getByText('申请编号：12')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /提交申请/ })).not.toBeInTheDocument();
  });

  it('refills rejected data and submits a new application', async () => {
    const onRefresh = vi.fn();
    const user = userEvent.setup();
    render(
      <MerchantOnboarding
        application={rejectedApplication}
        disabled={false}
        initializing={false}
        onRefresh={onRefresh}
        onSwitchAccount={() => undefined}
      />,
    );

    expect(screen.getByText('名称不符合规范')).toBeInTheDocument();
    expect(screen.getByDisplayValue('旧商店名称')).toBeInTheDocument();
    await user.clear(screen.getByLabelText('商家名称'));
    await user.type(screen.getByLabelText('商家名称'), '新商店名称');
    await user.click(screen.getByRole('button', { name: '重新提交申请' }));

    await waitFor(() => expect(mocks.authed).toHaveBeenCalledWith('/api/merchant/apply', expect.objectContaining({
      method: 'POST',
      jsonBody: { merchant_name: '新商店名称', contact_phone: '13800000003' },
    })));
    expect(onRefresh).toHaveBeenCalledOnce();
  });

  it('shows a disabled state without an application form', () => {
    render(
      <MerchantOnboarding
        application={null}
        disabled
        initializing={false}
        onRefresh={() => undefined}
        onSwitchAccount={() => undefined}
      />,
    );

    expect(screen.getByText('商家已停用')).toBeInTheDocument();
    expect(screen.queryByText('申请成为商家')).not.toBeInTheDocument();
  });
});
