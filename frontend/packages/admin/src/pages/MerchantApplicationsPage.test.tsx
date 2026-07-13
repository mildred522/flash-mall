import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import MerchantApplicationsPage from './MerchantApplicationsPage';

const mocks = vi.hoisted(() => ({ authed: vi.fn() }));

vi.mock('@flash-mall/shared', async (importOriginal) => ({
  ...await importOriginal<typeof import('@flash-mall/shared')>(),
  authed: mocks.authed,
}));

const pendingApplication = {
  apply_id: 11,
  user_id: 1001,
  merchant_name: '星河数码',
  contact_phone: '13800001001',
  status: 0 as const,
  status_text: 'pending' as const,
  merchant_id: 0,
  audit_remark: '',
  operator_id: 0,
  create_time: '2026-07-13 10:00:00',
  audit_time: '',
};

describe('MerchantApplicationsPage', () => {
  beforeEach(() => {
    mocks.authed.mockReset();
    mocks.authed.mockImplementation(async (path: string, options?: { jsonBody?: unknown }) => {
      if (path.startsWith('/api/admin/merchants/applications?')) {
        return {
          ok: true,
          status: 200,
          data: { items: [pendingApplication], total: 1, page: 1, page_size: 20 },
        };
      }
      if (path === '/api/admin/merchants/applications/audit') {
        return { ok: true, status: 200, data: { apply_id: 11, merchant_id: 100, status: 1, body: options?.jsonBody } };
      }
      return { ok: false, status: 404, data: {} };
    });
  });

  it('lists applications and approves a pending application', async () => {
    const user = userEvent.setup();
    render(<MerchantApplicationsPage />);

    expect(await screen.findByText('星河数码')).toBeInTheDocument();
    expect(mocks.authed).toHaveBeenCalledWith(expect.stringMatching(
      /^\/api\/admin\/merchants\/applications\?.*page=1.*page_size=20/,
    ));

    const row = screen.getByText('星河数码').closest('tr');
    expect(row).not.toBeNull();
    await user.click(within(row!).getByRole('button', { name: '通过' }));
    expect(await screen.findByText('通过入驻申请')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: /确认\s*通过/ }));

    await waitFor(() => expect(mocks.authed).toHaveBeenCalledWith(
      '/api/admin/merchants/applications/audit',
      expect.objectContaining({
        method: 'POST',
        jsonBody: { apply_id: 11, approve: true, remark: '' },
      }),
    ));
  });

  it('requires a reason before rejecting an application', async () => {
    const user = userEvent.setup();
    render(<MerchantApplicationsPage />);
    const row = (await screen.findByText('星河数码')).closest('tr');
    await user.click(within(row!).getByRole('button', { name: '驳回' }));

    await user.click(screen.getByRole('button', { name: /确认\s*驳回/ }));
    expect(await screen.findByText('请输入驳回原因')).toBeInTheDocument();
    expect(mocks.authed).not.toHaveBeenCalledWith(
      '/api/admin/merchants/applications/audit',
      expect.anything(),
    );

    await user.type(screen.getByLabelText('驳回原因'), '资质信息不完整');
    await user.click(screen.getByRole('button', { name: /确认\s*驳回/ }));
    await waitFor(() => expect(mocks.authed).toHaveBeenCalledWith(
      '/api/admin/merchants/applications/audit',
      expect.objectContaining({
        method: 'POST',
        jsonBody: { apply_id: 11, approve: false, remark: '资质信息不完整' },
      }),
    ));
  });
});
